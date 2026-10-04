// Tests for the slicer adapter package.
package slicer

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// fixtureDir returns the path to the shared test fixtures.
// From internal/slicer/ we go up two levels to the repo root, then into
// testdata/fixtures.
func fixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "fixtures")
}

// writeFixture copies a fixture profile (.json + .info) into the given
// destination directory under the chosen name. The name is the base name
// of the file pair that will be written.
func writeFixture(t *testing.T, dstDir, name, srcBase string) {
	t.Helper()
	for _, ext := range []string{".json", ".info"} {
		src := filepath.Join(fixtureDir(t), srcBase+ext)
		dst := filepath.Join(dstDir, name+ext)
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read fixture %s: %v", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", dst, err)
		}
	}
}

// --- BambuStudioAdapter ------------------------------------------------------

func TestBambuStudioID(t *testing.T) {
	a := NewBambuStudioAdapter("/tmp/user/uid", "uid", "1.10.0")
	if got := a.ID(); got != profile.SlicerBambuStudio {
		t.Errorf("ID() = %q, want %q", got, profile.SlicerBambuStudio)
	}
}

func TestBambuStudioVersionAndUserDir(t *testing.T) {
	a := NewBambuStudioAdapter("/tmp/user/uid", "uid", "1.10.0.32")
	if got := a.Version(); got != "1.10.0.32" {
		t.Errorf("Version() = %q, want %q", got, "1.10.0.32")
	}
	if got := a.UserDir(); got != "/tmp/user/uid" {
		t.Errorf("UserDir() = %q, want %q", got, "/tmp/user/uid")
	}
}

func TestBambuStudioFieldRegistryNotNil(t *testing.T) {
	a := NewBambuStudioAdapter("/tmp/user/uid", "uid", "")
	if got := a.FieldRegistry(); got == nil {
		t.Error("FieldRegistry() returned nil")
	}
}

func TestBambuStudioCategoryDir(t *testing.T) {
	a := NewBambuStudioAdapter("/tmp/user/uid", "uid", "")
	tests := []struct {
		cat  profile.Category
		want string
	}{
		{profile.CategoryMachine, "/tmp/user/uid/machine"},
		{profile.CategoryProcess, "/tmp/user/uid/process"},
		{profile.CategoryFilament, "/tmp/user/uid/filament"},
	}
	for _, tc := range tests {
		if got := a.CategoryDir(tc.cat); got != tc.want {
			t.Errorf("CategoryDir(%q) = %q, want %q", tc.cat, got, tc.want)
		}
	}
}

func TestBambuStudioListProfilesEmpty(t *testing.T) {
	// userDir points at a non-existent path: ListProfiles should return
	// an empty slice and no error.
	a := NewBambuStudioAdapter(t.TempDir(), "uid", "")
	got, err := a.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListProfiles = %v, want empty slice", got)
	}
}

func TestBambuStudioListProfiles(t *testing.T) {
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create fixture file pairs in the process dir.
	writeFixture(t, processDir, "0.2 base", "bs_0.2base")
	writeFixture(t, processDir, "Draft", "bs_0.2base")
	writeFixture(t, processDir, "Fine", "bs_0.2base")

	a := NewBambuStudioAdapter(root, "uid", "")
	got, err := a.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}

	want := []string{"0.2 base", "Draft", "Fine"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListProfiles = %v, want %v", got, want)
	}
}

func TestBambuStudioListProfilesSorted(t *testing.T) {
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write the profiles in non-alphabetical order and assert the result
	// is sorted regardless of insertion order.
	names := []string{"zebra", "alpha", "mango", "bravo"}
	for _, n := range names {
		writeFixture(t, processDir, n, "bs_0.2base")
	}

	a := NewBambuStudioAdapter(root, "uid", "")
	got, err := a.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}

	want := []string{"alpha", "bravo", "mango", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListProfiles = %v, want %v (sorted)", got, want)
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("ListProfiles result not sorted: %v", got)
	}
}

