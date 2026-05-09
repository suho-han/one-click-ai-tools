package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	g := new(errgroup.Group)
	ctx := context.Background()
	var mu sync.Mutex
	count := 0
	failureCount := 0

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
			output, err := runInstallWithFallback(ctx, manager, t)
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
				failureCount++
				return nil
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

	_ = g.Wait()
	if failureCount == 0 {
		fmt.Println("\nAll tools updated successfully!")
		return nil
	}
	return fmt.Errorf("%w: %d tool(s) failed", errors.New("update failed"), failureCount)
}

func anyBrewManaged(tools []Tool) bool {
	for _, t := range tools {
		if DetectManager(t) == Brew {
			return true
		}
	}
	return false
}

func runInstallWithFallback(ctx context.Context, manager Manager, t Tool) ([]byte, error) {
	if manager != Npm {
		cmd := manager.InstallCommandCtx(ctx, t)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return output, nil
		}
		return output, err
	}

	output, err := runNpmInstallRecoveringConflicts(ctx, t.Package, "")
	if err == nil {
		return output, nil
	}
	if isNpmConflictError(string(output)) {
		forceOut, forceErr := runNpmInstallRecoveringConflicts(ctx, t.Package, "", "--force")
		if forceErr == nil {
			return forceOut, nil
		}
		if !isNpmPermissionError(string(forceOut)) {
			return forceOut, forceErr
		}
		return runLocalNpmInstall(ctx, t, forceOut, forceErr)
	}
	if !isNpmPermissionError(string(output)) {
		return output, err
	}
	return runLocalNpmInstall(ctx, t, output, err)
}

func runLocalNpmInstall(ctx context.Context, t Tool, originalOutput []byte, originalErr error) ([]byte, error) {
	home, homeErr := os.UserHomeDir()
	if homeErr != nil || strings.TrimSpace(home) == "" {
		return originalOutput, originalErr
	}

	localPrefix := home + "/.local"
	fallbackOut, fallbackErr := runNpmInstallRecoveringConflicts(ctx, t.Package, localPrefix)
	if fallbackErr != nil {
		if isNpmConflictError(string(fallbackOut)) {
			return runNpmInstallRecoveringConflicts(ctx, t.Package, localPrefix, "--force")
		}
		return fallbackOut, fallbackErr
	}
	return fallbackOut, nil
}

func isNpmPermissionError(output string) bool {
	o := strings.ToLower(output)
	return strings.Contains(o, "npm err! code eacces") || strings.Contains(o, "permission denied")
}

func isNpmConflictError(output string) bool {
	o := strings.ToLower(output)
	return strings.Contains(o, "npm err! code eexist") ||
		strings.Contains(o, "file already exists") ||
		strings.Contains(o, "npm err! code enotempty") ||
		strings.Contains(o, "directory not empty")
}

func runNpmInstall(ctx context.Context, pkg, prefix string, extraArgs ...string) ([]byte, error) {
	args := []string{"install", "-g"}
	if prefix != "" {
		args = append(args, "--prefix", prefix)
	}
	args = append(args, extraArgs...)
	args = append(args, pkg)
	cmd := exec.CommandContext(ctx, "npm", args...)
	return cmd.CombinedOutput()
}

func runNpmInstallRecoveringConflicts(ctx context.Context, pkg, prefix string, extraArgs ...string) ([]byte, error) {
	output, err := runNpmInstall(ctx, pkg, prefix, extraArgs...)
	if err == nil || !isNpmConflictError(string(output)) {
		return output, err
	}
	if !cleanupNpmConflictDest(string(output)) {
		return output, err
	}
	return runNpmInstall(ctx, pkg, prefix, extraArgs...)
}

func cleanupNpmConflictDest(output string) bool {
	dest := extractNpmDestPath(output)
	if dest == "" {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return false
	}
	if !shouldRemoveNpmConflictDest(dest, home) {
		return false
	}
	return os.RemoveAll(dest) == nil
}

func extractNpmDestPath(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "npm ERR! dest "):
			return strings.TrimSpace(strings.TrimPrefix(line, "npm ERR! dest "))
		case strings.HasPrefix(line, "error dest "):
			return strings.TrimSpace(strings.TrimPrefix(line, "error dest "))
		}
	}
	return ""
}

func shouldRemoveNpmConflictDest(dest, home string) bool {
	cleanDest := filepath.Clean(dest)
	cleanHome := filepath.Clean(home)
	if cleanDest == "." || cleanHome == "." {
		return false
	}
	if !strings.HasPrefix(cleanDest, cleanHome+string(os.PathSeparator)) {
		return false
	}
	if !strings.Contains(cleanDest, string(os.PathSeparator)+"node_modules"+string(os.PathSeparator)) {
		return false
	}
	return strings.HasPrefix(filepath.Base(cleanDest), ".")
}
