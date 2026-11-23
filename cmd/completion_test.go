package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func runCompletion(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := runCLI(args, &out, &errb)
	return out.String(), errb.String(), code
}

func TestCompletionScripts(t *testing.T) {
	checks := map[string]func(string) bool{
		"bash": func(s string) bool { return strings.Contains(s, "oct") },
		"zsh":  func(s string) bool { return strings.HasPrefix(s, "#compdef oct") && strings.Contains(s, "_oct") },
		"fish": func(s string) bool { return strings.Contains(s, "complete -c oct") },
	}
	for shell, ok := range checks {
		out, _, code := runCompletion("completion", shell)
		if code != 0 || strings.TrimSpace(out) == "" || !ok(out) {
			t.Fatalf("%s: code=%d out=%.80q", shell, code, out)
		}
	}
}

func TestCompletionUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"completion"}, {"completion", "powershell"}} {
		out, errs, code := runCompletion(args...)
		if code != 2 || out != "" || !strings.Contains(errs, "Supported shells: bash, zsh, fish") {
			t.Fatalf("%v: code=%d out=%q err=%q", args, code, out, errs)
		}
	}
}

func TestCompletionErrorIsUsageError(t *testing.T) {
	var errb bytes.Buffer
	completionCmd.SetErr(&errb)
	defer completionCmd.SetErr(nil)
	err := completionCmd.RunE(completionCmd, []string{"nope"})
	if _, ok := err.(*usageError); !ok || exitCodeFor(err) != 2 {
		t.Fatalf("err=%v", err)
	}
}
