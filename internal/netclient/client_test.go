package netclient

import (
	"context"
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

func TestDoWithRetry(t *testing.T) {
	noRetrySleep(t)
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		if atomic.LoadInt32(&attempts) <= 2 {
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
