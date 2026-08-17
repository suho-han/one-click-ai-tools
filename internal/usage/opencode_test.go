package usage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOpenCodeGoAPIKey_Env(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("OPENCODE_API_KEY", "test-key-from-env")
	defer os.Unsetenv("OPENCODE_API_KEY")
	
	// Set HOME to avoid reading real auth.json
	os.Setenv("HOME", tmp)
	
	key, source := resolveOpenCodeGoAPIKey()
	if key != "test-key-from-env" {
		t.Errorf("expected key 'test-key-from-env', got '%s'", key)
	}
	if source != "env:OPENCODE_API_KEY" {
		t.Errorf("expected source 'env:OPENCODE_API_KEY', got '%s'", source)
	}
}

func TestResolveOpenCodeGoAPIKey_AuthJSON(t *testing.T) {
	tmp := t.TempDir()
	
	// Unset env to force auth.json lookup
	os.Unsetenv("OPENCODE_API_KEY")
	defer os.Unsetenv("OPENCODE_API_KEY")
	
	// Mock userHomeDir to use tmp directory
	origUserHomeDir := userHomeDir
	userHomeDir = func() (string, error) { return tmp, nil }
	defer func() { userHomeDir = origUserHomeDir }()
	
	// Create mock auth.json
	authDir := filepath.Join(tmp, ".local", "share", "opencode")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	authContent := `{
		"opencode-go": {
			"type": "api",
			"key": "test-key-from-auth-json"
		}
	}`
	authPath := filepath.Join(authDir, "auth.json")
	if err := os.WriteFile(authPath, []byte(authContent), 0644); err != nil {
		t.Fatal(err)
	}
	
	key, source := resolveOpenCodeGoAPIKey()
	if key != "test-key-from-auth-json" {
		t.Errorf("expected key 'test-key-from-auth-json', got '%s'", key)
	}
	if source != "auth.json" {
		t.Errorf("expected source 'auth.json', got '%s'", source)
	}
}

func TestResolveOpenCodeGoAPIKey_NotFound(t *testing.T) {
	tmp := t.TempDir()
	
	// Unset env and ensure no auth.json exists
	os.Unsetenv("OPENCODE_API_KEY")
	defer os.Unsetenv("OPENCODE_API_KEY")
	
	// Mock userHomeDir to use tmp directory
	origUserHomeDir := userHomeDir
	userHomeDir = func() (string, error) { return tmp, nil }
	defer func() { userHomeDir = origUserHomeDir }()
	
	key, source := resolveOpenCodeGoAPIKey()
	if key != "" {
		t.Errorf("expected empty key, got '%s'", key)
	}
	if source != "" {
		t.Errorf("expected empty source, got '%s'", source)
	}
}

func TestFetchOpenCodeUsage_NoAPIKey(t *testing.T) {
	tmp := t.TempDir()
	
	os.Unsetenv("OPENCODE_API_KEY")
	defer os.Unsetenv("OPENCODE_API_KEY")
	
	// Mock userHomeDir to use tmp directory
	origUserHomeDir := userHomeDir
	userHomeDir = func() (string, error) { return tmp, nil }
	defer func() { userHomeDir = origUserHomeDir }()
	
	result := FetchOpenCodeUsage()
	if result.Status != "warn" {
		t.Errorf("expected status 'warn', got '%s'", result.Status)
	}
	if !strings.Contains(result.Message, "OpenCode Go API key not found") {
		t.Errorf("expected message to contain 'OpenCode Go API key not found', got '%s'", result.Message)
	}
	if result.Source != "remote" {
		t.Errorf("expected source 'remote', got '%s'", result.Source)
	}
}

func TestFetchOpenCodeUsage_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Authorization header to start with 'Bearer ', got '%s'", auth)
		}
		
		// Return mock response
		resp := openCodeGoUsageResponse{
			Usage: map[string]openCodeGoWindowUsage{
				"rolling": {Status: "ok", Percent: 42.5, ResetsAt: "2026-08-17T21:00:00Z"},
				"weekly":  {Status: "ok", Percent: 65.0, ResetsAt: "2026-08-24T00:00:00Z"},
				"monthly": {Status: "ok", Percent: 80.0, ResetsAt: "2026-09-01T00:00:00Z"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	
	// Override endpoint for testing
	originalEndpoint := openCodeGoUsageEndpoint
	defer func() { openCodeGoUsageEndpoint = originalEndpoint }()
	openCodeGoUsageEndpoint = server.URL
	
	// Set API key
	os.Setenv("OPENCODE_API_KEY", "test-key")
	defer os.Unsetenv("OPENCODE_API_KEY")
	
	result := FetchOpenCodeUsage()
	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Used != "42" {
		t.Errorf("expected used '42', got '%s'", result.Used)
	}
	if result.Buckets["5h"] != "42" {
		t.Errorf("expected 5h bucket '42', got '%s'", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "65" {
		t.Errorf("expected 7d bucket '65', got '%s'", result.Buckets["7d"])
	}
	if result.Buckets["30d"] != "80" {
		t.Errorf("expected 30d bucket '80', got '%s'", result.Buckets["30d"])
	}
	if result.BucketResets["5h"] != "2026-08-17T21:00:00Z" {
		t.Errorf("expected 5h reset '2026-08-17T21:00:00Z', got '%s'", result.BucketResets["5h"])
	}
}

func TestFetchOpenCodeUsage_APIError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid token"}`))
	}))
	defer server.Close()
	
	// Override endpoint for testing
	originalEndpoint := openCodeGoUsageEndpoint
	defer func() { openCodeGoUsageEndpoint = originalEndpoint }()
	openCodeGoUsageEndpoint = server.URL
	
	// Set API key
	os.Setenv("OPENCODE_API_KEY", "invalid-key")
	defer os.Unsetenv("OPENCODE_API_KEY")
	
	result := FetchOpenCodeUsage()
	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if !strings.Contains(result.Message, "API error") {
		t.Errorf("expected message to contain 'API error', got '%s'", result.Message)
	}
}

func TestFetchOpenCodeUsage_EmptyUsage(t *testing.T) {
	// Create mock server that returns empty usage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openCodeGoUsageResponse{
			Usage: map[string]openCodeGoWindowUsage{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	
	// Override endpoint for testing
	originalEndpoint := openCodeGoUsageEndpoint
	defer func() { openCodeGoUsageEndpoint = originalEndpoint }()
	openCodeGoUsageEndpoint = server.URL
	
	// Set API key
	os.Setenv("OPENCODE_API_KEY", "test-key")
	defer os.Unsetenv("OPENCODE_API_KEY")
	
	result := FetchOpenCodeUsage()
	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if !strings.Contains(result.Message, "empty usage data") {
		t.Errorf("expected message to contain 'empty usage data', got '%s'", result.Message)
	}
}
