package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abelcondev/kez/internal/receipt"
	"github.com/abelcondev/kez/internal/tools"
)

func TestGitDeliveryIntentClassifiesCommands(t *testing.T) {
	cases := []struct {
		command      string
		commit, push bool
	}{
		{"git commit -m 'x'", true, false},
		{"git push", false, true},
		{"git add -A && git commit -m x && git push", true, true},
		{"git -C sub commit -m x", true, false},
		{"git status", false, false},
		{"echo hi", false, false},
		{"git log --oneline", false, false},
	}
	for _, tc := range cases {
		commit, push := gitDeliveryIntent("bash", map[string]any{"command": tc.command})
		if commit != tc.commit || push != tc.push {
			t.Fatalf("%q: got commit=%v push=%v, want commit=%v push=%v", tc.command, commit, push, tc.commit, tc.push)
		}
	}
}

func TestReceiptPushGateBlocksUncoveredHead(t *testing.T) {
	root := initGateRepo(t)
	options := Options{Cwd: root}
	call := ToolCall{ID: "1", Name: "bash"}

	// No receipt at all → push is refused.
	if result, blocked := receiptPushGate(context.Background(), options, call); !blocked {
		t.Fatalf("expected push blocked with no receipt")
	} else if result.DenialReason != DenialReceiptBlocked || result.Status != tools.StatusError {
		t.Fatalf("unexpected block result: %#v", result)
	}

	// Freeze + commit so HEAD's tree is covered → push allowed.
	writeGateFile(t, root, "a.txt", "one\n")
	if _, err := receipt.Freeze(context.Background(), root, true, ""); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	runGateGit(t, root, "add", "-A")
	runGateGit(t, root, "commit", "-m", "add a")
	if _, blocked := receiptPushGate(context.Background(), options, call); blocked {
		t.Fatalf("expected push allowed when HEAD tree is covered")
	}

	// Amend the content outside the reviewed path → HEAD no longer covered.
	writeGateFile(t, root, "a.txt", "one changed\n")
	runGateGit(t, root, "commit", "-a", "--amend", "--no-edit")
	if _, blocked := receiptPushGate(context.Background(), options, call); !blocked {
		t.Fatalf("expected push blocked after opaque amend")
	}
}

func TestEnsureReceiptForCommitFreezesCurrentTree(t *testing.T) {
	root := initGateRepo(t)
	options := Options{Cwd: root}
	writeGateFile(t, root, "main.go", "package main\n")

	ensureReceiptForCommit(context.Background(), options)

	r, found, err := receipt.Read(context.Background(), root)
	if err != nil || !found {
		t.Fatalf("expected a receipt after commit gate, found=%v err=%v", found, err)
	}
	match, err := receipt.MatchesWorkingTree(context.Background(), root, r)
	if err != nil || !match {
		t.Fatalf("commit-gate receipt should cover the working tree, match=%v err=%v", match, err)
	}
}

func initGateRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGateGit(t, root, "init")
	runGateGit(t, root, "config", "user.email", "test@kez.dev")
	runGateGit(t, root, "config", "user.name", "kez test")
	return root
}

func writeGateFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func runGateGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
