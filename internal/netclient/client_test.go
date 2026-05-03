package netclient

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDoWithRetry(t *testing.T) {
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

	// Override sleep for testing to make it fast
	// In a real scenario we might want to mock time, 
	// but for simplicity we'll just use a small delay if we could.
	// Since DoWithRetry uses time.Sleep directly, we'll just wait a bit.

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
