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

	var toolsToUpdate []Tool
	if len(enabledTools) == 0 {
		toolsToUpdate = Tools
	} else {
		for _, et := range enabledTools {
			for _, t := range Tools {
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
			// Print high-density logo if supported
			if t.LobeIcon != "" {
				ui.PrintIcon(t.LobeIcon, 32)
			}
			icon := ui.InlineIcon(t.LobeIcon, 6)
			if icon != "" {
				icon = icon + " "
			}
			fmt.Printf("[%d/%d] %s%s: Detecting manager... (using %s)\n", current, total, icon, t.Colorize(t.Name), manager)
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
