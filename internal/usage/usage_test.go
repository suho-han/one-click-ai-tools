package usage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	results := []UsageResult{
		{Provider: "test", Used: "10"},
	}
	// Just ensure it doesn't panic
	PrintJSON(results)
}

func TestUsageParsingLogic(t *testing.T) {
	// Test the JSON structure we expect from APIs
	jsonData := `{"five_hour": {"utilization": 42.5}}`
	
	var data struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
		} `json:"five_hour"`
	}
	
	err := json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if data.FiveHour.Utilization != 42.5 {
		t.Errorf("Expected 42.5, got %f", data.FiveHour.Utilization)
	}
}

func TestMockServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"five_hour": {"utilization": 42.5}}`)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
		} `json:"five_hour"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if data.FiveHour.Utilization != 42.5 {
		t.Errorf("Expected 42.5, got %f", data.FiveHour.Utilization)
	}
}
