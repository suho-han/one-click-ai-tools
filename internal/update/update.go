package update

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/ui"
	"golang.org/x/sync/errgroup"
)

var brewInstallMu sync.Mutex

type Options struct {
	DryRun  bool
	Explain bool
	Input   io.Reader
	Output  io.Writer
}

type Plan struct {
	Tool           Tool
	Manager        Manager
	Reason         string
	ActiveBinary   string
	ActivePath     string
	VersionBefore  string
	InstallCommand []string
}

var confirmInstallPrompt = defaultConfirmInstallPrompt

func Run(opts ...Options) error {
	config := Options{}
	if len(opts) > 0 {
		config = opts[0]
	}
	out := config.Output
	if out == nil {
		out = os.Stdout
	}
	in := config.Input
	if in == nil {
		in = os.Stdin
	}

	enabledTools := viper.GetStringSlice("enabled_tools")
	agentOrder := viper.GetStringSlice("agent_order")

	orderedTools := GetOrderedTools(agentOrder)
	toolsToUpdate := GetFilteredTools(enabledTools, orderedTools)

	if len(toolsToUpdate) == 0 {
		fmt.Fprintln(out, "No tools selected for update.")
		return nil
	}

	plans := ExplainPlans(toolsToUpdate)
	if config.Explain || config.DryRun {
		printPlans(out, plans)
	}
	if config.DryRun {
		fmt.Fprintf(out, "\nDry-run complete: %d tool(s) planned, 0 executed.\n", len(plans))
		return nil
	}

	confirmedTools, confirmedPlans, err := confirmMissingToolInstalls(in, out, toolsToUpdate, plans)
	if err != nil {
		return err
	}
	toolsToUpdate = confirmedTools
	plans = confirmedPlans
	if len(toolsToUpdate) == 0 {
		fmt.Fprintln(out, "No tools selected for update.")
		return nil
	}

	if runtime.GOOS == "darwin" && anyBrewManaged(toolsToUpdate) {
		fmt.Fprintln(out, "Updating Homebrew...")
		if brewOut, err := commandWithEnv("brew", "update").CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "brew update failed: %v\n%s\n", err, brewOut)
		}
	}

	total := len(toolsToUpdate)
	fmt.Fprintf(out, "Updating %d tools...\n", total)

	g := new(errgroup.Group)
	ctx := context.Background()
	var mu sync.Mutex
	count := 0
	failureCount := 0

	for _, t := range toolsToUpdate {
		tool := t
		g.Go(func() error {
			manager := ResolveManagerForInstall(tool)

			mu.Lock()
			count++
			current := count

			lines := ui.InlineIconLines(tool.LobeIcon, 6, 3)
			if len(lines) >= 3 {
				fmt.Fprintf(out, "      %s\n", lines[0])
				fmt.Fprintf(out, "[%d/%d] %s %s: Detecting manager... (using %s)\n", current, total, lines[1], tool.Colorize(tool.Name), manager)
				fmt.Fprintf(out, "      %s\n", lines[2])
			} else {
				fmt.Fprintf(out, "[%d/%d] %s: Detecting manager... (using %s)\n", current, total, tool.Colorize(tool.Name), manager)
			}
			mu.Unlock()

			versionBefore := manager.GetInstalledVersion(tool)
			start := time.Now()
			var output []byte
			var err error
			if manager == Brew {
				brewInstallMu.Lock()
				output, err = runInstallWithFallback(ctx, manager, tool)
				brewInstallMu.Unlock()
			} else {
				output, err = runInstallWithFallback(ctx, manager, tool)
			}
			duration := time.Since(start).Round(time.Second)
			versionAfter := manager.GetInstalledVersion(tool)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if manager.IsNoChangeOutput(string(output)) {
					fmt.Fprintf(out, "[%s] ✓ Already up to date in %v\n", tool.Colorize(tool.Name), duration)
					return nil
				}
				fmt.Fprintf(out, "[%s] ✗ Failed after %v: %v\nOutput: %s\n", tool.Colorize(tool.Name), duration, err, string(output))
				failureCount++
				return nil
			}

			alreadyUpToDate := (versionBefore != "" && versionBefore == versionAfter) || manager.IsNoChangeOutput(string(output))
			if alreadyUpToDate {
				fmt.Fprintf(out, "[%s] ✓ Already up to date in %v\n", tool.Colorize(tool.Name), duration)
			} else {
				fmt.Fprintf(out, "[%s] ✓ Updated successfully in %v\n", tool.Colorize(tool.Name), duration)
			}
			return nil
		})
	}

	_ = g.Wait()
	if failureCount == 0 {
		fmt.Fprintln(out, "\nAll tools updated successfully!")
		return nil
	}
	return fmt.Errorf("%w: %d tool(s) failed", errors.New("update failed"), failureCount)
}

