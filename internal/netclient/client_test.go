package netclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// noRetrySleep swaps the backoff for a no-op so tests don't wait real
// seconds; restored via t.Cleanup.
func noRetrySleep(t *testing.T) {
	t.Helper()
	orig := retrySleep
	retrySleep = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { retrySleep = orig })
}

// recordingRetrySleep captures the requested waits instead of sleeping, so
// tests can assert which backoff schedule (exponential vs Retry-After) ran.
func recordingRetrySleep(t *testing.T) *[]time.Duration {
	t.Helper()
	orig := retrySleep
	waits := []time.Duration{}
	retrySleep = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	t.Cleanup(func() { retrySleep = orig })
	return &waits
}

func TestDoWithRetry(t *testing.T) {
	waits := recordingRetrySleep(t)
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		if atomic.LoadInt32(&attempts) <= 2 {
			// A Retry-After is the only 429 shape that retries.
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		MaxRetries: 3,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.DoWithRetry(req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
	if len(*waits) != 2 || (*waits)[0] != time.Second || (*waits)[1] != time.Second {
		t.Errorf("waits = %v, want each 429 to wait the Retry-After (1s)", *waits)
	}
}

// TestDoWithRetry429WithoutRetryAfterSurfacesImmediately pins the rate-limit
// policy: a 429 without a usable Retry-After must not be retried on a fixed
// backoff — the caller's own rate-limit handling needs the response now.
func TestDoWithRetry429WithoutRetryAfterSurfacesImmediately(t *testing.T) {
	noRetrySleep(t)
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), MaxRetries: 3}
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.DoWithRetry(req)

	if err != nil {
		t.Fatalf("Expected the 429 response, got error %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %v", resp.Status)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("attempts = %d, want exactly 1 (no blind retry)", got)
	}
}

func TestDoWithRetry_Fail(t *testing.T) {
	noRetrySleep(t)
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		MaxRetries: 2,
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.DoWithRetry(req)

	if err != nil {
		t.Fatalf("Expected no network error (HTTP 500 is a response), got %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %v", resp.Status)
	}
	if atomic.LoadInt32(&attempts) != 3 { // 1 original + 2 retries
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithRetryStopsOnCanceledContext(t *testing.T) {
	noRetrySleep(t)
	var attempts int32
	ctx, cancel := context.WithCancel(context.Background())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		cancel() // fail the context after the first attempt
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), MaxRetries: 5}
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	resp, err := client.DoWithRetry(req)

	if err == nil {
		t.Fatalf("Expected context error, got nil (resp=%v)", resp)
	}
	if resp != nil {
		t.Errorf("Expected no response after cancellation, got %v", resp.Status)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("Expected 1 attempt after cancellation, got %d", got)
	}
}

func TestDoWithRetryAbortsWhenBackoffCanceled(t *testing.T) {
	orig := retrySleep
	retrySleep = func(ctx context.Context, d time.Duration) error { return context.DeadlineExceeded }
	t.Cleanup(func() { retrySleep = orig })

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), MaxRetries: 5}
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.DoWithRetry(req)

	if err == nil {
		t.Fatalf("Expected cancellation error from backoff, got nil (resp=%v)", resp)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("Expected 1 attempt when backoff is canceled, got %d", got)
	}
}

// fakeNetError lets tests pin the retry policy per error flavor.
type fakeNetError struct{ timeout, temporary bool }

func (e fakeNetError) Error() string   { return "fake net error" }
func (e fakeNetError) Timeout() bool   { return e.timeout }
func (e fakeNetError) Temporary() bool { return e.temporary }

type errTransport struct {
	err      error
	attempts *int32
}

func (t errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	atomic.AddInt32(t.attempts, 1)
	return nil, t.err
}

