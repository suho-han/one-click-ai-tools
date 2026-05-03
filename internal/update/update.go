package update

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
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
	var toolsToUpdate []Tool

	if len(enabledTools) == 0 {
		toolsToUpdate = orderedTools
	} else {
		for _, et := range enabledTools {
			for _, t := range orderedTools {
				if strings.EqualFold(et, t.BinaryName) {
					toolsToUpdate = append(toolsToUpdate, t)
					break
				}
			}
		}
	}

	if len(toolsToUpdate) == 0 {
		fmt.Println("No tools selected for update.")
		return nil
	}

	// Run brew update once if any tool might use brew
	if runtime.GOOS == "darwin" {
		fmt.Println("Updating Homebrew...")
		exec.Command("brew", "update").Run()
	}

	total := len(toolsToUpdate)
	fmt.Printf("Updating %d tools...\n", total)

	g, _ := errgroup.WithContext(context.Background())
	var mu sync.Mutex
	count := 0

	for _, t := range toolsToUpdate {
		t := t // capture range variable
		g.Go(func() error {
			manager := DetectManager(t)

			mu.Lock()
			count++
			current := count

			// Match config icon layout: 3 lines, 1:1 aspect ratio.
			lines := ui.InlineIconLines(t.LobeIcon, 6, 3)
			if len(lines) >= 3 {
				// Each tool block takes 3 lines. We lock the print to avoid interleaving.
				fmt.Printf("      %s\n", lines[0])
				fmt.Printf("[%d/%d] %s %s: Detecting manager... (using %s)\n", current, total, lines[1], t.Colorize(t.Name), manager)
				fmt.Printf("      %s\n", lines[2])
			} else {
				fmt.Printf("[%d/%d] %s: Detecting manager... (using %s)\n", current, total, t.Colorize(t.Name), manager)
			}
			mu.Unlock()

			start := time.Now()
			cmd := manager.InstallCommand(t)

			output, err := cmd.CombinedOutput()
			duration := time.Since(start).Round(time.Second)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Printf("[%s] ✗ Failed after %v: %v\nOutput: %s\n", t.Colorize(t.Name), duration, err, string(output))
				return err
			}

			fmt.Printf("[%s] ✓ Updated successfully in %v\n", t.Colorize(t.Name), duration)
			return nil
		})
	}

	err := g.Wait()
	if err == nil {
		fmt.Println("\nAll tools updated successfully!")
	}
	return err
}
