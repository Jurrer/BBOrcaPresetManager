// Package paths detects slicer installation dirs and selects the active
// user_id per slicer. See plan.md §9.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// SlicerPaths holds the resolved default user dir for a slicer plus the
// list of candidate user_id subdirs found inside it.
type SlicerPaths struct {
	UserDir         string
	CandidateUIDs   []string
	SelectedUID     string
	DetectedVersion string
}

// DetectPaths probes the host OS for known slicer install locations.
//
// macOS (MVP, plan §9):
//
//   - ~/Library/Application Support/BambuStudio/user/
//   - ~/Library/Application Support/OrcaSlicer/user/
//
// Linux/Windows are post-MVP.
//
// For each slicer, it returns the resolved user dir (after picking the
// most-populated candidate uid per plan §9 "User_id selection") and the
// full list of candidate uids so the Settings tab can override.
func DetectPaths() (bs SlicerPaths, orca SlicerPaths, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return SlicerPaths{}, SlicerPaths{}, err
	}

	bs, err = detectSlicer("BambuStudio",
		filepath.Join(home, "Library", "Application Support", "BambuStudio", "user"),
		profile.SlicerBambuStudio)
	if err != nil {
		return SlicerPaths{}, SlicerPaths{}, err
	}

	orca, err = detectSlicer("OrcaSlicer",
		filepath.Join(home, "Library", "Application Support", "OrcaSlicer", "user"),
		profile.SlicerOrcaSlicer)
	if err != nil {
		return SlicerPaths{}, SlicerPaths{}, err
	}

	return bs, orca, nil
}

// SelectUserID applies the auto-pick rule from plan §9:
//
//  1. Exclude empty dirs.
//  2. Pick the dir with the most profiles.
//  3. (Caller can override via Settings.)
func SelectUserID(candidates []string, profileCounts map[string]int) string {
	if len(candidates) == 0 {
		return ""
	}

	sorted := make([]string, len(candidates))
	copy(sorted, candidates)
	sort.Strings(sorted)

	best := ""
	bestCount := -1

	for _, uid := range sorted {
		count := profileCounts[uid]
		if count > bestCount {
			best = uid
			bestCount = count
		}
	}

	if bestCount <= 0 {
		return ""
	}

	return best
}

// detectSlicer scans a slicer's user parent directory (the `user/` dir)
// for candidate user_id subdirs, counts profiles in each, and
// auto-selects the best candidate via SelectUserID.
//
// If userParentDir does not exist, returns an empty SlicerPaths with no
// error (slicer not installed).
func detectSlicer(slicerName string, userParentDir string, source profile.SlicerID) (SlicerPaths, error) {
	entries, err := os.ReadDir(userParentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return SlicerPaths{}, nil
		}
		return SlicerPaths{}, fmt.Errorf("read user dir for %s: %w", slicerName, err)
	}

	var candidates []string
	profileCounts := make(map[string]int)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		uid := entry.Name()
		uidDir := filepath.Join(userParentDir, uid)
		count := countProfiles(uidDir)
		candidates = append(candidates, uid)
		profileCounts[uid] = count
	}

	sort.Strings(candidates)

	selected := SelectUserID(candidates, profileCounts)

	result := SlicerPaths{
		CandidateUIDs: candidates,
		SelectedUID:   selected,
	}
	if selected != "" {
		result.UserDir = filepath.Join(userParentDir, selected)
		result.DetectedVersion = detectVersion(result.UserDir, source)
	}

	return result, nil
}

// detectVersion samples one existing profile in the user dir and reads its
// "version" field (plan §5.2). It prefers the process category, then
// machine, then filament. Errors are handled gracefully: if no profile can
// be read (or none has a version field), an empty string is returned.
func detectVersion(userDir string, source profile.SlicerID) string {
	for _, cat := range []string{"process", "machine", "filament"} {
		catDir := filepath.Join(userDir, cat)
		entries, err := os.ReadDir(catDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			baseName := entry.Name()[:len(entry.Name())-len(".json")]
			rr, err := profile.Read(catDir, baseName, profile.Category(cat), source)
			if err != nil {
				continue
			}
			if v, ok := rr.Profile.JSON["version"]; ok {
				if s, ok := v.Canonical.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// countProfiles counts .json profile files across the machine, process,
// and filament subdirectories of a user_id dir.
func countProfiles(uidDir string) int {
	categories := []string{"machine", "process", "filament"}
	count := 0
	for _, cat := range categories {
		catDir := filepath.Join(uidDir, cat)
		entries, err := os.ReadDir(catDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				count++
			}
		}
	}
	return count
}
