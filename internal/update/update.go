package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/ui"
	"golang.org/x/sync/errgroup"
)

func Run() error {
	enabledTools := viper.GetStringSlice("enabled_tools")
	agentOrder := viper.GetStringSlice("agent_order")

	orderedTools := GetOrderedTools(agentOrder)
	toolsToUpdate := GetFilteredTools(enabledTools, orderedTools)

	if len(toolsToUpdate) == 0 {
		fmt.Println("No tools selected for update.")
		return nil
	}

	if runtime.GOOS == "darwin" && anyBrewManaged(toolsToUpdate) {
		fmt.Println("Updating Homebrew...")
		if out, err := exec.Command("brew", "update").CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "brew update failed: %v\n%s\n", err, out)
		}
	}

	total := len(toolsToUpdate)
	fmt.Printf("Updating %d tools...\n", total)

	g, ctx := errgroup.WithContext(context.Background())
	var mu sync.Mutex
	count := 0

	for _, t := range toolsToUpdate {
		g.Go(func() error {
			manager := DetectManager(t)

			mu.Lock()
			count++
			current := count

			lines := ui.InlineIconLines(t.LobeIcon, 6, 3)
			if len(lines) >= 3 {
				fmt.Printf("      %s\n", lines[0])
				fmt.Printf("[%d/%d] %s %s: Detecting manager... (using %s)\n", current, total, lines[1], t.Colorize(t.Name), manager)
				fmt.Printf("      %s\n", lines[2])
			} else {
				fmt.Printf("[%d/%d] %s: Detecting manager... (using %s)\n", current, total, t.Colorize(t.Name), manager)
			}
			mu.Unlock()

			versionBefore := manager.GetInstalledVersion(t)
			start := time.Now()
			cmd := manager.InstallCommandCtx(ctx, t)

			output, err := cmd.CombinedOutput()
			duration := time.Since(start).Round(time.Second)
			versionAfter := manager.GetInstalledVersion(t)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				// brew upgrade exits non-zero when nothing to upgrade; treat that as up to date.
				if manager.IsNoChangeOutput(string(output)) {
					fmt.Printf("[%s] ✓ Already up to date in %v\n", t.Colorize(t.Name), duration)
					return nil
				}
				fmt.Printf("[%s] ✗ Failed after %v: %v\nOutput: %s\n", t.Colorize(t.Name), duration, err, string(output))
				return err
			}

			alreadyUpToDate := (versionBefore != "" && versionBefore == versionAfter) ||
				manager.IsNoChangeOutput(string(output))
			if alreadyUpToDate {
				fmt.Printf("[%s] ✓ Already up to date in %v\n", t.Colorize(t.Name), duration)
			} else {
				fmt.Printf("[%s] ✓ Updated successfully in %v\n", t.Colorize(t.Name), duration)
			}
			return nil
		})
	}

	err := g.Wait()
	if err == nil {
		fmt.Println("\nAll tools updated successfully!")
	}
	return err
}

func anyBrewManaged(tools []Tool) bool {
	for _, t := range tools {
		if DetectManager(t) == Brew {
			return true
		}
	}
	return false
}
