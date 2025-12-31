// Package claude provides integration with Claude Code's local data.
package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SessionInfo represents a Claude Code session.
type SessionInfo struct {
	ID        string    `json:"id"`         // Session UUID
	Path      string    `json:"path"`       // Decoded project path
	Role      string    `json:"role"`       // Gas Town role (from beacon)
	Topic     string    `json:"topic"`      // Topic (from beacon)
	StartTime time.Time `json:"start_time"` // First message timestamp
	Summary   string    `json:"summary"`    // Session summary
	IsGasTown bool      `json:"is_gastown"` // Has [GAS TOWN] beacon
	FilePath  string    `json:"file_path"`  // Full path to JSONL file
}

// SessionFilter controls which sessions are returned.
type SessionFilter struct {
	GasTownOnly bool   // Only return Gas Town sessions
	Role        string // Filter by role (substring match)
	Rig         string // Filter by rig name
	Path        string // Filter by path (substring match)
	Limit       int    // Max sessions to return (0 = unlimited)
}

// gasTownPattern matches the beacon: [GAS TOWN] role • topic • timestamp
var gasTownPattern = regexp.MustCompile(`\[GAS TOWN\]\s+([^\s•]+)\s*(?:•\s*([^•]+?)\s*)?(?:•\s*(\S+))?\s*$`)

// DiscoverSessions finds Claude Code sessions matching the filter.
func DiscoverSessions(filter SessionFilter) ([]SessionInfo, error) {
	claudeDir := os.ExpandEnv("$HOME/.claude")
	projectsDir := filepath.Join(claudeDir, "projects")

	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil // No sessions yet
	}

	var sessions []SessionInfo

	// Walk project directories
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Decode path from directory name
		projectPath := decodePath(entry.Name())

		// Apply path/rig filter early
		if filter.Rig != "" && !strings.Contains(projectPath, "/"+filter.Rig+"/") {
			continue
		}
		if filter.Path != "" && !strings.Contains(projectPath, filter.Path) {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		sessionFiles, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, sf := range sessionFiles {
			if !strings.HasSuffix(sf.Name(), ".jsonl") {
				continue
			}

			// Skip agent files (they're subprocesses, not main sessions)
			if strings.HasPrefix(sf.Name(), "agent-") {
				continue
			}

			sessionPath := filepath.Join(projectDir, sf.Name())
			info, err := parseSession(sessionPath, projectPath)
			if err != nil {
				continue
			}

			// Apply filters
			if filter.GasTownOnly && !info.IsGasTown {
				continue
			}
			if filter.Role != "" {
				// Check Role field first, then path
				roleMatch := strings.Contains(strings.ToLower(info.Role), strings.ToLower(filter.Role))
				pathMatch := strings.Contains(strings.ToLower(info.Path), strings.ToLower(filter.Role))
				if !roleMatch && !pathMatch {
					continue
				}
			}

			sessions = append(sessions, info)
		}
	}

	// Sort by start time descending (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	// Apply limit
	if filter.Limit > 0 && len(sessions) > filter.Limit {
		sessions = sessions[:filter.Limit]
	}

	return sessions, nil
}

// parseSession reads a session JSONL file and extracts metadata.
func parseSession(filePath, projectPath string) (SessionInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return SessionInfo{}, err
	}
	defer file.Close()

	info := SessionInfo{
		Path:     projectPath,
		FilePath: filePath,
	}

	// Extract session ID from filename
	base := filepath.Base(filePath)
	info.ID = strings.TrimSuffix(base, ".jsonl")

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// First line is usually the summary
		if lineNum == 1 {
			var entry struct {
				Type    string `json:"type"`
				Summary string `json:"summary"`
			}
			if err := json.Unmarshal(line, &entry); err == nil && entry.Type == "summary" {
				info.Summary = entry.Summary
			}
			continue
		}

		// Look for user messages
		var entry struct {
			Type      string `json:"type"`
			SessionID string `json:"sessionId"`
			Timestamp string `json:"timestamp"`
			Message   json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Type == "user" {
			// Parse timestamp
			if entry.Timestamp != "" && info.StartTime.IsZero() {
				if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
					info.StartTime = t
				}
			}

			// Set session ID if not already set
			if info.ID == "" && entry.SessionID != "" {
				info.ID = entry.SessionID
			}

			// Look for Gas Town beacon in message
			if !info.IsGasTown {
				msgStr := extractMessageContent(entry.Message)
				if match := gasTownPattern.FindStringSubmatch(msgStr); match != nil {
					info.IsGasTown = true
					info.Role = match[1]
					if len(match) > 2 {
						info.Topic = strings.TrimSpace(match[2])
					}
				}
			}
		}

		// Stop after finding what we need
		if info.IsGasTown && !info.StartTime.IsZero() && lineNum > 20 {
			break
		}
	}

	return info, nil
}

// extractMessageContent extracts text content from a message JSON.
func extractMessageContent(msg json.RawMessage) string {
	if len(msg) == 0 {
		return ""
	}

	// Try as string first
	var str string
	if err := json.Unmarshal(msg, &str); err == nil {
		return str
	}

	// Try as object with role/content
	var obj struct {
		Content string `json:"content"`
		Role    string `json:"role"`
	}
	if err := json.Unmarshal(msg, &obj); err == nil {
		return obj.Content
	}

	return ""
}

// decodePath converts Claude's path-encoded directory names back to paths.
// e.g., "-Users-stevey-gt-gastown" -> "/Users/stevey/gt/gastown"
func decodePath(encoded string) string {
	// Replace leading dash with /
	if strings.HasPrefix(encoded, "-") {
		encoded = "/" + encoded[1:]
	}
	// Replace remaining dashes with /
	return strings.ReplaceAll(encoded, "-", "/")
}

// ShortID returns a shortened version of the session ID for display.
func (s SessionInfo) ShortID() string {
	if len(s.ID) > 8 {
		return s.ID[:8]
	}
	return s.ID
}

// FormatTime returns the start time in a compact format.
func (s SessionInfo) FormatTime() string {
	if s.StartTime.IsZero() {
		return "unknown"
	}
	return s.StartTime.Format("2006-01-02 15:04")
}

// RigFromPath extracts the rig name from the project path.
func (s SessionInfo) RigFromPath() string {
	// Look for known rig patterns
	parts := strings.Split(s.Path, "/")
	for i, part := range parts {
		if part == "gt" && i+1 < len(parts) {
			// Next part after gt/ is usually the rig
			return parts[i+1]
		}
	}
	return ""
}
