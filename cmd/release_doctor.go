package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/execenv"
)

type releaseDoctorReport struct {
	LocalVersion     string `json:"local_version"`
	LatestRelease    string `json:"latest_release,omitempty"`
	UpdateAvailable  bool   `json:"update_available"`
	WorkingTree      string `json:"working_tree"`
	WorkingTreeCount int    `json:"working_tree_count"`
	Branch           string `json:"branch,omitempty"`
	Remote           string `json:"remote,omitempty"`
	LatestError      string `json:"latest_error,omitempty"`
}

var releaseDoctorCmd = &cobra.Command{
	Use:     "release-doctor",
	GroupID: "maintenance",
	Short:   "Check release preflight in one compact report",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		report := collectReleaseDoctorReport(cmd.Context())
		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		printReleaseDoctorReport(cmd.OutOrStdout(), report)
		return nil
	},
}

var (
	releaseDoctorCommand       = execenv.Command
	releaseDoctorLatestRelease = fetchLatestReleaseTag
)

func collectReleaseDoctorReport(ctx context.Context) releaseDoctorReport {
	workingTree := strings.TrimSpace(runDoctorCommand("git", "status", "--short"))
	report := releaseDoctorReport{LocalVersion: rootCmd.Version, WorkingTree: workingTree}
	if report.WorkingTree == "" {
		report.WorkingTree = "clean"
	} else {
		report.WorkingTreeCount = len(strings.Split(report.WorkingTree, "\n"))
	}
	report.Branch = strings.TrimSpace(runDoctorCommand("git", "branch", "--show-current"))
	report.Remote = strings.TrimSpace(runDoctorCommand("git", "remote", "get-url", "origin"))

	latest, err := releaseDoctorLatestRelease(ctx, selfUpdateRepo)
	if err != nil {
		report.LatestError = err.Error()
	} else {
		report.LatestRelease = latest
		report.UpdateAvailable = compareReleaseVersions(normalizeReleaseTag(report.LocalVersion), latest) < 0
	}
	return report
}

func printReleaseDoctorReport(w io.Writer, report releaseDoctorReport) {
	treeState := "clean"
	if report.WorkingTreeCount > 0 {
		treeState = fmt.Sprintf("dirty(%d)", report.WorkingTreeCount)
	}
	latest := report.LatestRelease
	if latest == "" {
		latest = "-"
	}
	fmt.Fprintf(w, "release doctor: local=%s latest=%s branch=%s tree=%s\n", nonEmptyOrDash(report.LocalVersion), latest, nonEmptyOrDash(report.Branch), treeState)
	if report.LatestError != "" {
		fmt.Fprintf(w, "github release lookup: %s\n", report.LatestError)
	} else if report.UpdateAvailable {
		fmt.Fprintln(w, "update: available (run `oct update`)")
	} else if report.LatestRelease != "" {
		fmt.Fprintln(w, "update: up to date")
	}
	if report.Remote != "" {
		fmt.Fprintf(w, "remote: %s\n", report.Remote)
	}
	if report.WorkingTreeCount > 0 {
		fmt.Fprintf(w, "dirty files:\n%s\n", report.WorkingTree)
	}
}

func nonEmptyOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func runDoctorCommand(name string, args ...string) string {
	out, err := releaseDoctorCommand(name, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func init() {
	releaseDoctorCmd.Flags().Bool("json", false, "Output in JSON format")
	rootCmd.AddCommand(releaseDoctorCmd)
}
