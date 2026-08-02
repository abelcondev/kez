package receipt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreezeMatchesWorkingTreeAndDetectsDrift(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "main.go", "package main\n")

	r, err := Freeze(context.Background(), root, true, "all tests passed")
	if err != nil {
		t.Fatalf("Freeze returned error: %v", err)
	}
	if r.TreeHash == "" || !r.Verified || r.IssuedAt == "" {
		t.Fatalf("unexpected receipt: %#v", r)
	}
	if len(r.Files) != 1 || r.Files[0] != "main.go" {
		t.Fatalf("unexpected files: %#v", r.Files)
	}

	match, err := MatchesWorkingTree(context.Background(), root, r)
	if err != nil || !match {
		t.Fatalf("expected working tree to match receipt, match=%v err=%v", match, err)
	}

	// Any edit after freeze must break the match — content changed since review.
	writeFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	match, err = MatchesWorkingTree(context.Background(), root, r)
	if err != nil {
		t.Fatalf("MatchesWorkingTree error after edit: %v", err)
	}
	if match {
		t.Fatalf("expected drift after edit, but receipt still matched")
	}
}

func TestHeadTreeMatchesAfterCommit(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "a.txt", "one\n")

	r, err := Freeze(context.Background(), root, true, "")
	if err != nil {
		t.Fatalf("Freeze error: %v", err)
	}

	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "add a")

	// The committed tree is exactly what was frozen, so HEAD's tree matches.
	match, err := HeadTreeMatches(context.Background(), root, r)
	if err != nil || !match {
		t.Fatalf("expected HEAD tree to match receipt, match=%v err=%v", match, err)
	}

	// A new committed change that never went through a fresh freeze must not
	// match the old receipt — this is the amend/rebase/opaque-commit case.
	writeFile(t, root, "b.txt", "two\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "add b")
	match, err = HeadTreeMatches(context.Background(), root, r)
	if err != nil {
		t.Fatalf("HeadTreeMatches error: %v", err)
	}
	if match {
		t.Fatalf("expected stale receipt not to cover new HEAD")
	}
}

func TestReadReturnsFalseWithoutReceipt(t *testing.T) {
	root := initRepo(t)
	if _, ok, err := Read(context.Background(), root); err != nil || ok {
		t.Fatalf("expected no receipt, ok=%v err=%v", ok, err)
	}
}

func TestWriteReadRoundtripAndStoreLocation(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "x.txt", "hi\n")

	written, err := Freeze(context.Background(), root, false, "1 test failed")
	if err != nil {
		t.Fatalf("Freeze error: %v", err)
	}

	got, ok, err := Read(context.Background(), root)
	if err != nil || !ok {
		t.Fatalf("Read failed: ok=%v err=%v", ok, err)
	}
	if got.TreeHash != written.TreeHash || got.Verified != false || got.VerifySummary != "1 test failed" {
		t.Fatalf("roundtrip mismatch: wrote %#v got %#v", written, got)
	}

	// The receipt lives under the git dir, never in the working tree, so it is
	// invisible to status and can never be committed or captured by a freeze.
	if _, err := os.Stat(filepath.Join(root, ".git", "kez", "receipt.json")); err != nil {
		t.Fatalf("receipt not stored under .git/kez: %v", err)
	}
	if status := runGit(t, root, "status", "--porcelain"); strings.Contains(status, "receipt.json") {
		t.Fatalf("receipt leaked into working tree: %q", status)
	}
}

func TestWorkingTreeHashHandlesUnbornHead(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "first.txt", "seed\n")

	hash, files, err := WorkingTreeHash(context.Background(), root)
	if err != nil {
		t.Fatalf("WorkingTreeHash error on unborn HEAD: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected a tree hash for unborn HEAD")
	}
	if len(files) != 1 || files[0] != "first.txt" {
		t.Fatalf("unexpected files on unborn HEAD: %#v", files)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@kez.dev")
	runGit(t, root, "config", "user.name", "kez test")
	return root
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