func TestBambuStudioListProfilesIgnoresNonJSON(t *testing.T) {
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// A real .json file pair, plus a stray .txt and a stray .json with no
	// matching .info. Both .json files should be listed (the .json-only one
	// counts as a profile name from ListProfiles' perspective — ReadProfile
	// would fail on it).
	writeFixture(t, processDir, "real", "bs_0.2base")
	if err := os.WriteFile(filepath.Join(processDir, "stray.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write stray.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(processDir, "nojson.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write nojson.json: %v", err)
	}

	a := NewBambuStudioAdapter(root, "uid", "")
	got, err := a.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}

	want := []string{"nojson", "real"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListProfiles = %v, want %v", got, want)
	}
}

func TestBambuStudioListProfilesOtherCategoryEmpty(t *testing.T) {
	// Profiles exist under process/ but not under filament/.
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFixture(t, processDir, "0.2 base", "bs_0.2base")

	a := NewBambuStudioAdapter(root, "uid", "")
	got, err := a.ListProfiles(profile.CategoryFilament)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListProfiles(filament) = %v, want empty", got)
	}
}

func TestBambuStudioWriteProfileCreatesFiles(t *testing.T) {
	// Write a profile via the adapter; verify the .json and .info files
	// appear in the right directory. Write requires the category dir to
	// already exist.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "process"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	a := NewBambuStudioAdapter(root, "uid", "1.10.0.32")

	p := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "TestProf",
		JSON: map[string]profile.Value{
			"name":              {Raw: "TestProf", Canonical: "TestProf"},
			"version":           {Raw: "1.10.0.32", Canonical: "1.10.0.32"},
			"inner_wall_speed":  {Raw: "400", Canonical: "400"},
			"from":              {Raw: "User", Canonical: "User"},
			"print_settings_id": {Raw: "TestProf", Canonical: "TestProf"},
		},
		Info: profile.Info{
			UserID:      "uid",
			SettingID:   "PPUSabcdef",
			BaseID:      "GP079",
			UpdatedTime: 1234567890,
		},
		Source: profile.SlicerBambuStudio,
	}

	wr, err := a.WriteProfile(p)
	if err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}

	if _, err := os.Stat(wr.JSONPath); err != nil {
		t.Errorf("JSON file not written: %v", err)
	}
	if _, err := os.Stat(wr.InfoPath); err != nil {
		t.Errorf("Info file not written: %v", err)
	}

	wantJSON := filepath.Join(a.CategoryDir(profile.CategoryProcess), "TestProf.json")
	wantInfo := filepath.Join(a.CategoryDir(profile.CategoryProcess), "TestProf.info")
	if wr.JSONPath != wantJSON {
		t.Errorf("JSONPath = %q, want %q", wr.JSONPath, wantJSON)
	}
	if wr.InfoPath != wantInfo {
		t.Errorf("InfoPath = %q, want %q", wr.InfoPath, wantInfo)
	}
}

func TestBambuStudioReadProfileRoundTrip(t *testing.T) {
	// Write fixture copy, then read it back through the adapter and verify
	// key fields match.
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFixture(t, processDir, "0.2 base", "bs_0.2base")

	a := NewBambuStudioAdapter(root, "uid", "1.10.0.32")
	rr, err := a.ReadProfile(profile.CategoryProcess, "0.2 base")
	if err != nil {
		t.Fatalf("ReadProfile: %v", err)
	}

	if rr.Profile.Name != "0.2 base" {
		t.Errorf("Name = %q, want %q", rr.Profile.Name, "0.2 base")
	}
	if rr.Profile.Source != profile.SlicerBambuStudio {
		t.Errorf("Source = %q, want %q", rr.Profile.Source, profile.SlicerBambuStudio)
	}

	// BS wraps speed fields in arrays; Canonical should be scalar.
	if v, ok := rr.Profile.JSON["inner_wall_speed"]; !ok {
		t.Error("missing field inner_wall_speed")
	} else if v.Canonical != "400" {
		t.Errorf("Canonical[inner_wall_speed] = %v, want \"400\"", v.Canonical)
	}

	// Verify Info was loaded.
	if rr.Profile.Info.UserID != "2240525152" {
		t.Errorf("UserID = %q, want %q", rr.Profile.Info.UserID, "2240525152")
	}
	if rr.Profile.Info.SettingID != "PPUS249810809cffcb" {
		t.Errorf("SettingID = %q, want %q", rr.Profile.Info.SettingID, "PPUS249810809cffcb")
	}
}

