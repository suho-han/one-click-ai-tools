package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		target, ok, err := statuslineResolveTarget(cmd, args, "config", statuslineConfigTargets)
		if err != nil || !ok {
			return err
		}
		snippet, err := statuslineSnippet(target, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "# %s snippet for oct (rendering is delegated to `oct usage --format`)\n", target)
		fmt.Fprint(cmd.OutOrStdout(), snippet)
		return nil
	},
}

var statuslineInstallCmd = &cobra.Command{
	Use:   "install <swiftbar|xbar>",
	Short: "Install the oct.5m.sh plugin for SwiftBar or xbar",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, ok, err := statuslineResolveTarget(cmd, args, "install", statuslineInstallTargets)
		if err != nil || !ok {
			return err
		}
		path, _ := cmd.Flags().GetString("path")
		dir, _ := cmd.Flags().GetString("dir")
		force, _ := cmd.Flags().GetBool("force")
		return runStatuslineInstall(cmd.OutOrStdout(), target, path, dir, force, runtime.GOOS)
	},
}

// statuslinePickerModel is a minimal single-choice list for choosing a target.
type statuslinePickerModel struct {
	title     string
	targets   []string
	cursor    int
	selected  string
	cancelled bool
}

func (m statuslinePickerModel) Init() tea.Cmd { return nil }

func (m statuslinePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.targets)-1 {
			m.cursor++
		}
	case "enter":
		m.selected = m.targets[m.cursor]
		return m, tea.Quit
	case "esc", "q", "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m statuslinePickerModel) View() string {
	var b strings.Builder
	b.WriteString("? " + m.title + "\n\n")
	for i, t := range m.targets {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		b.WriteString(prefix + t + "\n")
	}
	b.WriteString("\n  up/down or j/k: move  enter: select  esc/q: cancel\n")
	return b.String()
}

// statuslinePick runs the picker; swapped in tests. Returns the chosen target
// and whether the user cancelled.
var statuslinePick = func(title string, targets []string) (string, bool, error) {
	finalModel, err := tea.NewProgram(statuslinePickerModel{title: title, targets: targets}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", false, err
	}
	m, ok := finalModel.(statuslinePickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}
	return m.selected, m.cancelled, nil
}

// statuslineResolveTarget returns the target from args or the picker. ok is
// false when the user cancelled (already reported on stdout).
func statuslineResolveTarget(cmd *cobra.Command, args []string, verb string, targets []string) (string, bool, error) {
	if len(args) == 1 {
		return args[0], true, nil
	}
	errw := cmd.ErrOrStderr()
	fail := func() (string, bool, error) {
		fmt.Fprintf(errw, "Supported targets: %s\n", strings.Join(targets, ", "))
		fmt.Fprintf(errw, "Example: oct statusline %s %s\n", verb, targets[0])
		return "", false, &usageError{fmt.Sprintf("pass a target explicitly: oct statusline %s <%s>", verb, strings.Join(targets, "|"))}
	}
	if !isTerminalPrompt(cmd.InOrStdin(), cmd.OutOrStdout()) {
		return fail()
	}
	target, cancelled, err := statuslinePick("Select statusline target for "+verb+":", targets)
	if err != nil {
		return fail()
	}
	if cancelled {
		fmt.Fprintln(cmd.OutOrStdout(), "Statusline cancelled.")
		return "", false, nil
	}
	return target, true, nil
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
