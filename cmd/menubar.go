package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var menubarDaemon bool
var menubarLegacy bool

type menubarDoctorReport struct {
	GOOS          string   `json:"goos"`
	ExecPath      string   `json:"exec_path"`
	WorkingDir    string   `json:"working_dir"`
	HelperPath    string   `json:"helper_path,omitempty"`
	HelperProject string   `json:"helper_project,omitempty"`
	Searched      []string `json:"searched"`
	LaunchMode    string   `json:"launch_mode"`
}

var menubarCmd = &cobra.Command{
	Use:     "menubar",
	GroupID: "core",
	Short:   "Run macOS menu bar app (status item)",
	Run: func(cmd *cobra.Command, args []string) {
		if menubarDaemon {
			if err := startMenubarDetached(); err != nil {
				fmt.Printf("menubar daemon start failed: %v\n", err)
				return
			}
			fmt.Println("menubar daemon started")
			return
		}

		if err := runMenubar(); err != nil {
			fmt.Printf("menubar failed: %v\n", err)
		}
	},
}

var menubarDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Show menubar helper resolution and launch diagnostics",
	RunE: func(cmd *cobra.Command, args []string) error {
		report, err := collectMenubarDoctorReport()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "menubar doctor\n")
		fmt.Fprintf(cmd.OutOrStdout(), "- goos: %s\n", report.GOOS)
		fmt.Fprintf(cmd.OutOrStdout(), "- exec: %s\n", report.ExecPath)
		fmt.Fprintf(cmd.OutOrStdout(), "- working dir: %s\n", report.WorkingDir)
		fmt.Fprintf(cmd.OutOrStdout(), "- launch mode: %s\n", report.LaunchMode)
		if report.HelperPath != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- helper: %s\n", report.HelperPath)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "- helper: not found")
		}
		if report.HelperProject != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "- helper project: %s\n", report.HelperProject)
		}
		if len(report.Searched) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "- searched:")
			for _, item := range report.Searched {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", item)
			}
		}
		return nil
	},
}

var menubarBuildHelperCmd = &cobra.Command{
	Use:   "build-helper",
	Short: "Build the Swift menubar helper app",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("menubar helper build is supported only on macOS")
		}
		projectDir, _, err := resolveMenubarProjectDirForCurrentProcess()
		if err != nil {
			return err
		}
		return buildMenubarHelper(projectDir)
	},
}

var menubarInstallHelperCmd = &cobra.Command{
	Use:   "install-helper",
	Short: "Install the built Swift menubar helper into ~/.local/bin",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("menubar helper install is supported only on macOS")
		}
		projectDir, _, err := resolveMenubarProjectDirForCurrentProcess()
		if err != nil {
			return err
		}
		dst, err := installMenubarHelper(projectDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "installed menubar helper: %s\n", dst)
		return nil
	},
}

func collectMenubarDoctorReport() (menubarDoctorReport, error) {
	execPath, err := os.Executable()
	if err != nil {
		return menubarDoctorReport{}, err
	}
	workingDir, _ := os.Getwd()
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	helperPath, searched := resolveMenubarHelperPath(env, execPath, workingDir)
	projectDir, projectSearched, _ := resolveMenubarProjectDir(execPath, workingDir)
	searched = append(searched, projectSearched...)
	launchMode := "legacy-fallback"
	if menubarLegacy {
		launchMode = "legacy-forced"
	} else if helperPath != "" {
		launchMode = "swift-helper"
	}
	return menubarDoctorReport{
		GOOS:          runtime.GOOS,
		ExecPath:      execPath,
		WorkingDir:    workingDir,
		HelperPath:    helperPath,
		HelperProject: projectDir,
		Searched:      dedupeStrings(searched),
		LaunchMode:    launchMode,
	}, nil
}

func resolveMenubarProjectDirForCurrentProcess() (string, []string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", nil, err
	}
	workingDir, _ := os.Getwd()
	return resolveMenubarProjectDir(execPath, workingDir)
}

func resolveMenubarProjectDir(execPath, workingDir string) (string, []string, error) {
	baseDirs := []string{workingDir, filepath.Dir(execPath)}
	searched := []string{}
	seen := map[string]struct{}{}
	for _, base := range baseDirs {
		if strings.TrimSpace(base) == "" {
			continue
		}
		cursor := filepath.Clean(base)
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(cursor, "macos", "OctMenubar")
			if _, ok := seen[candidate]; !ok {
				seen[candidate] = struct{}{}
				searched = append(searched, candidate)
			}
			if info, err := os.Stat(filepath.Join(candidate, "Package.swift")); err == nil && !info.IsDir() {
				return candidate, searched, nil
			}
			parent := filepath.Dir(cursor)
			if parent == cursor {
				break
			}
			cursor = parent
		}
	}
	return "", searched, fmt.Errorf("menubar helper project not found")
}

func defaultMenubarInstallPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin", "OctMenubarApp"), nil
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func init() {
	menubarCmd.Flags().BoolVar(&menubarDaemon, "daemon", false, "start menubar in background and return")
	menubarCmd.Flags().BoolVar(&menubarLegacy, "legacy", false, "force legacy systray/NSMenu menubar instead of Swift helper")
	menubarCmd.AddCommand(menubarDoctorCmd)
	menubarCmd.AddCommand(menubarBuildHelperCmd)
	menubarCmd.AddCommand(menubarInstallHelperCmd)
	rootCmd.AddCommand(menubarCmd)
}
