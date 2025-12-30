package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/mail"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/townlog"
	"github.com/steveyegge/gastown/internal/workspace"
)

// Callback message subject patterns for routing.
var (
	// POLECAT_DONE <name> - polecat signaled completion
	patternPolecatDone = regexp.MustCompile(`^POLECAT_DONE\s+(\S+)`)

	// Merge Request Rejected: <branch> - refinery rejected MR
	patternMergeRejected = regexp.MustCompile(`^Merge Request Rejected:\s+(.+)`)

	// Merge Request Completed: <branch> - refinery completed MR
	patternMergeCompleted = regexp.MustCompile(`^Merge Request Completed:\s+(.+)`)

	// HELP: <topic> - polecat requesting help
	patternHelp = regexp.MustCompile(`^HELP:\s+(.+)`)

	// ESCALATION: <topic> - witness escalating issue
	patternEscalation = regexp.MustCompile(`^ESCALATION:\s+(.+)`)

	// SLING_REQUEST: <bead-id> - request to sling work
	patternSling = regexp.MustCompile(`^SLING_REQUEST:\s+(\S+)`)

	// WITNESS_REPORT: <rig> - periodic health report from witness
	patternWitnessReport = regexp.MustCompile(`^WITNESS_REPORT:\s+(\S+)`)

	// REFINERY_REPORT: <rig> - periodic status from refinery
	patternRefineryReport = regexp.MustCompile(`^REFINERY_REPORT:\s+(\S+)`)
)

// CallbackType identifies the type of callback message.
type CallbackType string

const (
	CallbackPolecatDone    CallbackType = "polecat_done"
	CallbackMergeRejected  CallbackType = "merge_rejected"
	CallbackMergeCompleted CallbackType = "merge_completed"
	CallbackHelp           CallbackType = "help"
	CallbackEscalation     CallbackType = "escalation"
	CallbackSling          CallbackType = "sling"
	CallbackWitnessReport  CallbackType = "witness_report"
	CallbackRefineryReport CallbackType = "refinery_report"
	CallbackUnknown        CallbackType = "unknown"
)

// CallbackResult tracks the result of processing a callback.
type CallbackResult struct {
	MessageID    string
	CallbackType CallbackType
	From         string
	Subject      string
	Handled      bool
	Action       string
	Error        error
}

var callbacksCmd = &cobra.Command{
	Use:     "callbacks",
	GroupID: GroupAgents,
	Short:   "Handle agent callbacks",
	Long: `Handle callbacks from agents during Deacon patrol.

Callbacks are messages sent to the Mayor from:
- Witnesses reporting polecat status
- Refineries reporting merge results
- Polecats requesting help or escalation
- External triggers (webhooks, timers)

This command processes the Mayor's inbox and handles each message
appropriately, routing to other agents or updating state as needed.`,
}

var callbacksProcessCmd = &cobra.Command{
	Use:   "process",
	Short: "Process pending callbacks",
	Long: `Process all pending callbacks in the Mayor's inbox.

Reads unread messages from the Mayor's inbox and handles each based on
its type:

  POLECAT_DONE       - Log completion, update stats
  MERGE_COMPLETED    - Notify worker, close source issue
  MERGE_REJECTED     - Notify worker of rejection reason
  HELP:              - Route to human or handle if possible
  ESCALATION:        - Log and route to human
  SLING_REQUEST:     - Spawn polecat for the work
  WITNESS_REPORT:    - Log health status
  REFINERY_REPORT:   - Log queue status

Unknown message types are logged but left unprocessed.`,
	RunE: runCallbacksProcess,
}

var (
	callbacksDryRun  bool
	callbacksVerbose bool
)

func init() {
	callbacksProcessCmd.Flags().BoolVar(&callbacksDryRun, "dry-run", false, "Show what would be processed without taking action")
	callbacksProcessCmd.Flags().BoolVarP(&callbacksVerbose, "verbose", "v", false, "Show detailed processing info")

	callbacksCmd.AddCommand(callbacksProcessCmd)
	rootCmd.AddCommand(callbacksCmd)
}

