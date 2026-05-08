package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/config"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "oct",
	Short:   "One-Click Tools for AI Engineers",
	Long:    `A high-performance CLI tool to manage and update AI-related command-line tools across different platforms.`,
	Version: "0.4.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.oct/config.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home + "/.oct")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetDefault("icon_style", "braille")
	viper.SetDefault("usage_display_mode", "remaining")
	viper.SetDefault("usage_alert_enabled", false)
	viper.SetDefault("usage_alert_threshold_percent", 80.0)
	viper.SetDefault("usage_alert_cooldown_minutes", 360)
	viper.SetDefault("usage_alert_quiet_hours", "")
	viper.SetDefault("usage_alert_timezone", "")
	viper.SetDefault("usage_alert_thresholds", map[string]float64{"default": 80})
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		// Try to migrate if config not found
		if err := config.MigrateLegacyConfig(); err != nil {
			// fmt.Printf("Migration failed: %v\n", err)
		}
	}
}
