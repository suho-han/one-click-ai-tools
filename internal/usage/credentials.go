package usage

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Credential kinds, mirroring where oct's fetchers actually resolve
// credentials from (see the resolve* helpers in the fetcher files).
const (
	CredentialKindEnv      = "env"
	CredentialKindFile     = "file"
	CredentialKindKeychain = "keychain"
	CredentialKindCLI      = "cli"
	CredentialKindConfig   = "config"
	CredentialKindLocal    = "local"
)

// Credential statuses for a provider's whole chain.
const (
	// CredentialStatusOK means at least one source in the chain is present.
	CredentialStatusOK = "ok"
	// CredentialStatusMissing means every source in the chain is absent.
	CredentialStatusMissing = "missing"
	// CredentialStatusInfo means the provider needs no secret (CLI-delegated
	// or purely local data), so ok/missing would overstate the problem.
	CredentialStatusInfo = "info"
)

// CredentialSource is one place a provider looks for credentials. Probes
// report presence only — never the secret value itself.
type CredentialSource struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Found    bool   `json:"found"`
	Note     string `json:"note,omitempty"`
}

// CredentialStatus is the result of probing one provider's credential chain.
type CredentialStatus struct {
	Status  string             `json:"status"`
	Sources []CredentialSource `json:"sources"`
	// Resolved describes the source that would actually be used (the first
	// found one), "" when nothing was found.
	Resolved string `json:"resolved,omitempty"`
	// Note carries provider-level context (CLI delegation, local-only data).
	Note string `json:"note,omitempty"`
}

// credentialProbeTimeout bounds the local commands a probe may run (keychain,
// gh). Probes must stay fast: doctor runs them all in sequence.
const credentialProbeTimeout = 3 * time.Second

// credentialStatus derives the chain status and the resolved source.
func credentialStatus(sources []CredentialSource) CredentialStatus {
	status := CredentialStatus{Sources: sources}
	resolved := ""
	for _, source := range sources {
		if !source.Found {
			continue
		}
		resolved = credentialSourceLabel(source)
		break
	}
	if resolved != "" {
		status.Status = CredentialStatusOK
		status.Resolved = resolved
	} else {
		status.Status = CredentialStatusMissing
	}
	return status
}

// credentialSourceLabel renders one source the way the doctor shows it
// ("env KIMI_CODE_API_KEY", "file ~/.kimi-code/...").
func credentialSourceLabel(source CredentialSource) string {
	switch source.Kind {
	case CredentialKindEnv:
		return "env " + source.Location
	case CredentialKindFile, CredentialKindKeychain:
		return source.Kind + " " + source.Location
	case CredentialKindCLI:
		return "cli " + source.Location
	case CredentialKindConfig:
		return "config " + source.Location
	case CredentialKindLocal:
		return "local " + source.Location
	default:
		return source.Kind + " " + source.Location
	}
}

// credentialEnvFound reports whether an env var is set to a non-blank value.
func credentialEnvFound(name string) bool {
	return strings.TrimSpace(os.Getenv(name)) != ""
}

// credentialJSONFileFound reports whether a JSON credential file exists and
// carries a non-empty value at the given top-level keys.
func credentialJSONFileFound(path string, keys ...string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	for _, key := range keys {
		if s, ok := payload[key].(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

// credentialJSONEntryKeyPresent reports whether an auth.json-style map file
// ({"<entry>": {"key"|"access"|"accessToken": "..."}}) contains a non-empty
// credential under one of the given entries.
func credentialJSONEntryKeyPresent(path string, entries ...string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var payload map[string]map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	for _, entry := range entries {
		fields, ok := payload[entry]
		if !ok {
			continue
		}
		for _, key := range []string{"key", "access", "accessToken"} {
			if strings.TrimSpace(fields[key]) != "" {
				return true
			}
		}
	}
	return false
}

// credentialCommandPresent runs a local command and reports whether it exits
// successfully (used for keychain and gh probes; never hits the network).
// Package var so tests can stub command execution.
var credentialCommandPresent = func(name string, args ...string) bool {
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), credentialProbeTimeout)
	defer cancel()
	cmd := execCommand(ctx, name, args...)
	// Probes only care about exit status; discard output so secret values
	// (keychain passwords, gh tokens) are never captured.
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// credentialBinaryPresent reports whether a binary is on PATH. Package var
// so tests can stub PATH lookups.
var credentialBinaryPresent = func(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// credentialHomePath joins a path under the user's home, "" when the home is
// unresolvable.
func credentialHomePath(parts ...string) string {
	home, _ := userHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(append([]string{home}, parts...)...)
}

// DescribeProviderCredentials probes one provider's credential chain by
// canonical registry name, alias, or configured <provider>:<name> account
// row. ok=false when the name is unknown.
func DescribeProviderCredentials(name string) (CredentialStatus, bool) {
	resolved, ok := lookupProviderName(strings.ToLower(strings.TrimSpace(name)))
	if ok {
		for _, entry := range providers {
			if entry.Name != resolved {
				continue
			}
			if entry.DescribeCredential == nil {
				return CredentialStatus{
					Status: CredentialStatusInfo,
					Note:   "no credential probe registered",
				}, true
			}
			return entry.DescribeCredential(), true
		}
		return CredentialStatus{}, false
	}
	// Not a registry name; configured account rows probe their own source.
	target := strings.ToLower(strings.TrimSpace(name))
	for _, p := range accountProviders() {
		if p.Name != target {
			continue
		}
		if p.DescribeCredential == nil {
			return CredentialStatus{
				Status: CredentialStatusInfo,
				Note:   "no credential probe registered",
			}, true
		}
		return p.DescribeCredential(), true
	}
	return CredentialStatus{}, false
}

// CredentialProviderNames lists every provider with a credential probe, in
// registry order followed by configured <provider>:<name> account rows.
func CredentialProviderNames() []string {
	names := make([]string, 0, len(providers))
	for _, entry := range providers {
		if entry.DescribeCredential != nil {
			names = append(names, entry.Name)
		}
	}
	for _, p := range accountProviders() {
		names = append(names, p.Name)
	}
	return names
}
