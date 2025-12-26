package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/townlog"
	"github.com/steveyegge/gastown/internal/workspace"
)

// Log command flags
var (
	logTail   int
	logType   string
	logAgent  string
	logSince  string
	logFollow bool
)

var logCmd = &cobra.Command{
	Use:     "log",
	GroupID: GroupDiag,
	Short:   "View town activity log",
	Long: `View the centralized log of Gas Town agent lifecycle events.

Events logged include:
  spawn   - new agent created
  wake    - agent resumed
  nudge   - message injected into agent
  handoff - agent handed off to fresh session
  done    - agent finished work
  crash   - agent exited unexpectedly
  kill    - agent killed intentionally

Examples:
  gt log                     # Show last 20 events
  gt log -n 50               # Show last 50 events
  gt log --type spawn        # Show only spawn events
  gt log --agent gastown/    # Show events for gastown rig
  gt log --since 1h          # Show events from last hour
  gt log -f                  # Follow log (like tail -f)`,
	RunE: runLog,
}

func init() {
	logCmd.Flags().IntVarP(&logTail, "tail", "n", 20, "Number of events to show")
	logCmd.Flags().StringVarP(&logType, "type", "t", "", "Filter by event type (spawn,wake,nudge,handoff,done,crash,kill)")
	logCmd.Flags().StringVarP(&logAgent, "agent", "a", "", "Filter by agent prefix (e.g., gastown/, gastown/crew/max)")
	logCmd.Flags().StringVar(&logSince, "since", "", "Show events since duration (e.g., 1h, 30m, 24h)")
	logCmd.Flags().BoolVarP(&logFollow, "follow", "f", false, "Follow log output (like tail -f)")

	rootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	logPath := fmt.Sprintf("%s/logs/town.log", townRoot)

	// If following, use tail -f
	if logFollow {
		return followLog(logPath)
	}

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Printf("%s No log file yet (no events recorded)\n", style.Dim.Render("○"))
		return nil
	}

	// Read events
	events, err := townlog.ReadEvents(townRoot)
	if err != nil {
		return fmt.Errorf("reading events: %w", err)
	}

	if len(events) == 0 {
		fmt.Printf("%s No events in log\n", style.Dim.Render("○"))
		return nil
	}

	// Build filter
	filter := townlog.Filter{}

	if logType != "" {
		filter.Type = townlog.EventType(logType)
	}

	if logAgent != "" {
		filter.Agent = logAgent
	}

	if logSince != "" {
		duration, err := time.ParseDuration(logSince)
		if err != nil {
			return fmt.Errorf("invalid --since duration: %w", err)
		}
		filter.Since = time.Now().Add(-duration)
	}

	// Apply filter
	events = townlog.FilterEvents(events, filter)

	// Apply tail limit
	if logTail > 0 && len(events) > logTail {
		events = events[len(events)-logTail:]
	}

	if len(events) == 0 {
		fmt.Printf("%s No events match filter\n", style.Dim.Render("○"))
		return nil
	}

	// Print events
	for _, e := range events {
		printEvent(e)
	}

	return nil
}

// followLog uses tail -f to follow the log file.
func followLog(logPath string) error {
	// Check if log file exists, create empty if not
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		// Create logs directory and empty file
		if err := os.MkdirAll(fmt.Sprintf("%s", logPath[:len(logPath)-len("town.log")-1]), 0755); err != nil {
			return fmt.Errorf("creating logs directory: %w", err)
		}
		if _, err := os.Create(logPath); err != nil {
			return fmt.Errorf("creating log file: %w", err)
		}
	}

	fmt.Printf("%s Following %s (Ctrl+C to stop)\n\n", style.Dim.Render("○"), logPath)

	tailCmd := exec.Command("tail", "-f", logPath)
	tailCmd.Stdout = os.Stdout
	tailCmd.Stderr = os.Stderr

	return tailCmd.Run()
}

