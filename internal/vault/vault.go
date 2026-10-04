// Package vault tracks profile files via a git repository so every sync
// is recorded and can be reviewed or rolled back. See plan.md §8.
package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
)

// CommitInfo summarizes one entry in the vault's history (plan §10
// History tab).
type CommitInfo struct {
	Hash      string
	Message   string
	Timestamp int64
}

// Vault is the per-process handle to the vault git repo.
//
// Lifecycle (plan §8 "Vault initialization"):
//
//   - On first run, or if the configured path is new, Open calls
//     `git init` + an initial commit snapshotting the live mirror.
//   - On subsequent runs, Open syncs the mirror from the live slicer
//     dirs (so out-of-band edits are captured).
type Vault struct {
	path string
}

// Open initializes or re-attaches to the vault at `path`.
//
// If `path` does not exist it is created. If it is not yet a git repo
// (`git init` is run), and an initial commit is created so `History` is
// never empty. If `path` is already a vault, Open just re-attaches.
func Open(path string) (*Vault, error) {
	v := &Vault{path: path}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("create vault dir %s: %w", path, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("stat vault %s: %w", path, err)
	}

	gitDir := filepath.Join(path, ".git")
	isRepo := false
	if _, err := os.Stat(gitDir); err == nil {
		isRepo = true
	}

	if !isRepo {
		if err := v.runGitErr("init"); err != nil {
			return nil, fmt.Errorf("git init %s: %w", path, err)
		}
		// Provide a stable identity for commits inside the vault repo so
		// the initial commit (and subsequent ones) don't fail on hosts
		// without a global git user configured.
		if err := v.runGitErr("config", "user.email", "vault@bborcaprofsync.local"); err != nil {
			return nil, fmt.Errorf("git config user.email: %w", err)
		}
		if err := v.runGitErr("config", "user.name", "BBOrcaProfsync Vault"); err != nil {
			return nil, fmt.Errorf("git config user.name: %w", err)
		}
	}

	// Always make sure there is an initial commit. If the repo already
	// has commits, this is a no-op. This guarantees that subsequent
	// History() calls never have to handle a fresh repo specially.
	if err := v.ensureInitialCommit(); err != nil {
		return nil, fmt.Errorf("ensure initial commit: %w", err)
	}

	return v, nil
}

// ensureInitialCommit creates the initial snapshot commit if and only if
// the repo has no commits yet. Idempotent on re-Open.
func (v *Vault) ensureInitialCommit() error {
	_, err := v.runGit("rev-parse", "--verify", "HEAD")
	if err == nil {
		return nil // already has a HEAD
	}
	// No commits: create an initial snapshot. `--allow-empty` keeps the
	// initial commit valid even when the vault dir is empty (typical on
	// first launch).
	if err := v.runGitErr("commit", "--allow-empty", "-m", "initial: snapshot current profiles"); err != nil {
		return fmt.Errorf("create initial commit: %w", err)
	}
	return nil
}

// Path returns the filesystem path of the vault repo.
func (v *Vault) Path() string { return v.path }

// Stage mirrors a profile (its .json + .info) into the vault's working
// tree under the appropriate slicer/category subdir (plan §8 structure)
// and adds it to the git index.
func (v *Vault) Stage(p profile.Profile) error {
	if p.Source == "" {
		return fmt.Errorf("stage: profile %q has empty Source", p.Name)
	}
	if p.Category == "" {
		return fmt.Errorf("stage: profile %q has empty Category", p.Name)
	}
	if p.Info.UserID == "" {
		return fmt.Errorf("stage: profile %q has empty Info.UserID", p.Name)
	}

	mirrorDir := filepath.Join(v.path, string(p.Source), "user", p.Info.UserID, string(p.Category))
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		return fmt.Errorf("mkdir mirror %s: %w", mirrorDir, err)
	}
	if _, err := profile.Write(mirrorDir, p); err != nil {
		return fmt.Errorf("write mirror profile %s: %w", p.Name, err)
	}

	// `git add` is the safest stage: it picks up the .json + .info files
	// we just wrote, plus any other mirror files the user (or an out-of-
	// band edit) introduced since the last commit.
	if err := v.runGitErr("add", "--", v.path); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// nothingToCommitSubstrs are substrings git emits when a commit would be
// empty. Git's wording has varied slightly across versions (e.g. "no
// changes added to commit"), so we match a couple of substrings rather
// than an exact phrase.
var nothingToCommitSubstrs = []string{
	"nothing to commit",
	"no changes added",
}

// Commit records the currently-staged changes in the vault with the
// given message (plan §8 "Commit granularity"). Returns the commit hash.
// When there is nothing to commit, returns ("", nil).
func (v *Vault) Commit(message string) (string, error) {
	if message == "" {
		return "", fmt.Errorf("commit: empty message")
	}

	out, err := v.runGit("commit", "-m", message)
	if err != nil {
		// Detect git's "nothing to commit" / "no changes added" output
		// and treat it as a no-op rather than an error.
		for _, s := range nothingToCommitSubstrs {
			if strings.Contains(out, s) {
				return "", nil
			}
		}
		return "", fmt.Errorf("git commit: %w: %s", err, strings.TrimSpace(out))
	}

	hash, err := v.runGit("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("rev-parse HEAD after commit: %w: %s", err, hash)
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "", fmt.Errorf("rev-parse HEAD returned empty hash")
	}
	return hash, nil
}

// History returns the vault's commit log, newest first.
//
// Format chosen for parsing: `%H%x09%s%x09%ct` — hash, message, unix
// timestamp, separated by TABs. Tabs are illegal in commit messages per
// git's format rules (the on-the-wire format strips them), so the split
// is unambiguous.
func (v *Vault) History() ([]CommitInfo, error) {
	raw, err := v.runGit("log", "--format=%H%x09%s%x09%ct")
	if err != nil {
		return nil, fmt.Errorf("git log: %w: %s", err, raw)
	}

	out := strings.TrimSpace(raw)
	if out == "" {
		// No commits (shouldn't normally happen because Open seeds an
		// initial commit, but treat as empty history rather than error).
		return []CommitInfo{}, nil
	}

	var commits []CommitInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed git log line %q", line)
		}
		ts, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", parts[2], err)
		}
		commits = append(commits, CommitInfo{
			Hash:      parts[0],
			Message:   parts[1],
			Timestamp: ts,
		})
	}
	return commits, nil
}

// Rollback restores the vault's mirror to the state at `hash`, then
// copies those files back to the live slicer user dirs (plan §8
// Rollback, post-MVP).
//
// Current implementation only restores the vault's working tree (the
// post-MVP copy-back to slicer dirs is handled elsewhere).
func (v *Vault) Rollback(hash string) error {
	if hash == "" {
		return fmt.Errorf("rollback: empty hash")
	}
	if err := v.runGitErr("checkout", hash, "--", "."); err != nil {
		return fmt.Errorf("git checkout %s: %w", hash, err)
	}
	return nil
}

// --- git helpers --------------------------------------------------------

// runGit executes `git <args>` in the vault directory and returns
// combined stdout+stderr. Errors include the combined output so callers
// can surface git's diagnostics.
func (v *Vault) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = v.path
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), err
	}
	return buf.String(), nil
}

// runGitErr runs git and returns an error if the exit code is non-zero.
// The error wraps git's combined output for diagnostics.
func (v *Vault) runGitErr(args ...string) error {
	out, err := v.runGit(args...)
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(out))
	}
	return nil
}
