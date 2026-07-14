package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

type MacOS struct {
	LabelPrefix string
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
        <string>{{.Command}}</string>
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
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
</dict>
</plist>`

func (m *MacOS) Enable(task Task, interval string, hour int) error {
	cfg, err := taskDetails(task)
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	binPath := resolveBinaryPath()

	logPath := filepath.Join(home, ".oct", "logs", cfg.LogFile)
	data := struct {
		Label      string
		BinaryPath string
		Command    string
		Interval   string
		Hour       int
		LogPath    string
	}{
		Label:      launchAgentLabel(m.LabelPrefix, task),
		BinaryPath: binPath,
		Command:    cfg.Command,
		Interval:   interval,
		Hour:       hour,
		LogPath:    logPath,
	}

	plistPath := launchAgentPath(home, m.LabelPrefix, task)
	os.MkdirAll(filepath.Join(home, ".oct", "logs"), 0o755)
	os.MkdirAll(filepath.Dir(plistPath), 0o755)

	f, err := os.Create(plistPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpl, _ := template.New("plist").Parse(plistTemplate)
	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	exec.Command("launchctl", "unload", plistPath).Run()
	cmd := exec.Command("launchctl", "load", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load failed: %v, output: %s", err, string(output))
	}

	return nil
}

func (m *MacOS) Disable(task Task) error {
	home, _ := os.UserHomeDir()
	plistPath := launchAgentPath(home, m.LabelPrefix, task)
	exec.Command("launchctl", "unload", plistPath).Run()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *MacOS) Status(task Task) (string, error) {
	cmd := exec.Command("launchctl", "list", launchAgentLabel(m.LabelPrefix, task))
	if err := cmd.Run(); err == nil {
		return "enabled", nil
	}
	return "disabled", nil
}

func launchAgentLabel(prefix string, task Task) string {
	cfg, err := taskDetails(task)
	if err != nil {
		return prefix + ".unknown"
	}
	return prefix + "." + cfg.LabelSuffix
}

func launchAgentPath(home, prefix string, task Task) string {
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel(prefix, task)+".plist")
}
