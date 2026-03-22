// Package commands provides agent-agnostic command provisioning.
//
// Claude Code commands go to <configDir>/commands/<name>.md with YAML frontmatter
// (description, allowed-tools, argument-hint). OpenCode commands go to
// .opencode/skills/<name>/SKILL.md as auto-discoverable skills.
package commands

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/config"
)

//go:embed bodies/*.md

var bodiesFS embed.FS

// Field represents a frontmatter key-value pair.
type Field struct {
	Key   string
	Value string
}

// Command defines a slash command with agent-specific frontmatter.
type Command struct {
	Name        string
	Description string
	AgentFields map[string][]Field
}

// Commands is the registry of available commands.
var Commands = []Command{
	{
		Name:        "handoff",
		Description: "Hand off to fresh session, work continues from hook",
		AgentFields: map[string][]Field{
			"claude": {
				{"allowed-tools", "Bash(gt handoff:*)"},
				{"argument-hint", "[message]"},
			},
			// opencode: no extra fields needed — uses skill format
		},
	},
	{
		Name:        "review",
		Description: "Review code changes with structured grading (A-F)",
		AgentFields: map[string][]Field{
			"claude": {
				{"allowed-tools", "Bash(git diff:*), Bash(git rev-parse:*), Bash(gh pr diff:*)"},
				{"argument-hint", "[--staged | --branch | --pr <url>]"},
			},
		},
	},
}

// commandPath returns the filesystem path where a command file should be written
// for a given agent. OpenCode uses skills; others use commands.
func commandPath(workspacePath string, preset *config.AgentPresetInfo, name string) string {
	if preset.Name == config.AgentOpenCode {
		// OpenCode: skills at .opencode/skills/<name>/SKILL.md
		// These are auto-discovered and available as slash commands.
		return filepath.Join(workspacePath, ".opencode", "skills", name, "SKILL.md")
	}
	// Claude and others: <configDir>/commands/<name>.md
	return filepath.Join(workspacePath, preset.ConfigDir, "commands", name+".md")
}

// BuildCommand assembles frontmatter + body for an agent.
// OpenCode uses a simpler skill frontmatter (name + description only).
// Claude uses the full frontmatter (description + allowed-tools + argument-hint).
func BuildCommand(cmd Command, agent string) (string, error) {
	body, err := bodiesFS.ReadFile("bodies/" + cmd.Name + ".md")
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}

	var b strings.Builder
	b.WriteString("---\n")

	if strings.ToLower(agent) == string(config.AgentOpenCode) {
		// OpenCode skill frontmatter: just name + description.
		b.WriteString(fmt.Sprintf("name: %s\n", cmd.Name))
		b.WriteString(fmt.Sprintf("description: %s\n", cmd.Description))
	} else {
		// Claude-style frontmatter.
		b.WriteString(fmt.Sprintf("description: %s\n", cmd.Description))
		if fields, ok := cmd.AgentFields[agent]; ok {
			for _, f := range fields {
				b.WriteString(fmt.Sprintf("%s: %s\n", f.Key, f.Value))
			}
		}
	}

	b.WriteString("---\n\n")
	b.Write(body)

	return b.String(), nil
}

// ProvisionFor provisions commands for an agent.
func ProvisionFor(workspacePath, agent string) error {
	agent = strings.ToLower(agent)
	preset := config.GetAgentPresetByName(agent)
	if preset == nil || preset.ConfigDir == "" {
		return fmt.Errorf("unknown agent or no config dir: %s", agent)
	}

	for _, cmd := range Commands {
		path := commandPath(workspacePath, preset, cmd.Name)

		// Don't overwrite existing.
		if _, err := os.Stat(path); err == nil {
			continue
		}

		// Ensure parent directory exists.
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("creating dir for %s: %w", cmd.Name, err)
		}

		content, err := BuildCommand(cmd, agent)
		if err != nil {
			return fmt.Errorf("building %s: %w", cmd.Name, err)
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", cmd.Name, err)
		}
	}

	return nil
}

// MissingFor returns commands missing for an agent.
func MissingFor(workspacePath, agent string) []string {
	agent = strings.ToLower(agent)
	preset := config.GetAgentPresetByName(agent)
	if preset == nil || preset.ConfigDir == "" {
		return nil
	}

	var missing []string
	for _, cmd := range Commands {
		path := commandPath(workspacePath, preset, cmd.Name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, cmd.Name)
		}
	}

	return missing
}

// FindByName returns the command with the given name, or nil if not found.
func FindByName(name string) *Command {
	for i := range Commands {
		if Commands[i].Name == name {
			return &Commands[i]
		}
	}
	return nil
}

// Names returns the names of all registered commands.
func Names() []string {
	names := make([]string, len(Commands))
	for i, cmd := range Commands {
		names[i] = cmd.Name
	}
	return names
}

// IsKnownAgent returns true if the agent has a preset with a config directory.
func IsKnownAgent(agent string) bool {
	preset := config.GetAgentPresetByName(strings.ToLower(agent))
	return preset != nil && preset.ConfigDir != ""
}
