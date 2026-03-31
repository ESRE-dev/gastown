package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// mockTmuxEnv implements TmuxEnvGetter for testing.
type mockTmuxEnv struct {
	env map[string]string
}

func (m *mockTmuxEnv) GetEnvironment(session, key string) (string, error) {
	v, ok := m.env[session+"/"+key]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return v, nil
}

func TestAssignPort(t *testing.T) {
	tests := []struct {
		name string
		slot int
		want int
	}{
		{name: "slot_0_gets_base", slot: 0, want: 4096},
		{name: "slot_1_gets_base_plus_1", slot: 1, want: 4097},
		{name: "slot_5_gets_base_plus_5", slot: 5, want: 4101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssignPort(tt.slot)
			if got != tt.want {
				t.Errorf("AssignPort(%d) = %d, want %d", tt.slot, got, tt.want)
			}
		})
	}
}

func TestDiscoverPort(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		session   string
		wantPort  int
		wantFound bool
	}{
		{
			name:      "found_in_tmux_env",
			env:       map[string]string{"gt-rig-polecat-Toast/GT_OPENCODE_PORT": "4097"},
			session:   "gt-rig-polecat-Toast",
			wantPort:  4097,
			wantFound: true,
		},
		{
			name:      "not_found_returns_default",
			env:       map[string]string{},
			session:   "gt-rig-polecat-Toast",
			wantPort:  DefaultPort,
			wantFound: false,
		},
		{
			name:      "empty_value_returns_default",
			env:       map[string]string{"gt-rig-polecat-Toast/GT_OPENCODE_PORT": ""},
			session:   "gt-rig-polecat-Toast",
			wantPort:  DefaultPort,
			wantFound: false,
		},
		{
			name:      "invalid_value_returns_default",
			env:       map[string]string{"gt-rig-polecat-Toast/GT_OPENCODE_PORT": "not-a-number"},
			session:   "gt-rig-polecat-Toast",
			wantPort:  DefaultPort,
			wantFound: false,
		},
		{
			name:      "zero_port_returns_default",
			env:       map[string]string{"gt-rig-polecat-Toast/GT_OPENCODE_PORT": "0"},
			session:   "gt-rig-polecat-Toast",
			wantPort:  DefaultPort,
			wantFound: false,
		},
		{
			name:      "whitespace_trimmed",
			env:       map[string]string{"gt-rig-polecat-Toast/GT_OPENCODE_PORT": " 4098 "},
			session:   "gt-rig-polecat-Toast",
			wantPort:  4098,
			wantFound: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTmuxEnv{env: tt.env}
			port, found := DiscoverPort(mock, tt.session)
			if port != tt.wantPort {
				t.Errorf("DiscoverPort() port = %d, want %d", port, tt.wantPort)
			}
			if found != tt.wantFound {
				t.Errorf("DiscoverPort() found = %v, want %v", found, tt.wantFound)
			}
		})
	}
}

