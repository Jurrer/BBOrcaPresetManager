package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteResult reports what files were actually written to disk so callers
// can return them in API responses (plan.md §11 /api/sync -> writtenFiles).
type WriteResult struct {
	JSONPath string
	InfoPath string
}

// Write serializes a profile to the given directory as `name.json` +
// `name.info`, atomically (write temp, rename).
//
// Denormalization uses the field-type registry for the profile's source
// slicer: known array fields are re-wrapped in arrays; known scalar fields
// are emitted as scalars; fields the registry doesn't know are emitted
// using their Raw form to avoid data loss (plan §5.1, §5.5).
func Write(dir string, p Profile) (WriteResult, error) {
	registry := NewFieldTypeRegistry(p.Source, "")

	// Build output map with denormalized values. json.MarshalIndent sorts
	// map keys alphabetically, matching the observed fixture order.
	out := make(map[string]any, len(p.JSON))
	for key, val := range p.JSON {
		ft, known := registry.Lookup(key)
		switch {
		case known && ft == FieldTypeArray:
			out[key] = denormalizeArray(val.Canonical)
		case known && ft == FieldTypeScalar:
			out[key] = val.Canonical
		default:
			out[key] = val.Raw
		}
	}

	var jsonBuf strings.Builder
	enc := json.NewEncoder(&jsonBuf)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return WriteResult{}, fmt.Errorf("marshal json: %w", err)
	}
	// json.Encoder.Encode appends a trailing newline; the fixtures also
	// end with a newline so this is the desired behavior.

	infoBytes := formatInfo(p.Info)

	jsonPath := filepath.Join(dir, p.Name+".json")
	infoPath := filepath.Join(dir, p.Name+".info")
	jsonTmp := jsonPath + ".tmp"
	infoTmp := infoPath + ".tmp"

	// Clean up temp files on error.
	defer func() {
		os.Remove(jsonTmp)
		os.Remove(infoTmp)
	}()

	if err := os.WriteFile(jsonTmp, []byte(jsonBuf.String()), 0o644); err != nil {
		return WriteResult{}, fmt.Errorf("write json tmp: %w", err)
	}
	if err := os.WriteFile(infoTmp, infoBytes, 0o644); err != nil {
		return WriteResult{}, fmt.Errorf("write info tmp: %w", err)
	}
	if err := os.Rename(jsonTmp, jsonPath); err != nil {
		return WriteResult{}, fmt.Errorf("rename json: %w", err)
	}
	// jsonTmp no longer exists after successful rename; remove only infoTmp
	// on the next error path.
	if err := os.Rename(infoTmp, infoPath); err != nil {
		return WriteResult{}, fmt.Errorf("rename info: %w", err)
	}

	return WriteResult{JSONPath: jsonPath, InfoPath: infoPath}, nil
}

// denormalizeArray wraps a scalar value into a single-element array for
// slicers that expect array encoding. If the value is already an array it
// is returned as-is.
func denormalizeArray(v any) any {
	if _, ok := v.([]any); ok {
		return v
	}
	return []any{v}
}

// formatInfo renders an Info struct to the INI-like `.info` sidecar format.
func formatInfo(info Info) []byte {
	var sb strings.Builder
	fmt.Fprintf(&sb, "sync_info = %s\n", info.SyncInfo)
	fmt.Fprintf(&sb, "user_id = %s\n", info.UserID)
	fmt.Fprintf(&sb, "setting_id = %s\n", info.SettingID)
	fmt.Fprintf(&sb, "base_id = %s\n", info.BaseID)
	fmt.Fprintf(&sb, "updated_time = %s\n", strconv.FormatInt(info.UpdatedTime, 10))
	return []byte(sb.String())
}
