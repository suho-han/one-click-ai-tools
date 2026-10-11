package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	deviceProbeTimeout = 2 * time.Second
	maxProfilerOutput  = 64 << 10
)

// Device families are an allowlist; no machine name or hardware identifier is
// ever retained in analytics or in the cache.
func macFamily(machineName string) string {
	name := strings.TrimSpace(machineName)
	for _, family := range []string{"Mac mini", "MacBook Air", "MacBook Pro", "Mac Studio", "iMac", "Mac Pro"} {
		if name == family || strings.HasPrefix(name, family+" (") || strings.HasPrefix(name, family+" ") {
			return family
		}
	}
	return "Mac"
}

func validMacFamily(family string) bool {
	switch family {
	case "Mac mini", "MacBook Air", "MacBook Pro", "Mac Studio", "iMac", "Mac Pro", "Mac":
		return true
	}
	return false
}

func deviceFamily(goos, home string, probe func(context.Context) (string, error)) string {
	switch goos {
	case "linux":
		return "Linux PC"
	case "windows":
		return "Windows PC"
	case "darwin":
		// A valid installation ID is already established before this runs. Do
		// not create telemetry state solely to discover the device family.
		if home == "" {
			var err error
			home, err = os.UserHomeDir()
			if err != nil {
				return "Mac"
			}
		}
		path := filepath.Join(home, ".oct", "telemetry", "device-family")
		dir, err := os.Lstat(filepath.Dir(path))
		if err != nil || !dir.IsDir() || dir.Mode().Perm() != 0700 {
			return "Mac"
		}
		if family, err := readCachedFamily(path); err == nil {
			return family
		}
		ctx, cancel := context.WithTimeout(context.Background(), deviceProbeTimeout)
		defer cancel()
		name, err := probe(ctx)
		if err != nil {
			return "Mac"
		}
		family := macFamily(name)
		cacheFamily(path, family)
		return family
	default:
		return "Other"
	}
}

// system_profiler is invoked only on macOS, with a deadline and a strict
// stdout limit. Decode only machine_name; ignore serial numbers and other keys.
func systemProfilerMachineName(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "system_profiler", "SPHardwareDataType", "-json")
	cmd.Stderr = io.Discard
	var output boundedOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return decodeMachineName(output.Bytes())
}

func decodeMachineName(data []byte) (string, error) {
	var hardware struct {
		Items []struct {
			MachineName string `json:"machine_name"`
		} `json:"SPHardwareDataType"`
	}
	if err := json.Unmarshal(data, &hardware); err != nil || len(hardware.Items) == 0 {
		return "", errors.New("hardware model unavailable")
	}
	if hardware.Items[0].MachineName == "" {
		return "", errors.New("hardware model unavailable")
	}
	return hardware.Items[0].MachineName, nil
}

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > maxProfilerOutput-b.Len() {
		return 0, errors.New("hardware output too large")
	}
	return b.Buffer.Write(p)
}

func readCachedFamily(path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm() != 0600 {
		return "", errors.New("unsafe device cache")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return "", errors.New("device cache changed")
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(current, after) {
		return "", errors.New("device cache changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, 32))
	if err != nil {
		return "", err
	}
	family := strings.TrimSuffix(string(data), "\n")
	if !validMacFamily(family) {
		return "", errors.New("invalid device cache")
	}
	return family, nil
}

func cacheFamily(path, family string) {
	if !validMacFamily(family) {
		return
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil || !dir.IsDir() || dir.Mode().Perm() != 0700 {
		return
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return
	}
	created, err := file.Stat()
	if err != nil {
		file.Close()
		return
	}
	_, writeErr := io.WriteString(file, family+"\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		removeOwnFile(created, path)
	}
}
