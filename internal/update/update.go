package update

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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

	fmt.Printf("Updating %d tools...\n", len(toolsToUpdate))

	g, _ := errgroup.WithContext(context.Background())

	for _, t := range toolsToUpdate {
		t := t // capture range variable
		g.Go(func() error {
			return updateTool(t)
		})
	}

	return g.Wait()
}

func updateTool(t Tool) error {
	fmt.Printf("[%s] Starting update...\n", t.Name)
	
	// For now, assuming npm is used as in the bash version
	cmd := exec.Command("npm", "install", "-g", t.Package)
	
	// In a real implementation, we might want to capture output and format it nicely
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[%s] Error: %v\nOutput: %s\n", t.Name, err, string(output))
		return err
	}

	fmt.Printf("[%s] Successfully updated.\n", t.Name)
	return nil
}
