package update

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
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
			fmt.Printf("[%d/%d] %s: Detecting manager... (using %s)\n", current, total, t.Name, manager)
			mu.Unlock()

			start := time.Now()
			cmd := manager.InstallCommand(t)
			
			output, err := cmd.CombinedOutput()
			duration := time.Since(start).Round(time.Second)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Printf("[%s] ✗ Failed after %v: %v\nOutput: %s\n", t.Name, duration, err, string(output))
				return err
			}

			fmt.Printf("[%s] ✓ Updated successfully in %v\n", t.Name, duration)
			return nil
		})
	}

	err := g.Wait()
	if err == nil {
		fmt.Println("\nAll tools updated successfully!")
	}
	return err
}
