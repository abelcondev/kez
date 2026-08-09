package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abelcondev/kez/internal/sdd"
)

// setupProposalWithTasks scaffolds an SDD base with two pending tasks linked to
// the same decision, returning the workspace root and the two task stems.
func setupProposalWithTasks(t *testing.T) (root, task1, task2 string) {
	t.Helper()
	root = t.TempDir()
	if _, _, err := sdd.Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	now := time.Unix(0, 0).UTC()
	rel1, err := sdd.AddTask(root, "decisions/001-staff.md", "Backend CRUD", now)
	if err != nil {
		t.Fatalf("AddTask 1: %v", err)
	}
	rel2, err := sdd.AddTask(root, "decisions/001-staff.md", "Owner UI", now)
	if err != nil {
		t.Fatalf("AddTask 2: %v", err)
	}
	stem := func(rel string) string { return strings.TrimSuffix(filepath.Base(rel), ".md") }
	return root, stem(rel1), stem(rel2)
}

func TestSDDDonePrintsPendingChecklistWhileProposalIncomplete(t *testing.T) {
	root, task1, task2 := setupProposalWithTasks(t)

	var out, errBuf bytes.Buffer
	if code := runSDDDone(root, []string{task1}, &out, &errBuf); code != exitSuccess {
		t.Fatalf("runSDDDone = %d, stderr=%s", code, errBuf.String())
	}
	got := out.String()

	for _, want := range []string{
		"Proposal tasks (this PR):",
		"[x] " + task1,
		"[ ] " + task2,
		"1 task(s) still pending",
		"git push origin HEAD",
		"leave the PR a draft",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("done output missing %q\n---\n%s", want, got)
		}
	}
	// The proposal is not complete yet, so it must NOT tell the user to mark it ready.
	if strings.Contains(got, "gh pr ready") {
		t.Errorf("done output told to mark PR ready while a task is still pending:\n%s", got)
	}
}

func TestSDDDoneTellsToMarkReadyWhenProposalComplete(t *testing.T) {
	root, task1, task2 := setupProposalWithTasks(t)

	// Close the first task, then the second — the second closes the proposal.
	var sink bytes.Buffer
	if code := runSDDDone(root, []string{task1}, &sink, &sink); code != exitSuccess {
		t.Fatalf("runSDDDone task1 = %d, stderr=%s", code, sink.String())
	}

	var out, errBuf bytes.Buffer
	if code := runSDDDone(root, []string{task2}, &out, &errBuf); code != exitSuccess {
		t.Fatalf("runSDDDone task2 = %d, stderr=%s", code, errBuf.String())
	}
	got := out.String()

	for _, want := range []string{
		"[x] " + task1,
		"[x] " + task2,
		"Every task in this proposal is done",
		"gh pr ready",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("done output missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "still pending") {
		t.Errorf("done output reported pending tasks after the proposal was complete:\n%s", got)
	}
}
