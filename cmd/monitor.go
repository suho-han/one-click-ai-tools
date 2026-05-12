package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/usage"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Always-on usage monitoring screen",
	Run: func(cmd *cobra.Command, args []string) {
		interval, _ := cmd.Flags().GetDuration("interval")
		statePath, _ := cmd.Flags().GetString("state-path")
		once, _ := cmd.Flags().GetBool("once")
		sortBy, _ := cmd.Flags().GetString("sort-by")
		desc, _ := cmd.Flags().GetBool("desc")
		top, _ := cmd.Flags().GetInt("top")
		compact, _ := cmd.Flags().GetBool("compact")

		if interval <= 0 {
			interval = 30 * time.Second
		}

		runOnce := func() {
			results, err := usage.GetUsage()
			now := time.Now()
			if err != nil {
				fmt.Printf("[%s] error: %v\n", now.Format("2006-01-02 15:04:05"), err)
				return
			}
			results = sortMonitorResults(results, sortBy, desc)
			if top > 0 && top < len(results) {
				results = results[:top]
			}
			printMonitorScreen(results, now, compact)
			if err := usage.SaveSnapshot(statePath, results, now); err != nil {
				fmt.Printf("snapshot write error: %v\n", err)
			}
		}

		runOnce()
		if once {
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runOnce()
		}
	},
}

func printMonitorScreen(results []usage.UsageResult, now time.Time, compact bool) {
	width := monitorTerminalWidth()
	if width <= 100 {
		compact = true
	}
	msgWidth := monitorMessageWidth(width)

	fmt.Print("\033[H\033[2J") // clear screen
	fmt.Printf("oct monitor  |  %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("-", width))
	if compact {
		fmt.Printf("%-14s %-8s %-8s %-8s %-10s\n", "provider", "5h", "7d", "sev", "status")
	} else {
		fmt.Printf("%-14s %-8s %-8s %-8s %-10s %-10s %-8s %s\n", "provider", "5h", "7d", "sev", "used", "limit", "status", "message")
	}

	mode := strings.ToLower(strings.TrimSpace(viper.GetString("usage_display_mode")))
	if mode != "used" && mode != "remaining" {
		mode = "used"
	}

	for _, r := range results {
		five := bucketVal(r, "5h", mode)
		seven := bucketVal(r, "7d", mode)
		sev := colorizeSeverityLabel(usageSeverity(r))
		u := r.Used
		if mode == "remaining" {
			if rem, ok := usageRemaining(r.Used, r.Unit); ok {
				u = rem
			}
		}
		msg := truncateMonitorText(r.Message, msgWidth)
		statusLabel := colorizeMonitorStatus(r.Status)
		providerLabel := colorizeMonitorProvider(r.Provider)
		if compact {
			fmt.Printf("%-14s %-8s %-8s %-8s %-10s\n", providerLabel, five, seven, sev, statusLabel)
		} else {
			fmt.Printf("%-14s %-8s %-8s %-8s %-10s %-10s %-8s %s\n", providerLabel, five, seven, sev, u, r.Limit, statusLabel, msg)
		}
	}

	fmt.Println()
	fmt.Printf("snapshot: %s\n", usage.DefaultSnapshotPath())
	fmt.Println("Ctrl+C to stop")
}

func monitorTerminalWidth() int {
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 40 {
			return n
		}
	}
	return 100
}

func monitorMessageWidth(width int) int {
	switch {
	case width <= 100:
		return 0
	case width <= 120:
		return 20
	default:
		return 32
	}
}

func truncateMonitorText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func colorizeMonitorProvider(provider string) string {
	p := strings.ToLower(provider)
	code := ""
	switch {
	case strings.Contains(p, "gemini"):
		code = "94"
	case strings.Contains(p, "claude"):
		code = "93"
	case strings.Contains(p, "codex"), strings.Contains(p, "openai"):
		code = "96"
	case strings.Contains(p, "copilot"), strings.Contains(p, "github"):
		code = "95"
	case strings.Contains(p, "cursor"):
		code = "92"
	case strings.Contains(p, "opencode"):
		code = "97"
	}
	if code == "" {
		return provider
	}
	return "\x1b[1;" + code + "m" + provider + "\x1b[0m"
}

