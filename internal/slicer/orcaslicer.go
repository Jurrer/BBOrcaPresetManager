// OrcaSlicerAdapter implements SlicerAdapter for OrcaSlicer.
//
// Default macOS path: ~/Library/Application Support/OrcaSlicer/user/<uid>/
// setting_id format: UUID for native accounts (plan §5.3).
package slicer

import (
	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// OrcaSlicerAdapter embeds baseAdapter, specializing only the slicer ID
// (which drives the field-type registry).
type OrcaSlicerAdapter struct {
	baseAdapter
}

// NewOrcaSlicerAdapter constructs an adapter rooted at the given user dir
// and uid. version may be empty — auto-detection happens lazily.
func NewOrcaSlicerAdapter(userDir, uid, version string) *OrcaSlicerAdapter {
	return &OrcaSlicerAdapter{baseAdapter: newBaseAdapter(profile.SlicerOrcaSlicer, userDir, uid, version)}
}
