//go:build darwin && cgo

package cmd

import (
	"context"
	"os/exec"
	"time"

	"github.com/getlantern/systray"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

func selectedMenubarToolNames() []string {
	// Standalone providers ride in the same usage results as installable
	// tools; give them loading rows too so the dropdown matches what a
	// refresh actually returns instead of silently dropping their lines.
	standalone := usage.SelectedStandaloneNames()
	tools := usage.SelectedTools()
	if len(tools) == 0 && len(standalone) == 0 {
		return []string{"No enabled providers"}
	}
	names := make([]string, 0, len(tools)+len(standalone))
	for _, tool := range tools {
		name := tool.Name
		if name == "" {
			name = tool.BinaryName
		}
		names = append(names, name)
	}
	return append(names, standalone...)
}

func (ui *menubarUI) run() {
	ticker := time.NewTicker(ui.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			go ui.refreshUsage()
		case <-ui.refreshItem.ClickedCh:
			go ui.refreshUsage()
		case <-ui.usageItem.ClickedCh:
			go ui.openInTerminal(ui.command("usage"))
		case <-ui.sessionRefreshItem.ClickedCh:
			go ui.openInTerminal(ui.command("session-refresh"))
		case <-ui.monitorItem.ClickedCh:
			go ui.openInTerminal(ui.command("monitor", "--once"))
		case <-ui.alertItem.ClickedCh:
			go ui.openInTerminal(ui.command("usage", "--notify"))
		case <-ui.quitItem.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (ui *menubarUI) command(args ...string) string {
	return buildMenubarExecCommand(ui.execPath, args...)
}

func (ui *menubarUI) refreshUsage() {
	ui.mu.Lock()
	if ui.refreshing {
		ui.mu.Unlock()
		return
	}
	ui.refreshing = true
	ui.mu.Unlock()

	ui.refreshItem.SetTitle("Refreshing…")
	ui.refreshItem.Disable()
	ui.applySnapshot(buildMenubarLoadingSnapshot(ui.toolNames))

	results, err := menubarFetchUsage(ui.ctx)
	now := time.Now()
	if err != nil {
		ui.applySnapshot(buildMenubarErrorSnapshot(ui.toolNames, now, err))
	} else {
		ui.applySnapshot(buildMenubarUsageSnapshot(results, now))
	}

	ui.refreshItem.SetTitle("Refresh now")
	ui.refreshItem.Enable()
	ui.mu.Lock()
	ui.refreshing = false
	ui.mu.Unlock()
}

func (ui *menubarUI) applySnapshot(snapshot menubarSnapshot) {
	systray.SetTitle(snapshot.Title)
	systray.SetTooltip(snapshot.Tooltip)
	ui.statusItem.SetTitle(snapshot.SummaryLine)
	ui.updatedItem.SetTitle(snapshot.UpdatedLine)
	ui.autoRefreshItem.SetTitle(menubarAutoRefreshLabel(ui.refreshInterval))
	ui.nextRefreshItem.SetTitle(menubarNextRefreshLabel(snapshot.LastRefreshAt, ui.refreshInterval))
	ui.providersLabelItem.SetTitle(menubarProviderSectionTitle(len(snapshot.ProviderLines)))

	for i := range ui.providerGroups {
		group := &ui.providerGroups[i]
		if i < len(snapshot.ProviderLines) {
			group.summary.SetTitle(snapshot.ProviderLines[i])
			group.summary.Show()
			if i < len(snapshot.ProviderDetails) && len(snapshot.ProviderDetails[i]) > 0 {
				group.summary.Enable()
				ui.syncProviderDetails(group, snapshot.ProviderDetails[i])
			} else {
				group.summary.Disable()
				ui.syncProviderDetails(group, nil)
			}
			continue
		}
		group.summary.Hide()
		ui.syncProviderDetails(group, nil)
	}
}

func (ui *menubarUI) syncProviderDetails(group *menubarProviderGroup, details []string) {
	for len(group.details) < len(details) {
		child := group.summary.AddSubMenuItem("", "Provider detail")
		child.Disable()
		group.details = append(group.details, child)
	}
	for i, child := range group.details {
		if i < len(details) {
			child.SetTitle(details[i])
			child.Show()
			child.Disable()
			continue
		}
		child.Hide()
	}
}

// openInTerminal launches a CLI action in Terminal.app. The menubar has no
// stderr, so a failed launch (denied automation permission, timeout) is
// surfaced through the tooltip instead of being swallowed.
func (ui *menubarUI) openInTerminal(command string) {
	if err := runInTerminal(command); err != nil {
		systray.SetTooltip(truncateMenubarText("oct: "+err.Error(), 64))
	}
}

// runInTerminalTimeout bounds osascript: it can otherwise block indefinitely
// on an unanswered automation-permission dialog.
var runInTerminalTimeout = 30 * time.Second

func runInTerminal(command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), runInTerminalTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "osascript", "-e", buildTerminalAppleScript(command)).Run()
}
