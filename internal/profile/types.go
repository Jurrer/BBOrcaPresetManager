// Package profile defines the internal representation of a print profile
// and its sidecar metadata, as described in plan.md §4.
package profile

// Category identifies which of the three profile kinds a profile belongs to.
// See GLOSSARY.md — "Profile" section.
type Category string

const (
	CategoryMachine  Category = "machine"
	CategoryProcess  Category = "process"
	CategoryFilament Category = "filament"
)

// SlicerID identifies which slicer a profile originated from.
type SlicerID string

const (
	SlicerBambuStudio SlicerID = "bambustudio"
	SlicerOrcaSlicer  SlicerID = "orcaslicer"
)

// Value represents a single setting field's raw and canonical forms.
// Raw holds the value exactly as parsed from JSON (string, []string, etc.).
// Canonical is the format-normalized form used for comparison in the diff
// engine (e.g. a single-element array unwrapped to a scalar string).
type Value struct {
	Raw       any
	Canonical any
}

// Profile is the in-memory representation of a user profile loaded from a
// slicer user dir (parsed from `name.json`) and its sidecar (parsed from
// `name.info`).
type Profile struct {
	Category Category         // machine | process | filament
	Name     string           // from JSON "name"
	JSON     map[string]Value // normalized field -> value
	Info     Info             // parsed .info sidecar
	Source   SlicerID         // which slicer it came from
}

// Info holds the parsed contents of a profile's `.info` sidecar.
// See GLOSSARY.md — "setting_id", "Lineage fields".
type Info struct {
	SyncInfo    string
	UserID      string
	SettingID   string
	BaseID      string
	UpdatedTime int64
}

// FieldKind categorizes a JSON field for diff/transform purposes.
// See plan.md §4 "Field categorization".
type FieldKind string

const (
	FieldKindSetting  FieldKind = "setting"  // tunable, selectable for sync
	FieldKindIdentity FieldKind = "identity" // name, print_settings_id, from
	FieldKindLineage  FieldKind = "lineage"  // inherits (JSON), base_id (.info)
	FieldKindVersion  FieldKind = "version"  // version (JSON), stamped on write
)