func runCallbacksProcess(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Get Mayor's mailbox
	router := mail.NewRouter(townRoot)
	mailbox, err := router.GetMailbox("mayor/")
	if err != nil {
		return fmt.Errorf("getting mayor mailbox: %w", err)
	}

	// Get unread messages
	messages, err := mailbox.ListUnread()
	if err != nil {
		return fmt.Errorf("listing unread messages: %w", err)
	}

	if len(messages) == 0 {
		fmt.Printf("%s No pending callbacks\n", style.Dim.Render("○"))
		return nil
	}

	fmt.Printf("%s Processing %d callback(s)\n", style.Bold.Render("●"), len(messages))

	var results []CallbackResult
	for _, msg := range messages {
		result := processCallback(townRoot, msg, callbacksDryRun)
		results = append(results, result)

		// Print result
		if result.Error != nil {
			fmt.Printf("  %s %s: %v\n",
				style.Error.Render("✗"),
				msg.Subject,
				result.Error)
		} else if result.Handled {
			fmt.Printf("  %s [%s] %s\n",
				style.Bold.Render("✓"),
				result.CallbackType,
				result.Action)
		} else {
			fmt.Printf("  %s [%s] %s\n",
				style.Dim.Render("○"),
				result.CallbackType,
				result.Action)
		}

		if callbacksVerbose {
			fmt.Printf("      From: %s\n", msg.From)
			fmt.Printf("      Subject: %s\n", msg.Subject)
		}
	}

	// Summary
	handled := 0
	errors := 0
	for _, r := range results {
		if r.Handled {
			handled++
		}
		if r.Error != nil {
			errors++
		}
	}

	fmt.Println()
	if callbacksDryRun {
		fmt.Printf("%s Dry run: would process %d/%d callbacks\n",
			style.Dim.Render("○"), handled, len(results))
	} else {
		fmt.Printf("%s Processed %d/%d callbacks",
			style.Bold.Render("✓"), handled, len(results))
		if errors > 0 {
			fmt.Printf(" (%d errors)", errors)
		}
		fmt.Println()
	}

	return nil
}

