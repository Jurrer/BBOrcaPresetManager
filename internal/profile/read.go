package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadResult bundles the result of reading a profile from disk, plus the
// raw JSON bytes (so callers can re-emit the file without losing structure
// that the parser doesn't model).
type ReadResult struct {
	Profile Profile
	RawJSON []byte
}

// Read parses a profile's `.json` file and its `.info` sidecar from the
// given directory. The base name is the profile name without extension.
//
// On read, single-element arrays are unwrapped to scalars in the Canonical
// field (plan.md §5.1). The Raw field preserves the original Go value so
// Write can re-emit fields the field-type registry doesn't know about.
func Read(dir, baseName string, category Category, source SlicerID) (ReadResult, error) {
	jsonPath := filepath.Join(dir, baseName+".json")
	infoPath := filepath.Join(dir, baseName+".info")

	rawJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		return ReadResult{}, fmt.Errorf("read json %s: %w", jsonPath, err)
	}
	rawInfo, err := os.ReadFile(infoPath)
	if err != nil {
		return ReadResult{}, fmt.Errorf("read info %s: %w", infoPath, err)
	}

	// Decode JSON preserving raw per-field bytes so we can build both Raw
	// and Canonical forms.
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &rawFields); err != nil {
		return ReadResult{}, fmt.Errorf("parse json %s: %w", jsonPath, err)
	}

	fields := make(map[string]Value, len(rawFields))
	for key, rawMsg := range rawFields {
		var v any
		if err := json.Unmarshal(rawMsg, &v); err != nil {
			return ReadResult{}, fmt.Errorf("parse field %q: %w", key, err)
		}
		fields[key] = Value{
			Raw:       v,
			Canonical: normalizeValue(v),
		}
	}

	info, err := parseInfo(string(rawInfo))
	if err != nil {
		return ReadResult{}, fmt.Errorf("parse info %s: %w", infoPath, err)
	}

	name := baseName
	if n, ok := fields["name"]; ok {
		if s, ok := n.Canonical.(string); ok {
			name = s
		}
	}

	p := Profile{
		Category: category,
		Name:     name,
		JSON:     fields,
		Info:     info,
		Source:   source,
	}

	return ReadResult{Profile: p, RawJSON: rawJSON}, nil
}

// normalizeValue unwraps single-element arrays to scalars so that
// `["400"]` and `"400"` compare equal in the diff engine (plan §5.1).
// Multi-element arrays are left as-is.
func normalizeValue(v any) any {
	switch val := v.(type) {
	case []any:
		if len(val) == 1 {
			return val[0]
		}
		return val
	default:
		return v
	}
}

// parseInfo parses the INI-like `.info` sidecar format:
//
//	sync_info = <value>
//	user_id = <value>
//	setting_id = <value>
//	base_id = <value>
//	updated_time = <unix-epoch-int>
//
// The separator is ` = ` (space-equals-space). Blank lines are skipped.
func parseInfo(data string) (Info, error) {
	var info Info
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "sync_info":
			info.SyncInfo = val
		case "user_id":
			info.UserID = val
		case "setting_id":
			info.SettingID = val
		case "base_id":
			info.BaseID = val
		case "updated_time":
			ts, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return info, fmt.Errorf("parse updated_time %q: %w", val, err)
			}
			info.UpdatedTime = ts
		}
	}
	return info, nil
}
