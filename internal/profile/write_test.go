package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRoundTripBS(t *testing.T) {
	dir := fixtureDir(t)
	rr, err := Read(dir, "bs_0.2base", CategoryProcess, SlicerBambuStudio)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	tmp := t.TempDir()
	wr, err := Write(tmp, rr.Profile)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if _, err := os.Stat(wr.JSONPath); err != nil {
		t.Errorf("JSON file not written: %v", err)
	}
	if _, err := os.Stat(wr.InfoPath); err != nil {
		t.Errorf("Info file not written: %v", err)
	}

	// Re-read the written file and verify key fields match.
	rr2, err := Read(tmp, rr.Profile.Name, CategoryProcess, SlicerBambuStudio)
	if err != nil {
		t.Fatalf("Re-read failed: %v", err)
	}

	// Round-trip: BS array fields should still be arrays after write+read.
	checkFields := []struct {
		field string
		want  any
	}{
		{"inner_wall_speed", "400"},
		{"outer_wall_speed", "400"},
		{"initial_layer_speed", "100"},
		{"inner_wall_acceleration", "6000"},
		{"top_surface_speed", "300"},
		{"print_extruder_id", "1"},
		{"print_extruder_variant", "Direct Drive Standard"},
		{"small_perimeter_threshold", "15"},
		{"brim_type", "no_brim"},
		{"version", "1.10.0.32"},
		{"name", "0.2 base"},
	}
	for _, tc := range checkFields {
		v, ok := rr2.Profile.JSON[tc.field]
		if !ok {
			t.Errorf("missing field %q after round-trip", tc.field)
			continue
		}
		if v.Canonical != tc.want {
			t.Errorf("Canonical[%q] = %v, want %v", tc.field, v.Canonical, tc.want)
		}
	}

	// Verify the Raw form of speed fields is still arrays in BS.
	if _, ok := rr2.Profile.JSON["inner_wall_speed"].Raw.([]any); !ok {
		t.Errorf("Raw[inner_wall_speed] should be []any after BS round-trip, got %T", rr2.Profile.JSON["inner_wall_speed"].Raw)
	}
}

func TestWriteRoundTripOrca(t *testing.T) {
	dir := fixtureDir(t)
	rr, err := Read(dir, "orca_0.2base", CategoryProcess, SlicerOrcaSlicer)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	tmp := t.TempDir()
	if _, err := Write(tmp, rr.Profile); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	rr2, err := Read(tmp, rr.Profile.Name, CategoryProcess, SlicerOrcaSlicer)
	if err != nil {
		t.Fatalf("Re-read failed: %v", err)
	}

	// Orca: speed fields should be scalars (not arrays) after round-trip.
	speedFields := []string{
		"inner_wall_speed", "outer_wall_speed", "initial_layer_speed",
		"sparse_infill_speed", "top_surface_speed", "inner_wall_acceleration",
	}
	for _, f := range speedFields {
		v, ok := rr2.Profile.JSON[f]
		if !ok {
			t.Errorf("missing field %q", f)
			continue
		}
		if _, isArray := v.Raw.([]any); isArray {
			t.Errorf("Raw[%q] should be scalar for Orca, got array", f)
		}
	}

	// print_extruder_id should still be array for Orca.
	if v, ok := rr2.Profile.JSON["print_extruder_id"]; ok {
		if _, isArray := v.Raw.([]any); !isArray {
			t.Errorf("Raw[print_extruder_id] should be []any for Orca, got %T", v.Raw)
		}
		if v.Canonical != "1" {
			t.Errorf("Canonical[print_extruder_id] = %v, want \"1\"", v.Canonical)
		}
	}
}

