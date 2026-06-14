package usage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var usageCommandOutput = defaultUsageCommandOutput

func withPlan(result UsageResult, plan string, source string) UsageResult {
	result.Plan = normalizePlanLabel(plan)
	result.PlanSource = normalizePlanSource(source)
	return result
}

func withPlanDetection(result UsageResult, detector func() (string, string)) UsageResult {
	plan, source := detector()
	return withPlan(result, plan, source)
}

func normalizePlanLabel(plan string) string {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		return "unknown"
	}
	return strings.ToLower(strings.Join(strings.Fields(plan), " "))
}

func normalizePlanSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "unknown"
	}
	return source
}

func detectCodexPlan() (string, string) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "unknown", "codex auth unavailable"
	}
	path := filepath.Join(home, ".codex", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "unknown", "codex auth.json missing"
	}
	var auth struct {
		Tokens map[string]string `json:"tokens"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return "unknown", "codex auth.json parse failed"
	}
	for _, key := range []string{"id_token", "access_token"} {
		if plan := detectPlanFromJWTToken(auth.Tokens[key]); plan != "" {
			return plan, "codex auth.jwt " + key
		}
	}
	return "unknown", "codex auth.jwt has no plan claim"
}

func detectCursorPlan() (string, string) {
	if output, err := usageCommandOutput(3*time.Second, "cursor-agent", "about"); err == nil {
		if plan := parseCursorAboutPlan(output); plan != "" {
			return plan, "cursor-agent about"
		}
	}
	if token := readCursorAuthToken(); token != "" {
		if plan := detectPlanFromJWTToken(token); plan != "" {
			return plan, "cursor auth.jwt"
		}
	}
	return "unknown", "cursor plan not exposed"
}

func detectClaudePlan(token string) (string, string) {
	if plan := detectPlanFromJWTToken(token); plan != "" {
		return plan, "claude oauth token claim"
	}
	return "unknown", "claude plan not exposed"
}

func detectCopilotPlan() (string, string) {
	return "unknown", "github copilot plan not exposed by current api integration"
}

func detectOpenCodePlan() (string, string) {
	return "unknown", "local opencode session logs do not expose plan"
}

func detectAntigravityPlan() (string, string) {
	return "unknown", "antigravity cli does not expose tier; see app settings"
}

func defaultUsageCommandOutput(timeout time.Duration, name string, args ...string) (string, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, trimmed)
	}
	return string(out), nil
}

func parseCursorAboutPlan(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "subscription tier") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return ""
		}
		return normalizePlanLabel(strings.Join(fields[2:], " "))
	}
	return ""
}

func detectPlanFromJWTToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := decodeJWTPayload(parts[1])
	if err != nil {
		return ""
	}
	if plan := extractPlanCandidate(payload); plan != "" {
		return normalizePlanLabel(plan)
	}
	return ""
}

func decodeJWTPayload(segment string) (map[string]any, error) {
	segment += strings.Repeat("=", (4-len(segment)%4)%4)
	decoded, err := base64.URLEncoding.DecodeString(segment)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(segment, "="))
		if err != nil {
			return nil, err
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func extractPlanCandidate(v any) string {
	switch x := v.(type) {
	case map[string]any:
		priority := []string{
			"chatgpt_plan_type",
			"plan",
			"plan_type",
			"tier",
			"subscription_tier",
			"subscription_plan",
			"account_plan",
			"product",
		}
		for _, key := range priority {
			if val, ok := x[key]; ok {
				if plan := scalarPlanCandidate(val); plan != "" {
					return plan
				}
			}
		}
		for _, val := range x {
			if plan := extractPlanCandidate(val); plan != "" {
				return plan
			}
		}
	case []any:
		for _, item := range x {
			if plan := extractPlanCandidate(item); plan != "" {
				return plan
			}
		}
	}
	return ""
}

func scalarPlanCandidate(v any) string {
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return ""
		}
		lower := strings.ToLower(s)
		for _, candidate := range []string{"free", "pro", "plus", "max", "team", "business", "enterprise", "student", "individual", "unknown"} {
			if lower == candidate || strings.Contains(lower, candidate) {
				return s
			}
		}
	case map[string]any:
		for _, key := range []string{"name", "type", "tier", "plan"} {
			if val, ok := x[key]; ok {
				if plan := scalarPlanCandidate(val); plan != "" {
					return plan
				}
			}
		}
	}
	return ""
}
