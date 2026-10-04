package diff

import (
	"testing"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

const fixtureDir = "../../testdata/fixtures"

// readFixture reads a fixture profile pair from testdata/fixtures.
func readFixture(t *testing.T, baseName string, source profile.SlicerID) profile.Profile {
	t.Helper()
	rr, err := profile.Read(fixtureDir, baseName, profile.CategoryProcess, source)
	if err != nil {
		t.Fatalf("profile.Read(%q): %v", baseName, err)
	}
	return rr.Profile
}

func findField(t *testing.T, fields []DiffField, name string) DiffField {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("field %q not found in diff", name)
	return DiffField{}
}

func TestComputeDiff_FixturePair_TopSurfacePatternDiffers(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)
	f := findField(t, d.Fields, "top_surface_pattern")

	if f.Status != StatusDiffer {
		t.Errorf("top_surface_pattern status = %s, want %s", f.Status, StatusDiffer)
	}
	if f.SourceValue != "zig-zag" {
		t.Errorf("top_surface_pattern source = %v, want zig-zag", f.SourceValue)
	}
	if f.DestValue != "rectilinear" {
		t.Errorf("top_surface_pattern dest = %v, want rectilinear", f.DestValue)
	}
	if !f.Selectable {
		t.Errorf("top_surface_pattern should be selectable")
	}
	if f.Kind != profile.FieldKindSetting {
		t.Errorf("top_surface_pattern kind = %s, want %s", f.Kind, profile.FieldKindSetting)
	}
}

func TestComputeDiff_FixturePair_SpeedFieldsMatch(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)

	speedFields := []string{
		"inner_wall_speed",
		"outer_wall_speed",
		"internal_solid_infill_speed",
		"sparse_infill_speed",
		"top_surface_speed",
		"initial_layer_speed",
		"initial_layer_infill_speed",
		"inner_wall_acceleration",
		"outer_wall_acceleration",
	}
	for _, name := range speedFields {
		f := findField(t, d.Fields, name)
		if f.Status != StatusMatch {
			t.Errorf("%s status = %s, want %s (BS canonical=%v, Orca canonical=%v)",
				name, f.Status, StatusMatch, f.SourceValue, f.DestValue)
		}
		if !f.Selectable {
			t.Errorf("%s should be selectable", name)
		}
	}
}

func TestComputeDiff_FixturePair_NoOnlySourceOrDest(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)

	for _, f := range d.Fields {
		if f.Status == StatusOnlySource || f.Status == StatusOnlyDest {
			t.Errorf("field %q has status %s; fixture pair should have all fields in both",
				f.Name, f.Status)
		}
	}
}

func TestComputeDiff_FixturePair_IdentityFieldsNotSelectable(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)

	identityFields := []string{"name", "print_settings_id", "from"}
	for _, name := range identityFields {
		f := findField(t, d.Fields, name)
		if f.Selectable {
			t.Errorf("identity field %q should not be selectable", name)
		}
		if f.Kind != profile.FieldKindIdentity {
			t.Errorf("field %q kind = %s, want %s", name, f.Kind, profile.FieldKindIdentity)
		}
	}
}

func TestComputeDiff_FixturePair_LineageAndVersionNotSelectable(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)

	// inherits is a JSON field → appears as a DiffField with lineage kind.
	inherits := findField(t, d.Fields, "inherits")
	if inherits.Selectable {
		t.Errorf("inherits should not be selectable")
	}
	if inherits.Kind != profile.FieldKindLineage {
		t.Errorf("inherits kind = %s, want %s", inherits.Kind, profile.FieldKindLineage)
	}

	// version is a JSON field → appears as a DiffField with version kind.
	version := findField(t, d.Fields, "version")
	if version.Selectable {
		t.Errorf("version should not be selectable")
	}
	if version.Kind != profile.FieldKindVersion {
		t.Errorf("version kind = %s, want %s", version.Kind, profile.FieldKindVersion)
	}
}

func TestComputeDiff_FixturePair_SettingFieldsSelectable(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)

	settings := []string{
		"inner_wall_speed",
		"top_surface_pattern",
		"sparse_infill_pattern",
		"enable_support",
		"brim_type",
	}
	for _, name := range settings {
		f := findField(t, d.Fields, name)
		if !f.Selectable {
			t.Errorf("setting field %q should be selectable", name)
		}
		if f.Kind != profile.FieldKindSetting {
			t.Errorf("field %q kind = %s, want %s", name, f.Kind, profile.FieldKindSetting)
		}
	}
}

func TestComputeDiff_FixturePair_DiffContext(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)
	ctx := d.Context

	if ctx.SourceName != "0.2 base" {
		t.Errorf("SourceName = %q, want 0.2 base", ctx.SourceName)
	}
	if ctx.DestName != "0.2 base" {
		t.Errorf("DestName = %q, want 0.2 base", ctx.DestName)
	}
	if ctx.SourceSettingID != "PPUS249810809cffcb" {
		t.Errorf("SourceSettingID = %q, want PPUS249810809cffcb", ctx.SourceSettingID)
	}
	if ctx.DestSettingID != "785471be-2f8f-5ee6-a9ab-805bdbd30ab5" {
		t.Errorf("DestSettingID = %q, want 785471be-...", ctx.DestSettingID)
	}
	if ctx.SourceBaseID != "GP079" || ctx.DestBaseID != "GP079" {
		t.Errorf("BaseID = %q/%q, want GP079/GP079", ctx.SourceBaseID, ctx.DestBaseID)
	}
	if ctx.SourceVersion != "1.10.0.32" || ctx.DestVersion != "1.10.0.32" {
		t.Errorf("Version = %q/%q, want 1.10.0.32/1.10.0.32", ctx.SourceVersion, ctx.DestVersion)
	}
	if ctx.SourceInherits != "0.20mm Standard @BBL A1" || ctx.DestInherits != "0.20mm Standard @BBL A1" {
		t.Errorf("Inherits = %q/%q, want 0.20mm Standard @BBL A1", ctx.SourceInherits, ctx.DestInherits)
	}
}

