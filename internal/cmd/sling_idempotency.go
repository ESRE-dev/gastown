package cmd

import "strings"

// normalizeAgentID trims surrounding whitespace and trailing slash for comparison.
func normalizeAgentID(v string) string {
	return strings.TrimSuffix(strings.TrimSpace(v), "/")
}

// matchesSlingTarget returns true when target should be treated as equivalent
// to the existing assignee for idempotent sling behavior.
func matchesSlingTarget(target, assignee, selfAgent string) bool {
	assigneeNorm := normalizeAgentID(assignee)
	if assigneeNorm == "" {
		return false
	}

	target = strings.TrimSpace(target)
	if target == "" || target == "." {
		selfNorm := normalizeAgentID(selfAgent)
		return selfNorm != "" && selfNorm == assigneeNorm
	}

	targetNorm := normalizeAgentID(target)
	if targetNorm == assigneeNorm {
		return true
	}

	parts := strings.Split(targetNorm, "/")

	// Dog pool target (deacon/dogs) is equivalent to any specific dog assignee.
	if targetNorm == "deacon/dogs" && strings.HasPrefix(assigneeNorm, "deacon/dogs/") {
		return true
	}

	// Rig-only target maps to polecat dispatch within that rig.
	if len(parts) == 1 && strings.HasPrefix(assigneeNorm, targetNorm+"/polecats/") {
		return true
	}

	// Two-segment shorthand targets like rig/name can resolve to polecat or crew.
	if len(parts) == 2 {
		rig := parts[0]
		nameOrRole := parts[1]
		if nameOrRole != "" &&
			nameOrRole != "polecats" &&
			nameOrRole != "crew" &&
			nameOrRole != "witness" &&
			nameOrRole != "refinery" {
			return assigneeNorm == rig+"/polecats/"+nameOrRole ||
				assigneeNorm == rig+"/crew/"+nameOrRole
		}
	}

	return false
}