func TestIsOpenCodeSession(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		session string
		want    bool
	}{
		{
			name:    "opencode_agent",
			env:     map[string]string{"sess/GT_AGENT": "opencode"},
			session: "sess",
			want:    true,
		},
		{
			name:    "claude_agent",
			env:     map[string]string{"sess/GT_AGENT": "claude"},
			session: "sess",
			want:    false,
		},
		{
			name:    "no_agent_set",
			env:     map[string]string{},
			session: "sess",
			want:    false,
		},
		{
			name:    "empty_agent",
			env:     map[string]string{"sess/GT_AGENT": ""},
			session: "sess",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTmuxEnv{env: tt.env}
			got := IsOpenCodeSession(mock, tt.session)
			if got != tt.want {
				t.Errorf("IsOpenCodeSession() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInjectPortFlag(t *testing.T) {
	tests := []struct {
		name    string
		command string
		port    int
		want    string
	}{
		{
			name:    "before_prompt_flag",
			command: "exec env GT_RIG=gs opencode --prompt 'hello world'",
			port:    4097,
			want:    "exec env GT_RIG=gs opencode --port 4097 --prompt 'hello world'",
		},
		{
			name:    "no_prompt_flag",
			command: "exec env GT_RIG=gs opencode",
			port:    4097,
			want:    "exec env GT_RIG=gs opencode --port 4097",
		},
		{
			name:    "default_port",
			command: "opencode --prompt 'test'",
			port:    4096,
			want:    "opencode --port 4096 --prompt 'test'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InjectPortFlag(tt.command, tt.port)
			if got != tt.want {
				t.Errorf("InjectPortFlag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	t.Run("healthy_server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/global/health" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(HealthResponse{Healthy: true, Version: "1.0.0"})
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		err := HealthCheck(port)
		if err != nil {
			t.Errorf("HealthCheck() unexpected error: %v", err)
		}
	})

	t.Run("unhealthy_status_code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		err := HealthCheck(port)
		if err == nil {
			t.Error("HealthCheck() expected error for 503 response")
		}
	})

	t.Run("connection_refused", func(t *testing.T) {
		err := HealthCheck(49999) // unlikely to be in use
		if err == nil {
			t.Error("HealthCheck() expected error for connection refused")
		}
	})
}

func TestWaitForHealthy(t *testing.T) {
	t.Run("immediately_healthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(HealthResponse{Healthy: true, Version: "1.0.0"})
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := WaitForHealthy(ctx, port)
		if err != nil {
			t.Errorf("WaitForHealthy() unexpected error: %v", err)
		}
	})

	t.Run("timeout_on_unreachable", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := WaitForHealthy(ctx, 49999)
		if err == nil {
			t.Error("WaitForHealthy() expected timeout error")
		}
	})

	t.Run("becomes_healthy_after_delay", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(HealthResponse{Healthy: true, Version: "1.0.0"})
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := WaitForHealthy(ctx, port)
		if err != nil {
			t.Errorf("WaitForHealthy() unexpected error: %v", err)
		}
		if callCount < 3 {
			t.Errorf("expected at least 3 calls, got %d", callCount)
		}
	})
}

func TestGetSessionStatus(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		wantMap    map[string]StatusInfo
	}{
		{
			name:       "busy_session",
			response:   `{"abc-123": {"type": "busy"}}`,
			statusCode: http.StatusOK,
			wantMap:    map[string]StatusInfo{"abc-123": {Type: "busy"}},
		},
		{
			name:       "idle_session_present",
			response:   `{"abc-123": {"type": "idle"}}`,
			statusCode: http.StatusOK,
			wantMap:    map[string]StatusInfo{"abc-123": {Type: "idle"}},
		},
		{
			name:       "empty_map",
			response:   `{}`,
			statusCode: http.StatusOK,
			wantMap:    map[string]StatusInfo{},
		},
		{
			name:       "multiple_sessions",
			response:   `{"sess-1": {"type": "busy"}, "sess-2": {"type": "idle"}}`,
			statusCode: http.StatusOK,
			wantMap: map[string]StatusInfo{
				"sess-1": {Type: "busy"},
				"sess-2": {Type: "idle"},
			},
		},
		{
			name:       "retry_variant",
			response:   `{"sess-1": {"type": "retry", "attempt": 3, "message": "rate limited", "next": 1700000000}}`,
			statusCode: http.StatusOK,
			wantMap: map[string]StatusInfo{
				"sess-1": {Type: "retry", Attempt: 3, Message: "rate limited", Next: 1700000000},
			},
		},
		{
			name:       "non_200_status",
			response:   `{}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/session/status" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				fmt.Fprint(w, tt.response)
			}))
			defer srv.Close()

			port := extractPort(t, srv.URL)
			ctx := context.Background()
			got, err := GetSessionStatus(ctx, port)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetSessionStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.wantMap) {
				t.Fatalf("GetSessionStatus() returned %d entries, want %d", len(got), len(tt.wantMap))
			}
			for k, wantInfo := range tt.wantMap {
				gotInfo, ok := got[k]
				if !ok {
					t.Errorf("GetSessionStatus() missing key %q", k)
					continue
				}
				if gotInfo.Type != wantInfo.Type {
					t.Errorf("key %q: Type = %q, want %q", k, gotInfo.Type, wantInfo.Type)
				}
				if gotInfo.Attempt != wantInfo.Attempt {
					t.Errorf("key %q: Attempt = %d, want %d", k, gotInfo.Attempt, wantInfo.Attempt)
				}
				if gotInfo.Message != wantInfo.Message {
					t.Errorf("key %q: Message = %q, want %q", k, gotInfo.Message, wantInfo.Message)
				}
				if gotInfo.Next != wantInfo.Next {
					t.Errorf("key %q: Next = %d, want %d", k, gotInfo.Next, wantInfo.Next)
				}
			}
		})
	}
}

func TestStatusInfoMethods(t *testing.T) {
	tests := []struct {
		name     string
		info     StatusInfo
		wantIdle bool
		wantBusy bool
	}{
		{name: "idle", info: StatusInfo{Type: "idle"}, wantIdle: true, wantBusy: false},
		{name: "busy", info: StatusInfo{Type: "busy"}, wantIdle: false, wantBusy: true},
		{name: "retry", info: StatusInfo{Type: "retry", Attempt: 2}, wantIdle: false, wantBusy: true},
		{name: "unknown_type", info: StatusInfo{Type: "unknown"}, wantIdle: false, wantBusy: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.IsIdle(); got != tt.wantIdle {
				t.Errorf("IsIdle() = %v, want %v", got, tt.wantIdle)
			}
			if got := tt.info.IsBusy(); got != tt.wantBusy {
				t.Errorf("IsBusy() = %v, want %v", got, tt.wantBusy)
			}
		})
	}
}

func TestWaitForIdle(t *testing.T) {
	t.Run("immediately_idle_empty_map", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{}`)
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := WaitForIdle(ctx, port); err != nil {
			t.Fatalf("WaitForIdle() unexpected error: %v", err)
		}
	})

	t.Run("immediately_idle_session_absent", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"other-session": {"type": "idle"}}`)
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := WaitForIdle(ctx, port); err != nil {
			t.Fatalf("WaitForIdle() unexpected error: %v", err)
		}
	})

	t.Run("timeout_while_busy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"abc": {"type": "busy"}}`)
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := WaitForIdle(ctx, port)
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})

	t.Run("timeout_while_retry", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"abc": {"type": "retry", "attempt": 1, "message": "rate limit", "next": 1700000000}}`)
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := WaitForIdle(ctx, port)
		if err == nil {
			t.Fatal("expected timeout error for retry status")
		}
	})

	t.Run("becomes_idle_after_busy", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			if callCount <= 3 {
				fmt.Fprint(w, `{"abc": {"type": "busy"}}`)
			} else {
				fmt.Fprint(w, `{}`)
			}
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := WaitForIdle(ctx, port); err != nil {
			t.Fatalf("WaitForIdle() unexpected error: %v", err)
		}
		// Should have polled at least 3 (busy) + 2 (consecutive idle) = 5 times
		if callCount < 5 {
			t.Errorf("expected at least 5 calls, got %d", callCount)
		}
	})

	t.Run("multiple_sessions_one_busy", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			if callCount <= 2 {
				fmt.Fprint(w, `{"sess-1": {"type": "idle"}, "sess-2": {"type": "busy"}}`)
			} else {
				fmt.Fprint(w, `{"sess-1": {"type": "idle"}}`)
			}
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := WaitForIdle(ctx, port); err != nil {
			t.Fatalf("WaitForIdle() unexpected error: %v", err)
		}
		if callCount < 4 {
			t.Errorf("expected at least 4 calls, got %d", callCount)
		}
	})
}

// extractPort parses the port from a test server URL.
func extractPort(t *testing.T, rawURL string) int {
	t.Helper()
	// URL format: http://127.0.0.1:PORT
	parts := strings.Split(rawURL, ":")
	if len(parts) < 3 {
		t.Fatalf("unexpected URL format: %s", rawURL)
	}
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("parsing port from %s: %v", rawURL, err)
	}
	return port
}

func TestNudgeViaHTTP(t *testing.T) {
	t.Run("successful delivery", func(t *testing.T) {
		var receivedBody promptAsyncRequest
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/session/test-session/prompt_async") {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", ct)
			}
			if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
				t.Errorf("decoding request body: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		err := NudgeViaHTTP(context.Background(), port, "test-session", "Hello agent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(receivedBody.Parts) != 1 {
			t.Fatalf("expected 1 part, got %d", len(receivedBody.Parts))
		}
		if receivedBody.Parts[0].Type != "text" {
			t.Errorf("expected type 'text', got %q", receivedBody.Parts[0].Type)
		}
		if receivedBody.Parts[0].Text != "Hello agent" {
			t.Errorf("expected text 'Hello agent', got %q", receivedBody.Parts[0].Text)
		}
	})

	t.Run("empty session ID", func(t *testing.T) {
		err := NudgeViaHTTP(context.Background(), 4096, "", "message")
		if err == nil {
			t.Fatal("expected error for empty session ID")
		}
	})

	t.Run("server returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		err := NudgeViaHTTP(context.Background(), port, "test-session", "Hello")
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		err := NudgeViaHTTP(context.Background(), 1, "test-session", "Hello")
		if err == nil {
			t.Fatal("expected error for connection refused")
		}
	})
}