func (p Plan) IsInstalled() bool {
	if strings.TrimSpace(p.ActivePath) != "" {
		return true
	}
	if strings.TrimSpace(p.VersionBefore) != "" {
		return true
	}
	return p.Reason == "active binary path" || p.Reason == "installed package lookup"
}

func confirmMissingToolInstalls(in io.Reader, out io.Writer, tools []Tool, plans []Plan) ([]Tool, []Plan, error) {
	confirmedTools := make([]Tool, 0, len(tools))
	confirmedPlans := make([]Plan, 0, len(plans))

	for i, plan := range plans {
		if plan.IsInstalled() {
			confirmedTools = append(confirmedTools, tools[i])
			confirmedPlans = append(confirmedPlans, plan)
			continue
		}

		install, err := confirmInstallPrompt(in, out, plan)
		if err != nil {
			return nil, nil, err
		}
		if !install {
			fmt.Fprintf(out, "Skipping %s (not installed).\n", plan.Tool.Name)
			continue
		}
		confirmedTools = append(confirmedTools, tools[i])
		confirmedPlans = append(confirmedPlans, plan)
	}

	return confirmedTools, confirmedPlans, nil
}

func defaultConfirmInstallPrompt(in io.Reader, out io.Writer, plan Plan) (bool, error) {
	cmd := strings.Join(plan.InstallCommand, " ")
	if cmd == "" {
		cmd = string(plan.Manager)
	}
	fmt.Fprintf(out, "%s is not installed. Install now?\nCommand: %s\n[Y/n]: ", plan.Tool.Name, cmd)

	reader := bufio.NewReader(in)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes", nil
}

func ExplainPlans(tools []Tool) []Plan {
	plans := make([]Plan, 0, len(tools))
	for _, tool := range tools {
		manager, reason := explainResolvedManager(tool)
		binary, path := firstResolvedBinary(tool)
		cmd := manager.InstallCommandCtx(context.Background(), tool)
		version := manager.GetInstalledVersion(tool)
		plans = append(plans, Plan{
			Tool:           tool,
			Manager:        manager,
			Reason:         reason,
			ActiveBinary:   binary,
			ActivePath:     path,
			VersionBefore:  version,
			InstallCommand: append([]string{cmd.Path}, cmd.Args[1:]...),
		})
	}
	return plans
}

func explainResolvedManager(t Tool) (Manager, string) {
	if m, ok := managerFromPackagePrefix(t.Package); ok {
		return m, "package prefix"
	}
	if isAntigravityTool(t) {
		return AntigravityInstaller, "tool-specific installer"
	}
	if isCursorTool(t) {
		return CursorAgent, "tool-specific installer"
	}
	if m, ok := detectManagerFromBinaryPath(t); ok {
		return m, "active binary path"
	}
	for _, manager := range []Manager{Brew, Pnpm, Yarn, Npm} {
		if matchesInstalledPackage(manager, t) {
			return manager, "installed package lookup"
		}
	}
	if m, ok := defaultManagerForTool(t); ok {
		return m, "default fallback"
	}
	return Unknown, "unknown"
}

func firstResolvedBinary(t Tool) (string, string) {
	for _, binary := range preferredBinaries(t) {
		if path, err := binaryLookup(binary); err == nil && strings.TrimSpace(path) != "" {
			return binary, path
		}
	}
	return "", ""
}

func printPlans(w io.Writer, plans []Plan) {
	fmt.Fprintln(w, "update plan")
	for _, plan := range plans {
		fmt.Fprintf(w, "- %s\n", plan.Tool.Name)
		fmt.Fprintf(w, "  manager: %s (%s)\n", plan.Manager, plan.Reason)
		if plan.ActiveBinary != "" {
			fmt.Fprintf(w, "  active binary: %s -> %s\n", plan.ActiveBinary, plan.ActivePath)
		}
		if plan.VersionBefore != "" {
			fmt.Fprintf(w, "  version before: %s\n", plan.VersionBefore)
		}
		fmt.Fprintf(w, "  package: %s\n", plan.Tool.Package)
		fmt.Fprintf(w, "  command: %s\n", strings.Join(plan.InstallCommand, " "))
	}
}

func anyBrewManaged(tools []Tool) bool {
	for _, t := range tools {
		if ResolveManagerForInstall(t) == Brew {
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
	cmd := commandContextWithEnv(ctx, "npm", args...)
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
