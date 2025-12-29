package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/crew"
	"github.com/steveyegge/gastown/internal/git"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/workspace"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"stat"},
	GroupID: GroupDiag,
	Short:   "Show overall town status",
	Long: `Display the current status of the Gas Town workspace.

Shows town name, registered rigs, active polecats, and witness status.`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(statusCmd)
}

// TownStatus represents the overall status of the workspace.
type TownStatus struct {
	Name     string        `json:"name"`
	Location string        `json:"location"`
	Agents   []AgentRuntime `json:"agents"`   // Global agents (Mayor, Deacon)
	Rigs     []RigStatus   `json:"rigs"`
	Summary  StatusSum     `json:"summary"`
}

// AgentRuntime represents the runtime state of an agent.
type AgentRuntime struct {
	Name      string `json:"name"`       // Display name (e.g., "mayor", "witness")
	Address   string `json:"address"`    // Full address (e.g., "gastown/witness")
	Session   string `json:"session"`    // tmux session name
	Role      string `json:"role"`       // Role type
	Running   bool   `json:"running"`    // Is tmux session running?
	HasWork   bool   `json:"has_work"`   // Has pinned work?
	WorkTitle string `json:"work_title,omitempty"` // Title of pinned work
	HookBead  string `json:"hook_bead,omitempty"`  // Pinned bead ID from agent bead
	State     string `json:"state,omitempty"`      // Agent state from agent bead
}

// RigStatus represents status of a single rig.
type RigStatus struct {
	Name         string          `json:"name"`
	Polecats     []string        `json:"polecats"`
	PolecatCount int             `json:"polecat_count"`
	Crews        []string        `json:"crews"`
	CrewCount    int             `json:"crew_count"`
	HasWitness   bool            `json:"has_witness"`
	HasRefinery  bool            `json:"has_refinery"`
	Hooks        []AgentHookInfo `json:"hooks,omitempty"`
	Agents       []AgentRuntime  `json:"agents,omitempty"` // Runtime state of all agents in rig
}

// AgentHookInfo represents an agent's hook (pinned work) status.
type AgentHookInfo struct {
	Agent    string `json:"agent"`              // Agent address (e.g., "gastown/toast", "gastown/witness")
	Role     string `json:"role"`               // Role type (polecat, crew, witness, refinery)
	HasWork  bool   `json:"has_work"`           // Whether agent has pinned work
	Molecule string `json:"molecule,omitempty"` // Attached molecule ID
	Title    string `json:"title,omitempty"`    // Pinned bead title
}

