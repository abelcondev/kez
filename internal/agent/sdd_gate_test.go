package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/abelcondev/kez/internal/sdd"
)

// TestSDDContextApprovalGateIsFriendly pins the friendly framing of a human
// approval gate: the injected next-step block must NOT tell the user to run the
// CLI command themselves. It must ask for a plain-language approval and instruct
// the agent to run the command on the user's behalf. This guards the behavior
// that makes gates conversational ("aprobado") instead of a copy-paste chore.
func TestSDDContextApprovalGateIsFriendly(t *testing.T) {
	root := t.TempDir()
	if _, err := sdd.SeedProposal(root, "Owner auth with email + password", time.Now()); err != nil {
		t.Fatalf("SeedProposal: %v", err)
	}

	ctx := sddContext(root)
	if ctx == "" {
		t.Fatal("sddContext returned empty for a scaffolded SDD workspace")
	}

	// The gate must be presented conversationally and hand the command to the
	// agent, not the user.
	mustContain := []string{
		"human approval gate",
		"Do NOT ask them to run a command",
		"aprobado",
		"run this command yourself on their behalf",
		"kez sdd approve", // the actual command the agent runs
	}
	for _, want := range mustContain {
		if !strings.Contains(ctx, want) {
			t.Errorf("gate block missing %q\n--- block ---\n%s", want, ctx)
		}
	}

	// Regression guard: the old wording made the user the one who runs it.
	if strings.Contains(ctx, "the user reviews `sdd/proposal.md` and runs") {
		t.Error("gate block still tells the user to run the command themselves")
	}
}

// TestSDDContextNonGateStepShowsCommandPlainly ensures the friendly-gate
// rewrite did not change how ordinary (non-gate) next steps render: they still
// surface the command as a bare backticked line without the approval framing.
func TestSDDContextNonGateStepShowsCommandPlainly(t *testing.T) {
	root := t.TempDir()
	if _, _, err := sdd.Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	// No proposal in review and no decisions yet → the loop's next step is the
	// discovery proposal, a non-gate action.
	ctx := sddContext(root)
	if ctx == "" {
		t.Fatal("sddContext returned empty for a scaffolded SDD workspace")
	}
	if strings.Contains(ctx, "human approval gate") {
		t.Errorf("non-gate step should not render approval-gate framing\n--- block ---\n%s", ctx)
	}
	if !strings.Contains(ctx, "kez sdd propose") {
		t.Errorf("non-gate step should surface its command plainly\n--- block ---\n%s", ctx)
	}
}
