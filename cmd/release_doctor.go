package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/execenv"
)

type releaseDoctorReport struct {
	LocalVersion     string `json:"local_version"`
	RegistryLatest   string `json:"registry_latest,omitempty"`
	WorkingTree      string `json:"working_tree"`
	WorkingTreeCount int    `json:"working_tree_count"`
	Branch           string `json:"branch,omitempty"`
	Remote           string `json:"remote,omitempty"`
	NPMUserConfig    string `json:"npm_userconfig,omitempty"`
	NPMWhoami        string `json:"npm_whoami,omitempty"`
	NPMWhoamiError   string `json:"npm_whoami_error,omitempty"`
	RepoNPMRC        string `json:"repo_npmrc,omitempty"`
}

const npmPackageName = "one-click-ai-tools"

var releaseDoctorCmd = &cobra.Command{
	Use:     "release-doctor",
	GroupID: "maintenance",
	Short:   "Check release preflight in one compact report",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		report := collectReleaseDoctorReport()
		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		printReleaseDoctorReport(cmd.OutOrStdout(), report)
		return nil
	},
}

func collectReleaseDoctorReport() releaseDoctorReport {
	workingTree := strings.TrimSpace(runDoctorCommand("git", "status", "--short"))
	report := releaseDoctorReport{LocalVersion: rootCmd.Version, WorkingTree: workingTree}
	if report.WorkingTree == "" {
		report.WorkingTree = "clean"
	} else {
		report.WorkingTreeCount = len(strings.Split(report.WorkingTree, "\n"))
	}
	report.Branch = strings.TrimSpace(runDoctorCommand("git", "branch", "--show-current"))
	report.Remote = strings.TrimSpace(runDoctorCommand("git", "remote", "get-url", "origin"))
	report.RegistryLatest = strings.TrimSpace(runDoctorCommand("npm", "view", npmPackageName, "version", "--registry=https://registry.npmjs.org/"))
	report.NPMUserConfig = strings.TrimSpace(runDoctorCommand("npm", "config", "get", "userconfig"))

	whoami := execenv.Command("npm", "whoami")
	if out, err := whoami.CombinedOutput(); err == nil {
		report.NPMWhoami = strings.TrimSpace(string(out))
	} else {
		report.NPMWhoamiError = firstNonEmptyLine(string(out))
		if report.NPMWhoamiError == "" {
			report.NPMWhoamiError = err.Error()
		}
	}

	if wd, err := os.Getwd(); err == nil {
		repoNPMRC := filepath.Join(wd, ".npmrc")
		if info, err := os.Lstat(repoNPMRC); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if target, err := os.Readlink(repoNPMRC); err == nil {
					report.RepoNPMRC = "symlink -> " + target
				}
			} else {
				report.RepoNPMRC = "regular file"
			}
		}
	}
	return report
}

func printReleaseDoctorReport(w io.Writer, report releaseDoctorReport) {
	treeState := "clean"
	if report.WorkingTreeCount > 0 {
		treeState = fmt.Sprintf("dirty(%d)", report.WorkingTreeCount)
	}
	registry := report.RegistryLatest
	if registry == "" {
		registry = "-"
	}
	fmt.Fprintf(w, "release doctor: local=%s registry=%s branch=%s tree=%s\n", nonEmptyOrDash(report.LocalVersion), registry, nonEmptyOrDash(report.Branch), treeState)
	fmt.Fprintf(w, "npm: repo=%s userconfig=%s whoami=%s\n", nonEmptyOrDash(report.RepoNPMRC), nonEmptyOrDash(report.NPMUserConfig), npmWhoamiStatus(report))
	if report.Remote != "" {
		fmt.Fprintf(w, "remote: %s\n", report.Remote)
	}
	if report.WorkingTreeCount > 0 {
		fmt.Fprintf(w, "dirty files:\n%s\n", report.WorkingTree)
	}
}

func npmWhoamiStatus(report releaseDoctorReport) string {
	if report.NPMWhoami != "" {
		return report.NPMWhoami
	}
	if report.NPMWhoamiError != "" {
		return report.NPMWhoamiError
	}
	return "-"
}

func nonEmptyOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func runDoctorCommand(name string, args ...string) string {
	out, err := execenv.Command(name, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func init() {
	releaseDoctorCmd.Flags().Bool("json", false, "Output in JSON format")
	rootCmd.AddCommand(releaseDoctorCmd)
}
