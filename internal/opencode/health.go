package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthResponse is the response from OpenCode's GET /global/health endpoint.
type HealthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
}

// HealthCheck performs a single health check against an OpenCode HTTP server.
// Returns nil if the server is healthy, or an error describing the failure.
func HealthCheck(port int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return HealthCheckWithContext(ctx, port)
}

// HealthCheckWithContext performs a health check with the given context.
func HealthCheckWithContext(ctx context.Context, port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/global/health", port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating health request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("decoding health response: %w", err)
	}

	if !health.Healthy {
		return fmt.Errorf("server reports unhealthy")
	}

	return nil
}

// WaitForHealthy polls the health endpoint until the server is ready or the
// context is cancelled. This replaces the blunt ReadyDelayMs sleep with a
// responsive health check that typically completes in 2-3 seconds.
//
// Poll interval starts at 200ms with exponential backoff up to 1s.
func WaitForHealthy(ctx context.Context, port int) error {
	interval := 200 * time.Millisecond
	maxInterval := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for OpenCode health on port %d: %w", port, ctx.Err())
		default:
		}

		if err := HealthCheckWithContext(ctx, port); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for OpenCode health on port %d: %w", port, ctx.Err())
		case <-time.After(interval):
		}

		// Exponential backoff
		interval = interval * 3 / 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}
}

// SessionStatus represents the status of an OpenCode session.
type SessionStatus struct {
	SessionID string `json:"sessionID"`
	Status    struct {
		Type string `json:"type"` // "idle", "running", etc.
	} `json:"status"`
}

// GetSessionStatus queries the current session status via the HTTP API.
// Returns the status or an error if the request fails.
func GetSessionStatus(ctx context.Context, port int) (*SessionStatus, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/session/status", port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating status request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("session status request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("session status returned status %d", resp.StatusCode)
	}

	var status SessionStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decoding session status: %w", err)
	}

	return &status, nil
}

// WaitForIdle polls the session status endpoint until the agent reports idle
// or the context is cancelled. This replaces pane-buffer regex matching with
// structured status detection (gastown-p6k.2).
//
// Uses 2 consecutive idle polls (matching tmux.WaitForIdle's strategy) to
// filter transient idle states during inter-tool-call gaps.
//
// Poll interval is 200ms (matching tmux.WaitForIdle's polling rate).
func WaitForIdle(ctx context.Context, port int) error {
	const (
		pollInterval        = 200 * time.Millisecond
		requiredConsecutive = 2
	)
	consecutiveIdle := 0

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for OpenCode idle on port %d: %w", port, ctx.Err())
		default:
		}

		status, err := GetSessionStatus(ctx, port)
		if err != nil {
			consecutiveIdle = 0
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout waiting for OpenCode idle on port %d: %w", port, ctx.Err())
			case <-time.After(pollInterval):
			}
			continue
		}

		if status.Status.Type == "idle" {
			consecutiveIdle++
			if consecutiveIdle >= requiredConsecutive {
				return nil
			}
		} else {
			consecutiveIdle = 0
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for OpenCode idle on port %d: %w", port, ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}
