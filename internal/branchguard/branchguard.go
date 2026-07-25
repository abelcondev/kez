// Package branchguard implements kez's compiled "work on a branch, not on the
// protected branch" gate. Like the line-length gate in package qualitygate it
// runs inside the write_file and edit_file tools, so the model cannot dodge it
// by choosing a different tool.
//
// The gate exists for repos that only accept changes via pull request: it
// refuses to write source code while HEAD is on a protected branch (main /
// master / the remote default), pushing feature work onto a feature branch from
// the very first edit instead of at push time. It is OFF unless the repo opts in
// — a `.kez/require-branch` marker at the git root, or KEZ_REQUIRE_BRANCH=on —
// so a fresh project's foundations phase and casual single-branch repos are
// untouched.
package branchguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abelcondev/kez/internal/qualitygate"
)

// State is the resolved guard decision for a workspace at write time.
type State struct {
	Enabled   bool   // the repo opted in (marker or env)
	Branch    string // current branch name, "" when detached or unreadable
	Protected bool   // Branch is a protected/default branch
}

// GuardError is returned when a write is blocked because the repo requires a
// feature branch. Its message is model-facing: it names the branch and the fix.
type GuardError struct {
	Branch string
	Path   string
}

func (e *GuardError) Error() string {
	return fmt.Sprintf(
		"Error: write blocked — you are on protected branch %q, but this repo requires feature branches + PRs. "+
			"Create a feature branch and retry: `git checkout -b feat/<short-name>`, then implement, push, and open a PR. "+
			"%s was NOT written. (Override for this run with KEZ_REQUIRE_BRANCH=off.)",
		e.Branch, e.Path)
}

// Check enforces the branch guard for a write of relativePath under
// workspaceRoot, reading its configuration and git state from the environment
// and filesystem. Returns nil when the write is allowed.
func Check(workspaceRoot, relativePath string) error {
	return checkState(Resolve(workspaceRoot, os.Getenv), relativePath)
}

// checkState is the pure decision core: it needs only a resolved State and the
// path, so tests can exercise every branch without touching git or the fs.
func checkState(state State, relativePath string) error {
	if !state.Enabled || !state.Protected {
		return nil
	}
	// Only code is gated; SDD notes, docs, and config may be edited on any branch
	// (matching the line-length gate's notion of "code").
	if !qualitygate.IsGatedSourceFile(relativePath) {
		return nil
	}
	return &GuardError{Branch: state.Branch, Path: relativePath}
}

// Resolve inspects the repo containing workspaceRoot and returns the guard
// State. A non-git path, an opted-out repo, or an unreadable HEAD all resolve to
// "do not block" (fail open) — the guard only ever blocks when it is certain the
// repo opted in AND HEAD is on a protected branch.
func Resolve(workspaceRoot string, getenv func(string) string) State {
	if getenv == nil {
		getenv = os.Getenv
	}
	start := strings.TrimSpace(workspaceRoot)
	if start == "" {
		return State{}
	}
	root, ok := findGitRoot(start)
	if !ok {
		return State{}
	}
	if !enabled(root, getenv) {
		return State{}
	}
	gitDir, ok := resolveGitDir(root)
	if !ok {
		return State{Enabled: true}
	}
	branch := currentBranch(gitDir)
	protected := branch != "" && protectedBranches(gitDir, getenv)[branch]
	return State{Enabled: true, Branch: branch, Protected: protected}
}

// enabled resolves opt-in: KEZ_REQUIRE_BRANCH forces on/off; otherwise the
// presence of a `.kez/require-branch` marker at the git root enables the guard.
func enabled(root string, getenv func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(getenv("KEZ_REQUIRE_BRANCH"))) {
	case "on", "1", "true", "yes":
		return true
	case "off", "0", "false", "no":
		return false
	}
	if _, err := os.Stat(filepath.Join(root, ".kez", "require-branch")); err == nil {
		return true
	}
	return false
}

// findGitRoot walks up from start looking for a `.git` directory or file,
// returning the repository (work-tree) root.
func findGitRoot(start string) (string, bool) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// resolveGitDir returns the git directory holding HEAD for the repo at root. It
// handles both a normal `.git` directory and a `.git` file (linked worktrees and
// submodules) whose contents point at the real git dir via "gitdir: <path>".
func resolveGitDir(root string) (string, bool) {
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return gitPath, true
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return gitDir, true
}

// currentBranch reads the checked-out branch from HEAD, returning "" for a
// detached HEAD (a raw commit sha) or an unreadable HEAD.
func currentBranch(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	const prefix = "ref: refs/heads/"
	if strings.HasPrefix(head, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(head, prefix))
	}
	return ""
}

// protectedBranches is the set of branch names the guard treats as protected:
// main and master always, plus the remote default (refs/remotes/origin/HEAD)
// when recorded, plus any names from KEZ_PROTECTED_BRANCHES.
func protectedBranches(gitDir string, getenv func(string) string) map[string]bool {
	set := map[string]bool{"main": true, "master": true}
	if data, err := os.ReadFile(filepath.Join(gitDir, "refs", "remotes", "origin", "HEAD")); err == nil {
		line := strings.TrimSpace(string(data))
		const prefix = "ref: refs/remotes/origin/"
		if strings.HasPrefix(line, prefix) {
			set[strings.TrimSpace(strings.TrimPrefix(line, prefix))] = true
		}
	}
	for _, name := range strings.Split(getenv("KEZ_PROTECTED_BRANCHES"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			set[name] = true
		}
	}
	return set
}
