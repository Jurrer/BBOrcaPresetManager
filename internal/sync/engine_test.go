package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
	"github.com/Jurrer/BBOrcaPresetManager/internal/slicer"
	"github.com/Jurrer/BBOrcaPresetManager/internal/vault"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "fixtures")
}

func copyFixture(t *testing.T, dstDir, name, srcBase string) {
	t.Helper()
	for _, ext := range []string{".json", ".info"} {
		src := filepath.Join(fixtureDir(t), srcBase+ext)
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read fixture %s: %v", src, err)
		}
		dst := filepath.Join(dstDir, name+ext)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", dst, err)
		}
	}
}

// readFixtureProfile loads a fixture profile from testdata/fixtures/ in
// normalized internal form.
func readFixtureProfile(t *testing.T, srcBase string, source profile.SlicerID) profile.Profile {
	t.Helper()
	rr, err := profile.Read(fixtureDir(t), srcBase, profile.CategoryProcess, source)
	if err != nil {
		t.Fatalf("profile.Read %s: %v", srcBase, err)
	}
	return rr.Profile
}

// setupEngine creates temp BS + Orca user dirs, a vault in a temp dir,
// and returns a ready Engine plus the BS and Orca temp root paths.
func setupEngine(t *testing.T) (*Engine, string, string) {
	t.Helper()
	bsDir := t.TempDir()
	orcaDir := t.TempDir()
	vaultDir := t.TempDir()

	for _, c := range []profile.Category{profile.CategoryProcess} {
		if err := os.MkdirAll(filepath.Join(bsDir, string(c)), 0o755); err != nil {
			t.Fatalf("MkdirAll bs %s: %v", c, err)
		}
		if err := os.MkdirAll(filepath.Join(orcaDir, string(c)), 0o755); err != nil {
			t.Fatalf("MkdirAll orca %s: %v", c, err)
		}
	}

	bs := slicer.NewBambuStudioAdapter(bsDir, "2240525152", "1.10.0.32")
	orca := slicer.NewOrcaSlicerAdapter(orcaDir, "11b99db1-3c45-4482-a69b-7f430a909e28", "2.0.0")

	v, err := vault.Open(vaultDir)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}

	return NewEngine(bs, orca, v), bsDir, orcaDir
}

// TestSyncAppliesOverridesAndWrites tests the full sync flow: apply
// field overrides, stamp version, stage + commit vault, write slicer dir.
func TestSyncAppliesOverridesAndWrites(t *testing.T) {
	engine, bsDir, _ := setupEngine(t)

	dest := readFixtureProfile(t, "bs_0.2base", profile.SlicerBambuStudio)
	originalSettingID := dest.Info.SettingID
	originalVersion := dest.JSON["version"].Canonical

	overrides := FieldOverrides{
		"top_surface_pattern": "rectilinear",
	}

	res, err := engine.Sync(dest, profile.SlicerBambuStudio, overrides, false, "1.10.0.32")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Commit hash must be non-empty.
	if res.Commit == "" {
		t.Error("expected non-empty commit hash")
	}

	// Two files written: .json and .info.
	if len(res.WrittenFiles) != 2 {
		t.Fatalf("expected 2 written files, got %d", len(res.WrittenFiles))
	}
	for _, p := range res.WrittenFiles {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("written file missing: %s: %v", p, err)
		}
	}

	// Verify the dest file exists in the BS process dir.
	destJSON := filepath.Join(bsDir, "process", dest.Name+".json")
	if _, err := os.Stat(destJSON); err != nil {
		t.Errorf("dest JSON not written to slicer dir: %v", err)
	}

	// Re-read the written profile and verify the override was applied.
	rr, err := profile.Read(filepath.Join(bsDir, "process"), dest.Name, profile.CategoryProcess, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("re-read dest: %v", err)
	}
	got := rr.Profile
	if v := got.JSON["top_surface_pattern"].Canonical; v != "rectilinear" {
		t.Errorf("top_surface_pattern = %v, want rectilinear", v)
	}

	// Version should be stamped.
	if v := got.JSON["version"].Canonical; v != "1.10.0.32" {
		t.Errorf("version = %v, want 1.10.0.32", v)
	}

	// isNew=false: setting_id must be preserved.
	if got.Info.SettingID != originalSettingID {
		t.Errorf("setting_id changed: got %q, want %q", got.Info.SettingID, originalSettingID)
	}

	// Original version should match (fixture already has 1.10.0.32).
	if originalVersion != "1.10.0.32" {
		t.Errorf("fixture version unexpected: %v", originalVersion)
	}

	// Vault history must be non-empty (committed).
	hist, err := engine.vault.History()
	if err != nil {
		t.Fatalf("vault.History: %v", err)
	}
	if len(hist) == 0 {
		t.Error("vault history is empty after sync")
	}

	// Commit message should contain the profile name and category.
	foundSync := false
	for _, c := range hist {
		if strings.Contains(c.Message, dest.Name) && strings.Contains(c.Message, "process") {
			foundSync = true
			break
		}
	}
	if !foundSync {
		t.Error("no commit message referencing the profile name found in history")
	}
}

