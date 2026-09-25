package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/execenv"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
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
	Short:   "🩺 Run environment diagnostics",
}

var doctorShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Compare raw vs bootstrapped PATH resolution",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		verbose, _ := cmd.Flags().GetBool("verbose")
		report := collectShellDoctorReport([]string{"oct", "gh", "brew", "claude", "commandcode", "codex", "copilot", "cursor-agent", "opencode", "agy"})
		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		printShellDoctorReport(cmd.OutOrStdout(), report, verbose)
		return nil
	},
}

func collectShellDoctorReport(names []string) shellDoctorReport {
	rawPath := normalizePathList(strings.TrimSpace(os.Getenv("PATH")))
	report := shellDoctorReport{
		RawPATH:          rawPath,
		BootstrappedPATH: normalizePathList(execenv.BuildPATH(rawPath)),
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

func printShellDoctorReport(w io.Writer, report shellDoctorReport, verbose bool) {
	okCount := 0
	missingRaw := 0
	missingBoot := 0
	changed := 0
	interesting := make([]shellDoctorBinary, 0, len(report.Binaries))
	for _, item := range report.Binaries {
		same := item.RawResolvedPath != "" && item.RawResolvedPath == item.BootResolvedPath
		switch {
		case same:
			okCount++
		case item.RawResolvedPath == "":
			missingRaw++
			interesting = append(interesting, item)
		case item.BootResolvedPath == "":
			missingBoot++
			interesting = append(interesting, item)
		default:
			changed++
			interesting = append(interesting, item)
		}
	}
	fmt.Fprintf(w, "shell doctor: ok=%d changed=%d missing_raw=%d missing_boot=%d\n", okCount, changed, missingRaw, missingBoot)
	fmt.Fprintf(w, "raw PATH : %s\n", compactPathForDisplay(report.RawPATH))
	fmt.Fprintf(w, "boot PATH: %s\n", compactPathForDisplay(report.BootstrappedPATH))
	if verbose {
		interesting = report.Binaries
	}
	if len(interesting) == 0 {
		fmt.Fprintln(w, "bins: all tracked binaries resolve identically")
		return
	}
	fmt.Fprintln(w, "bins:")
	for _, item := range interesting {
		raw := compactBinaryPath(item.RawResolvedPath)
		boot := compactBinaryPath(item.BootResolvedPath)
		if raw == boot {
			fmt.Fprintf(w, "- %s raw=%s boot=%s\n", item.Name, raw, boot)
			continue
		}
		fmt.Fprintf(w, "- %s raw=%s | boot=%s\n", item.Name, raw, boot)
	}
}

func normalizePathList(pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return ""
	}
	seen := map[string]struct{}{}
	parts := make([]string, 0)
	for _, part := range filepath.SplitList(pathValue) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cleaned := filepath.Clean(part)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		parts = append(parts, cleaned)
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func compactPathForDisplay(pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return "-"
	}
	parts := filepath.SplitList(pathValue)
	if len(parts) <= 6 {
		return pathValue
	}
	return strings.Join(parts[:4], string(os.PathListSeparator)) + string(os.PathListSeparator) + "…" + string(os.PathListSeparator) + strings.Join(parts[len(parts)-2:], string(os.PathListSeparator))
}

func compactBinaryPath(pathValue string) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return "-"
	}
	return pathValue
}

type credentialDoctorSource struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Found    bool   `json:"found"`
	Note     string `json:"note,omitempty"`
}

type credentialDoctorProvider struct {
	Provider string                   `json:"provider"`
	Status   string                   `json:"status"`
	Resolved string                   `json:"resolved,omitempty"`
	Note     string                   `json:"note,omitempty"`
	Sources  []credentialDoctorSource `json:"sources"`
}

type credentialDoctorSummary struct {
	Total   int `json:"total"`
	OK      int `json:"ok"`
	Missing int `json:"missing"`
	Info    int `json:"info"`
}

type credentialDoctorReport struct {
	Summary   credentialDoctorSummary    `json:"summary"`
	Providers []credentialDoctorProvider `json:"providers"`
}

var doctorCredentialsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Report which credential source each provider would use",
	Long: `Probe every provider's credential chain (env vars, credential files,
keychain entries, CLI delegation) and report which source a usage fetch would
pick. Local checks only: no network calls, and secret values are never read
for display.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		verbose, _ := cmd.Flags().GetBool("verbose")
		report := collectCredentialDoctorReport(usage.CredentialProviderNames())
		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		printCredentialDoctorReport(cmd.OutOrStdout(), report, verbose)
		return nil
	},
}

func collectCredentialDoctorReport(names []string) credentialDoctorReport {
	report := credentialDoctorReport{}
	for _, name := range names {
		status, _ := usage.DescribeProviderCredentials(name)
		entry := credentialDoctorProvider{
			Provider: name,
			Status:   status.Status,
			Resolved: status.Resolved,
			Note:     status.Note,
		}
		for _, source := range status.Sources {
			entry.Sources = append(entry.Sources, credentialDoctorSource{
				Kind:     source.Kind,
				Location: source.Location,
				Found:    source.Found,
				Note:     source.Note,
			})
		}
		switch status.Status {
		case usage.CredentialStatusOK:
			report.Summary.OK++
		case usage.CredentialStatusMissing:
			report.Summary.Missing++
		default:
			report.Summary.Info++
		}
		report.Summary.Total++
		report.Providers = append(report.Providers, entry)
	}
	return report
}

func printCredentialDoctorReport(w io.Writer, report credentialDoctorReport, verbose bool) {
	fmt.Fprintf(w, "credentials doctor: ok=%d missing=%d info=%d\n",
		report.Summary.OK, report.Summary.Missing, report.Summary.Info)
	for _, entry := range report.Providers {
		line := fmt.Sprintf("- %-12s %-7s", entry.Provider, entry.Status)
		if entry.Resolved != "" {
			line += " " + entry.Resolved
		}
		if entry.Note != "" {
			line += " — " + entry.Note
		}
		fmt.Fprintln(w, line)
		if verbose {
			for _, source := range entry.Sources {
				marker := " "
				if source.Found {
					marker = "x"
				}
				note := ""
				if source.Note != "" {
					note = " (" + source.Note + ")"
				}
				fmt.Fprintf(w, "    [%s] %-9s %s%s\n", marker, source.Kind, source.Location, note)
			}
		}
	}
}

func init() {
	doctorShellCmd.Flags().Bool("json", false, "Output in JSON format")
	doctorShellCmd.Flags().Bool("verbose", false, "Show all tracked binaries, not only mismatches/missing ones")
	doctorCredentialsCmd.Flags().Bool("json", false, "Output in JSON format")
	doctorCredentialsCmd.Flags().Bool("verbose", false, "Show every source in each provider's credential chain, not only the resolved one")
	doctorCmd.AddCommand(doctorShellCmd)
	doctorCmd.AddCommand(doctorCredentialsCmd)
	rootCmd.AddCommand(doctorCmd)
}
