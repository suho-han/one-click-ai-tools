package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/config"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "oct",
	Short:   "One binary that organizes your AI coding CLIs",
	Long:    `One binary that organizes your AI coding CLIs — update them all, watch every quota plan, schedule the maintenance.`,
	Version: "0.1.6-beta.4",
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	// Errors are reported once by runCLI/Execute on stderr; cobra must not
	// print them (or usage) itself.
	SilenceErrors: true,
	SilenceUsage:  true,
}

// runCLI runs the root command with explicit streams and returns the process
// exit code, keeping error output on the same stderr the caller can observe.
func runCLI(args []string, stdout, stderr io.Writer) int {
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "oct: %v\n", err)
		return 1
	}
	return 0
}

// Execute is the binary entrypoint. Errors reach stderr exactly once (cobra
// is silenced; runCLI prints the returned error).
func Execute() {
	reorderRootCommands()
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func init() {
	cobra.EnableCommandSorting = false
	cobra.OnInitialize(initConfig)
	cobra.AddTemplateFunc("commandHelpLine", commandHelpLine)

	rootCmd.AddGroup(
		&cobra.Group{ID: "core", Title: "⚡ Core Commands (frequently used)"},
		&cobra.Group{ID: "manage", Title: "⚙️ Configuration & Scheduling"},
		&cobra.Group{ID: "maintenance", Title: "🛠️ Update & Maintenance"},
		&cobra.Group{ID: "help", Title: "🧭 Help & Shell"},
	)
	rootCmd.SetHelpCommand(newRootHelpCommand())
	rootCmd.SetUsageTemplate(rootUsageTemplate)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.oct/config.yaml)")
}

const rootUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{commandHelpLine .}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{commandHelpLine .}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{commandHelpLine .}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func commandHelpLine(cmd *cobra.Command) string {
	emoji, description := splitHelpEmoji(cmd.Short)
	name := fmt.Sprintf("%-*s", cmd.NamePadding(), cmd.Name())
	if emoji == "" {
		return fmt.Sprintf("%s %s", name, description)
	}
	// The space between emoji and name keeps double-width glyphs from
	// rendering on top of the first letter in terminals that undercount emoji
	// width.
	return fmt.Sprintf("%s %s %s", emoji, name, description)
}

func splitHelpEmoji(short string) (string, string) {
	emoji, description, ok := strings.Cut(short, " ")
	if !ok {
		return "", short
	}
	for _, r := range emoji {
		if r > 127 {
			return emoji, description
		}
	}
	return "", short
}

func newRootHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "help [command]",
		GroupID: "help",
		Short:   "❓ Help about any command",
		Long: `Help provides help for any command in the application.
Simply type oct help [path to command] for full details.`,
		ValidArgsFunction: func(c *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			var completions []cobra.Completion
			cmd, _, err := c.Root().Find(args)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			if cmd == nil {
				cmd = c.Root()
			}
			for _, subCmd := range cmd.Commands() {
				if (subCmd.IsAvailableCommand() || subCmd.Name() == "help") && strings.HasPrefix(subCmd.Name(), toComplete) {
					completions = append(completions, cobra.CompletionWithDesc(subCmd.Name(), subCmd.Short))
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(c *cobra.Command, args []string) {
			cmd, _, err := c.Root().Find(args)
			if cmd == nil || err != nil {
				c.Printf("Unknown help topic %#q\n", args)
				cobra.CheckErr(c.Root().Usage())
				return
			}
			if cmd.Context() == nil {
				cmd.SetContext(c.Context())
			}
			cmd.InitDefaultHelpFlag()
			cmd.InitDefaultVersionFlag()
			cobra.CheckErr(cmd.Help())
		},
	}
}

func reorderRootCommands() {
	preferred := []string{"usage", "monitor", "menubar", "config", "alert", "schedule", "update", "agent-update", "session-refresh", "doctor", "release-doctor", "help"}
	current := rootCmd.Commands()
	if len(current) == 0 {
		return
	}

	byName := make(map[string]*cobra.Command, len(current))
	for _, c := range current {
		byName[c.Name()] = c
	}

	for _, c := range current {
		rootCmd.RemoveCommand(c)
	}

	added := make(map[string]bool, len(current))
	for _, name := range preferred {
		if c, ok := byName[name]; ok {
			rootCmd.AddCommand(c)
			added[name] = true
		}
	}

	for _, c := range current {
		if !added[c.Name()] {
			rootCmd.AddCommand(c)
		}
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to resolve home directory: %v\n", err)
			os.Exit(1)
		}

		viper.AddConfigPath(home + "/.oct")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetDefault("icon_style", "braille")
	viper.SetDefault("usage_alert_enabled", false)
	viper.SetDefault("usage_alert_threshold_percent", 80.0)
	viper.SetDefault("usage_alert_critical_percent", 98.0)
	viper.SetDefault("usage_alert_cooldown_minutes", 360)
	viper.SetDefault("usage_alert_quiet_hours", "")
	viper.SetDefault("usage_alert_timezone", "")
	viper.SetDefault("usage_alert_thresholds", map[string]float64{"default": 80})
	viper.SetDefault("session_refresh_enabled", false)
	viper.SetDefault("session_refresh_interval", "daily")
	viper.SetDefault("session_refresh_hour", 9)
	viper.SetDefault("menubar_refresh_interval", "1m")
	viper.SetDefault("menubar_title_mode", "oct")
	// Qwen Code has no public usage API; the provider counts today's local
	// token-usage records against this daily request cap.
	viper.SetDefault("qwen_daily_limit", 100)
	// Avoid accidental overrides from generic env vars like ENABLED_TOOLS.
	// Require explicit OCT_* variables (e.g., OCT_ENABLED_TOOLS) for env-based overrides.
	viper.SetEnvPrefix("OCT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		return
	} else {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			if cfgFile != "" {
				fmt.Fprintf(os.Stderr, "failed to read config file %q: %v\n", cfgFile, err)
				os.Exit(1)
			}
			if migrateErr := config.MigrateLegacyConfig(); migrateErr != nil {
				fmt.Fprintf(os.Stderr, "failed to migrate legacy config: %v\n", migrateErr)
				os.Exit(1)
			}
			return
		}

		configPath := cfgFile
		if configPath == "" {
			configPath = viper.ConfigFileUsed()
		}
		if configPath == "" {
			configPath = "$HOME/.oct/config.yaml"
		}
		fmt.Fprintf(os.Stderr, "failed to load config %s: %v\n", configPath, err)
		os.Exit(1)
	}
}