// TestSyncNewProfileGeneratesSettingID verifies that isNew=true causes
// a fresh setting_id to be generated for the dest slicer.
func TestSyncNewProfileGeneratesSettingID(t *testing.T) {
	engine, bsDir, _ := setupEngine(t)

	dest := readFixtureProfile(t, "bs_0.2base", profile.SlicerBambuStudio)
	oldSettingID := dest.Info.SettingID

	overrides := FieldOverrides{
		"top_surface_pattern": "rectilinear",
	}

	res, err := engine.Sync(dest, profile.SlicerBambuStudio, overrides, true, "1.10.0.32")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Commit == "" {
		t.Error("expected non-empty commit hash")
	}

	rr, err := profile.Read(filepath.Join(bsDir, "process"), dest.Name, profile.CategoryProcess, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("re-read dest: %v", err)
	}
	got := rr.Profile

	// setting_id must be a new PPUS ID, different from the original.
	if !strings.HasPrefix(got.Info.SettingID, "PPUS") {
		t.Errorf("expected PPUS setting_id, got %q", got.Info.SettingID)
	}
	if got.Info.SettingID == oldSettingID {
		t.Errorf("setting_id not regenerated: still %q", got.Info.SettingID)
	}
}

// TestSyncNoOverrides tests that an empty override map still produces
// a valid sync (version stamp, vault commit, write).
func TestSyncNoOverrides(t *testing.T) {
	engine, bsDir, _ := setupEngine(t)

	dest := readFixtureProfile(t, "bs_0.2base", profile.SlicerBambuStudio)

	res, err := engine.Sync(dest, profile.SlicerBambuStudio, FieldOverrides{}, false, "1.10.0.32")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Commit == "" {
		t.Error("expected non-empty commit hash")
	}

	// Verify the dest file is written and version stamped.
	rr, err := profile.Read(filepath.Join(bsDir, "process"), dest.Name, profile.CategoryProcess, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("re-read dest: %v", err)
	}
	got := rr.Profile
	if v := got.JSON["version"].Canonical; v != "1.10.0.32" {
		t.Errorf("version = %v, want 1.10.0.32", v)
	}
}

// TestSyncOrcaSlicerDest tests syncing to OrcaSlicer as dest.
func TestSyncOrcaSlicerDest(t *testing.T) {
	engine, _, orcaDir := setupEngine(t)

	dest := readFixtureProfile(t, "orca_0.2base", profile.SlicerOrcaSlicer)
	originalSettingID := dest.Info.SettingID

	overrides := FieldOverrides{
		"top_surface_pattern": "zig-zag",
	}

	res, err := engine.Sync(dest, profile.SlicerOrcaSlicer, overrides, false, "2.0.0")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Commit == "" {
		t.Error("expected non-empty commit hash")
	}

	rr, err := profile.Read(filepath.Join(orcaDir, "process"), dest.Name, profile.CategoryProcess, profile.SlicerOrcaSlicer)
	if err != nil {
		t.Fatalf("re-read dest: %v", err)
	}
	got := rr.Profile
	if v := got.JSON["top_surface_pattern"].Canonical; v != "zig-zag" {
		t.Errorf("top_surface_pattern = %v, want zig-zag", v)
	}
	if v := got.JSON["version"].Canonical; v != "2.0.0" {
		t.Errorf("version = %v, want 2.0.0", v)
	}
	if got.Info.SettingID != originalSettingID {
		t.Errorf("setting_id changed on non-new sync: got %q, want %q", got.Info.SettingID, originalSettingID)
	}
}
