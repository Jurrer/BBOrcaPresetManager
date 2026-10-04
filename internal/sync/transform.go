// Package sync applies selected-field overrides from a diff result to a
// destination profile, runs the transform rules from plan §5, and writes
// the result via the slicer adapter + vault.
package sync

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// StampVersion sets the destination profile's `version` field to the
// destination slicer's detected version (plan §5.2).
//
// If destSlicerVersion is empty, the version field is left unchanged.
// The returned profile is a copy with its own JSON map so the caller's
// original is not mutated.
func StampVersion(dest profile.Profile, destSlicerVersion string) profile.Profile {
	if destSlicerVersion == "" {
		return dest
	}
	out := dest // shallow copy of struct; JSON map is replaced below
	out.JSON = make(map[string]profile.Value, len(dest.JSON))
	for k, v := range dest.JSON {
		out.JSON[k] = v
	}
	out.JSON["version"] = profile.Value{
		Raw:       destSlicerVersion,
		Canonical: destSlicerVersion,
	}
	return out
}

// GenerateSettingID produces a new setting_id for a freshly created
// destination profile (plan §5.3). Format is slicer-specific:
//   - BambuStudio -> "PPUS" + 14 hex chars (7 random bytes).
//   - OrcaSlicer (native) -> RFC 4122 version 4 UUID.
//
// Returns ("", false) for unknown slicer — caller is expected to surface
// that as a configuration error.
func GenerateSettingID(slicer profile.SlicerID) (string, bool) {
	switch slicer {
	case profile.SlicerBambuStudio:
		var b [7]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", false
		}
		return "PPUS" + hex.EncodeToString(b[:]), true
	case profile.SlicerOrcaSlicer:
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", false
		}
		b[6] = (b[6] & 0x0f) | 0x40 // version 4
		b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
		return fmt.Sprintf("%s-%s-%s-%s-%s",
			hex.EncodeToString(b[0:4]),
			hex.EncodeToString(b[4:6]),
			hex.EncodeToString(b[6:8]),
			hex.EncodeToString(b[8:10]),
			hex.EncodeToString(b[10:16]),
		), true
	default:
		return "", false
	}
}

// RemapBaseID returns the dest slicer's equivalent base_id (plan §5.4)
// for a source profile whose JSON `inherits` names a system profile.
//
//   - ok=true, id=<found>     — equivalent exists in dest's system profiles.
//   - ok=false, id=""         — no equivalent found; engine preserves the
//     existing base_id and the UI should flag the user (plan §5.4).
//
// MVP: no cross-reference table is maintained yet, so this always returns
// ("", false). The engine handles ok=false by preserving the existing
// base_id unchanged.
func RemapBaseID(slicer profile.SlicerID, inheritsName string) (id string, ok bool) {
	return "", false
}
