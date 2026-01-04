package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/activity"
)

// Test error for simulating fetch failures
var errFetchFailed = errors.New("fetch failed")

// MockConvoyFetcher is a mock implementation for testing.
type MockConvoyFetcher struct {
	Convoys    []ConvoyRow
	MergeQueue []MergeQueueRow
	Polecats   []PolecatRow
	Error      error
}

func (m *MockConvoyFetcher) FetchConvoys() ([]ConvoyRow, error) {
	return m.Convoys, m.Error
}

func (m *MockConvoyFetcher) FetchMergeQueue() ([]MergeQueueRow, error) {
	return m.MergeQueue, nil
}

func (m *MockConvoyFetcher) FetchPolecats() ([]PolecatRow, error) {
	return m.Polecats, nil
}

func TestConvoyHandler_RendersTemplate(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{
			{
				ID:           "hq-cv-abc",
				Title:        "Test Convoy",
				Status:       "open",
				Progress:     "2/5",
				Completed:    2,
				Total:        5,
				LastActivity: activity.Calculate(time.Now().Add(-1 * time.Minute)),
			},
		},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Check convoy data is rendered
	if !strings.Contains(body, "hq-cv-abc") {
		t.Error("Response should contain convoy ID")
	}
	if !strings.Contains(body, "Test Convoy") {
		t.Error("Response should contain convoy title")
	}
	if !strings.Contains(body, "2/5") {
		t.Error("Response should contain progress")
	}
}

func TestConvoyHandler_LastActivityColors(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		wantClass string
	}{
		{"green for active", 30 * time.Second, "activity-green"},
		{"yellow for stale", 3 * time.Minute, "activity-yellow"},
		{"red for stuck", 10 * time.Minute, "activity-red"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockConvoyFetcher{
				Convoys: []ConvoyRow{
					{
						ID:           "hq-cv-test",
						Title:        "Test",
						Status:       "open",
						LastActivity: activity.Calculate(time.Now().Add(-tt.age)),
					},
				},
			}

			handler, err := NewConvoyHandler(mock)
			if err != nil {
				t.Fatalf("NewConvoyHandler() error = %v", err)
			}

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			body := w.Body.String()
			if !strings.Contains(body, tt.wantClass) {
				t.Errorf("Response should contain %q", tt.wantClass)
			}
		})
	}
}

func TestConvoyHandler_EmptyConvoys(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "No convoys") {
		t.Error("Response should show empty state message")
	}
}

func TestConvoyHandler_ContentType(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", contentType)
	}
}

func TestConvoyHandler_MultipleConvoys(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{
			{ID: "hq-cv-1", Title: "First Convoy", Status: "open"},
			{ID: "hq-cv-2", Title: "Second Convoy", Status: "closed"},
			{ID: "hq-cv-3", Title: "Third Convoy", Status: "open"},
		},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Check all convoys are rendered
	for _, id := range []string{"hq-cv-1", "hq-cv-2", "hq-cv-3"} {
		if !strings.Contains(body, id) {
			t.Errorf("Response should contain convoy %s", id)
		}
	}
}

// Integration tests for error handling

func TestConvoyHandler_FetchConvoysError(t *testing.T) {
	mock := &MockConvoyFetcher{
		Error: errFetchFailed,
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Failed to fetch convoys") {
		t.Error("Response should contain error message")
	}
}

// Integration tests for merge queue rendering

func TestConvoyHandler_MergeQueueRendering(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{},
		MergeQueue: []MergeQueueRow{
			{
				Number:     123,
				Repo:       "roxas",
				Title:      "Fix authentication bug",
				URL:        "https://github.com/test/repo/pull/123",
				CIStatus:   "pass",
				Mergeable:  "ready",
				ColorClass: "mq-green",
			},
			{
				Number:     456,
				Repo:       "gastown",
				Title:      "Add dashboard feature",
				URL:        "https://github.com/test/repo/pull/456",
				CIStatus:   "pending",
				Mergeable:  "pending",
				ColorClass: "mq-yellow",
			},
		},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Check merge queue section header
	if !strings.Contains(body, "Refinery Merge Queue") {
		t.Error("Response should contain merge queue section header")
	}

	// Check PR numbers are rendered
	if !strings.Contains(body, "#123") {
		t.Error("Response should contain PR #123")
	}
	if !strings.Contains(body, "#456") {
		t.Error("Response should contain PR #456")
	}

	// Check repo names
	if !strings.Contains(body, "roxas") {
		t.Error("Response should contain repo 'roxas'")
	}

	// Check CI status badges
	if !strings.Contains(body, "ci-pass") {
		t.Error("Response should contain ci-pass class for passing PR")
	}
	if !strings.Contains(body, "ci-pending") {
		t.Error("Response should contain ci-pending class for pending PR")
	}
}

