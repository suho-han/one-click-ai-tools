package update

import (
	"encoding/json"
	"fmt"
	"io"
)

// updateEvent is one newline-delimited JSON record of `agent-update --json`.
// A single flat shape with omitempty keeps both the encoder and the menu bar
// app's decoder tolerant: consumers key off `event` and treat absent fields
// as zero values, so adding fields later never breaks old clients.
type updateEvent struct {
	Event         string  `json:"event"`
	Total         int     `json:"total,omitempty"`
	Index         int     `json:"index,omitempty"`
	Binary        string  `json:"binary,omitempty"`
	Name          string  `json:"name,omitempty"`
	Manager       string  `json:"manager,omitempty"`
	VersionBefore string  `json:"version_before,omitempty"`
	Status        string  `json:"status,omitempty"`
	VersionAfter  string  `json:"version_after,omitempty"`
	DurationSec   float64 `json:"duration_s,omitempty"`
	Error         string  `json:"error,omitempty"`
	Failed        int     `json:"failed,omitempty"`
	// tool_check-only fields (agent-update --check). The bools are pointers
	// so omitempty cannot hide an explicit false: absent means the event
	// kind does not carry the field, false means "we checked, it is not".
	VersionInstalled string `json:"version_installed,omitempty"`
	VersionLatest    string `json:"version_latest,omitempty"`
	Outdated         *bool  `json:"outdated,omitempty"`
	LatestKnown      *bool  `json:"latest_known,omitempty"`
}

// Statuses reported in tool_done events.
const (
	EventStatusUpdated             = "updated"
	EventStatusUpToDate            = "up_to_date"
	EventStatusFailed              = "failed"
	EventStatusSkipped             = "skipped"
	EventStatusSkippedNotInstalled = "skipped_not_installed"
	// EventStatusNpmMissing marks a tool whose manager needs the Node.js
	// toolchain (npm/pnpm/yarn) but none of it is on PATH; the consumer can
	// point the user at a Node.js install.
	EventStatusNpmMissing = "npm_missing"
)

// eventEmitter writes NDJSON update events. The zero value emits nothing, so
// human-readable runs share the same call sites untouched.
type eventEmitter struct {
	out io.Writer
}

func newEventEmitter(enabled bool, out io.Writer) *eventEmitter {
	if !enabled {
		return &eventEmitter{}
	}
	return &eventEmitter{out: out}
}

func (e *eventEmitter) active() bool { return e.out != nil }

func (e *eventEmitter) emit(ev updateEvent) {
	if !e.active() {
		return
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	fmt.Fprintf(e.out, "%s\n", data)
}

func (e *eventEmitter) runStart(total int) {
	e.emit(updateEvent{Event: "run_start", Total: total})
}

func (e *eventEmitter) toolStart(tool Tool, manager Manager, index, total int, versionBefore string) {
	e.emit(updateEvent{
		Event:         "tool_start",
		Binary:        tool.BinaryName,
		Name:          tool.Name,
		Manager:       string(manager),
		Index:         index,
		Total:         total,
		VersionBefore: versionBefore,
	})
}

func (e *eventEmitter) toolDone(tool Tool, status, versionAfter string, durationSec float64, errString string) {
	e.emit(updateEvent{
		Event:        "tool_done",
		Binary:       tool.BinaryName,
		Name:         tool.Name,
		Status:       status,
		VersionAfter: versionAfter,
		DurationSec:  durationSec,
		Error:        errString,
	})
}

func (e *eventEmitter) skippedNotInstalled(plan Plan) {
	e.emit(updateEvent{
		Event:  "tool_done",
		Binary: plan.Tool.BinaryName,
		Name:   plan.Tool.Name,
		Status: EventStatusSkippedNotInstalled,
	})
}

func (e *eventEmitter) runDone(failed int) {
	e.emit(updateEvent{Event: "run_done", Failed: failed})
}

func boolPtr(b bool) *bool { return &b }

func (e *eventEmitter) checkStart(total int) {
	e.emit(updateEvent{Event: "check_start", Total: total})
}

// toolCheck reports one tool's installed version next to the latest version
// its manager knows about. `latestKnown` is false for managers with no query
// API (native updaters, install scripts): only running the updater can tell.
func (e *eventEmitter) toolCheck(plan Plan, latest string, latestKnown, outdated bool) {
	e.emit(updateEvent{
		Event:            "tool_check",
		Binary:           plan.Tool.BinaryName,
		Name:             plan.Tool.Name,
		Manager:          string(plan.Manager),
		VersionInstalled: plan.VersionBefore,
		VersionLatest:    latest,
		Outdated:         boolPtr(outdated),
		LatestKnown:      boolPtr(latestKnown),
	})
}

func (e *eventEmitter) checkDone() {
	e.emit(updateEvent{Event: "check_done"})
}
