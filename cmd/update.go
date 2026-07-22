package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

var (
	selfUpdateCommand    = exec.Command
	selfUpdateHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

var selfUpdateOpts selfUpdateOptions

var updateCmd = &cobra.Command{
	Use:     "update",
	GroupID: "maintenance",
	Short:   "Update oct package",
	Long:    `Update oct (one-click-tools) itself to the latest GitHub Release version.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSelfUpdate(cmd, selfUpdateOpts); err != nil {
			fmt.Printf("oct update failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func runSelfUpdate(cmd *cobra.Command, opts selfUpdateOptions) error {
	if runtime.GOOS == "darwin" && installedViaBrew() {
		if opts.check {
			fmt.Fprintln(cmd.OutOrStdout(), "oct is managed by Homebrew. Use: brew upgrade one-click-tools")
			return nil
		}
		brew := selfUpdateCommand("brew", "upgrade", "one-click-tools")
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

func installedViaBrew() bool {
	if _, err := exec.LookPath("brew"); err != nil {
		return false
	}
	out, err := selfUpdateCommand("brew", "list", "one-click-tools").CombinedOutput()
	return err == nil && len(out) > 0
}

func fetchLatestReleaseTag(ctx context.Context, repo string) (string, error) {
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

func installReleaseAsset(ctx context.Context, repo string, asset releaseAsset) error {
	tmpDir, err := os.MkdirTemp("", "oct-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadReleaseFile(ctx, asset.URL, archivePath); err != nil {
		return err
	}

	if err := verifyReleaseAssetChecksum(ctx, repo, asset, archivePath, false); err != nil {
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
	return replaceExecutable(binaryPath, targetPath)
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
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func verifyReleaseAssetChecksum(ctx context.Context, repo string, asset releaseAsset, archivePath string, require bool) error {
	checksumURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/checksums.txt", repo, releaseTagFromAssetURL(asset.URL))
	checksumPath := archivePath + ".checksums.txt"
	if err := downloadReleaseFile(ctx, checksumURL, checksumPath); err != nil {
		if require {
			return err
		}
		return nil
	}

	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}
	expected := checksumForAsset(string(data), asset.Name)
	if expected == "" {
		if require {
			return fmt.Errorf("checksum entry not found for %s", asset.Name)
		}
		return nil
	}

	actual, err := fileSHA256(archivePath)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s", asset.Name)
	}
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
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode|0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
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
	tmpTarget := target + ".new"
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(tmpTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode()|0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmpTarget, target)
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
