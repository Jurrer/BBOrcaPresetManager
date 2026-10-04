// BambuStudioAdapter implements SlicerAdapter for BambuStudio.
//
// Default macOS path: ~/Library/Application Support/BambuStudio/user/<uid>/
// setting_id format: "PPUS" + hex (plan §5.3).
package slicer

import (
	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// BambuStudioAdapter embeds baseAdapter, specializing only the slicer ID
// (which drives the field-type registry).
type BambuStudioAdapter struct {
	baseAdapter
}

// NewBambuStudioAdapter constructs an adapter rooted at the given user dir
// and uid. version may be empty — auto-detection happens lazily.
func NewBambuStudioAdapter(userDir, uid, version string) *BambuStudioAdapter {
	return &BambuStudioAdapter{baseAdapter: newBaseAdapter(profile.SlicerBambuStudio, userDir, uid, version)}
}
