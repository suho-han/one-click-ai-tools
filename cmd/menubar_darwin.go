//go:build darwin && cgo

package cmd

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/usage"
)

var menubarFetchUsage = usage.GetUsage

type menubarProviderGroup struct {
	summary *systray.MenuItem
	details []*systray.MenuItem
}

func runMenubar() error {
	systray.Run(onMenubarReady, func() {})
	return nil
}

func startMenubarDetached() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(execPath, "menubar")
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

type menubarUI struct {
	execPath             string
	toolNames            []string
	refreshInterval      time.Duration
	overviewItem         *systray.MenuItem
	statusItem           *systray.MenuItem
	updatedItem          *systray.MenuItem
	autoRefreshItem      *systray.MenuItem
	providersLabelItem   *systray.MenuItem
	providerGroups       []menubarProviderGroup
	openLabelItem        *systray.MenuItem
	refreshItem          *systray.MenuItem
	usageItem            *systray.MenuItem
	maintenanceLabelItem *systray.MenuItem
	sessionRefreshItem   *systray.MenuItem
	monitorItem          *systray.MenuItem
	alertItem            *systray.MenuItem
	quitItem             *systray.MenuItem
	mu                   sync.Mutex
	refreshing           bool
}

func onMenubarReady() {
	ui, err := newMenubarUI()
	if err != nil {
		systray.SetTitle("oct !!")
		systray.SetTooltip("one-click-tools menubar init failed")
		mErr := systray.AddMenuItem("menubar init failed: "+truncateMenubarText(err.Error(), 48), "Initialization error")
		mErr.Disable()
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit menubar")
		go func() {
			<-mQuit.ClickedCh
			systray.Quit()
		}()
		return
	}

	ui.applySnapshot(buildMenubarLoadingSnapshot(ui.toolNames))
	go ui.run()
	go ui.refreshUsage()
}

func newMenubarUI() (*menubarUI, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	toolNames := selectedMenubarToolNames()
	refreshInterval := menubarRefreshInterval(viper.GetString("menubar_refresh_interval"))
	systray.SetTitle("oct …")
	systray.SetTooltip("one-click-tools menubar")

	ui := &menubarUI{
		execPath:        execPath,
		toolNames:       toolNames,
		refreshInterval: refreshInterval,
	}
	ui.overviewItem = systray.AddMenuItem(menubarOverviewTitle(), "Menubar overview")
	ui.overviewItem.Disable()
	systray.AddSeparator()
	ui.statusItem = systray.AddMenuItem("Loading usage…", "Current usage summary")
	ui.statusItem.Disable()
	ui.updatedItem = systray.AddMenuItem("Last refresh: -", "Last refresh time")
	ui.updatedItem.Disable()
	ui.autoRefreshItem = systray.AddMenuItem(menubarAutoRefreshLabel(refreshInterval), "Automatic refresh interval")
	ui.autoRefreshItem.Disable()

	systray.AddSeparator()
	ui.providersLabelItem = systray.AddMenuItem(menubarProviderSectionTitle(len(toolNames)), "Provider section")
	ui.providersLabelItem.Disable()
	loadingSnapshot := buildMenubarLoadingSnapshot(toolNames)
	ui.providerGroups = make([]menubarProviderGroup, 0, len(toolNames))
	for i, name := range toolNames {
		summary := name + " · loading…"
		details := []string{"Provider: " + name, "Status: loading"}
		if i < len(loadingSnapshot.ProviderLines) {
			summary = loadingSnapshot.ProviderLines[i]
		}
		if i < len(loadingSnapshot.ProviderDetails) {
			details = loadingSnapshot.ProviderDetails[i]
		}
		group := menubarProviderGroup{summary: systray.AddMenuItem(summary, "Provider status details")}
		group.summary.Enable()
		for _, detail := range details {
			child := group.summary.AddSubMenuItem(detail, "Provider detail")
			child.Disable()
			group.details = append(group.details, child)
		}
		ui.providerGroups = append(ui.providerGroups, group)
	}
	if len(toolNames) > 0 {
		systray.AddSeparator()
	}

	ui.openLabelItem = systray.AddMenuItem("Open", "Navigation actions")
	ui.openLabelItem.Disable()
	ui.refreshItem = systray.AddMenuItem("Refresh now", "Refresh usage summary")
	ui.usageItem = systray.AddMenuItem("Open Usage", "Run current oct binary: usage")
	ui.monitorItem = systray.AddMenuItem("Open Monitor", "Run current oct binary: monitor --once")
	systray.AddSeparator()
	ui.maintenanceLabelItem = systray.AddMenuItem("Maintenance", "Maintenance actions")
	ui.maintenanceLabelItem.Disable()
	ui.sessionRefreshItem = systray.AddMenuItem("Run Session Refresh", "Run current oct binary: session-refresh")
	ui.alertItem = systray.AddMenuItem("Run Alert Check", "Run current oct binary: usage --notify")
	systray.AddSeparator()
	ui.quitItem = systray.AddMenuItem("Quit", "Quit menubar")
	return ui, nil
}

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
