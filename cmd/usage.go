package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/notify"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

type usageModel struct {
	results      []usage.UsageResult
	err          error
	done         bool
	spinner      int
	orderedTools []update.Tool
	activeIdx    int
	// ctx carries the command's cancellation into the TUI fetch; nil falls
	// back to context.Background().
	ctx context.Context
}

func (m usageModel) Init() tea.Cmd {
	fetchCtx := m.ctx
	if fetchCtx == nil {
		fetchCtx = context.Background()
	}
	return tea.Batch(
		func() tea.Msg {
			res, err := usageFetcher(fetchCtx)
			if err != nil {
				return err
			}
			return res
		},
		tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
			return switchProviderMsg{}
		}),
	)
}

func (m usageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case []usage.UsageResult:
		m.results = msg
		m.done = true
		return m, tea.Quit
	case error:
		m.err = msg
		m.done = true
		return m, tea.Quit
	case spinnerMsg:
		m.spinner++
		return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return spinnerMsg{}
		})
	case switchProviderMsg:
		if len(m.orderedTools) > 0 {
			m.activeIdx = (m.activeIdx + 1) % len(m.orderedTools)
		}
		return m, tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
			return switchProviderMsg{}
		})
	}
	return m, nil
}

type spinnerMsg struct{}
type switchProviderMsg struct{}

var usageFetcher = usage.GetUsage

// statuslineFormats are the statusbar-oriented output modes of --format.
// json/compact stay separate flags for back-compat but are accepted here too.
var statuslineFormats = map[string]bool{
	"waybar":   true,
	"polybar":  true,
	"swiftbar": true,
	"json":     true,
	"compact":  true,
}

func shouldAutoJSONFallback(jsonMode bool, compactMode bool, format string, isTTY bool) bool {
	return !jsonMode && !compactMode && format == "" && !isTTY
}

// resolveUsageOutputMode merges the back-compat bool flags with --format.
// An explicit --format wins; documented so scripts can pass both safely.
func resolveUsageOutputMode(jsonMode, compactMode bool, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		if jsonMode {
			return "json", nil
		}
		if compactMode {
			return "compact", nil
		}
		return "", nil
	}
	if !statuslineFormats[format] {
		return "", fmt.Errorf("invalid --format %q (want json, compact, waybar, polybar, or swiftbar)", format)
	}
	return format, nil
}

func usageOrderedTools() []update.Tool {
	return usage.SelectedTools()
}

func maybeSendUsageAlerts(cmd *cobra.Command, results []usage.UsageResult, force bool) {
	enabled := force || viper.GetBool("usage_alert_enabled")
	if !enabled {
		return
	}
	cfg := buildAlertConfigFromViper(true)
	// Alerts are best-effort: warn on failure instead of failing the usage
	// report itself.
	if err := notify.MaybeSendUsageAlerts(results, cfg, time.Now()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: usage alerts failed: %v\n", err)
	}
}

func (m usageModel) View() string {
	if m.done {
		return ""
	}
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	s := spinners[m.spinner%len(spinners)]

	target := "AI providers"
	if len(m.orderedTools) > 0 {
		t := m.orderedTools[m.activeIdx]
		target = t.Colorize(t.Name)
	}

	return fmt.Sprintf("\n  %s Fetching usage data from %s...\n", s, target)
}

