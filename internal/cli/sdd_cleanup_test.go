package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupCandidatesFiltersSafely(t *testing.T) {
	merged := []string{
		"main",             // default — never
		"* feat/002-auth",  // current (marked) — never, even though it matches
		"sdd/prop-staff",   // ✓ proposal branch
		"feat/003-catalog", // ✓ feature branch
		"hotfix/urgent",    // user's own branch — leave alone
		"",                 // blank line
	}
	got := cleanupCandidates(merged, "main", "feat/002-auth")
	want := []string{"sdd/prop-staff", "feat/003-catalog"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("cleanupCandidates = %v, want %v", got, want)
	}
}

func TestSDDCleanupDeletesMergedProposalBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "init")

	// A merged proposal branch (no new commits) and an unmerged one with work.
	git("branch", "sdd/prop-merged")
	git("checkout", "-b", "sdd/prop-unmerged")
	if err := os.WriteFile(filepath.Join(dir, "g.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "work")
	git("checkout", "main")

	var out, errBuf bytes.Buffer
	if code := runSDDCleanup(dir, nil, &out, &errBuf); code != exitSuccess {
		t.Fatalf("cleanup = %d, stderr=%s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "sdd/prop-merged (deleted)") {
		t.Errorf("expected merged branch deleted:\n%s", got)
	}
	if strings.Contains(got, "sdd/prop-unmerged") {
		t.Errorf("unmerged branch must not be deleted:\n%s", got)
	}

	// The merged ref is gone; the unmerged one survives.
	if exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", "refs/heads/sdd/prop-merged").Run() == nil {
		t.Errorf("sdd/prop-merged ref should be gone")
	}
	if exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", "refs/heads/sdd/prop-unmerged").Run() != nil {
		t.Errorf("sdd/prop-unmerged ref should still exist")
	}
}
