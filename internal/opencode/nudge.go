package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// promptPart is a single part of the OpenCode prompt_async request body.
type promptPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// promptAsyncRequest is the body of POST /session/:id/prompt_async.
type promptAsyncRequest struct {
	Parts []promptPart `json:"parts"`
}

// NudgeViaHTTP sends a message to an OpenCode agent via the HTTP API.
// Uses POST /session/:sessionID/prompt_async which returns 204 immediately.
//
// Returns nil on success, or an error if the HTTP delivery fails (caller
// should fall back to tmux send-keys).
func NudgeViaHTTP(ctx context.Context, port int, sessionID, message string) error {
	if sessionID == "" {
		return fmt.Errorf("empty session ID")
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/session/%s/prompt_async", port, sessionID)

	body := promptAsyncRequest{
		Parts: []promptPart{{Type: "text", Text: message}},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling nudge body: %w", err)
	}

	// Short timeout — nudge delivery should be near-instant.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating nudge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nudge HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// prompt_async returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nudge HTTP returned status %d", resp.StatusCode)
	}

	return nil
}
