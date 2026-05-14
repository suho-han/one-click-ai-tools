package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func FetchGeminiUsage() UsageResult {
	home, _ := os.UserHomeDir()
	oauthFile := filepath.Join(home, ".gemini", "oauth_creds.json")
	
	result := UsageResult{
		Provider: "gemini",
		Period:   "current",
		Used:     "n/a",
		Limit:    "100",
		Unit:     "percent",
		Source:   "oauth",
		Status:   "error",
	}

	data, err := os.ReadFile(oauthFile)
	if err != nil {
		result.Message = "No OAuth credentials found at ~/.gemini/oauth_creds.json"
		return result
	}

	var creds struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiryDate   int64  `json:"expiry_date"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		result.Message = "Failed to parse OAuth credentials"
		return result
	}

	token := creds.AccessToken
	// Token refresh logic would go here if expiry_date is past
	nowMs := time.Now().UnixNano() / 1e6
	if creds.ExpiryDate > 0 && creds.ExpiryDate <= (nowMs + 60000) && creds.RefreshToken != "" {
		// Attempt refresh (simplified)
		result.Message = "Token expired, refresh required (auto-refresh not fully implemented)"
		// return result // For now, let's try with existing token
	}

	if token == "" {
		result.Message = "No access token found"
		return result
	}

	// 1. loadCodeAssist to get project ID
	projectID, err := loadGeminiProjectID(token)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to load Gemini project ID: %v", err)
		return result
	}

	// 2. retrieveUserQuota
	used, limit, err := retrieveGeminiQuota(token, projectID)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to retrieve Gemini quota: %v", err)
		return result
	}

	result.Status = "ok"
	result.Used = fmt.Sprintf("%.0f", used)
	result.Limit = fmt.Sprintf("%.0f", limit)
	result.Message = "Quota fetched via Cloud Code API"
	return result
}

func loadGeminiProjectID(token string) (string, error) {
	url := "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	payload := []byte(`{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`)
	
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf("Invalid API Token (HTTP 401)")
		}
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var data struct {
		ProjectID string `json:"cloudaicompanionProject"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse project ID response: %w", err)
	}
	return data.ProjectID, nil
}

func retrieveGeminiQuota(token, projectID string) (float64, float64, error) {
	url := "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
	payload := []byte(fmt.Sprintf(`{"project":"%s"}`, projectID))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var data struct {
		Buckets []struct {
			ModelID           string  `json:"modelId"`
			RemainingFraction float64 `json:"remainingFraction"`
		} `json:"buckets"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, 0, fmt.Errorf("failed to parse quota response: %w", err)
	}

	for _, b := range data.Buckets {
		if b.ModelID != "" {
			used := (1.0 - b.RemainingFraction) * 100
			return used, 100, nil
		}
	}

	return 0, 0, fmt.Errorf("no quota buckets found")
}
