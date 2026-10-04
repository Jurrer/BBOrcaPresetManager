package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

func TestSelectUserID_NoCandidates(t *testing.T) {
	got := SelectUserID(nil, nil)
	if got != "" {
		t.Errorf("SelectUserID(nil, nil) = %q, want %q", got, "")
	}
}

func TestSelectUserID_AllEmpty(t *testing.T) {
	candidates := []string{"uid-a", "uid-b", "uid-c"}
	counts := map[string]int{"uid-a": 0, "uid-b": 0, "uid-c": 0}
	got := SelectUserID(candidates, counts)
	if got != "" {
		t.Errorf("SelectUserID all empty = %q, want %q", got, "")
	}
}

func TestSelectUserID_SingleCandidate(t *testing.T) {
	candidates := []string{"only-uid"}
	counts := map[string]int{"only-uid": 5}
	got := SelectUserID(candidates, counts)
	if got != "only-uid" {
		t.Errorf("SelectUserID single = %q, want %q", got, "only-uid")
	}
}

func TestSelectUserID_MultiplePicksMost(t *testing.T) {
	candidates := []string{"uid-a", "uid-b", "uid-c"}
	counts := map[string]int{"uid-a": 3, "uid-b": 7, "uid-c": 5}
	got := SelectUserID(candidates, counts)
	if got != "uid-b" {
		t.Errorf("SelectUserID multiple = %q, want %q (most profiles)", got, "uid-b")
	}
}

func TestSelectUserID_TiePicksAlphabetical(t *testing.T) {
	candidates := []string{"zebra", "alpha", "mango"}
	counts := map[string]int{"zebra": 4, "alpha": 4, "mango": 4}
	got := SelectUserID(candidates, counts)
	if got != "alpha" {
		t.Errorf("SelectUserID tie = %q, want %q (alphabetically first)", got, "alpha")
	}
}

func TestSelectUID_TieDifferentOrder(t *testing.T) {
	// Ensure input order doesn't affect tie-breaking.
	candidates := []string{"charlie", "alpha", "bravo"}
	counts := map[string]int{"charlie": 2, "alpha": 2, "bravo": 2}
	got := SelectUserID(candidates, counts)
	if got != "alpha" {
		t.Errorf("SelectUserID tie (different order) = %q, want %q", got, "alpha")
	}
}

func TestDetectSlicer_NonExistentDir(t *testing.T) {
	sp, err := detectSlicer("TestSlicer", "/nonexistent/path/that/does/not/exist", profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("detectSlicer non-existent dir returned error: %v", err)
	}
	if sp.UserDir != "" || sp.SelectedUID != "" || len(sp.CandidateUIDs) != 0 {
		t.Errorf("detectSlicer non-existent dir = %+v, want empty SlicerPaths", sp)
	}
}

func TestDetectSlicer_TempDir(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(root, "user")

	// uid-fewer: 1 profile in process/
	uidFewer := filepath.Join(userDir, "uid-fewer", "process")
	if err := os.MkdirAll(uidFewer, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProfileJSON(t, uidFewer, "p1")

	// uid-more: 3 profiles across categories
	uidMore := filepath.Join(userDir, "uid-more")
	for _, cat := range []string{"process", "machine", "filament"} {
		dir := filepath.Join(uidMore, cat)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeProfileJSON(t, dir, "p2")
	}

	sp, err := detectSlicer("TestSlicer", userDir, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("detectSlicer returned error: %v", err)
	}

	if len(sp.CandidateUIDs) != 2 {
		t.Fatalf("CandidateUIDs len = %d, want 2", len(sp.CandidateUIDs))
	}

	if sp.SelectedUID != "uid-more" {
		t.Errorf("SelectedUID = %q, want %q", sp.SelectedUID, "uid-more")
	}

	wantUserDir := filepath.Join(userDir, "uid-more")
	if sp.UserDir != wantUserDir {
		t.Errorf("UserDir = %q, want %q", sp.UserDir, wantUserDir)
	}
}

func TestDetectSlicer_AllEmptyUIDs(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(root, "user")

	for _, uid := range []string{"uid-a", "uid-b"} {
		if err := os.MkdirAll(filepath.Join(userDir, uid), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	sp, err := detectSlicer("TestSlicer", userDir, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("detectSlicer returned error: %v", err)
	}

	if sp.SelectedUID != "" {
		t.Errorf("SelectedUID = %q, want empty (all UIDs empty)", sp.SelectedUID)
	}
	if sp.UserDir != "" {
		t.Errorf("UserDir = %q, want empty", sp.UserDir)
	}
	if len(sp.CandidateUIDs) != 2 {
		t.Errorf("CandidateUIDs len = %d, want 2", len(sp.CandidateUIDs))
	}
}

func TestDetectSlicer_IgnoresNonDirEntries(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(root, "user")

	// Create a UID dir with profiles.
	uidDir := filepath.Join(userDir, "uid-real", "process")
	if err := os.MkdirAll(uidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProfileJSON(t, uidDir, "p1")

	// Create a non-directory entry (file) inside user/.
	if err := os.WriteFile(filepath.Join(userDir, "hints.cereal"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	sp, err := detectSlicer("TestSlicer", userDir, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("detectSlicer returned error: %v", err)
	}

	if len(sp.CandidateUIDs) != 1 {
		t.Fatalf("CandidateUIDs len = %d, want 1 (file entry ignored)", len(sp.CandidateUIDs))
	}
	if sp.CandidateUIDs[0] != "uid-real" {
		t.Errorf("CandidateUIDs[0] = %q, want %q", sp.CandidateUIDs[0], "uid-real")
	}
}

func writeProfileJSON(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name+".json")
	if err := os.WriteFile(p, []byte(`{"name":"`+name+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectSlicer_VersionDetection(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(root, "user")

	uidDir := filepath.Join(userDir, "uid-ver", "process")
	if err := os.MkdirAll(uidDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a profile with a version field + .info sidecar so profile.Read works.
	jsonContent := `{"name":"ptest","version":"1.10.0.32"}`
	if err := os.WriteFile(filepath.Join(uidDir, "ptest.json"), []byte(jsonContent), 0o644); err != nil {
		t.Fatal(err)
	}
	infoContent := "sync_info = \nuser_id = uid-ver\nsetting_id = s\nbase_id = b\nupdated_time = 1\n"
	if err := os.WriteFile(filepath.Join(uidDir, "ptest.info"), []byte(infoContent), 0o644); err != nil {
		t.Fatal(err)
	}

	sp, err := detectSlicer("BambuStudio", userDir, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("detectSlicer: %v", err)
	}
	if sp.DetectedVersion != "1.10.0.32" {
		t.Errorf("DetectedVersion = %q, want %q", sp.DetectedVersion, "1.10.0.32")
	}
}

func TestDetectSlicer_VersionNoProfiles(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(root, "user")

	// Create a uid dir with a non-profile file (no .json).
	uidDir := filepath.Join(userDir, "uid-empty", "process")
	if err := os.MkdirAll(uidDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sp, err := detectSlicer("BambuStudio", userDir, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("detectSlicer: %v", err)
	}
	if sp.DetectedVersion != "" {
		t.Errorf("DetectedVersion = %q, want empty (no readable profiles)", sp.DetectedVersion)
	}
}
