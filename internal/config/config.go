package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Config holds cispectre settings loaded from .cispectre.yaml.
type Config struct {
	IdleDays int     `yaml:"idle_days"`
	MinCost  float64 `yaml:"min_cost"`
	Format   string  `yaml:"format"`
	Token    string  `yaml:"token"`
}

// Load reads config from ~/.cispectre.yaml then ./.cispectre.yaml.
// Later files override earlier ones. Missing files are silently ignored.
func Load() Config {
	var cfg Config

	if home, err := os.UserHomeDir(); err == nil {
		loadFile(filepath.Join(home, ".cispectre.yaml"), &cfg)
	}

	loadFile(".cispectre.yaml", &cfg)

	return cfg
}

// LoadFrom reads config from a specific path. Used for testing.
func LoadFrom(path string) Config {
	var cfg Config
	loadFile(path, &cfg)
	return cfg
}

func loadFile(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = yaml.Unmarshal(data, cfg)
}

// Apply sets Cobra flag defaults from config values.
// Flags explicitly set by the user on the command line are not overridden.
func (c Config) Apply(cmd *cobra.Command) {
	flags := cmd.Flags()

	if c.Format != "" && !flags.Changed("format") {
		_ = flags.Set("format", c.Format)
	}
	if c.Token != "" && !flags.Changed("token") {
		_ = flags.Set("token", c.Token)
	}
	if c.IdleDays != 0 && !flags.Changed("idle-days") {
		_ = flags.Set("idle-days", intToStr(c.IdleDays))
	}
	if c.MinCost != 0 && !flags.Changed("min-cost") {
		_ = flags.Set("min-cost", floatToStr(c.MinCost))
	}
}

func intToStr(v int) string {
	return fmt.Sprintf("%d", v)
}

func floatToStr(v float64) string {
	return fmt.Sprintf("%g", v)
}