// TestShouldRetryPolicy pins the transport-error retry semantics: timeouts
// retry; deprecated Temporary()-only errors and plain errors do not.
func TestShouldRetryPolicy(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantAttempts int32
	}{
		{name: "timeout error retried", err: fakeNetError{timeout: true}, wantAttempts: 4},
		{name: "temporary-only error not retried", err: fakeNetError{temporary: true}, wantAttempts: 1},
		{name: "plain error not retried", err: errors.New("boom"), wantAttempts: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noRetrySleep(t)
			var attempts int32
			client := &Client{
				HTTPClient: &http.Client{Transport: errTransport{err: tt.err, attempts: &attempts}},
				MaxRetries: 3,
			}
			req, _ := http.NewRequest("GET", "http://example.invalid", nil)
			resp, err := client.DoWithRetry(req)

			if err == nil {
				t.Fatalf("DoWithRetry() = %v, want the transport error", resp)
			}
			if got := atomic.LoadInt32(&attempts); got != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", got, tt.wantAttempts)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name     string
		resp     *http.Response
		err      error
		expected string
	}{
		{
			name:     "401 Unauthorized",
			resp:     &http.Response{StatusCode: http.StatusUnauthorized},
			expected: "Invalid API Token (HTTP 401): Please check your credentials using 'oct config'.",
		},
		{
			name:     "429 Too Many Requests",
			resp:     &http.Response{StatusCode: http.StatusTooManyRequests},
			expected: "Rate Limited (HTTP 429): Too many requests. Please wait a moment.",
		},
		{
			name:     "500 Internal Server Error",
			resp:     &http.Response{StatusCode: http.StatusInternalServerError},
			expected: "Server Error (HTTP 500): The provider's server is having issues.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.resp, tt.err)
			if got != tt.expected {
				t.Errorf("FormatError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTruncateBodyRuneSafe(t *testing.T) {
	got := truncateBody("가나다", 4)
	if got != "가..." {
		t.Fatalf("unexpected truncate result: %q", got)
	}
}

func TestDoWithRetryPostBodyReset(t *testing.T) {
	noRetrySleep(t)
	var bodies []string
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		if atomic.AddInt32(&attempts, 1) <= 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		MaxRetries: 3,
	}

	// A bytes.Reader body has no Seeker through req.Body; the retry must go
	// through req.GetBody instead of failing with "does not support seeking".
	req, _ := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader([]byte(`{"ok":true}`)))
	resp, err := client.DoWithRetry(req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
	if len(bodies) != 2 || bodies[0] != `{"ok":true}` || bodies[1] != `{"ok":true}` {
		t.Errorf("Bodies = %q, want the full body on both attempts", bodies)
	}
}

func TestRetryAfterDelay(t *testing.T) {
	httpDate := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	farFutureDate := time.Now().Add(2 * time.Hour).UTC().Format(http.TimeFormat)
	tests := []struct {
		name      string
		header    string
		want      time.Duration
		wantOK    bool
		withinCap bool
	}{
		{name: "delay seconds", header: "2", want: 2 * time.Second, wantOK: true},
		{name: "padded delay seconds", header: " 3 ", want: 3 * time.Second, wantOK: true},
		{name: "zero means no guidance", header: "0", wantOK: false},
		{name: "negative means no guidance", header: "-5", wantOK: false},
		{name: "unparseable means no guidance", header: "soon", wantOK: false},
		{name: "absent means no guidance", header: "", wantOK: false},
		{name: "beyond cap surfaces to caller", header: "3600", wantOK: false},
		{name: "http date within cap", header: httpDate, wantOK: true, withinCap: true},
		{name: "far future http date surfaces to caller", header: farFutureDate, wantOK: false},
		{name: "past http date means no guidance", header: "Mon, 02 Jan 2006 15:04:05 GMT", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tt.header != "" {
				resp.Header.Set("Retry-After", tt.header)
			}
			got, ok := retryAfterDelay(resp)
			if ok != tt.wantOK {
				t.Fatalf("retryAfterDelay(%q) ok = %t, want %t", tt.header, ok, tt.wantOK)
			}
			switch {
			case tt.wantOK && tt.withinCap:
				if got <= 0 || got > maxRetryAfterDelay {
					t.Errorf("retryAfterDelay(%q) = %v, want within (0, %v]", tt.header, got, maxRetryAfterDelay)
				}
			case tt.wantOK && got != tt.want:
				t.Errorf("retryAfterDelay(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}
