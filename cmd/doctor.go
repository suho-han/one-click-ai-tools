package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-tools/internal/execenv"
)

type shellDoctorBinary struct {
	Name             string `json:"name"`
	RawResolvedPath  string `json:"raw_resolved_path,omitempty"`
	BootResolvedPath string `json:"boot_resolved_path,omitempty"`
}

type shellDoctorReport struct {
	RawPATH          string              `json:"raw_path"`
	BootstrappedPATH string              `json:"bootstrapped_path"`
	Binaries         []shellDoctorBinary `json:"binaries"`
}

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	GroupID: "maintenance",
	Short:   "Diagnostics for shell/path/runtime issues",
}

var doctorShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Show raw vs bootstrapped PATH and binary resolution",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		report := collectShellDoctorReport([]string{"oct", "node", "npm", "gh", "brew", "claude", "codex", "copilot", "cursor-agent", "opencode", "agy"})
		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "shell doctor\n")
		fmt.Fprintf(cmd.OutOrStdout(), "- raw PATH: %s\n", report.RawPATH)
		fmt.Fprintf(cmd.OutOrStdout(), "- bootstrapped PATH: %s\n", report.BootstrappedPATH)
		fmt.Fprintln(cmd.OutOrStdout(), "- binaries:")
		for _, item := range report.Binaries {
			raw := item.RawResolvedPath
			if raw == "" {
				raw = "not found"
			}
			boot := item.BootResolvedPath
			if boot == "" {
				boot = "not found"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", item.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "    raw: %s\n", raw)
			fmt.Fprintf(cmd.OutOrStdout(), "    bootstrapped: %s\n", boot)
		}
		return nil
	},
}

func collectShellDoctorReport(names []string) shellDoctorReport {
	rawPath := strings.TrimSpace(os.Getenv("PATH"))
	report := shellDoctorReport{
		RawPATH:          rawPath,
		BootstrappedPATH: execenv.BuildPATH(rawPath),
	}
	for _, name := range names {
		entry := shellDoctorBinary{Name: name}
		if raw, err := exec.LookPath(name); err == nil {
			entry.RawResolvedPath = raw
		}
		if boot, err := execenv.LookPath(name); err == nil {
			entry.BootResolvedPath = boot
		}
		report.Binaries = append(report.Binaries, entry)
	}
	return report
}

func init() {
	doctorShellCmd.Flags().Bool("json", false, "Output in JSON format")
	doctorCmd.AddCommand(doctorShellCmd)
	rootCmd.AddCommand(doctorCmd)
}
