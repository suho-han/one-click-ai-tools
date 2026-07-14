//go:build darwin && cgo

package cmd

import (
	"os/exec"
	"time"

	"github.com/getlantern/systray"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

func selectedMenubarToolNames() []string {
	tools := usage.SelectedTools()
	if len(tools) == 0 {
		return []string{"No enabled providers"}
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		name := tool.Name
		if name == "" {
			name = tool.BinaryName
		}
		names = append(names, name)
	}
	return names
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
			_ = runInTerminal(ui.command("usage"))
		case <-ui.sessionRefreshItem.ClickedCh:
			_ = runInTerminal(ui.command("session-refresh"))
		case <-ui.monitorItem.ClickedCh:
			_ = runInTerminal(ui.command("monitor", "--once"))
		case <-ui.alertItem.ClickedCh:
			_ = runInTerminal(ui.command("usage", "--notify"))
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

	results, err := menubarFetchUsage()
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

func runInTerminal(command string) error {
	return exec.Command("osascript", "-e", buildTerminalAppleScript(command)).Run()
}
