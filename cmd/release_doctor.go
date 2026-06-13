package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-tools/internal/execenv"
)

type releaseDoctorReport struct {
	LocalVersion   string `json:"local_version"`
	RegistryLatest string `json:"registry_latest,omitempty"`
	WorkingTree    string `json:"working_tree"`
	Branch         string `json:"branch,omitempty"`
	Remote         string `json:"remote,omitempty"`
	NPMUserConfig  string `json:"npm_userconfig,omitempty"`
	NPMWhoami      string `json:"npm_whoami,omitempty"`
	NPMWhoamiError string `json:"npm_whoami_error,omitempty"`
	RepoNPMRC      string `json:"repo_npmrc,omitempty"`
}

var releaseDoctorCmd = &cobra.Command{
	Use:     "release-doctor",
	GroupID: "maintenance",
	Short:   "Check npm/git/tag release preflight state",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		report := collectReleaseDoctorReport()
		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "release doctor")
		fmt.Fprintf(cmd.OutOrStdout(), "- local version: %s\n", report.LocalVersion)
		if report.RegistryLatest != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- registry latest: %s\n", report.RegistryLatest)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "- working tree: %s\n", report.WorkingTree)
		if report.Branch != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- branch: %s\n", report.Branch)
		}
		if report.Remote != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- remote: %s\n", report.Remote)
		}
		if report.RepoNPMRC != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- repo .npmrc: %s\n", report.RepoNPMRC)
		}
		if report.NPMUserConfig != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- npm userconfig: %s\n", report.NPMUserConfig)
		}
		if report.NPMWhoami != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- npm whoami: %s\n", report.NPMWhoami)
		}
		if report.NPMWhoamiError != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- npm whoami error: %s\n", report.NPMWhoamiError)
		}
		return nil
	},
}

func collectReleaseDoctorReport() releaseDoctorReport {
	report := releaseDoctorReport{LocalVersion: rootCmd.Version, WorkingTree: strings.TrimSpace(runDoctorCommand("git", "status", "--short"))}
	if report.WorkingTree == "" {
		report.WorkingTree = "clean"
	}
	report.Branch = strings.TrimSpace(runDoctorCommand("git", "branch", "--show-current"))
	report.Remote = strings.TrimSpace(runDoctorCommand("git", "remote", "get-url", "origin"))
	report.RegistryLatest = strings.TrimSpace(runDoctorCommand("npm", "view", "one-click-tools", "version", "--registry=https://registry.npmjs.org/"))
	report.NPMUserConfig = strings.TrimSpace(runDoctorCommand("npm", "config", "get", "userconfig"))

	whoami := execenv.Command("npm", "whoami")
	if out, err := whoami.CombinedOutput(); err == nil {
		report.NPMWhoami = strings.TrimSpace(string(out))
	} else {
		report.NPMWhoamiError = strings.TrimSpace(string(out))
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

func runDoctorCommand(name string, args ...string) string {
	out, err := execenv.Command(name, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func init() {
	releaseDoctorCmd.Flags().Bool("json", false, "Output in JSON format")
	rootCmd.AddCommand(releaseDoctorCmd)
}
