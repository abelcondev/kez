package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustWriteRepoFile creates parent dirs and writes content, for fabricating a
// minimal git work-tree (branchguard reads .git/HEAD directly, no git binary).
func mustWriteRepoFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWriteFileBranchGuardEndToEnd exercises the real tool path: with the
// require-branch marker present, write_file blocks code on a protected branch
// but allows docs there and code on a feature branch.
func TestWriteFileBranchGuardEndToEnd(t *testing.T) {
	root := t.TempDir()
	mustWriteRepoFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")
	mustWriteRepoFile(t, filepath.Join(root, ".kez", "require-branch"), "")

	// Code on the protected branch is blocked.
	res := NewScopedWriteFileTool(root, nil).Run(context.Background(), map[string]any{
		"path": "app.go", "content": "package main\n",
	})
	if res.Status != StatusError || !strings.Contains(res.Output, "protected branch") {
		t.Fatalf("expected branch-guard block on main, got status=%v output=%q", res.Status, res.Output)
	}

	// Docs are allowed on the protected branch.
	res = NewScopedWriteFileTool(root, nil).Run(context.Background(), map[string]any{
		"path": "notes.md", "content": "hi\n",
	})
	if res.Status == StatusError {
		t.Fatalf("markdown must be allowed on the protected branch, got %q", res.Output)
	}

	// Code on a feature branch is allowed.
	mustWriteRepoFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/feat/login\n")
	res = NewScopedWriteFileTool(root, nil).Run(context.Background(), map[string]any{
		"path": "feature.go", "content": "package main\n",
	})
	if res.Status == StatusError {
		t.Fatalf("code on a feature branch must be allowed, got %q", res.Output)
	}
}

// TestWriteFileBranchGuardOffWithoutMarker confirms a repo that never opted in
// is untouched: code writes on main are allowed.
func TestWriteFileBranchGuardOffWithoutMarker(t *testing.T) {
	root := t.TempDir()
	mustWriteRepoFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")

	res := NewScopedWriteFileTool(root, nil).Run(context.Background(), map[string]any{
		"path": "app.go", "content": "package main\n",
	})
	if res.Status == StatusError {
		t.Fatalf("without the marker the guard must stay off, got %q", res.Output)
	}
}
