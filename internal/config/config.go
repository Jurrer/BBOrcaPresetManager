// Package config loads/saves ~/.bborcaprofsync.yaml. See plan.md §9.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SlicerConfig holds per-slicer overrides. Empty fields mean
// "use auto-detected default" (plan §9).
type SlicerConfig struct {
	UserDir string `yaml:"user_dir" json:"user_dir"`
	UserID  string `yaml:"user_id" json:"user_id"`
}

// VaultConfig holds vault-path overrides.
type VaultConfig struct {
	Path string `yaml:"path" json:"path"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int `yaml:"port" json:"port"`
}

// Config is the top-level YAML shape (plan §9).
type Config struct {
	Slicers struct {
		BambuStudio SlicerConfig `yaml:"bambustudio" json:"bambustudio"`
		OrcaSlicer  SlicerConfig `yaml:"orcaslicer" json:"orcaslicer"`
	} `yaml:"slicers" json:"slicers"`
	Vault          VaultConfig       `yaml:"vault" json:"vault"`
	Server         ServerConfig      `yaml:"server" json:"server"`
	SlicerVersions map[string]string `yaml:"slicer_versions" json:"slicer_versions"`
}

// DefaultPath returns the conventional config path: ~/.bborcaprofsync.yaml.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.bborcaprofsync.yaml"
}

// Load reads the config file at path, returning defaults if the file
// doesn't exist.
func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return withDefaults(Config{}), nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	return withDefaults(c), nil
}

// Save writes the config to path atomically (temp + rename).
func Save(path string, c Config) error {
	if path == "" {
		path = DefaultPath()
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config dir %s: %w", dir, err)
		}
	}

	data, err := yaml.Marshal(&c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".bborcaprofsync-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp to %s: %w", path, err)
	}

	return nil
}

const defaultPort = 9876

// withDefaults applies sensible defaults for missing/zero fields.
func withDefaults(c Config) Config {
	if c.Server.Port == 0 {
		c.Server.Port = defaultPort
	}
	if c.SlicerVersions == nil {
		c.SlicerVersions = make(map[string]string)
	}
	return c
}