// printEvent prints a single event with styling.
func printEvent(e townlog.Event) {
	ts := e.Timestamp.Format("2006-01-02 15:04:05")

	// Color-code event types
	var typeStr string
	switch e.Type {
	case townlog.EventSpawn:
		typeStr = style.Success.Render("[spawn]")
	case townlog.EventWake:
		typeStr = style.Bold.Render("[wake]")
	case townlog.EventNudge:
		typeStr = style.Dim.Render("[nudge]")
	case townlog.EventHandoff:
		typeStr = style.Bold.Render("[handoff]")
	case townlog.EventDone:
		typeStr = style.Success.Render("[done]")
	case townlog.EventCrash:
		typeStr = style.Error.Render("[crash]")
	case townlog.EventKill:
		typeStr = style.Warning.Render("[kill]")
	default:
		typeStr = fmt.Sprintf("[%s]", e.Type)
	}

	detail := formatEventDetail(e)
	fmt.Printf("%s %s %s %s\n", style.Dim.Render(ts), typeStr, e.Agent, detail)
}

// formatEventDetail returns a human-readable detail string for an event.
func formatEventDetail(e townlog.Event) string {
	switch e.Type {
	case townlog.EventSpawn:
		if e.Context != "" {
			return fmt.Sprintf("spawned for %s", e.Context)
		}
		return "spawned"
	case townlog.EventWake:
		if e.Context != "" {
			return fmt.Sprintf("resumed (%s)", e.Context)
		}
		return "resumed"
	case townlog.EventNudge:
		if e.Context != "" {
			return fmt.Sprintf("nudged with %q", truncateStr(e.Context, 40))
		}
		return "nudged"
	case townlog.EventHandoff:
		if e.Context != "" {
			return fmt.Sprintf("handed off (%s)", e.Context)
		}
		return "handed off"
	case townlog.EventDone:
		if e.Context != "" {
			return fmt.Sprintf("completed %s", e.Context)
		}
		return "completed work"
	case townlog.EventCrash:
		if e.Context != "" {
			return fmt.Sprintf("exited unexpectedly (%s)", e.Context)
		}
		return "exited unexpectedly"
	case townlog.EventKill:
		if e.Context != "" {
			return fmt.Sprintf("killed (%s)", e.Context)
		}
		return "killed"
	default:
		if e.Context != "" {
			return fmt.Sprintf("%s (%s)", e.Type, e.Context)
		}
		return string(e.Type)
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// LogEvent is a helper that logs an event from anywhere in the codebase.
// It finds the town root and logs the event.
func LogEvent(eventType townlog.EventType, agent, context string) error {
	townRoot, err := workspace.FindFromCwd()
	if err != nil {
		return err // Silently fail if not in a workspace
	}
	if townRoot == "" {
		return nil
	}

	logger := townlog.NewLogger(townRoot)
	return logger.Log(eventType, agent, context)
}

// LogEventWithRoot logs an event when the town root is already known.
func LogEventWithRoot(townRoot string, eventType townlog.EventType, agent, context string) error {
	logger := townlog.NewLogger(townRoot)
	return logger.Log(eventType, agent, context)
}

// Convenience functions for common events

// LogSpawn logs a spawn event.
func LogSpawn(townRoot, agent, issueID string) error {
	return LogEventWithRoot(townRoot, townlog.EventSpawn, agent, issueID)
}

// LogWake logs a wake event.
func LogWake(townRoot, agent, context string) error {
	return LogEventWithRoot(townRoot, townlog.EventWake, agent, context)
}

// LogNudge logs a nudge event.
func LogNudge(townRoot, agent, message string) error {
	return LogEventWithRoot(townRoot, townlog.EventNudge, agent, strings.TrimSpace(message))
}

// LogHandoff logs a handoff event.
func LogHandoff(townRoot, agent, context string) error {
	return LogEventWithRoot(townRoot, townlog.EventHandoff, agent, context)
}

// LogDone logs a done event.
func LogDone(townRoot, agent, issueID string) error {
	return LogEventWithRoot(townRoot, townlog.EventDone, agent, issueID)
}

// LogCrash logs a crash event.
func LogCrash(townRoot, agent, reason string) error {
	return LogEventWithRoot(townRoot, townlog.EventCrash, agent, reason)
}

// LogKill logs a kill event.
func LogKill(townRoot, agent, reason string) error {
	return LogEventWithRoot(townRoot, townlog.EventKill, agent, reason)
}
