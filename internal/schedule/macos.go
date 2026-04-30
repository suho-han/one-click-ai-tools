package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

type MacOS struct {
	Label string
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>agent-update</string>
    </array>
    <key>StartCalendarInterval</key>
    {{if eq .Interval "weekly"}}
    <dict>
        <key>Hour</key><integer>{{.Hour}}</integer>
        <key>Minute</key><integer>0</integer>
        <key>Weekday</key><integer>1</integer>
    </dict>
    {{else}}
    <dict>
        <key>Hour</key><integer>{{.Hour}}</integer>
        <key>Minute</key><integer>0</integer>
    </dict>
    {{end}}
    <key>StandardOutPath</key>
    <string>{{.Home}}/.oct/logs/schedule.log</string>
    <key>StandardErrorPath</key>
    <string>{{.Home}}/.oct/logs/schedule.log</string>
</dict>
</plist>`

func (m *MacOS) Enable(interval string, hour int) error {
	home, _ := os.UserHomeDir()
	binPath, err := exec.LookPath("oct")
	if err != nil {
		// Fallback to absolute path of current executable if 'oct' not in PATH
		binPath, _ = os.Executable()
	}

	data := struct {
		Label      string
		BinaryPath string
		Interval   string
		Hour       int
		Home       string
	}{
		Label:      m.Label,
		BinaryPath: binPath,
		Interval:   interval,
		Hour:       hour,
		Home:       home,
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", m.Label+".plist")
	
	// Ensure log directory exists
	os.MkdirAll(filepath.Join(home, ".oct", "logs"), 0755)

	f, err := os.Create(plistPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpl, _ := template.New("plist").Parse(plistTemplate)
	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	// Unload if already loaded
	exec.Command("launchctl", "unload", plistPath).Run()
	
	// Load
	cmd := exec.Command("launchctl", "load", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load failed: %v, output: %s", err, string(output))
	}

	return nil
}

func (m *MacOS) Disable() error {
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", m.Label+".plist")
	
	exec.Command("launchctl", "unload", plistPath).Run()
	return os.Remove(plistPath)
}

func (m *MacOS) Status() (string, error) {
	cmd := exec.Command("launchctl", "list", m.Label)
	if err := cmd.Run(); err == nil {
		return "enabled", nil
	}
	return "disabled", nil
}
