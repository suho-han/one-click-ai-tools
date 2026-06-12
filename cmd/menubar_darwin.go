//go:build darwin && cgo

package cmd

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/suho-han/one-click-tools/internal/usage"
)

var menubarFetchUsage = usage.GetUsage

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
	execPath           string
	toolNames          []string
	statusItem         *systray.MenuItem
	updatedItem        *systray.MenuItem
	providerItems      []*systray.MenuItem
	refreshItem        *systray.MenuItem
	usageItem          *systray.MenuItem
	sessionRefreshItem *systray.MenuItem
	monitorItem        *systray.MenuItem
	quitItem           *systray.MenuItem
	mu                 sync.Mutex
	refreshing         bool
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
	systray.SetTitle("oct …")
	systray.SetTooltip("one-click-tools menubar")

	ui := &menubarUI{
		execPath:  execPath,
		toolNames: toolNames,
	}
	ui.statusItem = systray.AddMenuItem("Loading usage…", "Current usage summary")
	ui.statusItem.Disable()
	ui.updatedItem = systray.AddMenuItem("Last refresh: -", "Last refresh time")
	ui.updatedItem.Disable()
	ui.providerItems = make([]*systray.MenuItem, 0, len(toolNames))

	systray.AddSeparator()
	for _, name := range toolNames {
		item := systray.AddMenuItem(name+" · loading…", "Provider status")
		item.Disable()
		ui.providerItems = append(ui.providerItems, item)
	}
	if len(toolNames) > 0 {
		systray.AddSeparator()
	}

	ui.refreshItem = systray.AddMenuItem("Refresh now", "Refresh usage summary")
	ui.usageItem = systray.AddMenuItem("Open Usage", "Run current oct binary: usage")
	ui.sessionRefreshItem = systray.AddMenuItem("Run Session Refresh", "Run current oct binary: session-refresh")
	ui.monitorItem = systray.AddMenuItem("Open Monitor", "Run current oct binary: monitor --once")
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
	for {
		select {
		case <-ui.refreshItem.ClickedCh:
			go ui.refreshUsage()
		case <-ui.usageItem.ClickedCh:
			_ = runInTerminal(ui.command("usage"))
		case <-ui.sessionRefreshItem.ClickedCh:
			_ = runInTerminal(ui.command("session-refresh"))
		case <-ui.monitorItem.ClickedCh:
			_ = runInTerminal(ui.command("monitor", "--once"))
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
	for i, item := range ui.providerItems {
		if i < len(snapshot.ProviderLines) {
			item.SetTitle(snapshot.ProviderLines[i])
			item.Show()
			item.Disable()
			continue
		}
		item.Hide()
	}
}

func runInTerminal(command string) error {
	return exec.Command("osascript", "-e", buildTerminalAppleScript(command)).Run()
}