func TestConvoyHandler_EmptyMergeQueue(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys:    []ConvoyRow{},
		MergeQueue: []MergeQueueRow{},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should show empty state for merge queue
	if !strings.Contains(body, "No PRs in queue") {
		t.Error("Response should show empty merge queue message")
	}
}

// Integration tests for polecat workers rendering

func TestConvoyHandler_PolecatWorkersRendering(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{},
		Polecats: []PolecatRow{
			{
				Name:         "dag",
				Rig:          "roxas",
				SessionID:    "gt-roxas-dag",
				LastActivity: activity.Calculate(time.Now().Add(-30 * time.Second)),
				StatusHint:   "Running tests...",
			},
			{
				Name:         "nux",
				Rig:          "roxas",
				SessionID:    "gt-roxas-nux",
				LastActivity: activity.Calculate(time.Now().Add(-5 * time.Minute)),
				StatusHint:   "Waiting for input",
			},
		},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Check polecat section header
	if !strings.Contains(body, "Polecat Workers") {
		t.Error("Response should contain polecat workers section header")
	}

	// Check polecat names
	if !strings.Contains(body, "dag") {
		t.Error("Response should contain polecat 'dag'")
	}
	if !strings.Contains(body, "nux") {
		t.Error("Response should contain polecat 'nux'")
	}

	// Check rig names
	if !strings.Contains(body, "roxas") {
		t.Error("Response should contain rig 'roxas'")
	}

	// Check status hints
	if !strings.Contains(body, "Running tests...") {
		t.Error("Response should contain status hint")
	}

	// Check activity colors (dag should be green, nux should be yellow/red)
	if !strings.Contains(body, "activity-green") {
		t.Error("Response should contain activity-green for recent activity")
	}
}

// Integration tests for work status rendering

func TestConvoyHandler_WorkStatusRendering(t *testing.T) {
	tests := []struct {
		name           string
		workStatus     string
		wantClass      string
		wantStatusText string
	}{
		{"complete status", "complete", "work-complete", "complete"},
		{"active status", "active", "work-active", "active"},
		{"stale status", "stale", "work-stale", "stale"},
		{"stuck status", "stuck", "work-stuck", "stuck"},
		{"waiting status", "waiting", "work-waiting", "waiting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockConvoyFetcher{
				Convoys: []ConvoyRow{
					{
						ID:           "hq-cv-test",
						Title:        "Test Convoy",
						Status:       "open",
						WorkStatus:   tt.workStatus,
						Progress:     "1/2",
						Completed:    1,
						Total:        2,
						LastActivity: activity.Calculate(time.Now()),
					},
				},
			}

			handler, err := NewConvoyHandler(mock)
			if err != nil {
				t.Fatalf("NewConvoyHandler() error = %v", err)
			}

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			body := w.Body.String()

			// Check work status class is applied
			if !strings.Contains(body, tt.wantClass) {
				t.Errorf("Response should contain class %q for work status %q", tt.wantClass, tt.workStatus)
			}

			// Check work status text is displayed
			if !strings.Contains(body, tt.wantStatusText) {
				t.Errorf("Response should contain status text %q", tt.wantStatusText)
			}
		})
	}
}

// Integration tests for progress bar rendering

