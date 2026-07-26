package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setActiveProposal(t *testing.T, root, title string) {
	t.Helper()
	doc := "---\ntype: Proposal\ntitle: " + title + "\nstatus: in-review\n---\n\n# Proposal\n\nx\n"
	if err := os.WriteFile(filepath.Join(root, DirName, "proposal.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
}

func TestNextMissingKnowledgeBase(t *testing.T) {
	root := t.TempDir()
	state, err := ReadLoopState(root, "main")
	if err != nil {
		t.Fatalf("ReadLoopState: %v", err)
	}
	if state.Present {
		t.Fatalf("expected Present=false")
	}
	if got := state.Next().Command; got != "kez sdd init" {
		t.Fatalf("next command = %q, want kez sdd init", got)
	}
}

func TestNextActiveProposalIsApproveGate(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	setActiveProposal(t, root, "Architecture")

	action := mustState(t, root, "feat/x").Next()
	if !action.Gate {
		t.Fatalf("active proposal must be a human gate")
	}
	if !strings.Contains(action.Command, `--title "Architecture"`) {
		t.Fatalf("approve command = %q", action.Command)
	}
}

func TestNextNoDecisionsProposesDiscovery(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	action := mustState(t, root, "main").Next()
	if !strings.HasPrefix(action.Command, "kez sdd propose") {
		t.Fatalf("want a propose command, got %q", action.Command)
	}
}

func TestNextPendingTaskOnProtectedBranchWantsFeatureBranch(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")
	writeArtifact(t, root, "tasks/002-owner-auth.md", "Task", "Owner auth", "pending")

	action := mustState(t, root, "main").Next()
	if action.Command != "git checkout -b feat/owner-auth" {
		t.Fatalf("branch command = %q", action.Command)
	}
}

func TestNextPendingTaskOnFeatureBranchImplements(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")
	writeArtifact(t, root, "tasks/002-owner-auth.md", "Task", "Owner auth", "pending")

	action := mustState(t, root, "feat/owner-auth").Next()
	if action.Command != "" {
		t.Fatalf("implement step should have no command, got %q", action.Command)
	}
	if !strings.Contains(action.Summary, "002-owner-auth") {
		t.Fatalf("summary = %q", action.Summary)
	}
}

func TestNextAllTasksDoneProposesTaskOrNext(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")
	writeArtifact(t, root, "decisions/002-catalog.md", "Decision", "Catalog", "approved")
	writeArtifact(t, root, "tasks/001-foundations.md", "Task", "Foundations", "done")

	action := mustState(t, root, "feat/x").Next()
	if !strings.Contains(action.Command, "decisions/002-catalog.md") {
		t.Fatalf("task command should target latest decision, got %q", action.Command)
	}
}

func TestFeatureSlugDropsSequencePrefix(t *testing.T) {
	cases := map[string]string{
		"002-owner-auth": "owner-auth",
		"010-order-flow": "order-flow",
		"no-prefix":      "no-prefix",
	}
	for in, want := range cases {
		if got := featureSlug(in); got != want {
			t.Fatalf("featureSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustState(t *testing.T, root, branch string) LoopState {
	t.Helper()
	state, err := ReadLoopState(root, branch)
	if err != nil {
		t.Fatalf("ReadLoopState: %v", err)
	}
	return state
}