// --- OrcaSlicerAdapter -------------------------------------------------------

func TestOrcaSlicerID(t *testing.T) {
	a := NewOrcaSlicerAdapter("/tmp/user/uid", "uid", "2.0.0")
	if got := a.ID(); got != profile.SlicerOrcaSlicer {
		t.Errorf("ID() = %q, want %q", got, profile.SlicerOrcaSlicer)
	}
}

func TestOrcaSlicerVersionAndUserDir(t *testing.T) {
	a := NewOrcaSlicerAdapter("/tmp/user/uid", "uid", "2.0.0")
	if got := a.Version(); got != "2.0.0" {
		t.Errorf("Version() = %q, want %q", got, "2.0.0")
	}
	if got := a.UserDir(); got != "/tmp/user/uid" {
		t.Errorf("UserDir() = %q, want %q", got, "/tmp/user/uid")
	}
}

func TestOrcaSlicerFieldRegistryNotNil(t *testing.T) {
	a := NewOrcaSlicerAdapter("/tmp/user/uid", "uid", "")
	if got := a.FieldRegistry(); got == nil {
		t.Error("FieldRegistry() returned nil")
	}
}

func TestOrcaSlicerCategoryDir(t *testing.T) {
	a := NewOrcaSlicerAdapter("/tmp/user/uid", "uid", "")
	tests := []struct {
		cat  profile.Category
		want string
	}{
		{profile.CategoryMachine, "/tmp/user/uid/machine"},
		{profile.CategoryProcess, "/tmp/user/uid/process"},
		{profile.CategoryFilament, "/tmp/user/uid/filament"},
	}
	for _, tc := range tests {
		if got := a.CategoryDir(tc.cat); got != tc.want {
			t.Errorf("CategoryDir(%q) = %q, want %q", tc.cat, got, tc.want)
		}
	}
}

func TestOrcaSlicerListProfilesEmpty(t *testing.T) {
	a := NewOrcaSlicerAdapter(t.TempDir(), "uid", "")
	got, err := a.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListProfiles = %v, want empty slice", got)
	}
}

func TestOrcaSlicerListProfiles(t *testing.T) {
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	writeFixture(t, processDir, "0.2 base", "orca_0.2base")
	writeFixture(t, processDir, "Draft", "orca_0.2base")
	writeFixture(t, processDir, "Fine", "orca_0.2base")

	a := NewOrcaSlicerAdapter(root, "uid", "")
	got, err := a.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}

	want := []string{"0.2 base", "Draft", "Fine"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListProfiles = %v, want %v", got, want)
	}
}

func TestOrcaSlicerListProfilesSorted(t *testing.T) {
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	names := []string{"zebra", "alpha", "mango", "bravo"}
	for _, n := range names {
		writeFixture(t, processDir, n, "orca_0.2base")
	}

	a := NewOrcaSlicerAdapter(root, "uid", "")
	got, err := a.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}

	want := []string{"alpha", "bravo", "mango", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListProfiles = %v, want %v (sorted)", got, want)
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("ListProfiles result not sorted: %v", got)
	}
}

func TestOrcaSlicerWriteProfileCreatesFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "process"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	a := NewOrcaSlicerAdapter(root, "uid", "2.0.0")

	p := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "OrcaTest",
		JSON: map[string]profile.Value{
			"name":              {Raw: "OrcaTest", Canonical: "OrcaTest"},
			"version":           {Raw: "2.0.0", Canonical: "2.0.0"},
			"inner_wall_speed":  {Raw: "400", Canonical: "400"},
			"from":              {Raw: "User", Canonical: "User"},
			"print_settings_id": {Raw: "OrcaTest", Canonical: "OrcaTest"},
		},
		Info: profile.Info{
			UserID:      "11b99db1-3c45-4482-a69b-7f430a909e28",
			SettingID:   "785471be-2f8f-5ee6-a9ab-805bdbd30ab5",
			BaseID:      "GP079",
			UpdatedTime: 1234567890,
		},
		Source: profile.SlicerOrcaSlicer,
	}

	wr, err := a.WriteProfile(p)
	if err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}

	if _, err := os.Stat(wr.JSONPath); err != nil {
		t.Errorf("JSON file not written: %v", err)
	}
	if _, err := os.Stat(wr.InfoPath); err != nil {
		t.Errorf("Info file not written: %v", err)
	}

	wantJSON := filepath.Join(a.CategoryDir(profile.CategoryProcess), "OrcaTest.json")
	wantInfo := filepath.Join(a.CategoryDir(profile.CategoryProcess), "OrcaTest.info")
	if wr.JSONPath != wantJSON {
		t.Errorf("JSONPath = %q, want %q", wr.JSONPath, wantJSON)
	}
	if wr.InfoPath != wantInfo {
		t.Errorf("InfoPath = %q, want %q", wr.InfoPath, wantInfo)
	}
}

