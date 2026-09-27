package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const selfUpdateRepo = "suho-han/one-click-ai-tools"

type selfUpdateOptions struct {
	yes   bool
	check bool
}

type githubLatestRelease struct {
	TagName string `json:"tag_name"`
}

type releaseAsset struct {
	Name string
	URL  string
}

// selfUpdateHTTPClient is the client for GitHub release downloads. Timeout is
// zero because archive bodies are large; per-call deadlines are applied in
// fetchLatestReleaseTag / installReleaseAsset / verifyReleaseAssetChecksum
// and connection phases are bounded by the transport below.
var (
	selfUpdateCommandContext = exec.CommandContext
	checksumBaseURL          = "https://github.com"
	selfUpdateHTTPClient     = &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}
)

var selfUpdateOpts selfUpdateOptions

var updateCmd = &cobra.Command{
	Use:     "update",
	GroupID: "maintenance",
	Short:   "⬆️ Update oct package",
	Long:    `Update oct (one-click-tools) itself to the latest GitHub Release version.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSelfUpdate(cmd, selfUpdateOpts)
	},
}

func runSelfUpdate(cmd *cobra.Command, opts selfUpdateOptions) error {
	if runtime.GOOS == "darwin" && installedViaBrew() {
		if opts.check {
			fmt.Fprintln(cmd.OutOrStdout(), "oct is managed by Homebrew. Use: brew upgrade one-click-tools")
			return nil
		}
		// A stuck brew upgrade would otherwise hang `oct update` forever.
		brewCtx, cancelBrew := context.WithTimeout(cmd.Context(), 10*time.Minute)
		defer cancelBrew()
		brew := selfUpdateCommandContext(brewCtx, "brew", "upgrade", "one-click-tools")
		brew.Stdout = cmd.OutOrStdout()
		brew.Stderr = cmd.ErrOrStderr()
		return brew.Run()
	}

	current := normalizeReleaseTag(rootCmd.Version)
	latest, err := fetchLatestReleaseTag(cmd.Context(), selfUpdateRepo)
	if err != nil {
		return err
	}

	cmp := compareReleaseVersions(current, latest)
	if cmp >= 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "oct is up to date (%s).\n", current)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "oct update available: %s -> %s\n", current, latest)
	if opts.check {
		fmt.Fprintln(cmd.OutOrStdout(), "Run `oct update --yes` to install it non-interactively.")
		return nil
	}

	if !opts.yes {
		ok, err := confirmSelfUpdate(cmd, current, latest)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "oct update skipped.")
			return nil
		}
	}

	asset, err := releaseAssetFor(runtime.GOOS, runtime.GOARCH, latest)
	if err != nil {
		return err
	}
	if err := installReleaseAsset(cmd.Context(), selfUpdateRepo, asset); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "oct updated successfully to %s.\n", latest)
	return nil
}

// installedViaBrew reports whether Homebrew manages this install by checking
// the Cellar formula directory directly; spawning `brew list` costs a full
// Ruby startup (often 0.5-2s) on every `oct update`.
func installedViaBrew() bool {
	if _, err := exec.LookPath("brew"); err != nil {
		return false
	}
	for _, dir := range brewCellarDirs() {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// brewCellarDirs lists candidate Cellar formula paths, honoring
// HOMEBREW_PREFIX before the platform defaults.
func brewCellarDirs() []string {
	if prefix := strings.TrimSpace(os.Getenv("HOMEBREW_PREFIX")); prefix != "" {
		return []string{filepath.Join(prefix, "Cellar", "one-click-tools")}
	}
	return []string{
		"/opt/homebrew/Cellar/one-click-tools",
		"/usr/local/Cellar/one-click-tools",
	}
}

func fetchLatestReleaseTag(ctx context.Context, repo string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "one-click-ai-tools")

	resp, err := selfUpdateHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub latest release lookup failed: HTTP %d", resp.StatusCode)
	}

	var release githubLatestRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return "", errors.New("GitHub latest release response did not include tag_name")
	}
	return normalizeReleaseTag(release.TagName), nil
}

func confirmSelfUpdate(cmd *cobra.Command, current, latest string) (bool, error) {
	in := cmd.InOrStdin()
	out := cmd.OutOrStdout()
	if !isTerminalPrompt(in, out) {
		fmt.Fprintln(out, "Non-interactive shell detected. Run `oct update --yes` to install it.")
		return false, nil
	}

	fmt.Fprintf(out, "Update oct from %s to %s? [y/N]: ", current, latest)
	reader := bufio.NewReader(in)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}

func isTerminalPrompt(in io.Reader, out io.Writer) bool {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK {
		return false
	}
	inInfo, err := inFile.Stat()
	if err != nil {
		return false
	}
	outInfo, err := outFile.Stat()
	if err != nil {
		return false
	}
	return inInfo.Mode()&os.ModeCharDevice != 0 && outInfo.Mode()&os.ModeCharDevice != 0
}

func releaseAssetFor(goos, goarch, tag string) (releaseAsset, error) {
	platform, ok := map[string]string{
		"darwin":  "darwin",
		"linux":   "linux",
		"windows": "windows",
	}[goos]
	if !ok {
		return releaseAsset{}, fmt.Errorf("unsupported OS for self-update: %s", goos)
	}

	arch, ok := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
	}[goarch]
	if !ok {
		return releaseAsset{}, fmt.Errorf("unsupported architecture for self-update: %s", goarch)
	}

	ext := ".tar.gz"
	if platform == "windows" {
		ext = ".zip"
	}
	name := fmt.Sprintf("one-click-ai-tools_%s_%s%s", platform, arch, ext)
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s", selfUpdateRepo, tag)
	return releaseAsset{Name: name, URL: base + "/" + name}, nil
}

// selfUpdateGOOS is a seam so tests can exercise the darwin-only helper
// install path on any host.
var selfUpdateGOOS = runtime.GOOS

// menubarHelperBinaryName is the Swift menubar helper bundled at the root of
// darwin release tarballs (see the darwin-assets job in release.yml).
const menubarHelperBinaryName = "OctMenubarApp"

// installMenubarHelperFromArchive best-effort installs the Swift menubar
// helper shipped in the same release archive as the oct binary. Unlike the
// oct replacement (fail-closed), helper failures only produce a warning:
// the legacy menubar keeps working, and archives from older releases do not
// contain a helper at all. The temp+rename copy (copyExecutableFile) leaves
// a running helper's inode intact.
func installMenubarHelperFromArchive(archivePath, extractDir string) error {
	if selfUpdateGOOS != "darwin" {
		return nil
	}
	if err := extractTarGzBinary(archivePath, extractDir, menubarHelperBinaryName); err != nil {
		return fmt.Errorf("helper not bundled in release archive: %w", err)
	}
	dst, err := defaultMenubarInstallPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyExecutableFile(filepath.Join(extractDir, menubarHelperBinaryName), dst)
}

func installReleaseAsset(ctx context.Context, repo string, asset releaseAsset) error {
	tmpDir, err := os.MkdirTemp("", "oct-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	downloadCtx, cancelDownload := context.WithTimeout(ctx, 2*time.Minute)
	err = downloadReleaseFile(downloadCtx, asset.URL, archivePath)
	cancelDownload()
	if err != nil {
		return err
	}

	if err := verifyReleaseAssetChecksum(ctx, repo, asset, archivePath); err != nil {
		return err
	}
	if err := verifyReleaseAttestation(ctx, repo, archivePath); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(asset.Name, ".zip") {
		if err := extractZipBinary(archivePath, extractDir, executableName()); err != nil {
			return err
		}
	} else {
		if err := extractTarGzBinary(archivePath, extractDir, executableName()); err != nil {
			return err
		}
	}

	binaryPath := filepath.Join(extractDir, executableName())
	targetPath, err := currentExecutablePath()
	if err != nil {
		return err
	}
	if err := replaceExecutable(binaryPath, targetPath); err != nil {
		return err
	}
	// The oct replacement above is the critical step and has already
	// succeeded; a helper install failure from here on must not fail the
	// update, so warn instead.
	if err := installMenubarHelperFromArchive(archivePath, extractDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: menubar helper not updated: %v\n", err)
	}
	return nil
}

func downloadReleaseFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "one-click-ai-tools")

	resp, err := selfUpdateHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	// A close-time flush error would leave a truncated archive or
	// checksums.txt; surface it instead of dropping it via defer.
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// verifyReleaseAssetChecksum is fail-closed: any failure to obtain or match
// the published checksum aborts the install.
func verifyReleaseAssetChecksum(ctx context.Context, repo string, asset releaseAsset, archivePath string) error {
	checksumURL := fmt.Sprintf("%s/%s/releases/download/%s/checksums.txt", checksumBaseURL, repo, releaseTagFromAssetURL(asset.URL))
	checksumPath := archivePath + ".checksums.txt"
	checksumCtx, cancelChecksum := context.WithTimeout(ctx, 30*time.Second)
	err := downloadReleaseFile(checksumCtx, checksumURL, checksumPath)
	cancelChecksum()
	if err != nil {
		return fmt.Errorf("checksums.txt unavailable for %s: %w (refusing to install without verification)", asset.Name, err)
	}

	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("checksums.txt unreadable for %s: %w", asset.Name, err)
	}
	expected := checksumForAsset(string(data), asset.Name)
	if expected == "" {
		return fmt.Errorf("checksum entry not found for %s (refusing to install)", asset.Name)
	}

	actual, err := fileSHA256(archivePath)
	if err != nil {
		return fmt.Errorf("checksum computation failed for %s: %w", asset.Name, err)
	}
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s", asset.Name)
	}
	return nil
}

// errAttestationUnsupported marks an installed gh CLI that predates the
// `gh attestation` command; it is a skip condition, not a verification failure.
var errAttestationUnsupported = errors.New("gh CLI predates attestation support")

var (
	// Seams so tests can fake the gh lookup and the verification subprocess.
	attestationGhLookup  = func() (string, error) { return exec.LookPath("gh") }
	runAttestationVerify = func(ctx context.Context, ghPath, artifactPath, repo string) error {
		ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, ghPath, "attestation", "verify", artifactPath,
			"-R", repo, "--digest-alg", "sha256")
		var stderr bytes.Buffer
		cmd.Stdout = io.Discard
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			msg := strings.ToLower(stderr.String())
			if strings.Contains(msg, "unknown command") || strings.Contains(msg, "did you mean") {
				return errAttestationUnsupported
			}
			return err
		}
		return nil
	}
)

// verifyReleaseAttestation adds a Sigstore provenance check on top of the
// checksum: it proves the artifact was built by this repository's GitHub
// Actions, not merely that the download matches what was published (a
// checksum proves nothing when the release itself is compromised).
//
// Full verification needs the gh CLI, which is not a runtime dependency of
// oct, so a missing or too-old gh is a skip with a warning -- unless
// OCT_UPDATE_REQUIRE_ATTESTATION=1 turns the skip into an error, or
// OCT_UPDATE_SKIP_ATTESTATION=1 silences it entirely. An actual verification
// failure always aborts the install.
func verifyReleaseAttestation(ctx context.Context, repo, artifactPath string) error {
	if os.Getenv("OCT_UPDATE_SKIP_ATTESTATION") == "1" {
		fmt.Fprintln(os.Stderr, "warning: artifact attestation verification skipped (OCT_UPDATE_SKIP_ATTESTATION=1)")
		return nil
	}
	ghPath, err := attestationGhLookup()
	if err != nil {
		return skipReleaseAttestation("gh CLI not found")
	}
	if err := runAttestationVerify(ctx, ghPath, artifactPath, repo); err != nil {
		if errors.Is(err, errAttestationUnsupported) {
			return skipReleaseAttestation("installed gh CLI predates attestation support")
		}
		return fmt.Errorf("artifact attestation verification failed for %s: %w (set OCT_UPDATE_SKIP_ATTESTATION=1 to override)",
			filepath.Base(artifactPath), err)
	}
	return nil
}

func skipReleaseAttestation(reason string) error {
	if os.Getenv("OCT_UPDATE_REQUIRE_ATTESTATION") == "1" {
		return fmt.Errorf("attestation verification required (OCT_UPDATE_REQUIRE_ATTESTATION=1) but unavailable: %s", reason)
	}
	fmt.Fprintf(os.Stderr, "warning: skipping artifact attestation verification (%s)\n", reason)
	return nil
}

func checksumForAsset(checksums, assetName string) string {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			return fields[0]
		}
	}
	return ""
}

func releaseTagFromAssetURL(url string) string {
	marker := "/releases/download/"
	idx := strings.Index(url, marker)
	if idx < 0 {
		return "latest"
	}
	rest := url[idx+len(marker):]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		return rest[:slash]
	}
	return rest
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractTarGzBinary(archivePath, destDir, binaryName string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		return writeExtractedBinary(filepath.Join(destDir, binaryName), tr, header.FileInfo().Mode())
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

func extractZipBinary(archivePath, destDir, binaryName string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, file := range zr.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != binaryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return writeExtractedBinary(filepath.Join(destDir, binaryName), rc, file.FileInfo().Mode())
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

func writeExtractedBinary(path string, src io.Reader, mode os.FileMode) error {
	// Perm() strips any setuid/setgid bits from the archive entry; |0o755
	// guarantees the owner can execute.
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm()|0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, src)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "oct.exe"
	}
	return "oct"
}

func currentExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	return path, nil
}

func replaceExecutable(src, target string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// A unique temp name instead of target+".new" so two concurrent updates
	// cannot interleave writes into the same file.
	tmp, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, in); err != nil {
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
	// Perm() strips setuid/setgid from the source; |0o755 keeps it executable.
	if err := os.Chmod(tmpName, info.Mode().Perm()|0o755); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}

func normalizeReleaseTag(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "v0.0.0"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func compareReleaseVersions(a, b string) int {
	av := parseReleaseVersion(a)
	bv := parseReleaseVersion(b)
	for i := 0; i < len(av) && i < len(bv); i++ {
		if av[i] > bv[i] {
			return 1
		}
		if av[i] < bv[i] {
			return -1
		}
	}
	return 0
}

func parseReleaseVersion(version string) [3]int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	version = strings.SplitN(version, "-", 2)[0]
	version = strings.SplitN(version, "+", 2)[0]
	parts := strings.Split(version, ".")
	var parsed [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		value, err := strconv.Atoi(parts[i])
		if err == nil {
			parsed[i] = value
		}
	}
	return parsed
}

func init() {
	updateCmd.Flags().BoolVarP(&selfUpdateOpts.yes, "yes", "y", false, "Install the latest release without prompting")
	updateCmd.Flags().BoolVar(&selfUpdateOpts.check, "check", false, "Check for a newer release without installing")
	rootCmd.AddCommand(updateCmd)
}
