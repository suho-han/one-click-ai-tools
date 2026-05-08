package cmd

import (
	"fmt"
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
			printMonitorScreen(results, now)
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

func printMonitorScreen(results []usage.UsageResult, now time.Time) {
	fmt.Print("\033[H\033[2J") // clear screen
	fmt.Printf("oct monitor  |  %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("-", 88))
	fmt.Printf("%-14s %-8s %-8s %-10s %-10s %-8s %-7s %s\n", "provider", "5h", "7d", "used", "limit", "source", "status", "message")

	mode := strings.ToLower(strings.TrimSpace(viper.GetString("usage_display_mode")))
	if mode != "used" && mode != "remaining" {
		mode = "used"
	}

	for _, r := range results {
		five := bucketVal(r, "5h", mode)
		seven := bucketVal(r, "7d", mode)
		u := r.Used
		if mode == "remaining" {
			if rem, ok := usageRemaining(r.Used, r.Unit); ok {
				u = rem
			}
		}
		msg := r.Message
		if len(msg) > 32 {
			msg = msg[:32] + "..."
		}
		fmt.Printf("%-14s %-8s %-8s %-10s %-10s %-8s %-7s %s\n", r.Provider, five, seven, u, r.Limit, r.Source, r.Status, msg)
	}

	fmt.Println()
	fmt.Printf("snapshot: %s\n", usage.DefaultSnapshotPath())
	fmt.Println("Ctrl+C to stop")
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

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.Flags().Duration("interval", 30*time.Second, "refresh interval")
	monitorCmd.Flags().String("state-path", "", "snapshot file path (default: ~/.oct/state/usage-latest.json)")
	monitorCmd.Flags().Bool("once", false, "run one cycle and exit")
}