// processCallback handles a single callback message and returns the result.
func processCallback(townRoot string, msg *mail.Message, dryRun bool) CallbackResult {
	result := CallbackResult{
		MessageID: msg.ID,
		From:      msg.From,
		Subject:   msg.Subject,
	}

	// Classify the callback
	result.CallbackType = classifyCallback(msg.Subject)

	// Handle based on type
	switch result.CallbackType {
	case CallbackPolecatDone:
		result.Action, result.Error = handlePolecatDone(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	case CallbackMergeCompleted:
		result.Action, result.Error = handleMergeCompleted(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	case CallbackMergeRejected:
		result.Action, result.Error = handleMergeRejected(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	case CallbackHelp:
		result.Action, result.Error = handleHelp(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	case CallbackEscalation:
		result.Action, result.Error = handleEscalation(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	case CallbackSling:
		result.Action, result.Error = handleSling(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	case CallbackWitnessReport:
		result.Action, result.Error = handleWitnessReport(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	case CallbackRefineryReport:
		result.Action, result.Error = handleRefineryReport(townRoot, msg, dryRun)
		result.Handled = result.Error == nil

	default:
		result.Action = "unknown message type, skipped"
		result.Handled = false
	}

	// Archive handled messages (unless dry-run)
	if result.Handled && !dryRun {
		router := mail.NewRouter(townRoot)
		if mailbox, err := router.GetMailbox("mayor/"); err == nil {
			_ = mailbox.Delete(msg.ID)
		}
	}

	return result
}

// classifyCallback determines the type of callback from the subject line.
func classifyCallback(subject string) CallbackType {
	switch {
	case patternPolecatDone.MatchString(subject):
		return CallbackPolecatDone
	case patternMergeRejected.MatchString(subject):
		return CallbackMergeRejected
	case patternMergeCompleted.MatchString(subject):
		return CallbackMergeCompleted
	case patternHelp.MatchString(subject):
		return CallbackHelp
	case patternEscalation.MatchString(subject):
		return CallbackEscalation
	case patternSling.MatchString(subject):
		return CallbackSling
	case patternWitnessReport.MatchString(subject):
		return CallbackWitnessReport
	case patternRefineryReport.MatchString(subject):
		return CallbackRefineryReport
	default:
		return CallbackUnknown
	}
}

// handlePolecatDone processes a POLECAT_DONE callback.
// These come from Witnesses forwarding polecat completion notices.
func handlePolecatDone(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternPolecatDone.FindStringSubmatch(msg.Subject)
	polecatName := ""
	if len(matches) > 1 {
		polecatName = matches[1]
	}

	// Extract info from body
	var exitType, issueID string
	for _, line := range strings.Split(msg.Body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Exit:") {
			exitType = strings.TrimSpace(strings.TrimPrefix(line, "Exit:"))
		}
		if strings.HasPrefix(line, "Issue:") {
			issueID = strings.TrimSpace(strings.TrimPrefix(line, "Issue:"))
		}
	}

	if dryRun {
		return fmt.Sprintf("would log completion for %s (exit=%s, issue=%s)",
			polecatName, exitType, issueID), nil
	}

	// Log the completion
	logCallback(townRoot, fmt.Sprintf("polecat_done: %s completed with %s (issue: %s)",
		msg.From, exitType, issueID))

	return fmt.Sprintf("logged completion for %s", polecatName), nil
}

// handleMergeCompleted processes a merge completion callback from Refinery.
func handleMergeCompleted(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternMergeCompleted.FindStringSubmatch(msg.Subject)
	branch := ""
	if len(matches) > 1 {
		branch = matches[1]
	}

	// Extract MR ID and source issue from body
	var mrID, sourceIssue, mergeCommit string
	for _, line := range strings.Split(msg.Body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MR:") {
			mrID = strings.TrimSpace(strings.TrimPrefix(line, "MR:"))
		}
		if strings.HasPrefix(line, "Source:") {
			sourceIssue = strings.TrimSpace(strings.TrimPrefix(line, "Source:"))
		}
		if strings.HasPrefix(line, "Commit:") {
			mergeCommit = strings.TrimSpace(strings.TrimPrefix(line, "Commit:"))
		}
	}

	if dryRun {
		return fmt.Sprintf("would close source issue %s (mr=%s, commit=%s)",
			sourceIssue, mrID, mergeCommit), nil
	}

	// Log the merge
	logCallback(townRoot, fmt.Sprintf("merge_completed: branch %s merged (mr=%s, source=%s, commit=%s)",
		branch, mrID, sourceIssue, mergeCommit))

	// Close the source issue if we have it
	if sourceIssue != "" {
		cwd, _ := os.Getwd()
		bd := beads.New(cwd)
		reason := fmt.Sprintf("Merged in %s", mergeCommit)
		if err := bd.Close(sourceIssue, reason); err != nil {
			// Non-fatal: issue might already be closed or not exist
			return fmt.Sprintf("logged merge for %s (could not close %s: %v)",
				branch, sourceIssue, err), nil
		}
	}

	return fmt.Sprintf("logged merge for %s, closed %s", branch, sourceIssue), nil
}

// handleMergeRejected processes a merge rejection callback from Refinery.
func handleMergeRejected(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternMergeRejected.FindStringSubmatch(msg.Subject)
	branch := ""
	if len(matches) > 1 {
		branch = matches[1]
	}

	// Extract reason from body
	var reason string
	if strings.Contains(msg.Body, "Reason:") {
		parts := strings.SplitN(msg.Body, "Reason:", 2)
		if len(parts) > 1 {
			reason = strings.TrimSpace(parts[1])
			// Take just the first line of the reason
			if idx := strings.Index(reason, "\n"); idx > 0 {
				reason = reason[:idx]
			}
		}
	}

	if dryRun {
		return fmt.Sprintf("would log rejection for %s (reason: %s)", branch, reason), nil
	}

	// Log the rejection
	logCallback(townRoot, fmt.Sprintf("merge_rejected: branch %s rejected: %s", branch, reason))

	return fmt.Sprintf("logged rejection for %s", branch), nil
}

// handleHelp processes a HELP: request from a polecat.
func handleHelp(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternHelp.FindStringSubmatch(msg.Subject)
	topic := ""
	if len(matches) > 1 {
		topic = matches[1]
	}

	if dryRun {
		return fmt.Sprintf("would forward help request to overseer: %s", topic), nil
	}

	// Forward to overseer (human)
	router := mail.NewRouter(townRoot)
	fwd := &mail.Message{
		From:     "mayor/",
		To:       "overseer",
		Subject:  fmt.Sprintf("[FWD] HELP: %s", topic),
		Body:     fmt.Sprintf("Forwarded from: %s\n\n%s", msg.From, msg.Body),
		Priority: mail.PriorityHigh,
	}
	if err := router.Send(fwd); err != nil {
		return "", fmt.Errorf("forwarding to overseer: %w", err)
	}

	// Log the help request
	logCallback(townRoot, fmt.Sprintf("help_request: from %s: %s", msg.From, topic))

	return fmt.Sprintf("forwarded help request to overseer: %s", topic), nil
}

// handleEscalation processes an ESCALATION: from a Witness.
func handleEscalation(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternEscalation.FindStringSubmatch(msg.Subject)
	topic := ""
	if len(matches) > 1 {
		topic = matches[1]
	}

	if dryRun {
		return fmt.Sprintf("would forward escalation to overseer: %s", topic), nil
	}

	// Forward to overseer with urgent priority
	router := mail.NewRouter(townRoot)
	fwd := &mail.Message{
		From:     "mayor/",
		To:       "overseer",
		Subject:  fmt.Sprintf("[ESCALATION] %s", topic),
		Body:     fmt.Sprintf("Escalated by: %s\n\n%s", msg.From, msg.Body),
		Priority: mail.PriorityUrgent,
	}
	if err := router.Send(fwd); err != nil {
		return "", fmt.Errorf("forwarding escalation: %w", err)
	}

	// Log the escalation
	logCallback(townRoot, fmt.Sprintf("escalation: from %s: %s", msg.From, topic))

	return fmt.Sprintf("forwarded escalation to overseer: %s", topic), nil
}

// handleSling processes a SLING_REQUEST to spawn work on a polecat.
func handleSling(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternSling.FindStringSubmatch(msg.Subject)
	beadID := ""
	if len(matches) > 1 {
		beadID = matches[1]
	}

	// Extract rig from body
	var targetRig string
	for _, line := range strings.Split(msg.Body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Rig:") {
			targetRig = strings.TrimSpace(strings.TrimPrefix(line, "Rig:"))
		}
	}

	if targetRig == "" {
		return "", fmt.Errorf("no target rig specified in sling request")
	}

	if dryRun {
		return fmt.Sprintf("would sling %s to %s", beadID, targetRig), nil
	}

	// Log the sling (actual spawn happens via gt sling command)
	logCallback(townRoot, fmt.Sprintf("sling_request: bead %s to rig %s", beadID, targetRig))

	// Note: We don't actually spawn here - that would be done by the Deacon
	// executing the sling command based on this request.
	return fmt.Sprintf("logged sling request: %s to %s (execute with: gt sling %s %s)",
		beadID, targetRig, beadID, targetRig), nil
}

// handleWitnessReport processes a WITNESS_REPORT from a rig's Witness.
func handleWitnessReport(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternWitnessReport.FindStringSubmatch(msg.Subject)
	rig := ""
	if len(matches) > 1 {
		rig = matches[1]
	}

	// Extract stats from body
	var healthy, unhealthy, stuck int
	for _, line := range strings.Split(msg.Body, "\n") {
		line = strings.TrimSpace(line)
		fmt.Sscanf(line, "Healthy: %d", &healthy)
		fmt.Sscanf(line, "Unhealthy: %d", &unhealthy)
		fmt.Sscanf(line, "Stuck: %d", &stuck)
	}

	if dryRun {
		return fmt.Sprintf("would log witness report for %s (healthy=%d, unhealthy=%d)",
			rig, healthy, unhealthy), nil
	}

	// Log the report
	logCallback(townRoot, fmt.Sprintf("witness_report: rig %s: healthy=%d, unhealthy=%d, stuck=%d",
		rig, healthy, unhealthy, stuck))

	return fmt.Sprintf("logged witness report for %s", rig), nil
}

// handleRefineryReport processes a REFINERY_REPORT from a rig's Refinery.
func handleRefineryReport(townRoot string, msg *mail.Message, dryRun bool) (string, error) {
	matches := patternRefineryReport.FindStringSubmatch(msg.Subject)
	rig := ""
	if len(matches) > 1 {
		rig = matches[1]
	}

	// Extract stats from body
	var pending, processed, failed int
	for _, line := range strings.Split(msg.Body, "\n") {
		line = strings.TrimSpace(line)
		fmt.Sscanf(line, "Pending: %d", &pending)
		fmt.Sscanf(line, "Processed: %d", &processed)
		fmt.Sscanf(line, "Failed: %d", &failed)
	}

	if dryRun {
		return fmt.Sprintf("would log refinery report for %s (pending=%d, processed=%d)",
			rig, pending, processed), nil
	}

	// Log the report
	logCallback(townRoot, fmt.Sprintf("refinery_report: rig %s: pending=%d, processed=%d, failed=%d",
		rig, pending, processed, failed))

	return fmt.Sprintf("logged refinery report for %s", rig), nil
}

// logCallback logs a callback processing event to the town log.
func logCallback(townRoot, context string) {
	logger := townlog.NewLogger(townRoot)
	_ = logger.Log(townlog.EventCallback, "mayor/", context)
}
