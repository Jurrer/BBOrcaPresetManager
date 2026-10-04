package profile

import (
	"path/filepath"
	"testing"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	// internal/profile -> ../../testdata/fixtures
	return filepath.Join("..", "..", "testdata", "fixtures")
}

func TestReadBSFixture(t *testing.T) {
	dir := fixtureDir(t)
	rr, err := Read(dir, "bs_0.2base", CategoryProcess, SlicerBambuStudio)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// All 37 observed keys from the BS fixture.
	wantCount := 37
	if got := len(rr.Profile.JSON); got != wantCount {
		t.Errorf("field count = %d, want %d", got, wantCount)
	}

	// Profile name should come from JSON "name" field, not baseName.
	if got := rr.Profile.Name; got != "0.2 base" {
		t.Errorf("Name = %q, want %q", got, "0.2 base")
	}

	// RawJSON should contain the original file bytes.
	if len(rr.RawJSON) == 0 {
		t.Error("RawJSON is empty")
	}

	// BS wraps speeds in arrays — Canonical should be unwrapped scalars.
	tests := []struct {
		field string
		want  any
	}{
		{"inner_wall_speed", "400"},
		{"outer_wall_speed", "400"},
		{"initial_layer_speed", "100"},
		{"sparse_infill_speed", "400"},
		{"top_surface_speed", "300"},
		{"inner_wall_acceleration", "6000"},
		{"outer_wall_acceleration", "2000"},
		{"small_perimeter_threshold", "15"},
		{"overhang_2_4_speed", "100"},
		{"overhang_3_4_speed", "70"},
		{"overhang_4_4_speed", "50"},
		{"initial_layer_infill_speed", "100"},
		{"internal_solid_infill_speed", "400"},
		{"print_extruder_id", "1"},
		{"print_extruder_variant", "Direct Drive Standard"},
		// Scalar fields stay scalar.
		{"brim_type", "no_brim"},
		{"version", "1.10.0.32"},
		{"top_surface_pattern", "zig-zag"},
		{"name", "0.2 base"},
	}
	for _, tc := range tests {
		v, ok := rr.Profile.JSON[tc.field]
		if !ok {
			t.Errorf("missing field %q", tc.field)
			continue
		}
		if v.Canonical != tc.want {
			t.Errorf("Canonical[%q] = %v (%T), want %v (%T)", tc.field, v.Canonical, v.Canonical, tc.want, tc.want)
		}
	}

	// Verify Raw form preserves arrays for wrapped BS fields.
	if raw, ok := rr.Profile.JSON["inner_wall_speed"].Raw.([]any); ok {
		if len(raw) != 1 || raw[0] != "400" {
			t.Errorf("Raw[inner_wall_speed] = %v, want [\"400\"]", raw)
		}
	} else {
		t.Errorf("Raw[inner_wall_speed] is not []any, got %T", rr.Profile.JSON["inner_wall_speed"].Raw)
	}
}

func TestReadOrcaFixture(t *testing.T) {
	dir := fixtureDir(t)
	rr, err := Read(dir, "orca_0.2base", CategoryProcess, SlicerOrcaSlicer)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	wantCount := 37
	if got := len(rr.Profile.JSON); got != wantCount {
		t.Errorf("field count = %d, want %d", got, wantCount)
	}

	// Orca uses scalars for all speed fields.
	speedFields := []struct {
		field string
		want  any
	}{
		{"inner_wall_speed", "400"},
		{"outer_wall_speed", "400"},
		{"initial_layer_speed", "100"},
		{"sparse_infill_speed", "400"},
		{"top_surface_speed", "300"},
		{"inner_wall_acceleration", "6000"},
		{"small_perimeter_threshold", "15"},
	}
	for _, tc := range speedFields {
		v, ok := rr.Profile.JSON[tc.field]
		if !ok {
			t.Errorf("missing field %q", tc.field)
			continue
		}
		if v.Canonical != tc.want {
			t.Errorf("Canonical[%q] = %v, want %v", tc.field, v.Canonical, tc.want)
		}
		// Raw should also be a string (scalar), not an array.
		if _, isArray := v.Raw.([]any); isArray {
			t.Errorf("Raw[%q] is array, want scalar for Orca", tc.field)
		}
	}

	// Orca wraps print_extruder_id in an array — Canonical should be unwrapped.
	if v, ok := rr.Profile.JSON["print_extruder_id"]; ok {
		if v.Canonical != "1" {
			t.Errorf("Canonical[print_extruder_id] = %v, want \"1\"", v.Canonical)
		}
		if _, ok := v.Raw.([]any); !ok {
			t.Errorf("Raw[print_extruder_id] should be []any for Orca, got %T", v.Raw)
		}
	} else {
		t.Error("missing field print_extruder_id")
	}

	// Verify value divergence: Orca uses "rectilinear", BS uses "zig-zag".
	if v, ok := rr.Profile.JSON["top_surface_pattern"]; ok {
		if v.Canonical != "rectilinear" {
			t.Errorf("top_surface_pattern = %v, want \"rectilinear\"", v.Canonical)
		}
	}
}

