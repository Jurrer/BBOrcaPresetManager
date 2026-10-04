// Package slicer defines the SlicerAdapter interface and provides
// concrete implementations for BambuStudio and OrcaSlicer.
package slicer

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// SlicerAdapter abstracts a single slicer installation: where its user
// profile directories live, how to list/read/write profile file pairs,
// and what field-type registry it uses for write-time normalization.
//
// Plan §3 "Slicer adapters" + plan §12 internal/slicer/.
type SlicerAdapter interface {
	// ID returns the stable slicer identifier (e.g. "bambustudio").
	ID() profile.SlicerID

	// Version returns the detected slicer version (used for version
	// stamping per plan §5.2). Falls back to config override when
	// auto-detection fails.
	Version() string

	// UserDir returns the resolved user dir path for the configured
	// user_id (e.g. ~/Library/.../BambuStudio/user/<uid>/).
	UserDir() string

	// CategoryDir returns the per-category subdir (machine/process/filament).
	CategoryDir(c profile.Category) string

	// ListProfiles returns the names of all user profiles in a category.
	ListProfiles(c profile.Category) ([]string, error)

	// ReadProfile reads a profile by name + category and returns it
	// in normalized internal form.
	ReadProfile(c profile.Category, name string) (profile.ReadResult, error)

	// WriteProfile writes a profile (in internal form) back to disk,
	// applying denormalization per the field-type registry.
	WriteProfile(p profile.Profile) (profile.WriteResult, error)

	// FieldRegistry returns the per-slicer field-type registry used to
	// decide array-vs-scalar encoding on write.
	FieldRegistry() profile.FieldTypeRegistry
}

// baseAdapter holds the shared state and behavior common to all slicer
// adapters. The concrete BambuStudio and OrcaSlicer adapters embed it and
// differ only in their SlicerID (which drives the field-type registry).
type baseAdapter struct {
	id       profile.SlicerID
	userDir  string
	uid      string
	version  string
	registry profile.FieldTypeRegistry
}

// newBaseAdapter constructs a baseAdapter with the field-type registry
// for the given slicer ID.
func newBaseAdapter(id profile.SlicerID, userDir, uid, version string) baseAdapter {
	return baseAdapter{
		id:       id,
		userDir:  userDir,
		uid:      uid,
		version:  version,
		registry: profile.NewFieldTypeRegistry(id, version),
	}
}

func (a *baseAdapter) ID() profile.SlicerID { return a.id }

func (a *baseAdapter) Version() string { return a.version }

func (a *baseAdapter) UserDir() string { return a.userDir }

func (a *baseAdapter) CategoryDir(c profile.Category) string {
	return a.userDir + "/" + string(c)
}

// ListProfiles returns the names of all profiles in the given category
// directory, sorted alphabetically. A profile is identified by the
// presence of a `<name>.json` file (the `.info` sidecar is required for
// reads but listing only inspects `.json` files).
//
// A missing category directory is not an error — it returns an empty slice
// because the category may simply have no profiles yet.
func (a *baseAdapter) ListProfiles(c profile.Category) ([]string, error) {
	entries, err := os.ReadDir(a.CategoryDir(c))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		names = append(names, name[:len(name)-len(".json")])
	}
	sort.Strings(names)
	return names, nil
}

func (a *baseAdapter) ReadProfile(c profile.Category, name string) (profile.ReadResult, error) {
	return profile.Read(a.CategoryDir(c), name, c, a.id)
}

func (a *baseAdapter) WriteProfile(p profile.Profile) (profile.WriteResult, error) {
	return profile.Write(a.CategoryDir(p.Category), p)
}

func (a *baseAdapter) FieldRegistry() profile.FieldTypeRegistry { return a.registry }
