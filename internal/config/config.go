package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	EnabledTools []string `mapstructure:"enabled_tools"`
	IconStyle    string   `mapstructure:"icon_style"` // "braille" or "half-block"
}

func MigrateLegacyConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	legacyPath := filepath.Join(home, ".oct", "config")
	newPath := filepath.Join(home, ".oct", "config.yaml")

	// If new config already exists, skip migration
	if _, err := os.Stat(newPath); err == nil {
		return nil
	}

	// If legacy config doesn't exist, nothing to migrate
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(legacyPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var enabledTools []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "enabled_tools=") {
			value := strings.TrimPrefix(line, "enabled_tools=")
			if value != "all" && value != "" {
				enabledTools = strings.Split(value, ",")
			}
		}
	}

	viper.Set("enabled_tools", enabledTools)
	
	// Create directory if not exists
	err = os.MkdirAll(filepath.Dir(newPath), 0755)
	if err != nil {
		return err
	}

	err = viper.WriteConfigAs(newPath)
	if err != nil {
		return err
	}

	fmt.Printf("Migrated legacy config from %s to %s\n", legacyPath, newPath)
	
	// Optionally backup legacy config
	return os.Rename(legacyPath, legacyPath+".bak")
}
