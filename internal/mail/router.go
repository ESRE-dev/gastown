package mail

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/gastown/internal/tmux"
)

// Router handles message delivery via beads.
type Router struct {
	workDir string // directory to run bd commands in
	tmux    *tmux.Tmux
}

// NewRouter creates a new mail router.
// workDir should be a directory containing a .beads database.
func NewRouter(workDir string) *Router {
	return &Router{
		workDir: workDir,
		tmux:    tmux.NewTmux(),
	}
}

// Send delivers a message via beads create.
func (r *Router) Send(msg *Message) error {
	// Convert addresses to beads identities
	toIdentity := addressToIdentity(msg.To)
	fromIdentity := addressToIdentity(msg.From)

	// Build labels for sender and thread
	labels := []string{"from:" + fromIdentity}
	if msg.ThreadID != "" {
		labels = append(labels, "thread:"+msg.ThreadID)
	}
	if msg.ReplyTo != "" {
		labels = append(labels, "reply-to:"+msg.ReplyTo)
	}
	if msg.Type != "" && msg.Type != TypeNotification {
		labels = append(labels, "msg-type:"+string(msg.Type))
	}

	// Build command: bd create --type message --assignee <to> --title <subject> -d <body>
	args := []string{"create",
		"--type", "message",
		"--assignee", toIdentity,
		"--title", msg.Subject,
		"-d", msg.Body,
	}

	// Add priority flag
	beadsPriority := PriorityToBeads(msg.Priority)
	args = append(args, "--priority", fmt.Sprintf("%d", beadsPriority))

	// Add labels
	if len(labels) > 0 {
		args = append(args, "--labels", strings.Join(labels, ","))
	}

	// Add --silent to get just the issue ID
	args = append(args, "--silent")

	cmd := exec.Command("bd", args...)
	cmd.Dir = r.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return errors.New(errMsg)
		}
		return fmt.Errorf("sending message: %w", err)
	}

	// Get the created issue ID
	issueID := strings.TrimSpace(stdout.String())

	// Pin the message if requested
	if msg.Pinned && issueID != "" {
		pinCmd := exec.Command("bd", "pin", issueID)
		pinCmd.Dir = r.workDir
		_ = pinCmd.Run() // Best effort - don't fail if pin fails
	}

	// Notify recipient if they have an active session
	_ = r.notifyRecipient(msg)

	return nil
}

// GetMailbox returns a Mailbox for the given address.
func (r *Router) GetMailbox(address string) (*Mailbox, error) {
	return NewMailboxFromAddress(address, r.workDir), nil
}

// notifyRecipient sends a notification to a recipient's tmux session.
// Uses send-keys to echo a visible banner to ensure notification is seen.
// Supports mayor/, rig/polecat, and rig/refinery addresses.
func (r *Router) notifyRecipient(msg *Message) error {
	sessionID := addressToSessionID(msg.To)
	if sessionID == "" {
		return nil // Unable to determine session ID
	}

	// Check if session exists
	hasSession, err := r.tmux.HasSession(sessionID)
	if err != nil || !hasSession {
		return nil // No active session, skip notification
	}

	// Send visible notification banner to the terminal
	return r.tmux.SendNotificationBanner(sessionID, msg.From, msg.Subject)
}

// addressToSessionID converts a mail address to a tmux session ID.
// Returns empty string if address format is not recognized.
func addressToSessionID(address string) string {
	// Mayor address: "mayor/" or "mayor"
	if strings.HasPrefix(address, "mayor") {
		return "gt-mayor"
	}

	// Rig-based address: "rig/target"
	parts := strings.SplitN(address, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}

	rig := parts[0]
	target := parts[1]

	// Polecat: gt-rig-polecat
	// Refinery: gt-rig-refinery (if refinery has its own session)
	return fmt.Sprintf("gt-%s-%s", rig, target)
}