func colorizeSeverityLabel(sev string) string {
	switch strings.ToUpper(strings.TrimSpace(sev)) {
	case "CRIT":
		return "\x1b[1;91mCRIT\x1b[0m"
	case "WARN":
		return "\x1b[1;93mWARN\x1b[0m"
	case "OK":
		return "\x1b[1;92mOK\x1b[0m"
	default:
		return "\x1b[1;90m" + sev + "\x1b[0m"
	}
}

func colorizeMonitorStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "ok":
		return "\x1b[1;92mok\x1b[0m"
	case "warn":
		return "\x1b[1;93mwarn\x1b[0m"
	default:
		return "\x1b[1;91m" + status + "\x1b[0m"
	}
}

func usageSeverity(r usage.UsageResult) string {
	if !strings.EqualFold(r.Unit, "percent") {
		return "UNKNOWN"
	}
	maxV := -1.0
	if v, ok := strconvParseSafe(r.Used); ok {
		maxV = v
	}
	for _, raw := range r.Buckets {
		if v, ok := strconvParseSafe(raw); ok && v > maxV {
			maxV = v
		}
	}
	if maxV < 0 {
		return "UNKNOWN"
	}
	if maxV >= 95 {
		return "CRIT"
	}
	if maxV >= 85 {
		return "WARN"
	}
	return "OK"
}

func sortMonitorResults(results []usage.UsageResult, sortBy string, desc bool) []usage.UsageResult {
	out := make([]usage.UsageResult, len(results))
	copy(out, results)
	k := strings.ToLower(strings.TrimSpace(sortBy))
	if k == "" {
		k = "provider"
	}
	getMetric := func(r usage.UsageResult, metric string) float64 {
		switch metric {
		case "used":
			if v, ok := strconvParseSafe(r.Used); ok {
				return v
			}
		case "5h", "7d":
			if raw, ok := r.Buckets[metric]; ok {
				if v, ok := strconvParseSafe(raw); ok {
					return v
				}
			}
		}
		return -1
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch k {
		case "used", "5h", "7d":
			av := getMetric(a, k)
			bv := getMetric(b, k)
			if av == bv {
				less = strings.ToLower(a.Provider) < strings.ToLower(b.Provider)
			} else {
				less = av < bv
			}
		default:
			less = strings.ToLower(a.Provider) < strings.ToLower(b.Provider)
		}
		if desc {
			return !less
		}
		return less
	})
	return out
}

func usageRemaining(raw string, unit string) (string, bool) {
	if !strings.EqualFold(unit, "percent") {
		return "", false
	}
	f, err := strconvParse(raw)
	if err != nil {
		return "", false
	}
	rem := 100 - f
	if rem < 0 {
		rem = 0
	}
	return fmt.Sprintf("%.1f", rem), true
}

func bucketVal(r usage.UsageResult, key, mode string) string {
	v := "-"
	if x, ok := r.Buckets[key]; ok && x != "" {
		v = x
	}
	if mode == "remaining" {
		if rem, ok := usageRemaining(v, r.Unit); ok {
			v = rem
		}
	}
	if strings.EqualFold(r.Unit, "percent") && v != "-" {
		v += "%"
	}
	return v
}

func strconvParse(s string) (float64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	return strconv.ParseFloat(s, 64)
}

func strconvParseSafe(s string) (float64, bool) {
	v, err := strconvParse(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.Flags().Duration("interval", 30*time.Second, "refresh interval")
	monitorCmd.Flags().String("state-path", "", "snapshot file path (default: ~/.oct/state/usage-latest.json)")
	monitorCmd.Flags().Bool("once", false, "run one cycle and exit")
	monitorCmd.Flags().String("sort-by", "provider", "sort key: provider|used|5h|7d")
	monitorCmd.Flags().Bool("desc", false, "sort descending")
	monitorCmd.Flags().Int("top", 0, "show top N providers (0=all)")
	monitorCmd.Flags().Bool("compact", false, "compact monitor output")
}
