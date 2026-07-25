package branchguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckStateBlocksCodeOnProtectedBranch(t *testing.T) {
	blocked := State{Enabled: true, Branch: "main", Protected: true}
	if err := checkState(blocked, "internal/foo.go"); err == nil {
		t.Fatal("expected a block for code on the protected branch")
	}
	// Non-code (SDD notes, docs) is allowed even on the protected branch.
	if err := checkState(blocked, "sdd/proposal.md"); err != nil {
		t.Errorf("markdown must be allowed on protected branch, got %v", err)
	}
	if err := checkState(blocked, "README.md"); err != nil {
		t.Errorf("docs must be allowed on protected branch, got %v", err)
	}
}

func TestCheckStateAllowsWhenNotProtectedOrDisabled(t *testing.T) {
	cases := []State{
		{Enabled: true, Branch: "feat/x", Protected: false}, // feature branch
		{Enabled: false, Branch: "main", Protected: true},   // repo opted out
		{Enabled: true, Branch: "", Protected: false},       // detached HEAD
	}
	for _, st := range cases {
		if err := checkState(st, "internal/foo.go"); err != nil {
			t.Errorf("state %+v must allow, got %v", st, err)
		}
	}
}

func TestGuardErrorGuidesTheModel(t *testing.T) {
	err := checkState(State{Enabled: true, Branch: "main", Protected: true}, "internal/big.go")
	if err == nil {
		t.Fatal("expected a GuardError")
	}
	msg := err.Error()
	for _, want := range []string{"main", "internal/big.go", "git checkout -b", "KEZ_REQUIRE_BRANCH=off"} {
		if !contains(msg, want) {
			t.Errorf("guard message missing %q: %s", want, msg)
		}
	}
}

// --- Resolve: filesystem + env integration -----------------------------------

// initRepo makes root a git work-tree on branch, with an optional marker.
func initRepo(t *testing.T, branch string, withMarker bool) string {
	t.Helper()
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	mustMkdir(t, gitDir)
	mustWrite(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/"+branch+"\n")
	if withMarker {
		mustMkdir(t, filepath.Join(root, ".kez"))
		mustWrite(t, filepath.Join(root, ".kez", "require-branch"), "")
	}
	return root
}

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolveMarkerEnablesGuardOnProtectedBranch(t *testing.T) {
	root := initRepo(t, "main", true)
	st := Resolve(root, envMap(nil))
	if !st.Enabled || !st.Protected || st.Branch != "main" {
		t.Fatalf("marker on main should enable+protect, got %+v", st)
	}
}

func TestResolveNoMarkerNoEnvIsDisabled(t *testing.T) {
	root := initRepo(t, "main", false)
	if st := Resolve(root, envMap(nil)); st.Enabled {
		t.Fatalf("no marker + no env must be disabled, got %+v", st)
	}
}

func TestResolveEnvOnForcesGuardEvenWithoutMarker(t *testing.T) {
	root := initRepo(t, "master", false)
	st := Resolve(root, envMap(map[string]string{"KEZ_REQUIRE_BRANCH": "on"}))
	if !st.Enabled || !st.Protected {
		t.Fatalf("env on should enable+protect on master, got %+v", st)
	}
}

func TestResolveEnvOffBeatsMarker(t *testing.T) {
	root := initRepo(t, "main", true)
	if st := Resolve(root, envMap(map[string]string{"KEZ_REQUIRE_BRANCH": "off"})); st.Enabled {
		t.Fatalf("env off must override the marker, got %+v", st)
	}
}

func TestResolveFeatureBranchNotProtected(t *testing.T) {
	root := initRepo(t, "feat/login", true)
	st := Resolve(root, envMap(nil))
	if !st.Enabled {
		t.Fatal("marker should enable the guard")
	}
	if st.Protected {
		t.Fatalf("feature branch must not be protected, got %+v", st)
	}
	if err := checkState(st, "internal/foo.go"); err != nil {
		t.Errorf("writes on a feature branch must be allowed, got %v", err)
	}
}

func TestResolveCustomProtectedBranchViaEnv(t *testing.T) {
	root := initRepo(t, "develop", true)
	st := Resolve(root, envMap(map[string]string{"KEZ_PROTECTED_BRANCHES": "develop, release"}))
	if !st.Protected {
		t.Fatalf("develop should be protected via KEZ_PROTECTED_BRANCHES, got %+v", st)
	}
}

func TestResolveNonGitPathIsDisabled(t *testing.T) {
	if st := Resolve(t.TempDir(), envMap(map[string]string{"KEZ_REQUIRE_BRANCH": "on"})); st.Enabled {
		t.Fatalf("a non-git path must never enable the guard, got %+v", st)
	}
}

func TestResolveDetachedHeadIsNotProtected(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	mustMkdir(t, gitDir)
	mustWrite(t, filepath.Join(gitDir, "HEAD"), "9f8e7d6c5b4a\n") // raw sha → detached
	mustMkdir(t, filepath.Join(root, ".kez"))
	mustWrite(t, filepath.Join(root, ".kez", "require-branch"), "")

	st := Resolve(root, envMap(nil))
	if st.Protected || st.Branch != "" {
		t.Fatalf("detached HEAD must not be protected, got %+v", st)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
