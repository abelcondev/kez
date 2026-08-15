package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedTime(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, "2026-07-25T10:00:00Z")
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return ts
}

func TestPromoteWritesDecisionLogAndIndexAndResetsProposal(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	// Draft a real proposal in place of the empty template.
	proposal := "---\ntype: Proposal\ntitle: Magic-link auth\ndescription: Passwordless sign-in via email.\ntags: []\nstatus: in-review\n---\n\n# Proposal\n\nAdd magic-link authentication.\n\n# Context\n\nUsers dislike passwords.\n"
	if err := os.WriteFile(filepath.Join(root, "sdd", "proposal.md"), []byte(proposal), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	rel, err := Promote(root, "", fixedTime(t))
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if rel != filepath.Join("sdd", "decisions", "001-magic-link-auth.md") {
		t.Fatalf("decision path = %q", rel)
	}

	decision := readFile(t, filepath.Join(root, rel))
	for _, want := range []string{
		"type: Decision",
		"status: approved",
		"title: Magic-link auth",
		"timestamp: 2026-07-25T10:00:00Z",
		"# Decision", // relabeled from "# Proposal"
		"Add magic-link authentication.",
	} {
		if !strings.Contains(decision, want) {
			t.Errorf("decision missing %q\n---\n%s", want, decision)
		}
	}
	if strings.Contains(decision, "# Proposal") {
		t.Errorf("heading was not relabeled:\n%s", decision)
	}

	log := readFile(t, filepath.Join(root, "sdd", "log.md"))
	if !strings.Contains(log, "- 2026-07-25 — Decision 001: Magic-link auth. Passwordless sign-in via email.") {
		t.Errorf("log missing decision line:\n%s", log)
	}

	index := readFile(t, filepath.Join(root, "sdd", "index.md"))
	if !strings.Contains(index, "- [001 — Magic-link auth](decisions/001-magic-link-auth.md) — Passwordless sign-in via email.") {
		t.Errorf("index missing decision bullet:\n%s", index)
	}
	if !strings.Contains(index, "Everything here is written in English") {
		t.Errorf("index footer was clobbered:\n%s", index)
	}

	// Proposal reset to the empty template.
	reset := readFile(t, filepath.Join(root, "sdd", "proposal.md"))
	if !strings.Contains(reset, "status: empty") {
		t.Errorf("proposal was not reset:\n%s", reset)
	}
}

func TestPromoteRefusesEmptyProposal(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if _, err := Promote(root, "", fixedTime(t)); err == nil {
		t.Fatal("Promote on empty proposal should error")
	}
	// But an explicit --title lets it through.
	rel, err := Promote(root, "Explicit decision", fixedTime(t))
	if err != nil {
		t.Fatalf("Promote with title override: %v", err)
	}
	if rel != filepath.Join("sdd", "decisions", "001-explicit-decision.md") {
		t.Errorf("path = %q", rel)
	}
}

func TestPromoteNumbersSequentially(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	for i, title := range []string{"First", "Second", "Third"} {
		if _, err := Promote(root, title, fixedTime(t)); err != nil {
			t.Fatalf("Promote %d: %v", i, err)
		}
	}
	for _, want := range []string{"001-first.md", "002-second.md", "003-third.md"} {
		if _, err := os.Stat(filepath.Join(root, "sdd", "decisions", want)); err != nil {
			t.Errorf("expected %s: %v", want, err)
		}
	}
}

func TestAddTaskLinksDecisionAndIsPending(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	rel, err := AddTask(root, "decisions/003-auth.md", "Signup UI", fixedTime(t))
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if rel != filepath.Join("sdd", "tasks", "001-signup-ui.md") {
		t.Fatalf("task path = %q", rel)
	}
	task := readFile(t, filepath.Join(root, rel))
	for _, want := range []string{
		"type: Task",
		"status: pending",
		"decision: decisions/003-auth.md",
		"title: Signup UI",
		"```gherkin",
	} {
		if !strings.Contains(task, want) {
			t.Errorf("task missing %q\n---\n%s", want, task)
		}
	}

	st, err := ReadStatus(root)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if len(st.Tasks) != 1 || st.Tasks[0].Status != "pending" {
		t.Errorf("status tasks = %+v", st.Tasks)
	}
}

func TestPromotePreservesUITag(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	proposal := "---\ntype: Proposal\ntitle: Caja screen\ndescription: The cashier POS screen.\ntags: [ui]\nstatus: in-review\n---\n\n# Proposal\n\nBuild the caja screen.\n"
	if err := os.WriteFile(filepath.Join(root, "sdd", "proposal.md"), []byte(proposal), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	rel, err := Promote(root, "", fixedTime(t))
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if got := readFile(t, filepath.Join(root, rel)); !strings.Contains(got, "tags: [ui]") {
		t.Errorf("decision must carry the ui tag so the design gate fires:\n%s", got)
	}
}

func TestCompleteTaskMarksDoneAndLogs(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if _, err := AddTask(root, "decisions/001-arch.md", "Route guard", fixedTime(t)); err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	rel, err := CompleteTask(root, "001-route-guard", fixedTime(t))
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if got := readFile(t, filepath.Join(root, rel)); !strings.Contains(got, "status: done") {
		t.Errorf("task not marked done:\n%s", got)
	}
	if got := readFile(t, filepath.Join(root, "sdd", "log.md")); !strings.Contains(got, "Task 001-route-guard (Route guard) done.") {
		t.Errorf("log missing completion line:\n%s", got)
	}
	// Idempotency of the loop: a completed task drops out of pending.
	st, _ := ReadStatus(root)
	if len(st.Tasks) != 1 || st.Tasks[0].Status != "done" {
		t.Errorf("status = %+v, want one done task", st.Tasks)
	}
}

func TestAddAndApproveDesign(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	rel, err := AddDesign(root, "decisions/002-caja-ui.md", "Caja screen", fixedTime(t))
	if err != nil {
		t.Fatalf("AddDesign: %v", err)
	}
	if rel != filepath.Join("sdd", "designs", "001-caja-screen.md") {
		t.Fatalf("design path = %q", rel)
	}
	design := readFile(t, filepath.Join(root, rel))
	for _, want := range []string{"type: Design", "status: in-review", "decision: decisions/002-caja-ui.md", "## References", "## Wireframe", "## Composition"} {
		if !strings.Contains(design, want) {
			t.Errorf("design missing %q\n---\n%s", want, design)
		}
	}

	if _, err := ApproveDesign(root, "001-caja-screen", fixedTime(t)); err != nil {
		t.Fatalf("ApproveDesign: %v", err)
	}
	if got := readFile(t, filepath.Join(root, rel)); !strings.Contains(got, "status: approved") {
		t.Errorf("design not approved:\n%s", got)
	}
	if got := readFile(t, filepath.Join(root, "sdd", "log.md")); !strings.Contains(got, "Design 001-caja-screen (Caja screen) approved.") {
		t.Errorf("log missing design approval:\n%s", got)
	}
}

func TestCompleteTaskMissingErrors(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if _, err := CompleteTask(root, "999-nope", fixedTime(t)); err == nil {
		t.Fatal("CompleteTask on a missing task should error")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Magic-link auth":        "magic-link-auth",
		"  Trim  Me  ":           "trim-me",
		"Weird!!!Chars@@@Here":   "weirdcharshere",
		"003-already/slugged.md": "003-already-slugged-md",
		"":                       "untitled",
		"---":                    "untitled",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
