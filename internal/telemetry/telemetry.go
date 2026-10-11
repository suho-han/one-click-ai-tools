package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Endpoint is injected into release builds. Unconfigured development builds
// cannot send analytics or create an installation ID.
var Endpoint string

const (
	requestTimeout = 500 * time.Millisecond
	flushTimeout   = 200 * time.Millisecond
)

type Options struct {
	Endpoint  string
	HomeDir   string
	Version   string
	Providers []string
}

type Command struct {
	Path   string
	Source string
}

type properties struct {
	SchemaVersion int      `json:"schema_version"`
	Command       string   `json:"command"`
	Source        string   `json:"source"`
	Version       string   `json:"version"`
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	InvocationID  string   `json:"invocation_id"`
	Providers     []string `json:"providers"`
	DeviceFamily  string   `json:"device_family"`
	Success       *bool    `json:"success,omitempty"`
	DurationMS    *int64   `json:"duration_ms,omitempty"`
}

type captureEvent struct {
	Event      string     `json:"event"`
	DistinctID string     `json:"distinct_id"`
	Timestamp  string     `json:"timestamp"`
	Properties properties `json:"properties"`
}

type Session struct {
	client       *http.Client
	endpoint     string
	installation string
	properties   properties
	started      time.Time
	ctx          context.Context
	cancel       context.CancelFunc
	pending      sync.WaitGroup
	finishOnce   sync.Once
}

// Start prepares an installation ID and sends at most one started event. A
// disabled or invalid configuration does no file, random, or network work.
func Start(ctx context.Context, opts Options, command Command) *Session {
	if ctx == nil || ctx.Err() != nil || !validEndpoint(opts.Endpoint) || !validCommand(command) {
		return nil
	}
	installation, err := installationID(opts.HomeDir)
	if err != nil {
		return nil
	}
	invocation, err := newUUID()
	if err != nil {
		return nil
	}
	requestCtx, cancel := context.WithCancel(ctx)
	s := &Session{
		client: &http.Client{
			Timeout:       requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		endpoint:     opts.Endpoint,
		installation: installation,
		properties: properties{
			SchemaVersion: 2,
			Command:       command.Path,
			Source:        command.Source,
			Version:       opts.Version,
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			InvocationID:  invocation,
			Providers:     append([]string{}, opts.Providers...),
			DeviceFamily:  deviceFamily(runtime.GOOS, opts.HomeDir, systemProfilerMachineName),
		},
		started: time.Now(),
		ctx:     requestCtx,
		cancel:  cancel,
	}
	s.pending.Add(1)
	go s.send("oct_command_started", s.properties)
	return s
}

// Finish sends at most one finished event and gives both requests a combined
// 200ms flush window. Disabled sessions cancel any outstanding start request.
func (s *Session) Finish(success bool, enabled bool) {
	if s == nil {
		return
	}
	s.finishOnce.Do(func() {
		if !enabled {
			s.cancel()
			return
		}
		duration := time.Since(s.started).Milliseconds()
		if duration < 0 {
			duration = 0
		}
		props := s.properties
		props.Success = &success
		props.DurationMS = &duration
		s.pending.Add(1)
		go s.send("oct_command_finished", props)

		completed := make(chan struct{})
		go func() {
			s.pending.Wait()
			close(completed)
		}()
		timer := time.NewTimer(flushTimeout)
		defer timer.Stop()
		select {
		case <-completed:
		case <-timer.C:
		}
		s.cancel()
	})
}

func (s *Session) send(name string, props properties) {
	defer s.pending.Done()
	if s.ctx.Err() != nil {
		return
	}
	body, err := json.Marshal(captureEvent{
		Event: name, DistinctID: s.installation,
		Timestamp: time.Now().UTC().Format(time.RFC3339), Properties: props,
	})
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "oct-telemetry")
	resp, err := s.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func validEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}

func validCommand(c Command) bool {
	if c.Source != "cli" && c.Source != "noninteractive" && c.Source != "menubar" {
		return false
	}
	segments := strings.Split(c.Path, " ")
	if len(segments) < 2 || segments[0] != "oct" {
		return false
	}
	for _, segment := range segments[1:] {
		if segment == "" {
			return false
		}
		for _, r := range segment {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
