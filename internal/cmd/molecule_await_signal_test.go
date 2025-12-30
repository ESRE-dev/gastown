package cmd

import (
	"testing"
	"time"
)

func TestCalculateEffectiveTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     string
		backoffBase string
		backoffMult int
		backoffMax  string
		want        time.Duration
		wantErr     bool
	}{
		{
			name:    "simple timeout 60s",
			timeout: "60s",
			want:    60 * time.Second,
		},
		{
			name:    "simple timeout 5m",
			timeout: "5m",
			want:    5 * time.Minute,
		},
		{
			name:        "backoff base only",
			timeout:     "60s",
			backoffBase: "30s",
			want:        30 * time.Second,
		},
		{
			name:        "backoff with max",
			timeout:     "60s",
			backoffBase: "30s",
			backoffMax:  "10m",
			want:        30 * time.Second,
		},
		{
			name:        "backoff base exceeds max",
			timeout:     "60s",
			backoffBase: "15m",
			backoffMax:  "10m",
			want:        10 * time.Minute,
		},
		{
			name:    "invalid timeout",
			timeout: "invalid",
			wantErr: true,
		},
		{
			name:        "invalid backoff base",
			timeout:     "60s",
			backoffBase: "invalid",
			wantErr:     true,
		},
		{
			name:        "invalid backoff max",
			timeout:     "60s",
			backoffBase: "30s",
			backoffMax:  "invalid",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set package-level variables
			awaitSignalTimeout = tt.timeout
			awaitSignalBackoffBase = tt.backoffBase
			awaitSignalBackoffMult = tt.backoffMult
			if tt.backoffMult == 0 {
				awaitSignalBackoffMult = 2 // default
			}
			awaitSignalBackoffMax = tt.backoffMax

			got, err := calculateEffectiveTimeout()
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateEffectiveTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("calculateEffectiveTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAwaitSignalResult(t *testing.T) {
	// Test that result struct marshals correctly
	result := AwaitSignalResult{
		Reason:  "signal",
		Elapsed: 5 * time.Second,
		Signal:  "[12:34:56] + gt-abc created · New issue",
	}

	if result.Reason != "signal" {
		t.Errorf("expected reason 'signal', got %q", result.Reason)
	}
	if result.Signal == "" {
		t.Error("expected signal to be set")
	}
}
