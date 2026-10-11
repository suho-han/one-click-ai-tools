package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestTelemetryConfigOptOutPersistsAcrossResetAndPartialUpdate(t *testing.T) {
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	var stdout, stderr bytes.Buffer
	run := func(args ...string) int {
		stdout.Reset()
		stderr.Reset()
		return runCLI(append([]string{"--config", cfgPath}, args...), &stdout, &stderr)
	}
	if code := run("config", "set", "telemetry-enabled", "false"); code != 0 || stdout.String() != "Config updated.\n" {
		t.Fatalf("set false: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if code := run("config", "reset"); code != 0 {
		t.Fatalf("reset: code=%d stderr=%q", code, stderr.String())
	}
	if code := run("config", "update", "--payload", `{}`); code != 0 {
		t.Fatalf("empty update: code=%d stderr=%q", code, stderr.String())
	}
	if code := run("config", "list", "--json"); code != 0 {
		t.Fatalf("list: code=%d stderr=%q", code, stderr.String())
	}
	var snapshot struct {
		TelemetryEnabled bool `json:"telemetry_enabled"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &snapshot); err != nil || snapshot.TelemetryEnabled {
		t.Fatalf("opt-out lost: value=%v err=%v", snapshot.TelemetryEnabled, err)
	}
	if code := run("config", "update", "--payload", `{"telemetry_enabled":true}`); code != 0 {
		t.Fatalf("enable update: code=%d stderr=%q", code, stderr.String())
	}
	if code := run("config", "list", "--json"); code != 0 {
		t.Fatalf("list: code=%d stderr=%q", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &snapshot); err != nil || !snapshot.TelemetryEnabled {
		t.Fatalf("enable not saved: value=%v err=%v", snapshot.TelemetryEnabled, err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil || !bytes.Contains(data, []byte("telemetry_enabled: true")) {
		t.Fatalf("custom config not written: %q err=%v", data, err)
	}
}

func TestTelemetryConfigRejectsAmbiguousValueWithoutSaving(t *testing.T) {
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	original, _ := os.ReadFile(cfgPath)
	for _, value := range []string{"TRUE", "1", " false "} {
		var stdout, stderr bytes.Buffer
		if code := runCLI([]string{"--config", cfgPath, "config", "set", "telemetry-enabled", value}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "invalid telemetry-enabled value: use true or false") {
			t.Fatalf("value %q: code=%d stderr=%q", value, code, stderr.String())
		}
		data, err := os.ReadFile(cfgPath)
		if err != nil || !bytes.Equal(data, original) {
			t.Fatalf("invalid value %q changed config: %q err=%v", value, data, err)
		}
	}
}

func TestTelemetryConfigPayloadDistinguishesFalseFromOmitted(t *testing.T) {
	viperResetForTest(t)
	viper.Set("telemetry_enabled", true)
	var payload configUpdatePayload
	if err := json.Unmarshal([]byte(`{"telemetry_enabled":false}`), &payload); err != nil || payload.TelemetryEnabled == nil {
		t.Fatalf("decode false: %+v err=%v", payload, err)
	}
	if err := applyConfigUpdate(payload); err != nil || viper.GetBool("telemetry_enabled") {
		t.Fatalf("false not applied: %v", err)
	}
	if err := applyConfigUpdate(configUpdatePayload{}); err != nil || viper.GetBool("telemetry_enabled") {
		t.Fatalf("omitted value changed opt-out: %v", err)
	}
}
