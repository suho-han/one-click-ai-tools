//go:build darwin && cgo

package cmd

import (
	"os"
	"os/exec"
	"strings"
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
	if !menubarLegacy {
		if launched, err := launchSwiftMenubarHelper(false); launched || err != nil {
			return err
		}
	}
	systray.Run(onMenubarReady, func() {})
	return nil
}

func startMenubarDetached() error {
	if !menubarLegacy {
		if launched, err := launchSwiftMenubarHelper(true); launched || err != nil {
			return err
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmdArgs := []string{"menubar"}
	if menubarLegacy {
		cmdArgs = append(cmdArgs, "--legacy")
	}
	cmd := exec.Command(execPath, cmdArgs...)
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

func menubarEnvironmentMap() map[string]string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		env[key] = value
	}
	return env
}

type menubarUI struct {
	execPath             string
	toolNames            []string
	refreshInterval      time.Duration
	overviewItem         *systray.MenuItem
	statusItem           *systray.MenuItem
	updatedItem          *systray.MenuItem
	autoRefreshItem      *systray.MenuItem
	nextRefreshItem      *systray.MenuItem
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
	ui.nextRefreshItem = systray.AddMenuItem(menubarNextRefreshLabel(time.Time{}, refreshInterval), "Next automatic refresh time")
	ui.nextRefreshItem.Disable()

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
