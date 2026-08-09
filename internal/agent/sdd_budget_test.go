package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/abelcondev/kez/internal/sdd"
	"github.com/abelcondev/kez/internal/tools"
	"github.com/abelcondev/kez/internal/zeroruntime"
)

// writeSDDArtifact drops a minimal OKF artifact (frontmatter + heading) under
// <root>/sdd/<rel>, mirroring the sdd package's own test helper so these tests
// can position the loop without depending on unexported helpers.
func writeSDDArtifact(t *testing.T, root, rel, typ, title, status string) {
	t.Helper()
	body := "---\ntype: " + typ + "\ntitle: " + title + "\nstatus: " + status + "\n---\n\n# " + typ + "\n"
	path := filepath.Join(root, "sdd", filepath.FromSlash(rel))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write artifact %s: %v", rel, err)
	}
}

func TestSDDImplementPhaseDetectsPendingTask(t *testing.T) {
	root := t.TempDir()
	if _, _, err := sdd.Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	// A recorded decision plus a pending task positions the loop at the implement
	// phase (Next().Skill == sdd.SkillImplement) — the long TDD + review cycle the
	// budget bump targets.
	writeSDDArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")
	writeSDDArtifact(t, root, "tasks/002-owner-auth.md", "Task", "Owner auth", "pending")

	if !sddImplementPhase(root) {
		t.Fatalf("sddImplementPhase = false, want true for a workspace with a pending task")
	}
}

func TestSDDImplementPhaseFalseWhenNoPendingTask(t *testing.T) {
	root := t.TempDir()
	if _, _, err := sdd.Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	// A decision but no pending task routes to the stack phase, not implement, so
	// the budget must not be widened.
	writeSDDArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")

	if sddImplementPhase(root) {
		t.Fatalf("sddImplementPhase = true, want false when no task is pending")
	}
}

func TestRunWidensBudgetForSDDImplementPhase(t *testing.T) {
	root := t.TempDir()
	if _, _, err := sdd.Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeSDDArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")
	writeSDDArtifact(t, root, "tasks/002-owner-auth.md", "Task", "Owner auth", "pending")
	writeAgentTestFile(t, filepath.Join(root, "notes.txt"), "alpha")

	registry := tools.NewRegistry()
	registry.Register(tools.NewScopedReadFileTool(root, nil))
	readTurn := func(id string) []zeroruntime.StreamEvent {
		return []zeroruntime.StreamEvent{
			{Type: zeroruntime.StreamEventToolCallStart, ToolCallID: id, ToolName: "read_file"},
			{Type: zeroruntime.StreamEventToolCallDelta, ToolCallID: id, ArgumentsFragment: `{"path":"notes.txt"}`},
			{Type: zeroruntime.StreamEventToolCallEnd, ToolCallID: id},
			{Type: zeroruntime.StreamEventDone},
		}
	}
	// Two tool turns then a text answer: it needs 3 turns to finish. MaxTurns=1
	// alone would cut it off after the first, but the implement-phase budget
	// raises the ceiling to 3 so it completes naturally.
	provider := &mockProvider{
		turns: [][]zeroruntime.StreamEvent{
			readTurn("call-1"),
			readTurn("call-2"),
			{
				{Type: zeroruntime.StreamEventText, Content: "done implementing."},
				{Type: zeroruntime.StreamEventDone},
			},
		},
	}

	result, err := Run(context.Background(), "implement 002", provider, Options{
		Registry:               registry,
		Cwd:                    root,
		MaxTurns:               1,
		SDDImplementTurnBudget: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalAnswer != "done implementing." {
		t.Fatalf("expected the run to finish naturally within the widened budget, got %q", result.FinalAnswer)
	}
	if result.Turns != 3 {
		t.Fatalf("expected 3 turns under the widened budget, got %d", result.Turns)
	}
}

func TestRunIgnoresSDDBudgetOutsideImplementPhase(t *testing.T) {
	root := t.TempDir()
	if _, _, err := sdd.Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	// A decision but no pending task: the stack phase, not implement — so the
	// budget must be ignored and MaxTurns=1 must still cut the run off.
	writeSDDArtifact(t, root, "decisions/001-architecture.md", "Decision", "Architecture", "approved")
	writeAgentTestFile(t, filepath.Join(root, "notes.txt"), "alpha")

	registry := tools.NewRegistry()
	registry.Register(tools.NewScopedReadFileTool(root, nil))
	provider := &mockProvider{
		turns: [][]zeroruntime.StreamEvent{
			{
				{Type: zeroruntime.StreamEventToolCallStart, ToolCallID: "call-1", ToolName: "read_file"},
				{Type: zeroruntime.StreamEventToolCallDelta, ToolCallID: "call-1", ArgumentsFragment: `{"path":"notes.txt"}`},
				{Type: zeroruntime.StreamEventToolCallEnd, ToolCallID: "call-1"},
				{Type: zeroruntime.StreamEventDone},
			},
			{
				// Only ever reached as the post-max-turns finalization request,
				// never as a second loop turn.
				{Type: zeroruntime.StreamEventText, Content: "forced summary."},
				{Type: zeroruntime.StreamEventDone},
			},
		},
	}

	result, err := Run(context.Background(), "propose stack", provider, Options{
		Registry:               registry,
		Cwd:                    root,
		MaxTurns:               1,
		SDDImplementTurnBudget: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Turns != 1 {
		t.Fatalf("expected the budget to be ignored outside the implement phase (1 turn), got %d", result.Turns)
	}
}

func TestSDDImplementPhaseFalseWithoutKnowledgeBase(t *testing.T) {
	// No sdd/ base (a plain workspace) and an empty cwd both report false so the
	// bump is a no-op for every non-SDD run.
	if sddImplementPhase(t.TempDir()) {
		t.Fatalf("sddImplementPhase = true, want false for a workspace with no sdd/ base")
	}
	if sddImplementPhase("") {
		t.Fatalf("sddImplementPhase(\"\") = true, want false")
	}
}
