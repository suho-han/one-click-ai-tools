package schedule

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type launchAgentTemplateData struct {
	Label                string
	BinaryPath           string
	Command              string
	Interval             string
	Hour                 int
	StartIntervalSeconds int
	LogPath              string
}

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
    {{if gt .StartIntervalSeconds 0}}
    <key>StartInterval</key>
    <integer>{{.StartIntervalSeconds}}</integer>
    {{else}}
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
    {{end}}
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
</dict>
</plist>`

func (m *MacOS) Enable(task Task, interval string, hour int) error {
	interval, err := validateScheduleTiming(interval, hour)
	if err != nil {
		return err
	}
	cfg, err := taskDetails(task)
	if err != nil {
		return err
	}

	home, err := homeDirPath()
	if err != nil {
		return err
	}
	binPath := resolveBinaryPath()

	logPath := filepath.Join(home, ".oct", "logs", cfg.LogFile)
	data := launchAgentTemplateData{
		Label:                launchAgentLabel(m.LabelPrefix, task),
		BinaryPath:           binPath,
		Command:              cfg.Command,
		Interval:             interval,
		Hour:                 hour,
		StartIntervalSeconds: startIntervalSeconds(interval),
		LogPath:              logPath,
	}

	plistPath := launchAgentPath(home, m.LabelPrefix, task)
	os.MkdirAll(filepath.Join(home, ".oct", "logs"), 0o755)
	os.MkdirAll(filepath.Dir(plistPath), 0o755)

	f, err := os.Create(plistPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := renderLaunchAgentPlist(f, data); err != nil {
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
	home, err := homeDirPath()
	if err != nil {
		return err
	}
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

func renderLaunchAgentPlist(w io.Writer, data launchAgentTemplateData) error {
	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

func startIntervalSeconds(interval string) int {
	switch interval {
	case TwelveHourInterval:
		return 12 * 60 * 60
	case SixHourInterval:
		return 6 * 60 * 60
	case OneHourInterval:
		return 60 * 60
	default:
		return 0
	}
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
