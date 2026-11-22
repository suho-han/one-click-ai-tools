package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var menubarReconnectCmd = &cobra.Command{
	Use:          "reconnect <provider>",
	Hidden:       true,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("opening Terminal for reconnection requires macOS")
		}
		command, err := menubarReconnectCommand(args[0])
		if err != nil {
			return err
		}
		return openReconnectTerminal(cmd.Context(), command)
	},
}

func menubarReconnectCommand(provider string) (string, error) {
	if provider == "claude" || provider == "claude-code" {
		return buildMenubarExecCommand("claude", "auth", "login"), nil
	}
	home, err := resolveCodexAccountHome(provider)
	if err != nil {
		return "", err
	}
	if home == "" {
		home = strings.TrimSpace(os.Getenv("CODEX_HOME"))
		if home == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve Codex home: %w", err)
			}
			home = filepath.Join(userHome, ".codex")
		}
	}
	// Terminal starts a new shell; pin the same absolute home as the usage row.
	home, err = filepath.Abs(home)
	if err != nil {
		return "", fmt.Errorf("resolve Codex home: %w", err)
	}
	return buildMenubarExecCommand("/bin/mkdir", "-p", "-m", "700", "--", home) + " && " +
		buildMenubarExecCommand("/usr/bin/env", "CODEX_HOME="+home, "codex", "login"), nil
}

// Open a .command document so Terminal runs the login without Apple Events
// automation permission. Terminal opens it asynchronously; the script owns
// cleanup once its shell has opened the file.
func openReconnectTerminal(parent context.Context, command string) error {
	script, err := writeReconnectScript(command)
	if err != nil {
		return fmt.Errorf("prepare Terminal login: %w", err)
	}

	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/open", "-b", "com.apple.Terminal", script).CombinedOutput()
	if err != nil {
		_ = os.Remove(script)
		if ctx.Err() != nil {
			return fmt.Errorf("open Terminal: %w", ctx.Err())
		}
		if detail := strings.TrimSpace(string(output)); detail != "" {
			return fmt.Errorf("open Terminal: %s", detail)
		}
		return fmt.Errorf("open Terminal: %w", err)
	}
	return nil
}

func writeReconnectScript(command string) (string, error) {
	file, err := os.CreateTemp("", "oct-reconnect-*.command")
	if err != nil {
		return "", err
	}
	if _, err = file.WriteString("#!/bin/sh\n/bin/rm -f -- \"$0\"\n" + command + "\n"); err == nil {
		err = file.Chmod(0o700)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func runInTerminal(parent context.Context, command string) error {
	// A macOS automation prompt can block until the user answers.
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", buildTerminalAppleScript(command)).CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("open Terminal: %w", ctx.Err())
	}
	if err != nil {
		if detail := strings.TrimSpace(string(output)); detail != "" {
			return fmt.Errorf("open Terminal: %s", detail)
		}
		return fmt.Errorf("open Terminal: %w", err)
	}
	return nil
}
