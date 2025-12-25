package mail

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Common errors
var (
	ErrMessageNotFound = errors.New("message not found")
	ErrEmptyInbox      = errors.New("inbox is empty")
)

// Mailbox manages messages for an identity via beads.
type Mailbox struct {
	identity string // beads identity (e.g., "gastown/polecats/Toast")
	workDir  string // directory to run bd commands in
	beadsDir string // explicit .beads directory path (set via BEADS_DIR)
	wispDir  string // .beads-wisp directory for ephemeral messages
	path     string // for legacy JSONL mode (crew workers)
	legacy   bool   // true = use JSONL files, false = use beads
}

// NewMailbox creates a mailbox for the given JSONL path (legacy mode).
// Used by crew workers that have local JSONL inboxes.
func NewMailbox(path string) *Mailbox {
	return &Mailbox{
		path:   filepath.Join(path, "inbox.jsonl"),
		legacy: true,
	}
}

// NewMailboxBeads creates a mailbox backed by beads.
func NewMailboxBeads(identity, workDir string) *Mailbox {
	return &Mailbox{
		identity: identity,
		workDir:  workDir,
		legacy:   false,
	}
}

// NewMailboxFromAddress creates a beads-backed mailbox from a GGT address.
func NewMailboxFromAddress(address, workDir string) *Mailbox {
	beadsDir := filepath.Join(workDir, ".beads")
	// Wisp directory is .beads-wisp/.beads (bd init creates .beads/ subdirectory)
	wispDir := filepath.Join(workDir, ".beads-wisp", ".beads")
	return &Mailbox{
		identity: addressToIdentity(address),
		workDir:  workDir,
		beadsDir: beadsDir,
		wispDir:  wispDir,
		legacy:   false,
	}
}

// NewMailboxWithBeadsDir creates a mailbox with an explicit beads directory.
func NewMailboxWithBeadsDir(address, workDir, beadsDir string) *Mailbox {
	// Derive wispDir from beadsDir (.beads-wisp/.beads sibling structure)
	wispDir := filepath.Join(filepath.Dir(beadsDir), ".beads-wisp", ".beads")
	return &Mailbox{
		identity: addressToIdentity(address),
		workDir:  workDir,
		beadsDir: beadsDir,
		wispDir:  wispDir,
		legacy:   false,
	}
}

// Identity returns the beads identity for this mailbox.
func (m *Mailbox) Identity() string {
	return m.identity
}

// Path returns the JSONL path for legacy mailboxes.
func (m *Mailbox) Path() string {
	return m.path
}

// List returns all open messages in the mailbox.
func (m *Mailbox) List() ([]*Message, error) {
	if m.legacy {
		return m.listLegacy()
	}
	return m.listBeads()
}

func (m *Mailbox) listBeads() ([]*Message, error) {
	// Query persistent beads
	persistentMsgs, err := m.listFromDir(m.beadsDir, SourcePersistent)
	if err != nil {
		return nil, err
	}

	// Query wisp beads (ignore errors for missing dir)
	var wispMsgs []*Message
	if m.wispDir != "" {
		wispMsgs, _ = m.listFromDir(m.wispDir, SourceWisp)
	}

	// Merge and sort by timestamp (newest first)
	all := append(persistentMsgs, wispMsgs...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})

	return all, nil
}

// listFromDir queries messages from a specific beads directory.
func (m *Mailbox) listFromDir(beadsDir string, source MessageSource) ([]*Message, error) {
	// bd list --type=message --assignee=<identity> --json --status=open
	cmd := exec.Command("bd", "list",
		"--type", "message",
		"--assignee", m.identity,
		"--status", "open",
		"--json",
	)
	cmd.Dir = m.workDir
	cmd.Env = append(cmd.Environ(),
		"BEADS_DIR="+beadsDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, errors.New(errMsg)
		}
		return nil, err
	}

	// Parse JSON output
	var beadsMsgs []BeadsMessage
	if err := json.Unmarshal(stdout.Bytes(), &beadsMsgs); err != nil {
		// Empty inbox returns empty array or nothing
		if len(stdout.Bytes()) == 0 || stdout.String() == "null" {
			return nil, nil
		}
		return nil, err
	}

	// Convert to GGT messages and set source
	var messages []*Message
	for _, bm := range beadsMsgs {
		msg := bm.ToMessage()
		msg.Source = source
		msg.Ephemeral = (source == SourceWisp)
		messages = append(messages, msg)
	}

	return messages, nil
}