func TestReadInfoBS(t *testing.T) {
	dir := fixtureDir(t)
	rr, err := Read(dir, "bs_0.2base", CategoryProcess, SlicerBambuStudio)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	info := rr.Profile.Info
	if info.UserID != "2240525152" {
		t.Errorf("UserID = %q, want %q", info.UserID, "2240525152")
	}
	if info.SettingID != "PPUS249810809cffcb" {
		t.Errorf("SettingID = %q, want %q", info.SettingID, "PPUS249810809cffcb")
	}
	if info.BaseID != "GP079" {
		t.Errorf("BaseID = %q, want %q", info.BaseID, "GP079")
	}
	if info.UpdatedTime != 1775010477 {
		t.Errorf("UpdatedTime = %d, want %d", info.UpdatedTime, 1775010477)
	}
	if info.SyncInfo != "" {
		t.Errorf("SyncInfo = %q, want empty", info.SyncInfo)
	}
}

func TestReadInfoOrca(t *testing.T) {
	dir := fixtureDir(t)
	rr, err := Read(dir, "orca_0.2base", CategoryProcess, SlicerOrcaSlicer)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	info := rr.Profile.Info
	if info.UserID != "11b99db1-3c45-4482-a69b-7f430a909e28" {
		t.Errorf("UserID = %q, want UUID", info.UserID)
	}
	if info.SettingID != "785471be-2f8f-5ee6-a9ab-805bdbd30ab5" {
		t.Errorf("SettingID = %q, want UUID", info.SettingID)
	}
	if info.BaseID != "GP079" {
		t.Errorf("BaseID = %q, want %q", info.BaseID, "GP079")
	}
	if info.UpdatedTime != 1779474689 {
		t.Errorf("UpdatedTime = %d, want %d", info.UpdatedTime, 1779474689)
	}
}

func TestNormalizeValue(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"single-element string array", []any{"400"}, "400"},
		{"single-element mixed array", []any{float64(42)}, float64(42)},
		{"multi-element array", []any{"400", "500"}, []any{"400", "500"}},
		{"empty array", []any{}, []any{}},
		{"scalar string", "400", "400"},
		{"scalar number", float64(42), float64(42)},
		{"scalar bool", true, true},
		{"nil", nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeValue(tc.in)
			// For slice comparison, use deep equality.
			if g, ok := got.([]any); ok {
				if w, ok2 := tc.want.([]any); ok2 {
					if len(g) != len(w) {
						t.Errorf("normalizeValue(%v) = %v, want %v", tc.in, got, tc.want)
						return
					}
					for i := range g {
						if g[i] != w[i] {
							t.Errorf("normalizeValue(%v) = %v, want %v", tc.in, got, tc.want)
							return
						}
					}
					return
				}
			}
			if got != tc.want {
				t.Errorf("normalizeValue(%v) = %v (%T), want %v (%T)", tc.in, got, got, tc.want, tc.want)
			}
		})
	}
}

func TestParseInfo(t *testing.T) {
	input := "sync_info = \nuser_id = 2240525152\nsetting_id = PPUS249810809cffcb\nbase_id = GP079\nupdated_time = 1775010477\n"
	info, err := parseInfo(input)
	if err != nil {
		t.Fatalf("parseInfo failed: %v", err)
	}
	if info.UserID != "2240525152" {
		t.Errorf("UserID = %q", info.UserID)
	}
	if info.SettingID != "PPUS249810809cffcb" {
		t.Errorf("SettingID = %q", info.SettingID)
	}
	if info.UpdatedTime != 1775010477 {
		t.Errorf("UpdatedTime = %d", info.UpdatedTime)
	}
}

func TestParseInfoEmptyLines(t *testing.T) {
	input := "\nsync_info = \n\nuser_id = abc\n\n"
	info, err := parseInfo(input)
	if err != nil {
		t.Fatalf("parseInfo failed: %v", err)
	}
	if info.UserID != "abc" {
		t.Errorf("UserID = %q, want \"abc\"", info.UserID)
	}
}
