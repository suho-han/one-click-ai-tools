package netclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type Client struct {
	HTTPClient *http.Client
	MaxRetries int
}

var DefaultClient = &Client{
	HTTPClient: &http.Client{
		Timeout: 30 * time.Second,
	},
	MaxRetries: 3,
}

// retrySleep waits out the backoff but aborts on ctx cancellation so a
// canceled/deadline-exceeded request does not linger in a sleep.
var retrySleep = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) DoWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= c.MaxRetries; i++ {
		// Honor cancellation/deadline before starting another attempt.
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}

		// If it's a retry, we need to reset the request body if it exists
		if i > 0 && req.Body != nil {
			if seeker, ok := req.Body.(io.Seeker); ok {
				seeker.Seek(0, io.SeekStart)
			}
		}

		resp, err = c.HTTPClient.Do(req)

		if !c.shouldRetry(resp, err) || i == c.MaxRetries {
			return resp, err
		}

		// Close response body before retrying
		if resp != nil {
			resp.Body.Close()
		}

		// Exponential backoff: 1s, 2s, 4s...
		backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
		if sleepErr := retrySleep(req.Context(), backoff); sleepErr != nil {
			return nil, sleepErr
		}
	}

	return resp, err
}

func (c *Client) shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		// Retry timeouts only. net.Error.Temporary() is deprecated and
		// unreliable; every other transport error surfaces to the caller
		// (netclient's own 30s client timeout is a Timeout error, so the
		// slow-network case still retries).
		var netErr net.Error
		return errors.As(err, &netErr) && netErr.Timeout()
	}

	if resp != nil {
		switch resp.StatusCode {
		case http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		}
	}

	return false
}

// GetJSON GETs url with the supplied headers and decodes the 200 body into
// v. It applies the client's retry policy, honors ctx cancellation and the
// per-attempt timeout, and returns a descriptive error for any non-200
// response (body excerpt included).
func (c *Client) GetJSON(ctx context.Context, url string, headers map[string]string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateBody(string(body), 160))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}

func truncateBody(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	// Cut on a rune boundary so multi-byte text stays valid UTF-8.
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max] + "..."
}

func FormatError(resp *http.Response, err error) string {
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "Network timeout: Connection is unstable. Please check your internet."
		}
		return fmt.Sprintf("Network error: %v", err)
	}

	if resp == nil {
		return "Unknown network error"
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return "Invalid API Token (HTTP 401): Please check your credentials using 'oct config'."
	case http.StatusForbidden:
		return "Access Forbidden (HTTP 403): You don't have permission for this API. Check your plan."
	case http.StatusTooManyRequests:
		return "Rate Limited (HTTP 429): Too many requests. Please wait a moment."
	case http.StatusInternalServerError:
		return "Server Error (HTTP 500): The provider's server is having issues."
	default:
		return fmt.Sprintf("API Error (HTTP %d)", resp.StatusCode)
	}
}