func (m *Mailbox) listLegacy() ([]*Message, error) {
	file, err := os.Open(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var messages []*Message
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue // Skip malformed lines
		}
		messages = append(messages, &msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Sort by timestamp (newest first)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	return messages, nil
}

// ListUnread returns unread (open) messages.
func (m *Mailbox) ListUnread() ([]*Message, error) {
	if m.legacy {
		all, err := m.List()
		if err != nil {
			return nil, err
		}
		var unread []*Message
		for _, msg := range all {
			if !msg.Read {
				unread = append(unread, msg)
			}
		}
		return unread, nil
	}
	// For beads, inbox only returns open (unread) messages
	return m.List()
}

// Get returns a message by ID.
func (m *Mailbox) Get(id string) (*Message, error) {
	if m.legacy {
		return m.getLegacy(id)
	}
	return m.getBeads(id)
}

func (m *Mailbox) getBeads(id string) (*Message, error) {
	// Try persistent first
	msg, err := m.getFromDir(id, m.beadsDir, SourcePersistent)
	if err == nil {
		return msg, nil
	}

	// Try wisp storage
	if m.wispDir != "" {
		msg, err = m.getFromDir(id, m.wispDir, SourceWisp)
		if err == nil {
			return msg, nil
		}
	}

	return nil, ErrMessageNotFound
}

// getFromDir retrieves a message from a specific beads directory.
func (m *Mailbox) getFromDir(id, beadsDir string, source MessageSource) (*Message, error) {
	cmd := exec.Command("bd", "show", id, "--json")
	cmd.Dir = m.workDir
	cmd.Env = append(cmd.Environ(), "BEADS_DIR="+beadsDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if strings.Contains(errMsg, "not found") {
			return nil, ErrMessageNotFound
		}
		if errMsg != "" {
			return nil, errors.New(errMsg)
		}
		return nil, err
	}

	// bd show --json returns an array
	var bms []BeadsMessage
	if err := json.Unmarshal(stdout.Bytes(), &bms); err != nil {
		return nil, err
	}
	if len(bms) == 0 {
		return nil, ErrMessageNotFound
	}

	msg := bms[0].ToMessage()
	msg.Source = source
	msg.Ephemeral = (source == SourceWisp)
	return msg, nil
}

func (m *Mailbox) getLegacy(id string) (*Message, error) {
	messages, err := m.List()
	if err != nil {
		return nil, err
	}
	for _, msg := range messages {
		if msg.ID == id {
			return msg, nil
		}
	}
	return nil, ErrMessageNotFound
}

// MarkRead marks a message as read.
func (m *Mailbox) MarkRead(id string) error {
	if m.legacy {
		return m.markReadLegacy(id)
	}
	return m.markReadBeads(id)
}

func (m *Mailbox) markReadBeads(id string) error {
	// Try persistent first
	err := m.closeInDir(id, m.beadsDir)
	if err == nil {
		return nil
	}

	// Try wisp storage
	if m.wispDir != "" {
		err = m.closeInDir(id, m.wispDir)
		if err == nil {
			return nil
		}
	}

	return ErrMessageNotFound
}

// closeInDir closes a message in a specific beads directory.
func (m *Mailbox) closeInDir(id, beadsDir string) error {
	cmd := exec.Command("bd", "close", id)
	cmd.Dir = m.workDir
	cmd.Env = append(cmd.Environ(), "BEADS_DIR="+beadsDir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if strings.Contains(errMsg, "not found") {
			return ErrMessageNotFound
		}
		if errMsg != "" {
			return errors.New(errMsg)
		}
		return err
	}

	return nil
}

func (m *Mailbox) markReadLegacy(id string) error {
	messages, err := m.List()
	if err != nil {
		return err
	}

	found := false
	for _, msg := range messages {
		if msg.ID == id {
			msg.Read = true
			found = true
		}
	}

	if !found {
		return ErrMessageNotFound
	}

	return m.rewriteLegacy(messages)
}

// Delete removes a message.
func (m *Mailbox) Delete(id string) error {
	if m.legacy {
		return m.deleteLegacy(id)
	}
	return m.MarkRead(id) // beads: just acknowledge/close
}

func (m *Mailbox) deleteLegacy(id string) error {
	messages, err := m.List()
	if err != nil {
		return err
	}

	var filtered []*Message
	found := false
	for _, msg := range messages {
		if msg.ID == id {
			found = true
		} else {
			filtered = append(filtered, msg)
		}
	}

	if !found {
		return ErrMessageNotFound
	}

	return m.rewriteLegacy(filtered)
}

// Count returns the total and unread message counts.
func (m *Mailbox) Count() (total, unread int, err error) {
	messages, err := m.List()
	if err != nil {
		return 0, 0, err
	}

	total = len(messages)
	if m.legacy {
		for _, msg := range messages {
			if !msg.Read {
				unread++
			}
		}
	} else {
		// For beads, inbox only returns unread
		unread = total
	}

	return total, unread, nil
}

// Append adds a message to the mailbox (legacy mode only).
// For beads mode, use Router.Send() instead.
func (m *Mailbox) Append(msg *Message) error {
	if !m.legacy {
		return errors.New("use Router.Send() to send messages via beads")
	}
	return m.appendLegacy(msg)
}

func (m *Mailbox) appendLegacy(msg *Message) error {
	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Open for append
	file, err := os.OpenFile(m.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = file.WriteString(string(data) + "\n")
	return err
}

// rewriteLegacy rewrites the mailbox with the given messages.
func (m *Mailbox) rewriteLegacy(messages []*Message) error {
	// Sort by timestamp (oldest first for JSONL)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	// Write to temp file
	tmpPath := m.path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return err
		}
		_, _ = file.WriteString(string(data) + "\n")
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, m.path)
}

