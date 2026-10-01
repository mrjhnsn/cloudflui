// Package config manages the TOML profile store for cloudflui.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Profile is a single Cloudflare API credential set.
type Profile struct {
	Name      string `toml:"name"`
	APIKey    string `toml:"api_key"`
	AccountID string `toml:"account_id"`
}

// Config is the root TOML document.
type Config struct {
	DefaultProfile string    `toml:"default_profile"`
	Profiles       []Profile `toml:"profiles"`
}

// configPath overrides the default config location (used in tests).
var configPath string

// SetPath overrides the config file location. Pass "" to restore the default.
func SetPath(path string) {
	configPath = path
}

// Path returns the config file location (~/.config/cloudflui/config.toml).
func Path() string {
	if configPath != "" {
		return configPath
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "cloudflui", "config.toml")
}

// Load reads the config file. A missing file yields an empty config.
func Load() (*Config, error) {
	cfg := &Config{}
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config file, creating parent directories with 0700 perms.
func (c *Config) Save() error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Active returns the profile to use: the default profile if set, otherwise
// the first profile. Returns nil when no profiles exist.
func (c *Config) Active() *Profile {
	if len(c.Profiles) == 0 {
		return nil
	}
	if c.DefaultProfile != "" {
		for i := range c.Profiles {
			if c.Profiles[i].Name == c.DefaultProfile {
				return &c.Profiles[i]
			}
		}
	}
	return &c.Profiles[0]
}

// Profile returns the profile with the given name, or nil if not found.
func (c *Config) Profile(name string) *Profile {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i]
		}
	}
	return nil
}
