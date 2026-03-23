package opencode

import (
	"net/http"
	"time"
)

// httpClient is a dedicated client for OpenCode API calls.
// The 5s timeout acts as a safety net per-request, preventing
// indefinite hangs when the OpenCode server is unresponsive.
// Functions that already use http.NewRequestWithContext will be
// bounded by the shorter of the context deadline and this timeout.
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}
