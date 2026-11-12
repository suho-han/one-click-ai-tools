package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func runStatusline(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	for _, c := range []*cobra.Command{statuslineCmd, statuslineInstallCmd} {
		c.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	}
	statuslineCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	var out, errb bytes.Buffer
	code := runCLI(append([]string{"--config", writeTempConfig(t)}, args...), &out, &errb)
	return out.String(), errb.String(), code
}

func TestStatuslineParentPrintsTargets(t *testing.T) {
	out, _, code := runStatusline(t, "statusline")
	if code != 0 || !strings.Contains(out, "waybar, polybar, swiftbar, xbar") || !strings.Contains(out, "Example: oct statusline config waybar") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestStatuslineConfigGolden(t *testing.T) {
	golden := map[string]string{
		"waybar":  "\"custom/oct\": {\n  \"exec\": \"/opt/bin/oct usage --format waybar\",\n  \"return-type\": \"json\",\n  \"interval\": 60,\n  \"escape\": false\n}\n",
		"polybar": "[module/oct]\ntype = custom/script\ninterval = 60\nexec = /opt/bin/oct usage --format polybar\n",
	}
	for target, want := range golden {
		out, _, code := runStatusline(t, "statusline", "config", target, "--path", "/opt/bin/oct")
		if code != 0 || out != want {
			t.Errorf("%s: code=%d out=%q", target, code, out)
		}
	}
	for _, target := range []string{"swiftbar", "xbar"} {
		out, errOut, code := runStatusline(t, "statusline", "config", target)
		want := "#!/bin/bash\n# <bitbar.title>oct usage</bitbar.title>\n# <bitbar.version>v1.0</bitbar.version>\n# <bitbar.desc>AI CLI quota usage from one-click-ai-tools</bitbar.desc>\n# <bitbar.dependencies>oct</bitbar.dependencies>\noct usage --format swiftbar\n"
		if code != 0 || out != want {
			t.Errorf("%s: code=%d out=%q", target, code, out)
		}
		if strings.Contains(out, errOut) && errOut != "" {
			t.Errorf("explanation leaked to stdout")
		}
	}
}

func TestStatuslineConfigUnknownTarget(t *testing.T) {
	out, errOut, code := runStatusline(t, "statusline", "config", "nope")
	if code != 2 || out != "" || !strings.Contains(errOut, "supported: waybar") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestStatuslineInstallCreateRefuseForce(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "oct.5m.sh")
	out, errOut, code := runStatusline(t, "statusline", "install", "xbar", "--dir", dir)
	if code != 0 || !strings.Contains(out, dest) {
		t.Fatalf("install: code=%d out=%q err=%q", code, out, errOut)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(dest); err != nil || fi.Mode().Perm() != 0o755 {
			t.Fatalf("stat=%v err=%v", fi, err)
		}
	}
	if err := os.WriteFile(dest, []byte("custom"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, errOut, code = runStatusline(t, "statusline", "install", "xbar", "--dir", dir)
	if code == 0 || !strings.Contains(errOut, "--force") {
		t.Fatalf("expected refusal: code=%d err=%q", code, errOut)
	}
	if b, _ := os.ReadFile(dest); string(b) != "custom" {
		t.Fatalf("file overwritten without --force")
	}
	if _, _, code = runStatusline(t, "statusline", "install", "xbar", "--dir", dir, "--force"); code != 0 {
		t.Fatalf("force failed")
	}
	if b, _ := os.ReadFile(dest); !strings.Contains(string(b), "usage --format swiftbar") {
		t.Fatalf("force did not overwrite: %q", b)
	}
}

func TestStatuslineInstallRejectsUnsupported(t *testing.T) {
	_, errOut, code := runStatusline(t, "statusline", "install", "waybar", "--dir", t.TempDir())
	if code != 2 || !strings.Contains(errOut, "unsupported install target") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	dir := t.TempDir()
	err := runStatuslineInstall(&bytes.Buffer{}, "xbar", "oct", dir, false, "windows")
	if err == nil || !strings.Contains(err.Error(), "windows") {
		t.Fatalf("err=%v", err)
	}
	if _, serr := os.Stat(filepath.Join(dir, "oct.5m.sh")); serr == nil {
		t.Fatal("file written on windows")
	}
	if err := runStatuslineInstall(&bytes.Buffer{}, "swiftbar", "oct", "", false, "linux"); err == nil {
		t.Fatal("swiftbar default dir on linux should error")
	}
}