func TestComputeDiff_Sorting_DifferFirst(t *testing.T) {
	bs := readFixture(t, "bs_0.2base", profile.SlicerBambuStudio)
	orca := readFixture(t, "orca_0.2base", profile.SlicerOrcaSlicer)

	d := ComputeDiff(bs, orca)

	// Find index of top_surface_pattern (differ) and inner_wall_speed (match).
	var differIdx, matchIdx int = -1, -1
	for i, f := range d.Fields {
		if f.Name == "top_surface_pattern" {
			differIdx = i
		}
		if f.Name == "inner_wall_speed" {
			matchIdx = i
		}
	}
	if differIdx < 0 || matchIdx < 0 {
		t.Fatalf("could not find expected fields in diff (differ=%d, match=%d)", differIdx, matchIdx)
	}
	if differIdx >= matchIdx {
		t.Errorf("differ field at index %d should come before match field at index %d",
			differIdx, matchIdx)
	}

	// Verify overall ordering: all differs before all matches.
	seenMatch := false
	for _, f := range d.Fields {
		if f.Status == StatusMatch {
			seenMatch = true
		}
		if seenMatch && f.Status == StatusDiffer {
			t.Errorf("differ field %q appears after a match; sorting broken", f.Name)
		}
	}
}

func TestFieldKindOf_AllCategories(t *testing.T) {
	cases := []struct {
		name string
		want profile.FieldKind
	}{
		{"inner_wall_speed", profile.FieldKindSetting},
		{"top_surface_pattern", profile.FieldKindSetting},
		{"sparse_infill_pattern", profile.FieldKindSetting},
		{"enable_support", profile.FieldKindSetting},
		{"name", profile.FieldKindIdentity},
		{"print_settings_id", profile.FieldKindIdentity},
		{"from", profile.FieldKindIdentity},
		{"machine_id", profile.FieldKindIdentity},
		{"filament_id", profile.FieldKindIdentity},
		{"inherits", profile.FieldKindLineage},
		{"base_id", profile.FieldKindLineage},
		{"version", profile.FieldKindVersion},
	}
	for _, c := range cases {
		got := FieldKindOf(c.name)
		if got != c.want {
			t.Errorf("FieldKindOf(%q) = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestComputeDiff_OnlySource(t *testing.T) {
	source := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "synthetic",
		Source:   profile.SlicerBambuStudio,
		JSON: map[string]profile.Value{
			"inner_wall_speed": {Raw: "400", Canonical: "400"},
			"only_in_source":   {Raw: "1", Canonical: "1"},
		},
	}
	dest := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "synthetic",
		Source:   profile.SlicerOrcaSlicer,
		JSON: map[string]profile.Value{
			"inner_wall_speed": {Raw: "400", Canonical: "400"},
		},
	}

	d := ComputeDiff(source, dest)
	f := findField(t, d.Fields, "only_in_source")

	if f.Status != StatusOnlySource {
		t.Errorf("only_in_source status = %s, want %s", f.Status, StatusOnlySource)
	}
	if f.SourceValue != "1" {
		t.Errorf("only_in_source source = %v, want 1", f.SourceValue)
	}
	if f.DestValue != nil {
		t.Errorf("only_in_source dest = %v, want nil", f.DestValue)
	}
	if !f.Selectable {
		t.Errorf("only_in_source should be selectable (setting kind)")
	}
}

func TestComputeDiff_OnlyDest(t *testing.T) {
	source := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "synthetic",
		Source:   profile.SlicerBambuStudio,
		JSON: map[string]profile.Value{
			"inner_wall_speed": {Raw: "400", Canonical: "400"},
		},
	}
	dest := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "synthetic",
		Source:   profile.SlicerOrcaSlicer,
		JSON: map[string]profile.Value{
			"inner_wall_speed": {Raw: "400", Canonical: "400"},
			"only_in_dest":     {Raw: "2", Canonical: "2"},
		},
	}

	d := ComputeDiff(source, dest)
	f := findField(t, d.Fields, "only_in_dest")

	if f.Status != StatusOnlyDest {
		t.Errorf("only_in_dest status = %s, want %s", f.Status, StatusOnlyDest)
	}
	if f.SourceValue != nil {
		t.Errorf("only_in_dest source = %v, want nil", f.SourceValue)
	}
	if f.DestValue != "2" {
		t.Errorf("only_in_dest dest = %v, want 2", f.DestValue)
	}
}

func TestComputeDiff_MultiElementArrayComparison(t *testing.T) {
	source := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "synthetic",
		Source:   profile.SlicerBambuStudio,
		JSON: map[string]profile.Value{
			"print_extruder_id": {Raw: []any{"1"}, Canonical: "1"},
		},
	}
	dest := profile.Profile{
		Category: profile.CategoryProcess,
		Name:     "synthetic",
		Source:   profile.SlicerOrcaSlicer,
		JSON: map[string]profile.Value{
			"print_extruder_id": {Raw: []any{"1"}, Canonical: "1"},
		},
	}

	d := ComputeDiff(source, dest)
	f := findField(t, d.Fields, "print_extruder_id")

	if f.Status != StatusMatch {
		t.Errorf("print_extruder_id status = %s, want %s", f.Status, StatusMatch)
	}
}
