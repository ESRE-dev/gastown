package witness

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/mail"
)

// HandlerResult tracks the result of handling a protocol message.
type HandlerResult struct {
	MessageID    string
	ProtocolType ProtocolType
	Handled      bool
	Action       string
	WispCreated  string // ID of created wisp (if any)
	MailSent     string // ID of sent mail (if any)
	Error        error
}

// HandlePolecatDone processes a POLECAT_DONE message from a polecat.
// Creates a cleanup wisp for the polecat to trigger the verification flow.
func HandlePolecatDone(workDir, rigName string, msg *mail.Message) *HandlerResult {
	result := &HandlerResult{
		MessageID:    msg.ID,
		ProtocolType: ProtoPolecatDone,
	}

	// Parse the message
	payload, err := ParsePolecatDone(msg.Subject, msg.Body)
	if err != nil {
		result.Error = fmt.Errorf("parsing POLECAT_DONE: %w", err)
		return result
	}

	// Create a cleanup wisp for this polecat
	wispID, err := createCleanupWisp(workDir, payload.PolecatName, payload.IssueID, payload.Branch)
	if err != nil {
		result.Error = fmt.Errorf("creating cleanup wisp: %w", err)
		return result
	}

	result.Handled = true
	result.WispCreated = wispID
	result.Action = fmt.Sprintf("created cleanup wisp %s for polecat %s", wispID, payload.PolecatName)

	return result
}

// HandleLifecycleShutdown processes a LIFECYCLE:Shutdown message.
// Similar to POLECAT_DONE but triggered by daemon rather than polecat.
func HandleLifecycleShutdown(workDir, rigName string, msg *mail.Message) *HandlerResult {
	result := &HandlerResult{
		MessageID:    msg.ID,
		ProtocolType: ProtoLifecycleShutdown,
	}

	// Extract polecat name from subject
	matches := PatternLifecycleShutdown.FindStringSubmatch(msg.Subject)
	if len(matches) < 2 {
		result.Error = fmt.Errorf("invalid LIFECYCLE:Shutdown subject: %s", msg.Subject)
		return result
	}
	polecatName := matches[1]

	// Create a cleanup wisp
	wispID, err := createCleanupWisp(workDir, polecatName, "", "")
	if err != nil {
		result.Error = fmt.Errorf("creating cleanup wisp: %w", err)
		return result
	}

	result.Handled = true
	result.WispCreated = wispID
	result.Action = fmt.Sprintf("created cleanup wisp %s for shutdown %s", wispID, polecatName)

	return result
}

// HandleHelp processes a HELP message from a polecat requesting intervention.
// Assesses the request and either helps directly or escalates to Mayor.
func HandleHelp(workDir, rigName string, msg *mail.Message, router *mail.Router) *HandlerResult {
	result := &HandlerResult{
		MessageID:    msg.ID,
		ProtocolType: ProtoHelp,
	}

	// Parse the message
	payload, err := ParseHelp(msg.Subject, msg.Body)
	if err != nil {
		result.Error = fmt.Errorf("parsing HELP: %w", err)
		return result
	}

	// Assess the help request
	assessment := AssessHelpRequest(payload)

	if assessment.CanHelp {
		// Log that we can help - actual help is done by the Claude agent
		result.Handled = true
		result.Action = fmt.Sprintf("can help with '%s': %s", payload.Topic, assessment.HelpAction)
		return result
	}

	// Need to escalate to Mayor
	if assessment.NeedsEscalation {
		mailID, err := escalateToMayor(router, rigName, payload, assessment.EscalationReason)
		if err != nil {
			result.Error = fmt.Errorf("escalating to mayor: %w", err)
			return result
		}

		result.Handled = true
		result.MailSent = mailID
		result.Action = fmt.Sprintf("escalated '%s' to mayor: %s", payload.Topic, assessment.EscalationReason)
	}

	return result
}

// HandleMerged processes a MERGED message from the Refinery.
// Finds the cleanup wisp for this polecat and triggers the nuke.
func HandleMerged(workDir, rigName string, msg *mail.Message) *HandlerResult {
	result := &HandlerResult{
		MessageID:    msg.ID,
		ProtocolType: ProtoMerged,
	}

	// Parse the message
	payload, err := ParseMerged(msg.Subject, msg.Body)
	if err != nil {
		result.Error = fmt.Errorf("parsing MERGED: %w", err)
		return result
	}

	// Find the cleanup wisp for this polecat
	wispID, err := findCleanupWisp(workDir, payload.PolecatName)
	if err != nil {
		result.Error = fmt.Errorf("finding cleanup wisp: %w", err)
		return result
	}

	if wispID == "" {
		// No wisp found - polecat may have been cleaned up already
		result.Handled = true
		result.Action = fmt.Sprintf("no cleanup wisp found for %s (may be already cleaned)", payload.PolecatName)
		return result
	}

	result.Handled = true
	result.WispCreated = wispID // Reference to existing wisp
	result.Action = fmt.Sprintf("found cleanup wisp %s for %s, ready to nuke", wispID, payload.PolecatName)

	return result
}

// HandleSwarmStart processes a SWARM_START message from the Mayor.
// Creates a swarm tracking wisp to monitor batch polecat work.
func HandleSwarmStart(workDir string, msg *mail.Message) *HandlerResult {
	result := &HandlerResult{
		MessageID:    msg.ID,
		ProtocolType: ProtoSwarmStart,
	}

	// Parse the message
	payload, err := ParseSwarmStart(msg.Body)
	if err != nil {
		result.Error = fmt.Errorf("parsing SWARM_START: %w", err)
		return result
	}

	// Create a swarm tracking wisp
	wispID, err := createSwarmWisp(workDir, payload)
	if err != nil {
		result.Error = fmt.Errorf("creating swarm wisp: %w", err)
		return result
	}

	result.Handled = true
	result.WispCreated = wispID
	result.Action = fmt.Sprintf("created swarm tracking wisp %s for %s", wispID, payload.SwarmID)

	return result
}

