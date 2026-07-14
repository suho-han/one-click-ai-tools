package netclient

import (
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"time"
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

func (c *Client) DoWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= c.MaxRetries; i++ {
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
		time.Sleep(backoff)
	}

	return resp, err
}

func (c *Client) shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
			return true
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		return false
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
