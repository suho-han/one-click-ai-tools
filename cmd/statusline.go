package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const statuslinePluginName = "oct.5m.sh"

var (
	statuslineConfigTargets  = []string{"waybar", "polybar", "swiftbar", "xbar"}
	statuslineInstallTargets = []string{"swiftbar", "xbar"}
)

// usageError makes runCLI exit with code 2 (invalid invocation).
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func exitCodeFor(err error) int {
	var ue *usageError
	if errors.As(err, &ue) {
		return 2
	}
	return 1
}

var statuslineCmd = &cobra.Command{
	Use:   "statusline",
	Short: "Generate statusbar configs and plugins (waybar, polybar, SwiftBar, xbar)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Supported config targets:  %s\n", strings.Join(statuslineConfigTargets, ", "))
		fmt.Fprintf(out, "Supported install targets: %s\n", strings.Join(statuslineInstallTargets, ", "))
		fmt.Fprintln(out, "Example: oct statusline config waybar")
		return nil
	},
}

var statuslineConfigCmd = &cobra.Command{
	Use:   "config <waybar|polybar|swiftbar|xbar>",
	Short: "Print a paste-ready statusbar config snippet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		snippet, err := statuslineSnippet(args[0], path)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "# %s snippet for oct (rendering is delegated to `oct usage --format`)\n", args[0])
		fmt.Fprint(cmd.OutOrStdout(), snippet)
		return nil
	},
}

var statuslineInstallCmd = &cobra.Command{
	Use:   "install <swiftbar|xbar>",
	Short: "Install the oct.5m.sh plugin for SwiftBar or xbar",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		dir, _ := cmd.Flags().GetString("dir")
		force, _ := cmd.Flags().GetBool("force")
		return runStatuslineInstall(cmd.OutOrStdout(), args[0], path, dir, force, runtime.GOOS)
	},
}

func statuslineSnippet(target, execPath string) (string, error) {
	bin := statuslineQuote(execPath)
	switch target {
	case "waybar":
		return fmt.Sprintf(`"custom/oct": {
  "exec": "%s usage --format waybar",
  "return-type": "json",
  "interval": 60,
  "escape": false
}
`, jsonEscape(bin)), nil
	case "polybar":
		return fmt.Sprintf(`[module/oct]
type = custom/script
interval = 60
exec = %s usage --format polybar
`, bin), nil
	case "swiftbar", "xbar":
		return fmt.Sprintf(`#!/bin/bash
# <bitbar.title>oct usage</bitbar.title>
# <bitbar.version>v1.0</bitbar.version>
# <bitbar.desc>AI CLI quota usage from one-click-ai-tools</bitbar.desc>
# <bitbar.dependencies>oct</bitbar.dependencies>
%s usage --format swiftbar
`, bin), nil
	}
	return "", &usageError{fmt.Sprintf("unknown statusline target %q (supported: %s)", target, strings.Join(statuslineConfigTargets, ", "))}
}

func statuslineQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r == '/' || r == '.' || r == '_' || r == '-' || r == '+' || r == ':' || r == '~' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func jsonEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

func statuslineDefaultDir(target, goos string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch target {
	case "swiftbar":
		if goos != "darwin" {
			return "", fmt.Errorf("swiftbar is macOS-only; use --dir to choose a plugin directory explicitly")
		}
		return filepath.Join(home, "Library", "Application Support", "SwiftBar", "Plugins"), nil
	default:
		return filepath.Join(home, ".config", "xbar", "plugins"), nil
	}
}

func runStatuslineInstall(out io.Writer, target, execPath, dir string, force bool, goos string) error {
	if target != "swiftbar" && target != "xbar" {
		return &usageError{fmt.Sprintf("unsupported install target %q (supported: %s)", target, strings.Join(statuslineInstallTargets, ", "))}
	}
	if goos == "windows" {
		return fmt.Errorf("statusline install is not supported on windows (shell plugins only)")
	}
	if dir == "" {
		d, err := statuslineDefaultDir(target, goos)
		if err != nil {
			return err
		}
		dir = d
	}
	body, err := statuslineSnippet(target, execPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, statuslinePluginName)
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(dest, flags, 0o755)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; pass --force to overwrite", dest)
		}
		return err
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return err
	}
	fmt.Fprintf(out, "Installed %s\n", dest)
	fmt.Fprintf(out, "Refresh %s (menu > Refresh All) to load the plugin.\n", map[string]string{"swiftbar": "SwiftBar", "xbar": "xbar"}[target])
	return nil
}

func init() {
	rootCmd.AddCommand(statuslineCmd)
	statuslineCmd.AddCommand(statuslineConfigCmd, statuslineInstallCmd)
	statuslineCmd.PersistentFlags().String("path", "oct", "Path to the oct executable used in generated output")
	statuslineInstallCmd.Flags().String("dir", "", "Plugin directory (default: platform plugin directory)")
	statuslineInstallCmd.Flags().Bool("force", false, "Overwrite an existing plugin file")
}