func TestWriteCrossSlicer(t *testing.T) {
	// Read BS profile, write it as Orca — speed fields should become
	// scalars, print_extruder_id stays array.
	dir := fixtureDir(t)
	rr, err := Read(dir, "bs_0.2base", CategoryProcess, SlicerBambuStudio)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Switch source to Orca for writing.
	p := rr.Profile
	p.Source = SlicerOrcaSlicer

	tmp := t.TempDir()
	if _, err := Write(tmp, p); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	rr2, err := Read(tmp, p.Name, CategoryProcess, SlicerOrcaSlicer)
	if err != nil {
		t.Fatalf("Re-read failed: %v", err)
	}

	// Speed fields should now be scalars (not arrays).
	speedFields := []string{
		"inner_wall_speed", "outer_wall_speed", "initial_layer_speed",
	}
	for _, f := range speedFields {
		v, ok := rr2.Profile.JSON[f]
		if !ok {
			t.Errorf("missing field %q", f)
			continue
		}
		if _, isArray := v.Raw.([]any); isArray {
			t.Errorf("Raw[%q] should be scalar when written as Orca, got array", f)
		}
	}

	// print_extruder_id should still be array even in Orca.
	if v, ok := rr2.Profile.JSON["print_extruder_id"]; ok {
		if _, isArray := v.Raw.([]any); !isArray {
			t.Errorf("Raw[print_extruder_id] should be []any in Orca, got %T", v.Raw)
		}
	}
}

func TestWriteInfoSidecar(t *testing.T) {
	p := Profile{
		Name:   "test_profile",
		Source: SlicerBambuStudio,
		JSON:   map[string]Value{"name": {Raw: "test_profile", Canonical: "test_profile"}},
		Info: Info{
			SyncInfo:    "",
			UserID:      "2240525152",
			SettingID:   "PPUS123",
			BaseID:      "GP079",
			UpdatedTime: 1775010477,
		},
	}

	tmp := t.TempDir()
	wr, err := Write(tmp, p)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(wr.InfoPath)
	if err != nil {
		t.Fatalf("Read info failed: %v", err)
	}

	want := "sync_info = \nuser_id = 2240525152\nsetting_id = PPUS123\nbase_id = GP079\nupdated_time = 1775010477\n"
	if string(data) != want {
		t.Errorf("info content = %q, want %q", string(data), want)
	}
}

func TestWriteAtomicNoTempLeft(t *testing.T) {
	p := Profile{
		Name:   "atomic_test",
		Source: SlicerBambuStudio,
		JSON:   map[string]Value{"name": {Raw: "atomic_test", Canonical: "atomic_test"}},
		Info:   Info{UserID: "u", SettingID: "s", BaseID: "b", UpdatedTime: 1},
	}

	tmp := t.TempDir()
	_, err := Write(tmp, p)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// No .tmp files should remain after successful write.
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteUnknownFieldKeepsRaw(t *testing.T) {
	// A field not in the registry should be written using its Raw form.
	p := Profile{
		Name:   "unknown_test",
		Source: SlicerBambuStudio,
		JSON: map[string]Value{
			"name":              {Raw: "unknown_test", Canonical: "unknown_test"},
			"custom_slicer_opt": {Raw: []any{"custom_val"}, Canonical: "custom_val"},
		},
		Info: Info{UpdatedTime: 1},
	}

	tmp := t.TempDir()
	wr, err := Write(tmp, p)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read it back — but we can't use Read() because it expects .info
	// sidecar with valid content. Just check the JSON directly.
	rr2, err := Read(tmp, p.Name, CategoryProcess, SlicerBambuStudio)
	if err != nil {
		// If .info is malformed, write a proper one and retry.
		_ = os.WriteFile(filepath.Join(tmp, p.Name+".info"), []byte("sync_info = \nuser_id = x\nsetting_id = y\nbase_id = z\nupdated_time = 1\n"), 0o644)
		rr2, err = Read(tmp, p.Name, CategoryProcess, SlicerBambuStudio)
		if err != nil {
			t.Fatalf("Re-read failed: %v", err)
		}
	}

	// The unknown field's Raw should be the original array form.
	v, ok := rr2.Profile.JSON["custom_slicer_opt"]
	if !ok {
		t.Fatal("missing custom_slicer_opt after round-trip")
	}
	if _, isArray := v.Raw.([]any); !isArray {
		t.Errorf("unknown field Raw = %T, want []any (preserved as-is)", v.Raw)
	}
	// Canonical should be unwrapped since it's a single-element array.
	if v.Canonical != "custom_val" {
		t.Errorf("unknown field Canonical = %v, want \"custom_val\"", v.Canonical)
	}
	// Ensure JSON file path is correct.
	if wr.JSONPath == "" {
		t.Error("JSONPath should not be empty")
	}
}