// createCleanupWisp creates a wisp to track polecat cleanup.
func createCleanupWisp(workDir, polecatName, issueID, branch string) (string, error) {
	title := fmt.Sprintf("cleanup:%s", polecatName)
	description := fmt.Sprintf("Verify and cleanup polecat %s", polecatName)
	if issueID != "" {
		description += fmt.Sprintf("\nIssue: %s", issueID)
	}
	if branch != "" {
		description += fmt.Sprintf("\nBranch: %s", branch)
	}

	labels := strings.Join(CleanupWispLabels(polecatName, "pending"), ",")

	cmd := exec.Command("bd", "create",
		"--wisp",
		"--title", title,
		"--description", description,
		"--labels", labels,
	)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("%s", errMsg)
		}
		return "", err
	}

	// Extract wisp ID from output (bd create outputs "Created: <id>")
	output := strings.TrimSpace(stdout.String())
	if strings.HasPrefix(output, "Created:") {
		return strings.TrimSpace(strings.TrimPrefix(output, "Created:")), nil
	}

	// Try to extract ID from output
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Look for bead ID pattern (e.g., "gt-abc123")
		if strings.Contains(line, "-") && len(line) < 20 {
			return line, nil
		}
	}

	return output, nil
}

// createSwarmWisp creates a wisp to track swarm (batch) work.
func createSwarmWisp(workDir string, payload *SwarmStartPayload) (string, error) {
	title := fmt.Sprintf("swarm:%s", payload.SwarmID)
	description := fmt.Sprintf("Tracking batch: %s\nTotal: %d polecats", payload.SwarmID, payload.Total)

	labels := strings.Join(SwarmWispLabels(payload.SwarmID, payload.Total, 0, payload.StartedAt), ",")

	cmd := exec.Command("bd", "create",
		"--wisp",
		"--title", title,
		"--description", description,
		"--labels", labels,
	)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("%s", errMsg)
		}
		return "", err
	}

	output := strings.TrimSpace(stdout.String())
	if strings.HasPrefix(output, "Created:") {
		return strings.TrimSpace(strings.TrimPrefix(output, "Created:")), nil
	}

	return output, nil
}

// findCleanupWisp finds an existing cleanup wisp for a polecat.
func findCleanupWisp(workDir, polecatName string) (string, error) {
	cmd := exec.Command("bd", "list",
		"--wisp",
		"--labels", fmt.Sprintf("polecat:%s,state:merge-requested", polecatName),
		"--status", "open",
		"--json",
	)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Empty result is fine
		if strings.Contains(stderr.String(), "no issues found") {
			return "", nil
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("%s", errMsg)
		}
		return "", err
	}

	// Parse JSON to get the wisp ID
	output := strings.TrimSpace(stdout.String())
	if output == "" || output == "[]" || output == "null" {
		return "", nil
	}

	// Simple extraction - look for "id" field
	// Full JSON parsing would add dependency on encoding/json
	if idx := strings.Index(output, `"id":`); idx >= 0 {
		rest := output[idx+5:]
		rest = strings.TrimLeft(rest, ` "`)
		if endIdx := strings.IndexAny(rest, `",}`); endIdx > 0 {
			return rest[:endIdx], nil
		}
	}

	return "", nil
}

// escalateToMayor sends an escalation mail to the Mayor.
func escalateToMayor(router *mail.Router, rigName string, payload *HelpPayload, reason string) (string, error) {
	msg := &mail.Message{
		From:     fmt.Sprintf("%s/witness", rigName),
		To:       "mayor/",
		Subject:  fmt.Sprintf("Escalation: %s needs help", payload.Agent),
		Priority: mail.PriorityHigh,
		Body: fmt.Sprintf(`Agent: %s
Issue: %s
Topic: %s
Problem: %s
Tried: %s
Escalation reason: %s
Requested at: %s`,
			payload.Agent,
			payload.IssueID,
			payload.Topic,
			payload.Problem,
			payload.Tried,
			reason,
			payload.RequestedAt.Format(time.RFC3339),
		),
	}

	if err := router.Send(msg); err != nil {
		return "", err
	}

	return msg.ID, nil
}

// UpdateCleanupWispState updates a cleanup wisp's state label.
func UpdateCleanupWispState(workDir, wispID, newState string) error {
	// Get current labels to preserve other labels
	cmd := exec.Command("bd", "show", wispID, "--json")
	cmd.Dir = workDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("getting wisp: %w", err)
	}

	// Extract polecat name from existing labels for the update
	output := stdout.String()
	var polecatName string
	if idx := strings.Index(output, `polecat:`); idx >= 0 {
		rest := output[idx+8:]
		if endIdx := strings.IndexAny(rest, `",]}`); endIdx > 0 {
			polecatName = rest[:endIdx]
		}
	}

	if polecatName == "" {
		polecatName = "unknown"
	}

	// Update with new state
	newLabels := strings.Join(CleanupWispLabels(polecatName, newState), ",")

	updateCmd := exec.Command("bd", "update", wispID, "--labels", newLabels)
	updateCmd.Dir = workDir

	var stderr bytes.Buffer
	updateCmd.Stderr = &stderr

	if err := updateCmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}
		return err
	}

	return nil
}