func TestConvoyHandler_ProgressBarRendering(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{
			{
				ID:           "hq-cv-progress",
				Title:        "Progress Test",
				Status:       "open",
				WorkStatus:   "active",
				Progress:     "3/4",
				Completed:    3,
				Total:        4,
				LastActivity: activity.Calculate(time.Now()),
			},
		},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Check progress text
	if !strings.Contains(body, "3/4") {
		t.Error("Response should contain progress '3/4'")
	}

	// Check progress bar element
	if !strings.Contains(body, "progress-bar") {
		t.Error("Response should contain progress-bar class")
	}

	// Check progress fill with percentage (75%)
	if !strings.Contains(body, "progress-fill") {
		t.Error("Response should contain progress-fill class")
	}
	if !strings.Contains(body, "width: 75%") {
		t.Error("Response should contain 75% width for 3/4 progress")
	}
}

// Integration test for HTMX auto-refresh

func TestConvoyHandler_HTMXAutoRefresh(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Check htmx attributes for auto-refresh
	if !strings.Contains(body, "hx-get") {
		t.Error("Response should contain hx-get attribute for HTMX")
	}
	if !strings.Contains(body, "hx-trigger") {
		t.Error("Response should contain hx-trigger attribute for HTMX")
	}
	if !strings.Contains(body, "every 10s") {
		t.Error("Response should contain 'every 10s' trigger interval")
	}
}

// Integration test for full dashboard with all sections

func TestConvoyHandler_FullDashboard(t *testing.T) {
	mock := &MockConvoyFetcher{
		Convoys: []ConvoyRow{
			{
				ID:           "hq-cv-full",
				Title:        "Full Test Convoy",
				Status:       "open",
				WorkStatus:   "active",
				Progress:     "2/3",
				Completed:    2,
				Total:        3,
				LastActivity: activity.Calculate(time.Now().Add(-1 * time.Minute)),
			},
		},
		MergeQueue: []MergeQueueRow{
			{
				Number:     789,
				Repo:       "testrig",
				Title:      "Test PR",
				CIStatus:   "pass",
				Mergeable:  "ready",
				ColorClass: "mq-green",
			},
		},
		Polecats: []PolecatRow{
			{
				Name:         "worker1",
				Rig:          "testrig",
				SessionID:    "gt-testrig-worker1",
				LastActivity: activity.Calculate(time.Now()),
				StatusHint:   "Working...",
			},
		},
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Verify all three sections are present
	if !strings.Contains(body, "Gas Town Convoys") {
		t.Error("Response should contain main header")
	}
	if !strings.Contains(body, "hq-cv-full") {
		t.Error("Response should contain convoy data")
	}
	if !strings.Contains(body, "Refinery Merge Queue") {
		t.Error("Response should contain merge queue section")
	}
	if !strings.Contains(body, "#789") {
		t.Error("Response should contain PR data")
	}
	if !strings.Contains(body, "Polecat Workers") {
		t.Error("Response should contain polecat section")
	}
	if !strings.Contains(body, "worker1") {
		t.Error("Response should contain polecat data")
	}
}

// Test that merge queue and polecat errors are non-fatal

type MockConvoyFetcherWithErrors struct {
	Convoys          []ConvoyRow
	MergeQueueError  error
	PolecatsError    error
}

func (m *MockConvoyFetcherWithErrors) FetchConvoys() ([]ConvoyRow, error) {
	return m.Convoys, nil
}

func (m *MockConvoyFetcherWithErrors) FetchMergeQueue() ([]MergeQueueRow, error) {
	return nil, m.MergeQueueError
}

func (m *MockConvoyFetcherWithErrors) FetchPolecats() ([]PolecatRow, error) {
	return nil, m.PolecatsError
}

func TestConvoyHandler_NonFatalErrors(t *testing.T) {
	mock := &MockConvoyFetcherWithErrors{
		Convoys: []ConvoyRow{
			{ID: "hq-cv-test", Title: "Test", Status: "open", WorkStatus: "active"},
		},
		MergeQueueError: errFetchFailed,
		PolecatsError:   errFetchFailed,
	}

	handler, err := NewConvoyHandler(mock)
	if err != nil {
		t.Fatalf("NewConvoyHandler() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should still return OK even if merge queue and polecats fail
	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d (non-fatal errors should not fail request)", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Convoys should still render
	if !strings.Contains(body, "hq-cv-test") {
		t.Error("Response should contain convoy data even when other fetches fail")
	}
}
