package witness

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestBuildPatrolReceipt_StaleVerdictFromHookBead(t *testing.T) {
	receipt := BuildPatrolReceipt("gastown", ZombieResult{
		PolecatName: "atlas",
		AgentState:  "idle",
		HookBead:    "gt-abc123",
		Action:      "auto-nuked",
	})

	if receipt.Verdict != PatrolVerdictStale {
		t.Fatalf("Verdict = %q, want %q", receipt.Verdict, PatrolVerdictStale)
	}
	if receipt.RecommendedAction != "auto-nuked" {
		t.Fatalf("RecommendedAction = %q, want %q", receipt.RecommendedAction, "auto-nuked")
	}
}

func TestBuildPatrolReceipt_OrphanVerdictWithoutHookedWork(t *testing.T) {
	receipt := BuildPatrolReceipt("gastown", ZombieResult{
		PolecatName: "echo",
		AgentState:  "idle",
		Action:      "cleanup-wisp-created",
	})

	if receipt.Verdict != PatrolVerdictOrphan {
		t.Fatalf("Verdict = %q, want %q", receipt.Verdict, PatrolVerdictOrphan)
	}
}

func TestBuildPatrolReceipt_ErrorIncludedInEvidence(t *testing.T) {
	receipt := BuildPatrolReceipt("gastown", ZombieResult{
		PolecatName: "nux",
		AgentState:  "running",
		Error:       errors.New("nuke failed"),
	})

	if receipt.Evidence.Error != "nuke failed" {
		t.Fatalf("Evidence.Error = %q, want %q", receipt.Evidence.Error, "nuke failed")
	}
}

func TestBuildPatrolReceipts_JSONShape(t *testing.T) {
	receipts := BuildPatrolReceipts("gastown", &DetectZombiePolecatsResult{
		Zombies: []ZombieResult{
			{
				PolecatName: "atlas",
				AgentState:  "working",
				HookBead:    "gt-123",
				Action:      "auto-nuked",
			},
		},
	})
	if len(receipts) != 1 {
		t.Fatalf("len(receipts) = %d, want 1", len(receipts))
	}

	data, err := json.Marshal(receipts[0])
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded["verdict"] != string(PatrolVerdictStale) {
		t.Fatalf("decoded verdict = %v, want %q", decoded["verdict"], PatrolVerdictStale)
	}
	if decoded["recommended_action"] != "auto-nuked" {
		t.Fatalf("decoded recommended_action = %v, want %q", decoded["recommended_action"], "auto-nuked")
	}
	evidence, ok := decoded["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("decoded evidence missing or wrong type: %#v", decoded["evidence"])
	}
	if evidence["hook_bead"] != "gt-123" {
		t.Fatalf("decoded evidence.hook_bead = %v, want %q", evidence["hook_bead"], "gt-123")
	}
}
