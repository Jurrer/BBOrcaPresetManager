// Package vault tests the git-backed vault that mirrors profile files.
package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// gitAvailable returns true if `git` is on PATH. Tests are skipped
// otherwise so the package still builds on minimal CI images.
func gitAvailable() bool {
	for _, dir := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "git")); err == nil {
			return true
		}
	}
	return false
}

func skipIfNoGit(t *testing.T) {
	t.Helper()
	if !gitAvailable() {
		t.Skip("git binary not available on PATH; skipping")
	}
}

// newVault returns a Vault backed by a fresh temp dir. Fails the test on
// error so callers can use it inline in table-style assertions.
func newVault(t *testing.T) *Vault {
	t.Helper()
	skipIfNoGit(t)
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open fresh vault: %v", err)
	}
	return v
}

// fakeProfile builds a deterministic synthetic profile for testing.
// `top_surface_pattern` is used as the field under test because it is
// a scalar in both BambuStudio and OrcaSlicer (see
// internal/profile/fieldtype.go), so the round-trip JSON value can be
// compared directly without array-encoding complexity.
func fakeProfile(name string) profile.Profile {
	return profile.Profile{
		Category: profile.CategoryProcess,
		Name:     name,
		Source:   profile.SlicerBambuStudio,
		JSON: map[string]profile.Value{
			"name":                {Raw: name, Canonical: name},
			"version":             {Raw: "1.10.0.32", Canonical: "1.10.0.32"},
			"inner_wall_speed":    {Raw: "400", Canonical: "400"},
			"top_surface_pattern": {Raw: "zig-zag", Canonical: "zig-zag"},
		},
		Info: profile.Info{
			SyncInfo:    "",
			UserID:      name + "-uid",
			SettingID:   "PPUS249810809cffcb",
			BaseID:      "GP079",
			UpdatedTime: 1775010477,
		},
	}
}

// readMirrorField reads the .json mirror for `p` and returns the decoded
// value of `field`.
func readMirrorField(t *testing.T, v *Vault, p profile.Profile, field string) any {
	t.Helper()
	path := filepath.Join(v.Path(), string(p.Source), "user", p.Info.UserID, string(p.Category), p.Name+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mirror %s: %v", path, err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("parse mirror %s: %v", path, err)
	}
	return fields[field]
}

// --- Open ---------------------------------------------------------------

func TestOpenFreshCreatesRepoAndInitialCommit(t *testing.T) {
	v := newVault(t)

	if _, err := os.Stat(v.Path()); err != nil {
		t.Fatalf("vault path missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Path(), ".git")); err != nil {
		t.Fatalf("vault .git missing: %v", err)
	}
	history, err := v.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("want 1 initial commit, got %d", len(history))
	}
	if !strings.HasPrefix(history[0].Message, "initial") {
		t.Errorf("initial commit message = %q, want prefix %q", history[0].Message, "initial")
	}
	if history[0].Timestamp <= 0 {
		t.Errorf("initial commit timestamp = %d, want > 0", history[0].Timestamp)
	}
}

