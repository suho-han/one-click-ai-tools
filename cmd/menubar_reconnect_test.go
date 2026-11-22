package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestMenubarReconnectCommandRunsLoginForDisplayedAccount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Terminal login command is written for a Unix shell")
	}
	dir := t.TempDir()
	for _, bin := range []string{"codex", "claude"} {
		if err := os.WriteFile(filepath.Join(dir, bin), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" \"$CODEX_HOME\"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	defaultHome := filepath.Join(dir, "default")
	t.Setenv("CODEX_HOME", defaultHome)
	workHome := filepath.Join(dir, "work's profile")
	personalHome := filepath.Join(dir, "personal")
	t.Cleanup(func() { viper.Set("accounts", nil) })
	work := map[string]string{"name": "work", "home": workHome}
	personal := map[string]string{"name": "personal", "home": personalHome}

	for _, tc := range []struct {
		name, provider, want string
		accounts             []map[string]string
	}{
		{name: "default", provider: "codex", want: "login\n" + defaultHome},
		{name: "sole account", provider: "codex", want: "login\n" + workHome, accounts: []map[string]string{work}},
		{name: "named account", provider: "codex:personal", want: "login\n" + personalHome, accounts: []map[string]string{work, personal}},
		{name: "Claude", provider: "claude-code", want: "auth login\n" + defaultHome},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Set("accounts", map[string][]map[string]string{"codex": tc.accounts})
			command, err := menubarReconnectCommand(tc.provider)
			if err != nil {
				t.Fatal(err)
			}
			script, err := writeReconnectScript(command)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(script) })
			output, err := exec.Command(script).CombinedOutput()
			if got := strings.TrimSpace(string(output)); err != nil || got != tc.want {
				t.Fatalf("login received %q, want %q (error: %v)", got, tc.want, err)
			}
			if _, err := os.Stat(script); !os.IsNotExist(err) {
				t.Fatalf("Terminal login script was not removed after opening: %v", err)
			}
		})
	}
	if _, err := menubarReconnectCommand("codex:missing"); err == nil {
		t.Fatal("expected unknown account to be rejected, not logged in to the default home")
	}
}
