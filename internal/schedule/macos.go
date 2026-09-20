package schedule

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
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
    <string>{{xml .Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{xml .BinaryPath}}</string>
        <string>{{xml .Command}}</string>
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
    <string>{{xml .LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{xml .LogPath}}</string>
</dict>
</plist>`

// xmlEscape escapes text destined for XML string elements so paths containing
// & < > ' " cannot break the plist.
func xmlEscape(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return ""
	}
	return b.String()
}

func renderLaunchAgentPlist(w io.Writer, data launchAgentTemplateData) error {
	tmpl, err := template.New("plist").Funcs(template.FuncMap{
		"xml": xmlEscape,
	}).Parse(plistTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

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
	if err := os.MkdirAll(filepath.Join(home, ".oct", "logs"), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	var buf bytes.Buffer
	if err := renderLaunchAgentPlist(&buf, data); err != nil {
		return err
	}
	// Atomic write: launchctl must never load a half-written plist.
	if err := writeFileAtomic(plistPath, buf.Bytes(), 0o644); err != nil {
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

// launchctlNotFoundExit is the exit status launchctl returns when the given
// label is not loaded in the queried domain.
const launchctlNotFoundExit = 113

// realLaunchctlList reports whether the label is loaded. "Not loaded" is a
// normal outcome (false, nil); any other launchctl failure is surfaced as an
// error so it cannot masquerade as a disabled task.
func realLaunchctlList(label string) (bool, error) {
	cmd := exec.Command("launchctl", "list", label)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == launchctlNotFoundExit {
		return false, nil
	}
	if strings.Contains(stderr.String(), "Could not find service") {
		return false, nil
	}
	return false, fmt.Errorf("launchctl list %s failed: %v, output: %s", label, err, strings.TrimSpace(stderr.String()))
}

func (m *MacOS) Status(task Task) (string, error) {
	loaded, err := launchctlList(launchAgentLabel(m.LabelPrefix, task))
	if err != nil {
		return "", err
	}
	if loaded {
		return "enabled", nil
	}
	return "disabled", nil
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

// writeFileAtomic writes via a temp file + rename so launchctl can never
// observe a truncated plist after a crash mid-write.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
