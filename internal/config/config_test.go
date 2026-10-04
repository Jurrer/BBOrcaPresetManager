package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	p := DefaultPath()
	if !strings.HasSuffix(p, ".bborcaprofsync.yaml") {
		t.Errorf("DefaultPath() = %q, want suffix .bborcaprofsync.yaml", p)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load non-existent file returned error: %v", err)
	}
	if c.Server.Port != 9876 {
		t.Errorf("Port = %d, want 9876 (default)", c.Server.Port)
	}
	if c.Slicers.BambuStudio.UserDir != "" {
		t.Errorf("BS UserDir = %q, want empty", c.Slicers.BambuStudio.UserDir)
	}
	if c.Slicers.OrcaSlicer.UserDir != "" {
		t.Errorf("Orca UserDir = %q, want empty", c.Slicers.OrcaSlicer.UserDir)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `slicers:
  bambustudio:
    user_dir: "/custom/bs"
    user_id: "123"
  orcaslicer:
    user_dir: "/custom/orca"
    user_id: "abc"
vault:
  path: "/custom/vault"
server:
  port: 9999
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if c.Slicers.BambuStudio.UserDir != "/custom/bs" {
		t.Errorf("BS UserDir = %q, want /custom/bs", c.Slicers.BambuStudio.UserDir)
	}
	if c.Slicers.BambuStudio.UserID != "123" {
		t.Errorf("BS UserID = %q, want 123", c.Slicers.BambuStudio.UserID)
	}
	if c.Slicers.OrcaSlicer.UserDir != "/custom/orca" {
		t.Errorf("Orca UserDir = %q, want /custom/orca", c.Slicers.OrcaSlicer.UserDir)
	}
	if c.Slicers.OrcaSlicer.UserID != "abc" {
		t.Errorf("Orca UserID = %q, want abc", c.Slicers.OrcaSlicer.UserID)
	}
	if c.Vault.Path != "/custom/vault" {
		t.Errorf("Vault Path = %q, want /custom/vault", c.Vault.Path)
	}
	if c.Server.Port != 9999 {
		t.Errorf("Port = %d, want 9999", c.Server.Port)
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `slicers:
  bambustudio:
    user_dir: "/custom/bs"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if c.Server.Port != 9876 {
		t.Errorf("Port = %d, want 9876 (default applied)", c.Server.Port)
	}
	if c.Slicers.BambuStudio.UserDir != "/custom/bs" {
		t.Errorf("BS UserDir = %q, want /custom/bs", c.Slicers.BambuStudio.UserDir)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	original := Config{}
	original.Slicers.BambuStudio = SlicerConfig{UserDir: "/bs", UserID: "123"}
	original.Slicers.OrcaSlicer = SlicerConfig{UserDir: "/orca", UserID: "abc"}
	original.Vault = VaultConfig{Path: "/vault"}
	original.Server = ServerConfig{Port: 8484}
	original.SlicerVersions = map[string]string{"bambustudio": "1.10.0.32", "orcaslicer": "2.0.0"}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loaded.Slicers.BambuStudio.UserDir != "/bs" {
		t.Errorf("BS UserDir = %q, want /bs", loaded.Slicers.BambuStudio.UserDir)
	}
	if loaded.Slicers.BambuStudio.UserID != "123" {
		t.Errorf("BS UserID = %q, want 123", loaded.Slicers.BambuStudio.UserID)
	}
	if loaded.Slicers.OrcaSlicer.UserDir != "/orca" {
		t.Errorf("Orca UserDir = %q, want /orca", loaded.Slicers.OrcaSlicer.UserDir)
	}
	if loaded.Slicers.OrcaSlicer.UserID != "abc" {
		t.Errorf("Orca UserID = %q, want abc", loaded.Slicers.OrcaSlicer.UserID)
	}
	if loaded.Vault.Path != "/vault" {
		t.Errorf("Vault Path = %q, want /vault", loaded.Vault.Path)
	}
	if loaded.Server.Port != 8484 {
		t.Errorf("Port = %d, want 8484", loaded.Server.Port)
	}
	if loaded.SlicerVersions["bambustudio"] != "1.10.0.32" {
		t.Errorf("BS version = %q, want 1.10.0.32", loaded.SlicerVersions["bambustudio"])
	}
	if loaded.SlicerVersions["orcaslicer"] != "2.0.0" {
		t.Errorf("Orca version = %q, want 2.0.0", loaded.SlicerVersions["orcaslicer"])
	}
}

func TestSave_EmptyPathUsesDefault(t *testing.T) {
	// Save with empty path should write to DefaultPath().
	// To avoid clobbering the real config, we can't easily test this
	// without mocking. Instead, verify Save works with a real temp path
	// and creates parent dirs.
	path := filepath.Join(t.TempDir(), "nested", "deep", "config.yaml")
	c := Config{}
	c.Server = ServerConfig{Port: 1234}

	if err := Save(path, c); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestLoad_EmptyPathNoFile(t *testing.T) {
	// Load with empty path uses DefaultPath(). If that doesn't exist on
	// this machine, it should return defaults without error. If it does
	// exist, it should parse it. Either way, no error and port defaults
	// to 9876 if not set.
	c, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") returned error: %v", err)
	}
	// If the default config file exists, port could be non-zero. Only
	// assert the default if the file doesn't exist.
	defaultPath := DefaultPath()
	if _, statErr := os.Stat(defaultPath); statErr != nil && os.IsNotExist(statErr) {
		if c.Server.Port != 9876 {
			t.Errorf("Port = %d, want 9876 (default)", c.Server.Port)
		}
	}
}