// ListByThread returns all messages in a given thread.
func (m *Mailbox) ListByThread(threadID string) ([]*Message, error) {
	if m.legacy {
		return m.listByThreadLegacy(threadID)
	}
	return m.listByThreadBeads(threadID)
}

func (m *Mailbox) listByThreadBeads(threadID string) ([]*Message, error) {
	// bd message thread <thread-id> --json
	cmd := exec.Command("bd", "message", "thread", threadID, "--json")
	cmd.Dir = m.workDir
	cmd.Env = append(cmd.Environ(),
		"BD_IDENTITY="+m.identity,
		"BEADS_DIR="+m.beadsDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, errors.New(errMsg)
		}
		return nil, err
	}

	var beadsMsgs []BeadsMessage
	if err := json.Unmarshal(stdout.Bytes(), &beadsMsgs); err != nil {
		if len(stdout.Bytes()) == 0 || stdout.String() == "null" {
			return nil, nil
		}
		return nil, err
	}

	var messages []*Message
	for _, bm := range beadsMsgs {
		messages = append(messages, bm.ToMessage())
	}

	// Sort by timestamp (oldest first for thread view)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

func (m *Mailbox) listByThreadLegacy(threadID string) ([]*Message, error) {
	messages, err := m.List()
	if err != nil {
		return nil, err
	}

	var thread []*Message
	for _, msg := range messages {
		if msg.ThreadID == threadID {
			thread = append(thread, msg)
		}
	}

	// Sort by timestamp (oldest first for thread view)
	sort.Slice(thread, func(i, j int) bool {
		return thread[i].Timestamp.Before(thread[j].Timestamp)
	})

	return thread, nil
}
