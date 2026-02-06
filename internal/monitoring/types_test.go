package monitoring

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAgentStatus_String(t *testing.T) {
	tests := []struct {
		status AgentStatus
		want   string
	}{
		{StatusAvailable, "available"},
		{StatusWorking, "working"},
		{StatusThinking, "thinking"},
		{StatusBlocked, "blocked"},
		{StatusWaiting, "waiting"},
		{StatusReviewing, "reviewing"},
		{StatusIdle, "idle"},
		{StatusPaused, "paused"},
		{StatusError, "error"},
		{StatusOffline, "offline"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("got %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestStatusSource_String(t *testing.T) {
	tests := []struct {
		source StatusSource
		want   string
	}{
		{SourceBoss, "boss"},
		{SourceSelf, "self"},
		{SourceInferred, "inferred"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.source) != tt.want {
				t.Errorf("got %q, want %q", tt.source, tt.want)
			}
		})
	}
}

func TestStatusReport_JSON(t *testing.T) {
	now := time.Now().UTC()
	report := StatusReport{
		AgentID:   "test-agent",
		Status:    StatusWorking,
		Source:    SourceSelf,
		Timestamp: now,
		Message:   "Processing task",
		Pattern:   "working_pattern",
	}

	// Marshal to JSON
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded StatusReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Compare fields
	if decoded.AgentID != report.AgentID {
		t.Errorf("AgentID: got %q, want %q", decoded.AgentID, report.AgentID)
	}
	if decoded.Status != report.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, report.Status)
	}
	if decoded.Source != report.Source {
		t.Errorf("Source: got %q, want %q", decoded.Source, report.Source)
	}
	if !decoded.Timestamp.Equal(report.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", decoded.Timestamp, report.Timestamp)
	}
	if decoded.Message != report.Message {
		t.Errorf("Message: got %q, want %q", decoded.Message, report.Message)
	}
	if decoded.Pattern != report.Pattern {
		t.Errorf("Pattern: got %q, want %q", decoded.Pattern, report.Pattern)
	}
}

func TestStatusReport_JSONOmitsEmptyFields(t *testing.T) {
	report := StatusReport{
		AgentID:   "test-agent",
		Status:    StatusAvailable,
		Source:    SourceBoss,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Check that omitempty fields are not present
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := raw["message"]; ok {
		t.Error("message field should be omitted when empty")
	}
	if _, ok := raw["pattern"]; ok {
		t.Error("pattern field should be omitted when empty")
	}
}
