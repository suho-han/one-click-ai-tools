package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func newUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	var encoded [36]byte
	hex.Encode(encoded[0:8], raw[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], raw[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], raw[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], raw[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], raw[10:16])
	return string(encoded[:]), nil
}

func validUUID(id string) bool {
	if len(id) != 36 || id[14] != '4' || !(id[19] == '8' || id[19] == '9' || id[19] == 'a' || id[19] == 'b') {
		return false
	}
	for i := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if id[i] != '-' {
				return false
			}
			continue
		}
		if !(id[i] >= '0' && id[i] <= '9' || id[i] >= 'a' && id[i] <= 'f') {
			return false
		}
	}
	return true
}

func installationID(home string) (string, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil || home == "" {
			return "", errors.New("home unavailable")
		}
	}
	if !filepath.IsAbs(home) {
		return "", errors.New("home must be absolute")
	}
	homeInfo, err := os.Stat(home)
	if err != nil || !homeInfo.IsDir() {
		return "", errors.New("home unavailable")
	}
	dir := filepath.Join(home, ".oct", "telemetry")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "installation-id")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return readInstallationID(path)
	}
	if err != nil {
		return "", err
	}
	created, statErr := file.Stat()
	if statErr != nil {
		file.Close()
		return "", statErr
	}
	id, idErr := newUUID()
	if idErr != nil {
		file.Close()
		removeOwnFile(created, path)
		return "", idErr
	}
	_, writeErr := io.WriteString(file, id+"\n")
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		removeOwnFile(created, path)
		return "", errors.Join(writeErr, closeErr)
	}
	return id, nil
}

// Only remove a failed creation if the path still names our own file. Never
// remove a pre-existing damaged ID, which the user may repair explicitly.
func removeOwnFile(created os.FileInfo, path string) {
	current, err := os.Lstat(path)
	if err == nil && os.SameFile(created, current) {
		os.Remove(path)
	}
}

func readInstallationID(path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() {
		return "", errors.New("installation ID is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() {
		return "", errors.New("installation ID changed during read")
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(current, after) {
		return "", errors.New("installation ID changed during read")
	}
	data, err := io.ReadAll(io.LimitReader(file, 38))
	if err != nil {
		return "", err
	}
	id := strings.TrimSuffix(string(data), "\n")
	if !validUUID(id) {
		return "", errors.New("invalid installation ID")
	}
	return id, nil
}
