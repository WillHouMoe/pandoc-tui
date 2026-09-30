// Package config remembers the handful of choices pandoc-tui should not ask
// for twice: where pandoc lives, which folder the user works in, and the last
// value of every conversion option.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// AppName is used for the config and data directories.
const AppName = "pandoc-tui"

// Config is the on-disk state.
type Config struct {
	// PandocPath pins a specific binary; empty means "detect it".
	PandocPath string `json:"pandocPath,omitempty"`
	// LastConverter is the conversion the app reopens with.
	LastConverter string `json:"lastConverter,omitempty"`
	// LastInputDir and LastOutputDir remember where the file pickers should open.
	LastInputDir  string `json:"lastInputDir,omitempty"`
	LastOutputDir string `json:"lastOutputDir,omitempty"`
	// LastStyle is the style file path chosen last.
	LastStyle string `json:"lastStyle,omitempty"`
	// Values holds per-converter option values.
	Values map[string]map[string]string `json:"values,omitempty"`

	path string
}

// Path is where the config file lives.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Dir is the per-user config directory, honouring PANDOC_TUI_HOME.
func Dir() (string, error) {
	if home := os.Getenv("PANDOC_TUI_HOME"); home != "" {
		return home, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// StyleDir is where the style library lives.
func StyleDir() (string, error) {
	if home := os.Getenv("PANDOC_TUI_HOME"); home != "" {
		return filepath.Join(home, "styles"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName, "styles"), nil
}

// Load reads the config, returning defaults when nothing is on disk yet.
func Load() *Config {
	cfg := &Config{Values: map[string]map[string]string{}}
	path, err := Path()
	if err != nil {
		return cfg
	}
	cfg.path = path

	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	// A corrupt config should never stop the app from starting.
	_ = json.Unmarshal(raw, cfg)
	if cfg.Values == nil {
		cfg.Values = map[string]map[string]string{}
	}
	cfg.path = path
	return cfg
}

// ValuesFor returns a copy of the saved option values for a converter.
func (c *Config) ValuesFor(converterID string) map[string]string {
	out := map[string]string{}
	for k, v := range c.Values[converterID] {
		out[k] = v
	}
	return out
}

// SetValues stores the option values of a converter.
func (c *Config) SetValues(converterID string, values map[string]string) {
	if c.Values == nil {
		c.Values = map[string]map[string]string{}
	}
	copied := make(map[string]string, len(values))
	for k, v := range values {
		copied[k] = v
	}
	c.Values[converterID] = copied
}

// Save writes the config atomically enough for a CLI: write a sibling temp
// file, then rename it over the original.
func (c *Config) Save() error {
	path := c.path
	if path == "" {
		var err error
		path, err = Path()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ErrNoConfig is returned by helpers that require a loaded config.
var ErrNoConfig = errors.New("config not loaded")
