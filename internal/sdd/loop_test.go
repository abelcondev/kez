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
	if action.Skill != "sdd-discovery" {
		t.Fatalf("discovery phase skill = %q, want sdd-discovery", action.Skill)
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
	if action.Skill != "sdd-implement" {
		t.Fatalf("implement phase skill = %q, want sdd-implement", action.Skill)
	}
}

func TestNextMechanicalStepsNameNoSkill(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")
	writeArtifact(t, root, "tasks/002-owner-auth.md", "Task", "Owner auth", "pending")

	// Branching off a protected branch is mechanical git — no phase skill.
	if action := mustState(t, root, "main").Next(); action.Skill != "" {
		t.Fatalf("branch step skill = %q, want none", action.Skill)
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

func writeRaw(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, "sdd", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestNextBranchIsPerProposal(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeRaw(t, root, "decisions/002-catalog.md", "---\ntype: Decision\ntitle: Catalog\ntags: []\nstatus: approved\n---\n")
	writeRaw(t, root, "tasks/005-add-item.md", "---\ntype: Task\ntitle: Add item\ndecision: decisions/002-catalog.md\nstatus: pending\n---\n")

	action := mustState(t, root, "main").Next()
	if action.Command != "git checkout -b feat/002-catalog" {
		t.Fatalf("branch should be per proposal (feat/002-catalog), got %q", action.Command)
	}
}

func TestNextUIDecisionWithoutDesignWantsDesignGate(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeRaw(t, root, "decisions/002-caja-ui.md", "---\ntype: Decision\ntitle: Caja UI\ntags: [ui]\nstatus: approved\n---\n")
	writeRaw(t, root, "tasks/003-caja-screen.md", "---\ntype: Task\ntitle: Caja screen\ndecision: decisions/002-caja-ui.md\nstatus: pending\n---\n")

	// Even on a feature branch, a UI task with no approved design must design first.
	action := mustState(t, root, "feat/002-caja-ui").Next()
	if !strings.HasPrefix(action.Command, "kez sdd design decisions/002-caja-ui.md") {
		t.Fatalf("want a design command, got %q", action.Command)
	}
}

func TestNextDesignInReviewIsGate(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeRaw(t, root, "decisions/002-caja-ui.md", "---\ntype: Decision\ntitle: Caja UI\ntags: [ui]\nstatus: approved\n---\n")
	writeRaw(t, root, "tasks/003-caja-screen.md", "---\ntype: Task\ntitle: Caja screen\ndecision: decisions/002-caja-ui.md\nstatus: pending\n---\n")
	writeRaw(t, root, "designs/001-caja.md", "---\ntype: Design\ntitle: Caja\ndecision: decisions/002-caja-ui.md\nstatus: in-review\n---\n")

	action := mustState(t, root, "feat/002-caja-ui").Next()
	if !action.Gate || !strings.HasPrefix(action.Command, "kez sdd approve-design designs/001-caja") {
		t.Fatalf("design in review must be an approve-design gate, got %+v", action)
	}
}

func TestNextApprovedDesignUnblocksUITask(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeRaw(t, root, "decisions/002-caja-ui.md", "---\ntype: Decision\ntitle: Caja UI\ntags: [ui]\nstatus: approved\n---\n")
	writeRaw(t, root, "tasks/003-caja-screen.md", "---\ntype: Task\ntitle: Caja screen\ndecision: decisions/002-caja-ui.md\nstatus: pending\n---\n")
	writeRaw(t, root, "designs/001-caja.md", "---\ntype: Design\ntitle: Caja\ndecision: decisions/002-caja-ui.md\nstatus: approved\n---\n")

	// On main: no more design gate, straight to the per-proposal branch.
	if action := mustState(t, root, "main").Next(); action.Command != "git checkout -b feat/002-caja-ui" {
		t.Fatalf("approved design should unblock branching, got %q", action.Command)
	}
	// On the feature branch: implement (no command).
	if action := mustState(t, root, "feat/002-caja-ui").Next(); action.Command != "" {
		t.Fatalf("approved design + feature branch should implement, got %q", action.Command)
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
