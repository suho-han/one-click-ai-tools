//go:build darwin

package schedule

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// LaunchItem describes a launchd agent that runs at every login (RunAtLoad
// with no time trigger). The menubar daemon feature uses it to relaunch
// the Swift helper at login without keeping it alive: an explicit stop must
// win over launchd resurrecting the process.
type LaunchItem struct {
	Label            string
	ProgramArguments []string
	Environment      map[string]string
	LogPath          string
}

// LaunchItemState reports whether the item starts at login (Installed = the
// plist sits in ~/Library/LaunchAgents, which launchd loads at every login)
// and whether it is currently loaded. Loaded is incidental: an installed but
// unloaded item still comes back at the next login.
type LaunchItemState struct {
	Installed bool
	Loaded    bool
}

// launchctlRun shells out to launchctl; a package var so tests can capture
// load/unload calls without touching the user's launchd.
var launchctlRun = func(args ...string) (string, error) {
	cmd := exec.Command("launchctl", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

type launchItemEnvEntry struct {
	Key   string
	Value string
}

type launchItemTemplateData struct {
	Label            string
	ProgramArguments []string
	Environment      []launchItemEnvEntry
	LogPath          string
}

// RunAtLoad starts the program when the agent loads, which launchd does at
// every login — that replaces the time triggers the scheduled-task template
// always emits, so this template is separate rather than a template branch.
const launchItemPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{xml .Label}}</string>
    <key>ProgramArguments</key>
    <array>
{{range .ProgramArguments}}        <string>{{xml .}}</string>
{{end}}    </array>
{{if .Environment}}    <key>EnvironmentVariables</key>
    <dict>
{{range .Environment}}        <key>{{xml .Key}}</key>
        <string>{{xml .Value}}</string>
{{end}}    </dict>
{{end}}    <key>RunAtLoad</key>
    <true/>
{{if .LogPath}}    <key>StandardOutPath</key>
    <string>{{xml .LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{xml .LogPath}}</string>
{{end}}</dict>
</plist>`

func renderLaunchItemPlist(w io.Writer, item LaunchItem) error {
	// Sorted so the same item always renders byte-identical plists.
	env := make([]launchItemEnvEntry, 0, len(item.Environment))
	for key, value := range item.Environment {
		env = append(env, launchItemEnvEntry{Key: key, Value: value})
	}
	sort.Slice(env, func(i, j int) bool { return env[i].Key < env[j].Key })

	tmpl, err := template.New("launch-item-plist").Funcs(template.FuncMap{
		"xml": xmlEscape,
	}).Parse(launchItemPlistTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, launchItemTemplateData{
		Label:            item.Label,
		ProgramArguments: item.ProgramArguments,
		Environment:      env,
		LogPath:          item.LogPath,
	})
}

func launchItemPath(home, label string) string {
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

// InstallLaunchItem writes the RunAtLoad plist without touching launchd, so
// the program starts at the next login. Callers use it when the program is
// already running and an immediate load would create a second instance.
func InstallLaunchItem(item LaunchItem) error {
	_, err := installLaunchItem(item)
	return err
}

// EnableLaunchItem installs the item and loads it now; RunAtLoad starts the
// program immediately.
func EnableLaunchItem(item LaunchItem) error {
	plistPath, err := installLaunchItem(item)
	if err != nil {
		return err
	}
	// Best-effort unload first: load on an already-loaded label is a no-op,
	// so refreshing paths needs the old definition dropped. Not loaded is a
	// normal outcome, hence the ignored error.
	_, _ = launchctlRun("unload", plistPath)
	if output, err := launchctlRun("load", plistPath); err != nil {
		return fmt.Errorf("launchctl load failed: %v, output: %s", err, output)
	}
	return nil
}

func installLaunchItem(item LaunchItem) (string, error) {
	if strings.TrimSpace(item.Label) == "" {
		return "", errors.New("launch item label is required")
	}
	if len(item.ProgramArguments) == 0 || strings.TrimSpace(item.ProgramArguments[0]) == "" {
		return "", errors.New("launch item program arguments are required")
	}
	home, err := homeDirPath()
	if err != nil {
		return "", err
	}
	plistPath := launchItemPath(home, item.Label)
	if item.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(item.LogPath), 0o755); err != nil {
			return "", fmt.Errorf("create log dir: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return "", fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	var buf bytes.Buffer
	if err := renderLaunchItemPlist(&buf, item); err != nil {
		return "", err
	}
	if err := writeFileAtomic(plistPath, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return plistPath, nil
}

// DisableLaunchItem unloads and removes the plist. The unload error is
// ignored on purpose: the plist's absence is the source of truth, and the
// item may simply not be loaded.
func DisableLaunchItem(label string) error {
	if strings.TrimSpace(label) == "" {
		return errors.New("launch item label is required")
	}
	home, err := homeDirPath()
	if err != nil {
		return err
	}
	plistPath := launchItemPath(home, label)
	_, _ = launchctlRun("unload", plistPath)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LaunchItemStatus reads back the item state from disk plus launchd. A
// missing plist reports disabled without consulting launchctl at all.
func LaunchItemStatus(label string) (LaunchItemState, error) {
	if strings.TrimSpace(label) == "" {
		return LaunchItemState{}, errors.New("launch item label is required")
	}
	home, err := homeDirPath()
	if err != nil {
		return LaunchItemState{}, err
	}
	state := LaunchItemState{}
	if _, err := os.Stat(launchItemPath(home, label)); err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return LaunchItemState{}, err
	}
	state.Installed = true
	loaded, err := launchctlList(label)
	if err != nil {
		return LaunchItemState{}, err
	}
	state.Loaded = loaded
	return state, nil
}