func TestOpenExistingRepoNoReInit(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	v1, err := Open(dir)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := v1.Stage(fakeProfile("alpha")); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if _, err := v1.Commit("setup alpha"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	v2, err := Open(dir)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	history, err := v2.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 { // initial + setup alpha
		t.Fatalf("want 2 commits after re-open, got %d", len(history))
	}
	if history[0].Message != "setup alpha" {
		t.Errorf("newest commit message = %q, want %q", history[0].Message, "setup alpha")
	}
	if history[1].Message != "initial: snapshot current profiles" {
		t.Errorf("oldest commit message = %q, want initial", history[1].Message)
	}
}

func TestOpenCreatesMissingDir(t *testing.T) {
	skipIfNoGit(t)
	parent := t.TempDir()
	path := filepath.Join(parent, "nested", "vault")
	v, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if v.Path() != path {
		t.Errorf("Path = %q, want %q", v.Path(), path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("vault dir not created: %v", err)
	}
}

// --- Stage + Commit ------------------------------------------------------

func TestStageAndCommit(t *testing.T) {
	v := newVault(t)

	p := fakeProfile("alpha")
	if err := v.Stage(p); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	mirrorDir := filepath.Join(v.Path(), string(p.Source), "user", p.Info.UserID, string(p.Category))
	if _, err := os.Stat(filepath.Join(mirrorDir, p.Name+".json")); err != nil {
		t.Errorf("mirror json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mirrorDir, p.Name+".info")); err != nil {
		t.Errorf("mirror info missing: %v", err)
	}

	h, err := v.History()
	if err != nil {
		t.Fatalf("History pre-commit: %v", err)
	}
	if len(h) != 1 {
		t.Fatalf("pre-commit history = %d, want 1 (initial only)", len(h))
	}

	hash, err := v.Commit("sync: process alpha to orcaslicer")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if hash == "" {
		t.Fatal("Commit returned empty hash")
	}
	if len(hash) < 7 {
		t.Errorf("hash %q looks too short for git", hash)
	}

	h, err = v.History()
	if err != nil {
		t.Fatalf("History post-commit: %v", err)
	}
	if len(h) != 2 {
		t.Fatalf("post-commit history = %d, want 2", len(h))
	}
	if h[0].Hash != hash {
		t.Errorf("latest commit hash = %q, want %q", h[0].Hash, hash)
	}
	if h[0].Message != "sync: process alpha to orcaslicer" {
		t.Errorf("latest commit message = %q", h[0].Message)
	}
	if h[0].Timestamp <= time.Now().Add(-time.Hour).Unix() {
		t.Errorf("timestamp %d looks too old", h[0].Timestamp)
	}
}

func TestStageCreatesNestedDirs(t *testing.T) {
	v := newVault(t)
	p := fakeProfile("deep")
	if err := v.Stage(p); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	want := filepath.Join(v.Path(), "bambustudio", "user", "deep-uid", "process")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected nested dir %s, got: %v", want, err)
	}
}

func TestStageFromBothSlicers(t *testing.T) {
	v := newVault(t)

	bs := fakeProfile("profile-bs")
	bs.Source = profile.SlicerBambuStudio
	if err := v.Stage(bs); err != nil {
		t.Fatalf("Stage bs: %v", err)
	}

	orca := fakeProfile("profile-orca")
	orca.Source = profile.SlicerOrcaSlicer
	orca.Info.UserID = "orca-uid"
	if err := v.Stage(orca); err != nil {
		t.Fatalf("Stage orca: %v", err)
	}

	if _, err := os.Stat(filepath.Join(v.Path(), "bambustudio", "user", bs.Info.UserID, "process", bs.Name+".json")); err != nil {
		t.Errorf("bs mirror json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Path(), "orcaslicer", "user", orca.Info.UserID, "process", orca.Name+".json")); err != nil {
		t.Errorf("orca mirror json missing: %v", err)
	}

	if _, err := v.Commit("sync both"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestStageOverwriteAndCommit(t *testing.T) {
	v := newVault(t)
	p := fakeProfile("alpha")
	if err := v.Stage(p); err != nil {
		t.Fatalf("first Stage: %v", err)
	}
	if _, err := v.Commit("first"); err != nil {
		t.Fatalf("first Commit: %v", err)
	}

	p.JSON["top_surface_pattern"] = profile.Value{Raw: "rectilinear", Canonical: "rectilinear"}
	if err := v.Stage(p); err != nil {
		t.Fatalf("second Stage: %v", err)
	}
	if _, err := v.Commit("second"); err != nil {
		t.Fatalf("second Commit: %v", err)
	}

	h, err := v.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(h) != 3 {
		t.Fatalf("want 3 commits (initial + 2), got %d", len(h))
	}

	if got := readMirrorField(t, v, p, "top_surface_pattern"); got != "rectilinear" {
		t.Errorf("top_surface_pattern = %v, want rectilinear", got)
	}
}

// --- Commit empty ------------------------------------------------------

func TestCommitWithNothingStaged(t *testing.T) {
	v := newVault(t)
	hash, err := v.Commit("no-op")
	if err != nil {
		t.Fatalf("Commit empty: %v", err)
	}
	if hash != "" {
		t.Errorf("hash = %q, want empty", hash)
	}
	h, err := v.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(h) != 1 {
		t.Errorf("history = %d, want 1 (initial only)", len(h))
	}
}

// --- History ------------------------------------------------------------

func TestHistoryEmptyAfterJustInitial(t *testing.T) {
	v := newVault(t)
	h, err := v.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(h) != 1 {
		t.Fatalf("want 1 entry (initial), got %d", len(h))
	}
	if h[0].Hash == "" {
		t.Error("Hash empty")
	}
	if h[0].Message == "" {
		t.Error("Message empty")
	}
	if h[0].Timestamp <= 0 {
		t.Error("Timestamp not populated")
	}
}

func TestHistoryNewestFirst(t *testing.T) {
	v := newVault(t)
	for _, msg := range []string{"one", "two", "three"} {
		if err := v.Stage(fakeProfile(msg)); err != nil {
			t.Fatalf("Stage %s: %v", msg, err)
		}
		if _, err := v.Commit(msg); err != nil {
			t.Fatalf("Commit %s: %v", msg, err)
		}
	}
	h, err := v.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(h) != 4 {
		t.Fatalf("want 4, got %d", len(h))
	}
	want := []string{"three", "two", "one"}
	for i, msg := range want {
		if h[i].Message != msg {
			t.Errorf("h[%d].Message = %q, want %q", i, h[i].Message, msg)
		}
	}
}

// --- Rollback -----------------------------------------------------------

func TestRollbackRestoresMirror(t *testing.T) {
	v := newVault(t)
	p := fakeProfile("alpha")

	if err := v.Stage(p); err != nil {
		t.Fatalf("Stage 1: %v", err)
	}
	hash1, err := v.Commit("v1")
	if err != nil {
		t.Fatalf("Commit 1: %v", err)
	}

	p.JSON["top_surface_pattern"] = profile.Value{Raw: "rectilinear", Canonical: "rectilinear"}
	if err := v.Stage(p); err != nil {
		t.Fatalf("Stage 2: %v", err)
	}
	if _, err := v.Commit("v2"); err != nil {
		t.Fatalf("Commit 2: %v", err)
	}

	if got := readMirrorField(t, v, p, "top_surface_pattern"); got != "rectilinear" {
		t.Fatalf("pre-rollback top_surface_pattern = %v, want rectilinear", got)
	}

	if err := v.Rollback(hash1); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if got := readMirrorField(t, v, p, "top_surface_pattern"); got != "zig-zag" {
		t.Errorf("post-rollback top_surface_pattern = %v, want zig-zag", got)
	}

	h, err := v.History()
	if err != nil {
		t.Fatalf("History post-rollback: %v", err)
	}
	if len(h) != 3 {
		t.Errorf("history = %d, want 3 (rollback doesn't rewrite history)", len(h))
	}
}