// StatusSum provides summary counts.
type StatusSum struct {
	RigCount      int `json:"rig_count"`
	PolecatCount  int `json:"polecat_count"`
	CrewCount     int `json:"crew_count"`
	WitnessCount  int `json:"witness_count"`
	RefineryCount int `json:"refinery_count"`
	ActiveHooks   int `json:"active_hooks"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Find town root
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Load town config
	townConfigPath := constants.MayorTownPath(townRoot)
	townConfig, err := config.LoadTownConfig(townConfigPath)
	if err != nil {
		// Try to continue without config
		townConfig = &config.TownConfig{Name: filepath.Base(townRoot)}
	}

	// Load rigs config
	rigsConfigPath := constants.MayorRigsPath(townRoot)
	rigsConfig, err := config.LoadRigsConfig(rigsConfigPath)
	if err != nil {
		// Empty config if file doesn't exist
		rigsConfig = &config.RigsConfig{Rigs: make(map[string]config.RigEntry)}
	}

	// Create rig manager
	g := git.NewGit(townRoot)
	mgr := rig.NewManager(townRoot, rigsConfig, g)

	// Create tmux instance for runtime checks
	t := tmux.NewTmux()

	// Discover rigs
	rigs, err := mgr.DiscoverRigs()
	if err != nil {
		return fmt.Errorf("discovering rigs: %w", err)
	}

	// Create beads instance for agent bead lookups (gastown rig holds gt- prefix beads)
	gastownBeadsPath := filepath.Join(townRoot, "gastown", "mayor", "rig")
	agentBeads := beads.New(gastownBeadsPath)

	// Build status
	status := TownStatus{
		Name:     townConfig.Name,
		Location: townRoot,
		Agents:   discoverGlobalAgents(t, agentBeads),
		Rigs:     make([]RigStatus, 0, len(rigs)),
	}

	for _, r := range rigs {
		rs := RigStatus{
			Name:         r.Name,
			Polecats:     r.Polecats,
			PolecatCount: len(r.Polecats),
			HasWitness:   r.HasWitness,
			HasRefinery:  r.HasRefinery,
		}

		// Count crew workers
		crewGit := git.NewGit(r.Path)
		crewMgr := crew.NewManager(r, crewGit)
		if workers, err := crewMgr.List(); err == nil {
			for _, w := range workers {
				rs.Crews = append(rs.Crews, w.Name)
			}
			rs.CrewCount = len(workers)
		}

		// Discover hooks for all agents in this rig
		rs.Hooks = discoverRigHooks(r, rs.Crews)
		for _, hook := range rs.Hooks {
			if hook.HasWork {
				status.Summary.ActiveHooks++
			}
		}

		// Discover runtime state for all agents in this rig
		rs.Agents = discoverRigAgents(t, r, rs.Crews, agentBeads)

		status.Rigs = append(status.Rigs, rs)

		// Update summary
		status.Summary.PolecatCount += len(r.Polecats)
		status.Summary.CrewCount += rs.CrewCount
		if r.HasWitness {
			status.Summary.WitnessCount++
		}
		if r.HasRefinery {
			status.Summary.RefineryCount++
		}
	}
	status.Summary.RigCount = len(rigs)

	// Output
	if statusJSON {
		return outputStatusJSON(status)
	}
	return outputStatusText(status)
}

func outputStatusJSON(status TownStatus) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}

func outputStatusText(status TownStatus) error {
	// Header
	fmt.Printf("%s %s\n", style.Bold.Render("Town:"), status.Name)
	fmt.Printf("%s\n\n", style.Dim.Render(status.Location))

	// Tree characters
	const (
		treeBranch = "├── "
		treeLast   = "└── "
		treeVert   = "│   "
		treeSpace  = "    "
	)

	// Role icons
	roleIcons := map[string]string{
		"mayor":    "🎩",
		"deacon":   "🔔",
		"witness":  "👁",
		"refinery": "🏭",
		"crew":     "👷",
		"polecat":  "😺",
	}

	// Global Agents (Mayor, Deacon) - these are town-level roles
	hasRigs := len(status.Rigs) > 0
	for i, agent := range status.Agents {
		isLast := i == len(status.Agents)-1 && !hasRigs
		prefix := treeBranch
		if isLast {
			prefix = treeLast
		}

		icon := roleIcons[agent.Role]
		if icon == "" {
			icon = roleIcons[agent.Name] // fallback to name
		}

		roleLabel := style.Bold.Render(fmt.Sprintf("%s %s", icon, capitalizeFirst(agent.Name)))
		fmt.Printf("%s%s\n", prefix, roleLabel)

		// Show agent instance under role
		childPrefix := treeVert
		if isLast {
			childPrefix = treeSpace
		}

		statusStr := style.Success.Render("running")
		if !agent.Running {
			statusStr = style.Error.Render("stopped")
		}

		hookInfo := formatHookInfo(agent.HookBead, agent.WorkTitle, 35)
		stateInfo := ""
		if agent.State != "" && agent.State != "idle" {
			stateInfo = style.Dim.Render(fmt.Sprintf(" [%s]", agent.State))
		}

		fmt.Printf("%s%s%s %s%s%s\n", childPrefix, treeLast,
			style.Dim.Render("gt-"+agent.Name), statusStr, hookInfo, stateInfo)
	}

	if !hasRigs {
		fmt.Printf("\n%s\n", style.Dim.Render("No rigs registered. Use 'gt rig add' to add one."))
		return nil
	}

	// Rigs section
	fmt.Printf("%s%s\n", treeLast, style.Bold.Render("Rigs"))

	for ri, r := range status.Rigs {
		isLastRig := ri == len(status.Rigs)-1
		rigPrefix := treeVert
		if isLastRig {
			rigPrefix = treeSpace
		}

		rigBranch := treeBranch
		if isLastRig {
			rigBranch = treeLast
		}

		fmt.Printf("%s%s%s\n", treeSpace, rigBranch, style.Bold.Render(r.Name+"/"))

		// Group agents by role
		var witnesses, refineries, crews, polecats []AgentRuntime
		for _, agent := range r.Agents {
			switch agent.Role {
			case "witness":
				witnesses = append(witnesses, agent)
			case "refinery":
				refineries = append(refineries, agent)
			case "crew":
				crews = append(crews, agent)
			case "polecat":
				polecats = append(polecats, agent)
			}
		}

		// Count non-empty role groups
		roleGroups := 0
		if len(witnesses) > 0 {
			roleGroups++
		}
		if len(refineries) > 0 {
			roleGroups++
		}
		if len(crews) > 0 {
			roleGroups++
		}
		if len(polecats) > 0 {
			roleGroups++
		}

		groupsRendered := 0
		baseIndent := treeSpace + rigPrefix

		// Witness
		if len(witnesses) > 0 {
			groupsRendered++
			isLastGroup := groupsRendered == roleGroups
			groupBranch := treeBranch
			if isLastGroup {
				groupBranch = treeLast
			}
			fmt.Printf("%s%s%s %s\n", baseIndent, groupBranch,
				roleIcons["witness"], style.Bold.Render("Witness"))

			groupIndent := baseIndent + treeVert
			if isLastGroup {
				groupIndent = baseIndent + treeSpace
			}
			renderAgentList(witnesses, groupIndent, r.Hooks)
		}

		// Refinery
		if len(refineries) > 0 {
			groupsRendered++
			isLastGroup := groupsRendered == roleGroups
			groupBranch := treeBranch
			if isLastGroup {
				groupBranch = treeLast
			}
			fmt.Printf("%s%s%s %s\n", baseIndent, groupBranch,
				roleIcons["refinery"], style.Bold.Render("Refinery"))

			groupIndent := baseIndent + treeVert
			if isLastGroup {
				groupIndent = baseIndent + treeSpace
			}
			renderAgentList(refineries, groupIndent, r.Hooks)
		}

		// Crew
		if len(crews) > 0 {
			groupsRendered++
			isLastGroup := groupsRendered == roleGroups
			groupBranch := treeBranch
			if isLastGroup {
				groupBranch = treeLast
			}
			fmt.Printf("%s%s%s %s\n", baseIndent, groupBranch,
				roleIcons["crew"], style.Bold.Render("Crew"))

			groupIndent := baseIndent + treeVert
			if isLastGroup {
				groupIndent = baseIndent + treeSpace
			}
			renderAgentList(crews, groupIndent, r.Hooks)
		}

		// Polecats
		if len(polecats) > 0 {
			groupsRendered++
			isLastGroup := groupsRendered == roleGroups
			groupBranch := treeBranch
			if isLastGroup {
				groupBranch = treeLast
			}
			fmt.Printf("%s%s%s %s\n", baseIndent, groupBranch,
				roleIcons["polecat"], style.Bold.Render("Polecats"))

			groupIndent := baseIndent + treeVert
			if isLastGroup {
				groupIndent = baseIndent + treeSpace
			}
			renderAgentList(polecats, groupIndent, r.Hooks)
		}

		// No agents at all
		if roleGroups == 0 {
			fmt.Printf("%s%s%s\n", baseIndent, treeLast, style.Dim.Render("(no agents)"))
		}
	}

	return nil
}

// renderAgentList renders a list of agents under a role group
func renderAgentList(agents []AgentRuntime, indent string, hooks []AgentHookInfo) {
	const (
		treeBranch = "├── "
		treeLast   = "└── "
	)

	for i, agent := range agents {
		isLast := i == len(agents)-1
		branch := treeBranch
		if isLast {
			branch = treeLast
		}

		statusStr := style.Success.Render("running")
		if !agent.Running {
			statusStr = style.Error.Render("stopped")
		}

		hookInfo := formatHookInfo(agent.HookBead, agent.WorkTitle, 30)
		if hookInfo == "" {
			// Fall back to legacy Hooks array
			for _, h := range hooks {
				if h.Agent == agent.Address && h.HasWork {
					if h.Molecule != "" {
						hookInfo = fmt.Sprintf(" → %s", h.Molecule)
					} else if h.Title != "" {
						hookInfo = fmt.Sprintf(" → %s", truncateWithEllipsis(h.Title, 30))
					}
					break
				}
			}
		}

		stateInfo := ""
		if agent.State != "" && agent.State != "idle" {
			stateInfo = style.Dim.Render(fmt.Sprintf(" [%s]", agent.State))
		}

		fmt.Printf("%s%s%s %s%s%s\n", indent, branch, agent.Name, statusStr, hookInfo, stateInfo)
	}
}

// formatHookInfo formats the hook bead and title for display
func formatHookInfo(hookBead, title string, maxLen int) string {
	if hookBead == "" {
		return ""
	}
	if title == "" {
		return fmt.Sprintf(" → %s", hookBead)
	}
	title = truncateWithEllipsis(title, maxLen)
	return fmt.Sprintf(" → %s", title)
}

// truncateWithEllipsis shortens a string to maxLen, adding "..." if truncated
func truncateWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// discoverRigHooks finds all hook attachments for agents in a rig.
// It scans polecats, crew workers, witness, and refinery for handoff beads.
func discoverRigHooks(r *rig.Rig, crews []string) []AgentHookInfo {
	var hooks []AgentHookInfo

	// Create beads instance for the rig
	b := beads.New(r.Path)

	// Check polecats
	for _, name := range r.Polecats {
		hook := getAgentHook(b, name, r.Name+"/"+name, "polecat")
		hooks = append(hooks, hook)
	}

	// Check crew workers
	for _, name := range crews {
		hook := getAgentHook(b, name, r.Name+"/crew/"+name, "crew")
		hooks = append(hooks, hook)
	}

	// Check witness
	if r.HasWitness {
		hook := getAgentHook(b, "witness", r.Name+"/witness", "witness")
		hooks = append(hooks, hook)
	}

	// Check refinery
	if r.HasRefinery {
		hook := getAgentHook(b, "refinery", r.Name+"/refinery", "refinery")
		hooks = append(hooks, hook)
	}

	return hooks
}

// discoverGlobalAgents checks runtime state for town-level agents (Mayor, Deacon).
func discoverGlobalAgents(t *tmux.Tmux, agentBeads *beads.Beads) []AgentRuntime {
	var agents []AgentRuntime

	// Check Mayor
	mayorRunning, _ := t.HasSession(MayorSessionName)
	mayor := AgentRuntime{
		Name:    "mayor",
		Address: "mayor",
		Session: MayorSessionName,
		Role:    "coordinator",
		Running: mayorRunning,
	}
	// Look up agent bead for hook/state
	if issue, fields, err := agentBeads.GetAgentBead("gt-mayor"); err == nil && issue != nil {
		mayor.HookBead = fields.HookBead
		mayor.State = fields.AgentState
		if fields.HookBead != "" {
			mayor.HasWork = true
			// Try to get the title of the pinned bead
			if pinnedIssue, err := agentBeads.Show(fields.HookBead); err == nil {
				mayor.WorkTitle = pinnedIssue.Title
			}
		}
	}
	agents = append(agents, mayor)

	// Check Deacon
	deaconRunning, _ := t.HasSession(DeaconSessionName)
	deacon := AgentRuntime{
		Name:    "deacon",
		Address: "deacon",
		Session: DeaconSessionName,
		Role:    "health-check",
		Running: deaconRunning,
	}
	// Look up agent bead for hook/state
	if issue, fields, err := agentBeads.GetAgentBead("gt-deacon"); err == nil && issue != nil {
		deacon.HookBead = fields.HookBead
		deacon.State = fields.AgentState
		if fields.HookBead != "" {
			deacon.HasWork = true
			if pinnedIssue, err := agentBeads.Show(fields.HookBead); err == nil {
				deacon.WorkTitle = pinnedIssue.Title
			}
		}
	}
	agents = append(agents, deacon)

	return agents
}

// discoverRigAgents checks runtime state for all agents in a rig.
func discoverRigAgents(t *tmux.Tmux, r *rig.Rig, crews []string, agentBeads *beads.Beads) []AgentRuntime {
	var agents []AgentRuntime

	// Check Witness
	if r.HasWitness {
		sessionName := witnessSessionName(r.Name)
		running, _ := t.HasSession(sessionName)
		witness := AgentRuntime{
			Name:    "witness",
			Address: r.Name + "/witness",
			Session: sessionName,
			Role:    "witness",
			Running: running,
		}
		// Look up agent bead
		agentID := fmt.Sprintf("gt-witness-%s", r.Name)
		if issue, fields, err := agentBeads.GetAgentBead(agentID); err == nil && issue != nil {
			witness.HookBead = fields.HookBead
			witness.State = fields.AgentState
			if fields.HookBead != "" {
				witness.HasWork = true
				if pinnedIssue, err := agentBeads.Show(fields.HookBead); err == nil {
					witness.WorkTitle = pinnedIssue.Title
				}
			}
		}
		agents = append(agents, witness)
	}

	// Check Refinery
	if r.HasRefinery {
		sessionName := fmt.Sprintf("gt-%s-refinery", r.Name)
		running, _ := t.HasSession(sessionName)
		refinery := AgentRuntime{
			Name:    "refinery",
			Address: r.Name + "/refinery",
			Session: sessionName,
			Role:    "refinery",
			Running: running,
		}
		// Look up agent bead
		agentID := fmt.Sprintf("gt-refinery-%s", r.Name)
		if issue, fields, err := agentBeads.GetAgentBead(agentID); err == nil && issue != nil {
			refinery.HookBead = fields.HookBead
			refinery.State = fields.AgentState
			if fields.HookBead != "" {
				refinery.HasWork = true
				if pinnedIssue, err := agentBeads.Show(fields.HookBead); err == nil {
					refinery.WorkTitle = pinnedIssue.Title
				}
			}
		}
		agents = append(agents, refinery)
	}

	// Check Polecats
	for _, name := range r.Polecats {
		sessionName := fmt.Sprintf("gt-%s-%s", r.Name, name)
		running, _ := t.HasSession(sessionName)
		polecat := AgentRuntime{
			Name:    name,
			Address: r.Name + "/" + name,
			Session: sessionName,
			Role:    "polecat",
			Running: running,
		}
		// Look up agent bead
		agentID := fmt.Sprintf("gt-polecat-%s-%s", r.Name, name)
		if issue, fields, err := agentBeads.GetAgentBead(agentID); err == nil && issue != nil {
			polecat.HookBead = fields.HookBead
			polecat.State = fields.AgentState
			if fields.HookBead != "" {
				polecat.HasWork = true
				if pinnedIssue, err := agentBeads.Show(fields.HookBead); err == nil {
					polecat.WorkTitle = pinnedIssue.Title
				}
			}
		}
		agents = append(agents, polecat)
	}

	// Check Crew
	for _, name := range crews {
		sessionName := crewSessionName(r.Name, name)
		running, _ := t.HasSession(sessionName)
		crewAgent := AgentRuntime{
			Name:    name,
			Address: r.Name + "/crew/" + name,
			Session: sessionName,
			Role:    "crew",
			Running: running,
		}
		// Look up agent bead
		agentID := fmt.Sprintf("gt-crew-%s-%s", r.Name, name)
		if issue, fields, err := agentBeads.GetAgentBead(agentID); err == nil && issue != nil {
			crewAgent.HookBead = fields.HookBead
			crewAgent.State = fields.AgentState
			if fields.HookBead != "" {
				crewAgent.HasWork = true
				if pinnedIssue, err := agentBeads.Show(fields.HookBead); err == nil {
					crewAgent.WorkTitle = pinnedIssue.Title
				}
			}
		}
		agents = append(agents, crewAgent)
	}

	return agents
}

// getAgentHook retrieves hook status for a specific agent.
func getAgentHook(b *beads.Beads, role, agentAddress, roleType string) AgentHookInfo {
	hook := AgentHookInfo{
		Agent: agentAddress,
		Role:  roleType,
	}

	// Find handoff bead for this role
	handoff, err := b.FindHandoffBead(role)
	if err != nil || handoff == nil {
		return hook
	}

	// Check for attachment
	attachment := beads.ParseAttachmentFields(handoff)
	if attachment != nil && attachment.AttachedMolecule != "" {
		hook.HasWork = true
		hook.Molecule = attachment.AttachedMolecule
		hook.Title = handoff.Title
	} else if handoff.Description != "" {
		// Has content but no molecule - still has work
		hook.HasWork = true
		hook.Title = handoff.Title
	}

	return hook
}
