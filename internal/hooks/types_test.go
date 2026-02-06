package hooks

import (
	"errors"
	"testing"
	"time"
)

func TestEventTypes(t *testing.T) {
	expectedEvents := []EventType{
		EventPreSessionStart,
		EventPostSessionStart,
		EventPreShutdown,
		EventPostShutdown,
		EventOnPaneOutput,
		EventSessionIdle,
		EventMailReceived,
		EventWorkAssigned,
	}

	if len(AllEventTypes) != len(expectedEvents) {
		t.Errorf("expected %d event types, got %d", len(expectedEvents), len(AllEventTypes))
	}

	// Verify each expected event is in AllEventTypes
	eventMap := make(map[EventType]bool)
	for _, e := range AllEventTypes {
		eventMap[e] = true
	}

	for _, expected := range expectedEvents {
		if !eventMap[expected] {
			t.Errorf("expected event type %q not found in AllEventTypes", expected)
		}
	}
}

func TestSuccess(t *testing.T) {
	duration := 100 * time.Millisecond
	result := Success("test message", duration)

	if result.Block {
		t.Error("Success should not block")
	}
	if result.Message != "test message" {
		t.Errorf("expected message 'test message', got %q", result.Message)
	}
	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
	if result.Duration != duration {
		t.Errorf("expected duration %v, got %v", duration, result.Duration)
	}
}

func TestFailure(t *testing.T) {
	duration := 50 * time.Millisecond
	err := errors.New("test error")
	result := Failure(err, duration)

	if result.Block {
		t.Error("Failure should not block by default")
	}
	if result.Err != err {
		t.Errorf("expected error %v, got %v", err, result.Err)
	}
	if result.Message != "test error" {
		t.Errorf("expected message 'test error', got %q", result.Message)
	}
	if result.Duration != duration {
		t.Errorf("expected duration %v, got %v", duration, result.Duration)
	}
}

func TestBlockOperation(t *testing.T) {
	duration := 75 * time.Millisecond
	result := BlockOperation("blocking message", duration)

	if !result.Block {
		t.Error("BlockOperation should set Block to true")
	}
	if result.Message != "blocking message" {
		t.Errorf("expected message 'blocking message', got %q", result.Message)
	}
	if result.Duration != duration {
		t.Errorf("expected duration %v, got %v", duration, result.Duration)
	}
}

func TestHookConfigJSON(t *testing.T) {
	cfg := HookConfig{
		Type:    HookTypeCommand,
		Cmd:     "./test.sh",
		Timeout: 30,
	}

	// This test verifies the struct can be used with JSON marshaling
	// (actual marshaling is tested implicitly through runner tests)
	if cfg.Type != HookTypeCommand {
		t.Errorf("expected type %q, got %q", HookTypeCommand, cfg.Type)
	}
	if cfg.Cmd != "./test.sh" {
		t.Errorf("expected cmd './test.sh', got %q", cfg.Cmd)
	}
	if cfg.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", cfg.Timeout)
	}
}
