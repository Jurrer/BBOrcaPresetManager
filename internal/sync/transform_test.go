package sync

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

func TestStampVersionSetsVersion(t *testing.T) {
	dest := profile.Profile{
		JSON: map[string]profile.Value{
			"version": {Raw: "old", Canonical: "old"},
			"other":   {Raw: "x", Canonical: "x"},
		},
	}

	got := StampVersion(dest, "1.10.0.32")

	v, ok := got.JSON["version"]
	if !ok {
		t.Fatal("version field missing after stamp")
	}
	if v.Raw != "1.10.0.32" || v.Canonical != "1.10.0.32" {
		t.Errorf("version = {Raw: %v, Canonical: %v}, want both 1.10.0.32", v.Raw, v.Canonical)
	}

	// Other fields are preserved.
	if got.JSON["other"].Canonical != "x" {
		t.Errorf("other field changed: got %v, want x", got.JSON["other"].Canonical)
	}

	// Original dest must not be mutated.
	if dest.JSON["version"].Canonical != "old" {
		t.Errorf("original dest version mutated: got %v, want old", dest.JSON["version"].Canonical)
	}

	// The returned JSON map must be a separate allocation (copy).
	if len(dest.JSON) == 0 {
		t.Error("original JSON map was cleared or replaced")
	}
}

func TestStampVersionEmptyIsNoop(t *testing.T) {
	dest := profile.Profile{
		JSON: map[string]profile.Value{
			"version": {Raw: "1.0", Canonical: "1.0"},
		},
	}

	got := StampVersion(dest, "")
	if got.JSON["version"].Canonical != "1.0" {
		t.Errorf("empty version should be noop, got %v", got.JSON["version"].Canonical)
	}
}

func TestGenerateSettingIDBambuStudio(t *testing.T) {
	id, ok := GenerateSettingID(profile.SlicerBambuStudio)
	if !ok {
		t.Fatal("expected ok=true for BambuStudio")
	}
	if !strings.HasPrefix(id, "PPUS") {
		t.Fatalf("expected PPUS prefix, got %q", id)
	}
	hexPart := id[len("PPUS"):]
	hexRe := regexp.MustCompile(`^[0-9a-f]+$`)
	if !hexRe.MatchString(hexPart) {
		t.Errorf("hex part %q is not valid lowercase hex", hexPart)
	}
	if len(hexPart) < 13 {
		t.Errorf("hex part too short: %d chars, want >= 13", len(hexPart))
	}
}

func TestGenerateSettingIDOrcaSlicer(t *testing.T) {
	id, ok := GenerateSettingID(profile.SlicerOrcaSlicer)
	if !ok {
		t.Fatal("expected ok=true for OrcaSlicer")
	}
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRe.MatchString(id) {
		t.Errorf("not a valid UUID v4: %q", id)
	}
}

func TestGenerateSettingIDUnknownSlicer(t *testing.T) {
	id, ok := GenerateSettingID("unknown")
	if ok {
		t.Errorf("expected ok=false for unknown slicer, got id=%q", id)
	}
	if id != "" {
		t.Errorf("expected empty id for unknown slicer, got %q", id)
	}
}

func TestGenerateSettingIDUniqueness(t *testing.T) {
	id1, ok1 := GenerateSettingID(profile.SlicerBambuStudio)
	id2, ok2 := GenerateSettingID(profile.SlicerBambuStudio)
	if !ok1 || !ok2 {
		t.Fatal("expected ok=true for both calls")
	}
	if id1 == id2 {
		t.Errorf("two BambuStudio IDs are identical: %q", id1)
	}

	uid1, uok1 := GenerateSettingID(profile.SlicerOrcaSlicer)
	uid2, uok2 := GenerateSettingID(profile.SlicerOrcaSlicer)
	if !uok1 || !uok2 {
		t.Fatal("expected ok=true for both calls")
	}
	if uid1 == uid2 {
		t.Errorf("two OrcaSlicer UUIDs are identical: %q", uid1)
	}
}

func TestRemapBaseIDMVP(t *testing.T) {
	id, ok := RemapBaseID(profile.SlicerBambuStudio, "0.20mm Standard @BBL A1")
	if ok {
		t.Errorf("MVP RemapBaseID should return ok=false, got id=%q", id)
	}
	if id != "" {
		t.Errorf("MVP RemapBaseID should return empty id, got %q", id)
	}
}
