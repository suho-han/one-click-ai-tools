package usage

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/suho-han/one-click-tools/internal/netclient"
)

func TestRetrieveGeminiQuotaMaps5hAnd7d(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota" {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			body := `{"buckets":[{"bucketId":"five_hour","remainingFraction":0.71},{"bucketId":"seven_day","remainingFraction":0.28}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0

	used, limit, buckets, _, err := retrieveGeminiQuota("dummy", "proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 100 {
		t.Fatalf("expected limit=100, got %f", limit)
	}
	if used < 28.9 || used > 29.1 {
		t.Fatalf("expected used around 29, got %f", used)
	}
	if buckets["5h"] != "29.0" {
		t.Fatalf("expected 5h=29.0, got %s", buckets["5h"])
	}
	if buckets["7d"] != "72.0" {
		t.Fatalf("expected 7d=72.0, got %s", buckets["7d"])
	}
}

func TestRetrieveGeminiQuotaUnknownBucketFallback(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"buckets":[{"modelId":"gemini-2.5-pro","remainingFraction":0.55}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0

	used, _, buckets, _, err := retrieveGeminiQuota("dummy", "proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used < 44.9 || used > 45.1 {
		t.Fatalf("expected used around 45, got %f", used)
	}
	if len(buckets) != 0 {
		t.Fatalf("expected no classified buckets, got %v", buckets)
	}
}