var usageCmd = &cobra.Command{
	Use:     "usage",
	GroupID: "core",
	Short:   "📊 Show tool usage report",
	Long: `Show tool usage report for configured AI developer tools.

To properly fetch usage, ensure you are authenticated:
  - Antigravity: Run 'agy' once; usage is parsed from 'agy --print /usage'
  - Claude:  Run 'claude auth login'; fallback usage is parsed from 'claude --print /usage'
  - Command Code: Run 'commandcode login' or set COMMAND_CODE_API_KEY
  - Cursor CLI: Official CLI install is recommended; usage remains best-effort with local fallback
  - Copilot: Configure your token via 'oct config'
  - OpenCode: Reads usage from local session logs first (no API token)
  - Codex:   Automatically reads from local session logs
  - Kimi Code: Run 'kimi login' or set KIMI_CODE_API_KEY (experimental)
  - Qwen Code: Counts local usage records; daily cap is configurable (qwen_daily_limit) (experimental)
  - MiniMax:  Set MINIMAX_CODING_API_KEY (or MINIMAX_API_KEY) (experimental)

Standalone providers (fetched only when listed in agent_order or enabled_tools):
  - Z.ai (GLM):   Set ZAI_API_KEY / ZHIPU_API_KEY, or sign in via 'opencode auth login' (experimental)
  - DeepSeek:     Set DEEPSEEK_API_KEY (experimental)
  - OpenRouter:   Set OPENROUTER_API_KEY (spending-limit tracking)
  - Grok (xAI):   Run 'grok login' or set GROK_OAUTH_TOKEN (experimental)

(experimental) = live API responses not yet verified against a real
subscription; field shapes may still change. Verified live: codex, claude,
commandcode, opencode, antigravity, openrouter.

Legacy aliases 'gemini' and 'gemini-cli' still map to 'agy' for compatibility.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		compactMode, _ := cmd.Flags().GetBool("compact")
		notifyMode, _ := cmd.Flags().GetBool("notify")
		format, _ := cmd.Flags().GetString("format")
		fromSnapshot, _ := cmd.Flags().GetBool("from-snapshot")
		snapshotPath, _ := cmd.Flags().GetString("snapshot-path")

		outputMode, err := resolveUsageOutputMode(jsonMode, compactMode, format)
		if err != nil {
			return err
		}
		if fromSnapshot && outputMode == "" {
			return fmt.Errorf("--from-snapshot requires an explicit --format (waybar, polybar, swiftbar, json, or compact)")
		}

		isTTY := false
		if fi, err := os.Stdout.Stat(); err == nil {
			isTTY = (fi.Mode() & os.ModeCharDevice) != 0
		}

		// Auto-fallback for non-TTY environments (CI, pipes, cron, tool runners)
		if shouldAutoJSONFallback(jsonMode, compactMode, outputMode, isTTY) {
			outputMode = "json"
			fmt.Fprintln(os.Stderr, "[oct] non-TTY detected -> switching to --json (pretty output)")
		}

		if outputMode != "" {
			var results []usage.UsageResult
			if fromSnapshot {
				snapshot, err := usage.LoadSnapshot(snapshotPath)
				if err != nil {
					return fmt.Errorf("load snapshot (run 'oct monitor' to produce one): %w", err)
				}
				results = snapshot.Results
			} else {
				results, err = usageFetcher(cmd.Context())
				if err != nil {
					return fmt.Errorf("fetch usage: %w", err)
				}
				maybeSendUsageAlerts(cmd, results, notifyMode)
			}
			return printUsageOutputMode(cmd, outputMode, results)
		}

		selectedTools := usageOrderedTools()

		m := usageModel{
			orderedTools: selectedTools,
			ctx:          cmd.Context(),
		}
		p := tea.NewProgram(m)

		// Start spinner tick
		go func() {
			time.Sleep(100 * time.Millisecond)
			p.Send(spinnerMsg{})
		}()

		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("run usage ui: %w", err)
		}

		fm, ok := finalModel.(usageModel)
		if !ok {
			return fmt.Errorf("unexpected usage ui model type %T", finalModel)
		}
		if fm.err != nil {
			return fmt.Errorf("fetch usage: %w", fm.err)
		}

		if len(fm.results) > 0 {
			maybeSendUsageAlerts(cmd, fm.results, notifyMode)
			usage.PrintTable(fm.results)
			fmt.Println("\nTip: Run 'oct usage --help' for authentication instructions.")
		}
		return nil
	},
}

// printUsageOutputMode renders one structured output mode. The display mode
// for statusline formats comes from usage_display_mode (normalized), matching
// the menubar title; --compact stays pinned to "remaining" per its contract.
func printUsageOutputMode(cmd *cobra.Command, outputMode string, results []usage.UsageResult) error {
	switch outputMode {
	case "compact":
		usage.RenderCompactRemaining(os.Stdout, results)
		return nil
	case "json":
		if err := usage.PrintJSON(results); err != nil {
			return fmt.Errorf("print usage json: %w", err)
		}
		return nil
	case "waybar":
		return usage.RenderWaybarJSON(os.Stdout, results, viper.GetString("usage_display_mode"))
	case "polybar":
		return usage.RenderPolybarLine(os.Stdout, results, viper.GetString("usage_display_mode"))
	case "swiftbar":
		return usage.RenderSwiftBar(os.Stdout, results, viper.GetString("usage_display_mode"))
	default:
		return fmt.Errorf("unsupported usage output mode %q", outputMode)
	}
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.Flags().Bool("json", false, "Output in JSON format")
	usageCmd.Flags().Bool("compact", false, "Output compact remaining usage (C-45% X-25%)")
	usageCmd.Flags().Bool("notify", false, "Send usage alerts based on threshold/cooldown rules")
	usageCmd.Flags().String("format", "", "Structured output mode: json, compact, waybar, polybar, or swiftbar (overrides --json/--compact; statusline formats honor usage_display_mode)")
	usageCmd.Flags().Bool("from-snapshot", false, "Render from the last 'oct monitor' snapshot instead of fetching live (requires --format)")
	usageCmd.Flags().String("snapshot-path", "", "Snapshot file for --from-snapshot (default ~/.oct/state/usage-latest.json)")
}
