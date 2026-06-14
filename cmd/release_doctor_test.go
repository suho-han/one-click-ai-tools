package cmd

import (
	"encoding/json"
	"testing"
)

func TestReleaseDoctorJSONTagForNPMUserConfig(t *testing.T) {
	report := releaseDoctorReport{NPMUserConfig: "/tmp/.npmrc"}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	jsonText := string(data)
	if !contains(jsonText, `"npm_userconfig":"/tmp/.npmrc"`) {
		t.Fatalf("expected npm_userconfig field, got %s", jsonText)
	}
	if contains(jsonText, `"***"`) {
		t.Fatalf("unexpected legacy redacted field key in %s", jsonText)
	}
}
