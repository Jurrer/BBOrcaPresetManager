package sync

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
	"github.com/Jurrer/BBOrcaPresetManager/internal/slicer"
	"github.com/Jurrer/BBOrcaPresetManager/internal/vault"
)

// FieldOverrides is a map of field-name -> chosen value to apply to the
// dest profile. Values are the canonical (already normalized) form.
//
// Wire shape (plan §11 POST /api/sync body fields): map[string]any.
type FieldOverrides map[string]any

// SyncResult is the return value of Engine.Sync — mirrors the
// /api/sync response shape from plan §11.
type SyncResult struct {
	Commit       string   // vault commit hash
	WrittenFiles []string // absolute paths of files written
}

// Engine applies a diff's selected fields and writes the result via the
// slicer adapter and vault. It is constructed once at startup with
// references to both slicer adapters and the vault, and reused per request.
type Engine struct {
	bs    slicer.SlicerAdapter
	orca  slicer.SlicerAdapter
	vault *vault.Vault
}

// NewEngine wires the dependencies the sync engine needs at request time.
// The caller passes both slicer adapters; the dest adapter is selected per
// Sync call based on the destSlicer argument.
func NewEngine(bs, orca slicer.SlicerAdapter, v *vault.Vault) *Engine {
	return &Engine{bs: bs, orca: orca, vault: v}
}

// adapterFor returns the adapter for the given slicer ID.
func (e *Engine) adapterFor(slicerID profile.SlicerID) (slicer.SlicerAdapter, error) {
	switch slicerID {
	case profile.SlicerBambuStudio:
		return e.bs, nil
	case profile.SlicerOrcaSlicer:
		return e.orca, nil
	default:
		return nil, fmt.Errorf("sync: unknown dest slicer %q", slicerID)
	}
}

// Sync applies the overrides to a copy of `dest`, runs transforms
// (version stamp, setting_id gen if `isNew`, base_id remap, format
// normalize), and writes through the adapter + vault.
//
// Plan §7 "App:" flow:
//  1. Build resulting dest profile (apply field overrides).
//  2. Run transforms (version stamp, setting_id gen if isNew, base_id remap).
//  3. Stage in vault.
//  4. Commit vault.
//  5. Write slicer dir.
//  6. Return SyncResult (commit hash + written file paths).
//
// If vault commit fails, the slicer dir is not written. If the slicer
// write fails after a successful vault commit, the vault remains in a
// consistent committed state and the error is returned.
func (e *Engine) Sync(dest profile.Profile, destSlicer profile.SlicerID, overrides FieldOverrides, isNew bool, destSlicerVersion string) (SyncResult, error) {
	adapter, err := e.adapterFor(destSlicer)
	if err != nil {
		return SyncResult{}, err
	}

	// 1. Build resulting dest profile: copy dest and apply field overrides.
	result := dest
	result.Source = destSlicer
	result.JSON = make(map[string]profile.Value, len(dest.JSON))
	for k, v := range dest.JSON {
		result.JSON[k] = v
	}
	for field, val := range overrides {
		result.JSON[field] = profile.Value{Raw: val, Canonical: val}
	}

	// 2. Run transforms.
	result = StampVersion(result, destSlicerVersion)

	if isNew {
		id, ok := GenerateSettingID(destSlicer)
		if !ok {
			return SyncResult{}, fmt.Errorf("sync: cannot generate setting_id for slicer %q", destSlicer)
		}
		result.Info.SettingID = id
	}

	// Base_id remap: MVP preserves existing base_id when no equivalent
	// is found (plan §5.4).
	inherits := profile.JSONString(result.JSON, "inherits")
	if id, ok := RemapBaseID(destSlicer, inherits); ok {
		result.Info.BaseID = id
	}

	// Stamp UpdatedTime per plan §5.5: every sync updates the timestamp.
	result.Info.UpdatedTime = time.Now().Unix()

	// 3. Stage in vault.
	if err := e.vault.Stage(result); err != nil {
		return SyncResult{}, fmt.Errorf("sync: vault stage: %w", err)
	}

	// 4. Commit vault.
	fields := sortedKeys(overrides)
	msg := commitMessage(dest.Category, dest.Name, destSlicer, len(fields), fields)
	hash, err := e.vault.Commit(msg)
	if err != nil {
		return SyncResult{}, fmt.Errorf("sync: vault commit: %w", err)
	}

	// 5. Write slicer dir.
	wr, err := adapter.WriteProfile(result)
	if err != nil {
		return SyncResult{Commit: hash}, fmt.Errorf("sync: write slicer: %w", err)
	}

	// 6. Return SyncResult.
	return SyncResult{
		Commit:       hash,
		WrittenFiles: []string{wr.JSONPath, wr.InfoPath},
	}, nil
}

// sortedKeys returns the keys of m sorted alphabetically for deterministic
// commit messages.
func sortedKeys(m FieldOverrides) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// commitMessage builds the two-line vault commit message:
//
//	sync: <category> "<profile name>" → <dest slicer>
//	applied <N> fields: <field1>, <field2>, ...
func commitMessage(cat profile.Category, name string, destSlicer profile.SlicerID, n int, fields []string) string {
	line1 := fmt.Sprintf("sync: %s %q → %s", cat, name, destSlicer)
	line2 := fmt.Sprintf("applied %d fields: %s", n, strings.Join(fields, ", "))
	return line1 + "\n" + line2
}
