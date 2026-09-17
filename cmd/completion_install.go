package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func installCompletion(cmd *cobra.Command, shell completionShell, noDescriptions bool) error {
	scriptPath, err := completionScriptPath(shell)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		return fmt.Errorf("create completion directory: %w", err)
	}
	file, err := os.OpenFile(scriptPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open completion script: %w", err)
	}
	if err := generateCompletion(cmd.Root(), shell, noDescriptions, file); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write completion script: %w", err)
	}

	out := cmd.OutOrStdout()
	if shell.profile == "" {
		fmt.Fprintf(out, "Installed %s completion at %s\n", shell.name, scriptPath)
		return nil
	}
	profilePath, err := completionProfilePath(shell)
	if err != nil {
		return err
	}
	if err := addCompletionProfileBlock(profilePath, shell.sourceLine(scriptPath)); err != nil {
		return err
	}
	fmt.Fprintf(out, "Installed %s completion at %s\n", shell.name, scriptPath)
	fmt.Fprintf(out, "Updated %s. Open a new shell, or run: %s\n", profilePath, shell.sourceLine(scriptPath))
	return nil
}

func uninstallCompletion(cmd *cobra.Command, shell completionShell) error {
	scriptPath, err := completionScriptPath(shell)
	if err != nil {
		return err
	}
	if err := os.Remove(scriptPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove completion script: %w", err)
	}
	if shell.profile != "" {
		profilePath, err := completionProfilePath(shell)
		if err != nil {
			return err
		}
		if err := removeCompletionProfileBlock(profilePath); err != nil {
			return err
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Uninstalled %s completion\n", shell.name)
	return nil
}

func completionScriptPath(shell completionShell) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if shell.name == "fish" {
		return filepath.Join(home, ".config", "fish", "completions", "oct.fish"), nil
	}
	return filepath.Join(home, ".oct", "completions", "oct."+shell.extension), nil
}

func completionProfilePath(shell completionShell) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, shell.profile), nil
}

func addCompletionProfileBlock(profilePath, sourceLine string) error {
	body := ""
	if data, err := os.ReadFile(profilePath); err == nil {
		body = removeManagedBlock(string(data))
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read shell profile: %w", err)
	}
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += completionBlockStart + "\n" + sourceLine + "\n" + completionBlockEnd + "\n"
	return writeProfileFile(profilePath, []byte(body))
}

func removeCompletionProfileBlock(profilePath string) error {
	data, err := os.ReadFile(profilePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read shell profile: %w", err)
	}
	return writeProfileFile(profilePath, []byte(removeManagedBlock(string(data))))
}

// writeProfileFile replaces a shell startup file atomically: the replacement
// is fully written to a sibling temp file before the rename, so a failed
// write (e.g. out of space) leaves the user's existing profile intact.
func writeProfileFile(profilePath string, body []byte) error {
	dir := filepath.Dir(profilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create shell profile directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(profilePath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create shell profile temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("write shell profile: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("write shell profile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write shell profile: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("set shell profile permissions: %w", err)
	}
	if err := os.Rename(tmpName, profilePath); err != nil {
		return fmt.Errorf("replace shell profile: %w", err)
	}
	return nil
}

func removeManagedBlock(body string) string {
	start := strings.Index(body, completionBlockStart)
	if start == -1 {
		return body
	}
	end := strings.Index(body[start:], completionBlockEnd)
	if end == -1 {
		return body
	}
	end += start + len(completionBlockEnd)
	if end < len(body) && body[end] == '\n' {
		end++
	}
	return body[:start] + body[end:]
}

func (shell completionShell) sourceLine(scriptPath string) string {
	if shell.name == "powershell" {
		return fmt.Sprintf(shell.sourceTmpl, quotePowerShellPath(scriptPath))
	}
	return fmt.Sprintf(shell.sourceTmpl, quotePOSIXPath(scriptPath))
}

func quotePOSIXPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}

func quotePowerShellPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}

func powershellProfilePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join("Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	}
	return filepath.Join(".config", "powershell", "Microsoft.PowerShell_profile.ps1")
}