func TestOrcaSlicerReadProfileRoundTrip(t *testing.T) {
	root := t.TempDir()
	processDir := filepath.Join(root, "process")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFixture(t, processDir, "0.2 base", "orca_0.2base")

	a := NewOrcaSlicerAdapter(root, "uid", "2.0.0")
	rr, err := a.ReadProfile(profile.CategoryProcess, "0.2 base")
	if err != nil {
		t.Fatalf("ReadProfile: %v", err)
	}

	if rr.Profile.Name != "0.2 base" {
		t.Errorf("Name = %q, want %q", rr.Profile.Name, "0.2 base")
	}
	if rr.Profile.Source != profile.SlicerOrcaSlicer {
		t.Errorf("Source = %q, want %q", rr.Profile.Source, profile.SlicerOrcaSlicer)
	}

	// Orca uses scalar for speed fields; Raw should be a string.
	if v, ok := rr.Profile.JSON["inner_wall_speed"]; !ok {
		t.Error("missing field inner_wall_speed")
	} else {
		if v.Canonical != "400" {
			t.Errorf("Canonical[inner_wall_speed] = %v, want \"400\"", v.Canonical)
		}
		if _, isArray := v.Raw.([]any); isArray {
			t.Errorf("Raw[inner_wall_speed] should be scalar for Orca, got array")
		}
	}

	// Info should reflect the Orca UUID.
	if rr.Profile.Info.UserID != "11b99db1-3c45-4482-a69b-7f430a909e28" {
		t.Errorf("UserID = %q, want UUID", rr.Profile.Info.UserID)
	}
	if rr.Profile.Info.SettingID != "785471be-2f8f-5ee6-a9ab-805bdbd30ab5" {
		t.Errorf("SettingID = %q, want UUID", rr.Profile.Info.SettingID)
	}
}

// --- Cross-adapter ----------------------------------------------------------

func TestCrossAdapterListSameFixture(t *testing.T) {
	// Both adapters should be able to list the same fixture file pairs
	// from their respective category dirs. The list of names should be
	// identical regardless of slicer because ListProfiles just enumerates
	// the directory.
	root := t.TempDir()
	bsProcess := filepath.Join(root, "bs", "user", "uid", "process")
	orcaProcess := filepath.Join(root, "orca", "user", "uid", "process")
	for _, d := range []string{bsProcess, orcaProcess} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	names := []string{"alpha", "bravo", "charlie"}
	for _, n := range names {
		writeFixture(t, bsProcess, n, "bs_0.2base")
		writeFixture(t, orcaProcess, n, "orca_0.2base")
	}

	bs := NewBambuStudioAdapter(filepath.Join(root, "bs", "user", "uid"), "uid", "")
	orca := NewOrcaSlicerAdapter(filepath.Join(root, "orca", "user", "uid"), "uid", "")

	bsGot, err := bs.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("BS ListProfiles: %v", err)
	}
	orcaGot, err := orca.ListProfiles(profile.CategoryProcess)
	if err != nil {
		t.Fatalf("Orca ListProfiles: %v", err)
	}

	if !reflect.DeepEqual(bsGot, orcaGot) {
		t.Errorf("BS = %v, Orca = %v, want identical", bsGot, orcaGot)
	}
	if !reflect.DeepEqual(bsGot, names) {
		t.Errorf("BS ListProfiles = %v, want %v", bsGot, names)
	}
}
