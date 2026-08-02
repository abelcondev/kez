// Package receipt implements kez's content-bound review receipts. A receipt is
// immutable proof of exactly which working-tree content was reviewed: it stores
// the git tree hash of the working tree (computed without mutating the real
// index), whether objective verification passed, and when it was issued.
//
// The receipt is the seam that ties "what was reviewed" to "what is delivered".
// The commit gate refuses to commit content whose tree hash does not match a
// receipt; the push gate refuses to push a HEAD whose tree a receipt never
// covered — so amend/rebase/opaque mutations that bypass kez's reviewed edit
// path cannot silently reach a remote. Trust derives from a hash the system
// recomputes, not from the agent's narration that it "already reviewed".
//
// Receipts live at <git-dir>/kez/receipt.json (never inside the working tree),
// so freezing the tree never captures the receipt itself and the file is never
// committed. git runs host-native here (kez's own process), independent of the
// command sandbox.
package receipt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Receipt is the content-bound proof of a reviewed working tree.
type Receipt struct {
	// TreeHash is the git tree object hash of the full working tree at freeze
	// time (git write-tree over a temp index seeded from HEAD + `add -A`). It is
	// the content identity the delivery gates validate against.
	TreeHash string `json:"treeHash"`
	// Branch is the branch the receipt was frozen on, for the advisor.
	Branch string `json:"branch,omitempty"`
	// Files lists the changed paths at freeze time (informational).
	Files []string `json:"files,omitempty"`
	// Verified reports whether objective verification (tests/build) passed. A
	// false value never blocks a commit — WIP is allowed — but is surfaced to
	// the agent and recorded as evidence.
	Verified bool `json:"verified"`
	// VerifySummary is a short human-facing note about the verification outcome.
	VerifySummary string `json:"verifySummary,omitempty"`
	// IssuedAt is the RFC3339 UTC freeze time.
	IssuedAt string `json:"issuedAt"`
}

// WorkingTreeHash computes the git tree hash of the full working tree at root
// without touching the real index, and returns the changed file paths. It
// mirrors zerogit.stagedSnapshotDiff's temp-index technique but derives a tree
// object id (write-tree) as the stable content identity.
func WorkingTreeHash(ctx context.Context, root string) (string, []string, error) {
	tempDir, err := os.MkdirTemp("", "kez-receipt-index-")
	if err != nil {
		return "", nil, fmt.Errorf("prepare receipt index: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	env := []string{"GIT_INDEX_FILE=" + filepath.Join(tempDir, "index")}
	if _, err := git(ctx, root, env, "rev-parse", "--verify", "HEAD"); err != nil {
		if _, emptyErr := git(ctx, root, env, "read-tree", "--empty"); emptyErr != nil {
			return "", nil, fmt.Errorf("prepare empty receipt index: %w", emptyErr)
		}
	} else if _, err := git(ctx, root, env, "read-tree", "HEAD"); err != nil {
		return "", nil, fmt.Errorf("seed receipt index from HEAD: %w", err)
	}
	if _, err := git(ctx, root, env, "add", "-A"); err != nil {
		return "", nil, fmt.Errorf("stage receipt index: %w", err)
	}
	tree, err := git(ctx, root, env, "write-tree")
	if err != nil {
		return "", nil, fmt.Errorf("hash working tree: %w", err)
	}
	files, err := git(ctx, root, env, "diff", "--cached", "--name-only")
	if err != nil {
		return "", nil, fmt.Errorf("list receipt files: %w", err)
	}
	return strings.TrimSpace(tree), splitLines(files), nil
}

// Freeze computes the working-tree hash and writes a receipt carrying the given
// verification verdict. It is the single issuance point, called both at end of
// turn and (just-in-time) by the commit gate.
func Freeze(ctx context.Context, root string, verified bool, verifySummary string) (Receipt, error) {
	hash, files, err := WorkingTreeHash(ctx, root)
	if err != nil {
		return Receipt{}, err
	}
	branch, _ := git(ctx, root, nil, "rev-parse", "--abbrev-ref", "HEAD")
	r := Receipt{
		TreeHash:      hash,
		Branch:        strings.TrimSpace(branch),
		Files:         files,
		Verified:      verified,
		VerifySummary: strings.TrimSpace(verifySummary),
		IssuedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	if err := Write(ctx, root, r); err != nil {
		return Receipt{}, err
	}
	return r, nil
}

// MatchesWorkingTree reports whether the receipt's tree hash equals the current
// working tree — i.e. nothing changed since the content was reviewed.
func MatchesWorkingTree(ctx context.Context, root string, r Receipt) (bool, error) {
	if r.TreeHash == "" {
		return false, nil
	}
	hash, _, err := WorkingTreeHash(ctx, root)
	if err != nil {
		return false, err
	}
	return hash == r.TreeHash, nil
}

// HeadTreeMatches reports whether HEAD's committed tree equals the receipt's
// tree hash. The push gate uses this to reject a HEAD whose content a receipt
// never covered (e.g. an amend/rebase done outside kez's reviewed path).
func HeadTreeMatches(ctx context.Context, root string, r Receipt) (bool, error) {
	if r.TreeHash == "" {
		return false, nil
	}
	tree, err := git(ctx, root, nil, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(tree) == r.TreeHash, nil
}

// Read loads the stored receipt for root. The bool is false (with nil error)
// when no receipt exists yet.
func Read(ctx context.Context, root string) (Receipt, bool, error) {
	path, err := storePath(ctx, root)
	if err != nil {
		return Receipt{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Receipt{}, false, nil
		}
		return Receipt{}, false, fmt.Errorf("read receipt: %w", err)
	}
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return Receipt{}, false, fmt.Errorf("parse receipt: %w", err)
	}
	return r, true, nil
}

// Write persists the receipt atomically under the git dir.
func Write(ctx context.Context, root string, r Receipt) error {
	path, err := storePath(ctx, root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("prepare receipt dir: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encode receipt: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "receipt-*.tmp")
	if err != nil {
		return fmt.Errorf("stage receipt: %w", err)
	}
	tempName := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return fmt.Errorf("write receipt: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("flush receipt: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("commit receipt: %w", err)
	}
	return nil
}

// ChangedFiles lists paths that differ from HEAD — tracked modifications plus
// untracked files. It is a cheap signal for the route advisor and, unlike
// WorkingTreeHash, builds no temp index.
func ChangedFiles(ctx context.Context, root string) ([]string, error) {
	out, err := git(ctx, root, nil, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// Root returns the git working-tree root containing cwd. The bool is false when
// cwd is not inside a git repository, which callers treat as "no gate applies".
func Root(ctx context.Context, cwd string) (string, bool) {
	root, err := git(ctx, cwd, nil, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(root) == "" {
		return "", false
	}
	return strings.TrimSpace(root), true
}

// storePath resolves <git-dir>/kez/receipt.json, honoring worktrees.
func storePath(ctx context.Context, root string) (string, error) {
	dir, err := git(ctx, root, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", fmt.Errorf("locate git dir: %w", err)
	}
	return filepath.Join(strings.TrimSpace(dir), "kez", "receipt.json"), nil
}

// git runs git in dir with optional extra env and returns trimmed stdout, or an
// error carrying git's stderr. kez runs git host-native, so this never routes
// through the command sandbox.
func git(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	if len(env) > 0 {
		command.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func splitLines(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}
