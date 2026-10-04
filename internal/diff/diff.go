// Package diff computes field-level differences between two profiles
// of the same category, after format normalization. See plan.md §6.
package diff

import (
	"reflect"
	"sort"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// FieldStatus describes how a single field compares between source and dest.
type FieldStatus string

const (
	StatusMatch      FieldStatus = "match"       // canonical values equal
	StatusDiffer     FieldStatus = "differ"      // canonical values differ
	StatusOnlySource FieldStatus = "only-source" // present in source, absent in dest
	StatusOnlyDest   FieldStatus = "only-dest"   // present in dest, absent in source
)

// DiffField describes one field's comparison.
//
// Plan §6 columns: name, sourceValue, destValue, status, selectable.
// Selectable follows plan §4 field categorization (setting fields only;
// identity/lineage/version shown in context).
type DiffField struct {
	Name        string            `json:"name"`
	SourceValue any               `json:"source_value"`
	DestValue   any               `json:"dest_value"`
	Status      FieldStatus       `json:"status"`
	Selectable  bool              `json:"selectable"`
	Kind        profile.FieldKind `json:"kind"`
}

// Diff is the full result of comparing two profiles of the same category.
//
// Plan §6 "Context (not selectable)" block corresponds to Diff.Context.
type Diff struct {
	Fields  []DiffField `json:"fields"`
	Context DiffContext `json:"context"`
}

// DiffContext carries read-only fields surfaced around the diff table
// but never made selectable (plan §6 bottom block).
type DiffContext struct {
	SourceName      string `json:"source_name"`
	DestName        string `json:"dest_name"`
	SourceSettingID string `json:"source_setting_id"`
	DestSettingID   string `json:"dest_setting_id"`
	SourceBaseID    string `json:"source_base_id"`
	DestBaseID      string `json:"dest_base_id"`
	SourceVersion   string `json:"source_version"`
	DestVersion     string `json:"dest_version"`
	SourceInherits  string `json:"source_inherits"`
	DestInherits    string `json:"dest_inherits"`
}

// FieldKindOf classifies a JSON field name into its FieldKind category.
//
// Identity fields: name, print_settings_id / machine_id / filament_id, from.
// Lineage fields: inherits (JSON), base_id (.info).
// Version field: version (JSON).
// Everything else is a setting field. See plan.md §4.
func FieldKindOf(name string) profile.FieldKind {
	switch name {
	case "name", "print_settings_id", "machine_id", "filament_id", "from":
		return profile.FieldKindIdentity
	case "inherits", "base_id":
		return profile.FieldKindLineage
	case "version":
		return profile.FieldKindVersion
	default:
		return profile.FieldKindSetting
	}
}

// ComputeDiff returns the field-level diff between source and dest.
//
// Fields are compared by their canonical values (plan §5.1): single-element
// arrays are already unwrapped to scalars by the profile reader, so
// ["400"] (canonical "400") equals "400" (canonical "400"). Multi-element
// arrays are compared element-wise. Fields are sorted with differing fields
// first, then only-source, then only-dest, then matches (alphabetical within
// each group).
func ComputeDiff(source, dest profile.Profile) Diff {
	fields := computeFields(source, dest)
	sort.SliceStable(fields, func(i, j int) bool {
		oi, oj := statusOrder(fields[i].Status), statusOrder(fields[j].Status)
		if oi != oj {
			return oi < oj
		}
		return fields[i].Name < fields[j].Name
	})
	return Diff{
		Fields: fields,
		Context: DiffContext{
			SourceName:      source.Name,
			DestName:        dest.Name,
			SourceSettingID: source.Info.SettingID,
			DestSettingID:   dest.Info.SettingID,
			SourceBaseID:    source.Info.BaseID,
			DestBaseID:      dest.Info.BaseID,
			SourceVersion:   profile.JSONString(source.JSON, "version"),
			DestVersion:     profile.JSONString(dest.JSON, "version"),
			SourceInherits:  profile.JSONString(source.JSON, "inherits"),
			DestInherits:    profile.JSONString(dest.JSON, "inherits"),
		},
	}
}

// computeFields iterates the union of source and dest JSON fields and builds
// a DiffField for each.
func computeFields(source, dest profile.Profile) []DiffField {
	seen := make(map[string]bool)
	var fields []DiffField

	for name := range source.JSON {
		seen[name] = true
		fields = append(fields, makeField(name, source, dest))
	}
	for name := range dest.JSON {
		if seen[name] {
			continue
		}
		fields = append(fields, makeField(name, source, dest))
	}
	return fields
}

// makeField builds a single DiffField for the given field name.
func makeField(name string, source, dest profile.Profile) DiffField {
	src, srcOk := source.JSON[name]
	dst, dstOk := dest.JSON[name]
	kind := FieldKindOf(name)

	f := DiffField{
		Name:       name,
		Kind:       kind,
		Selectable: kind == profile.FieldKindSetting,
	}

	switch {
	case srcOk && dstOk:
		f.SourceValue = src.Canonical
		f.DestValue = dst.Canonical
		if canonicalEqual(src.Canonical, dst.Canonical) {
			f.Status = StatusMatch
		} else {
			f.Status = StatusDiffer
		}
	case srcOk:
		f.SourceValue = src.Canonical
		f.DestValue = nil
		f.Status = StatusOnlySource
	case dstOk:
		f.SourceValue = nil
		f.DestValue = dst.Canonical
		f.Status = StatusOnlyDest
	}

	return f
}

// canonicalEqual reports whether two canonical values are equal. After
// profile-reader normalization, single-element arrays are unwrapped to
// scalars, so reflect.DeepEqual suffices for both scalar and multi-element
// array comparison.
func canonicalEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// statusOrder maps a FieldStatus to a sort priority (lower = earlier).
func statusOrder(s FieldStatus) int {
	switch s {
	case StatusDiffer:
		return 0
	case StatusOnlySource:
		return 1
	case StatusOnlyDest:
		return 2
	case StatusMatch:
		return 3
	default:
		return 4
	}
}
