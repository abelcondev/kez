package sdd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldCreatesOKFBaseAndIsIdempotent(t *testing.T) {
	root := t.TempDir()

	created, skipped, err := Scaffold(root)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("first scaffold skipped %v, want none", skipped)
	}

	want := []string{
		filepath.Join("sdd", "context.md"),
		filepath.Join("sdd", "decisions", "_template.md"),
		filepath.Join("sdd", "designs", "_template.md"),
		filepath.Join("sdd", "index.md"),
		filepath.Join("sdd", "log.md"),
		filepath.Join("sdd", "proposal.md"),
		filepath.Join("sdd", "tasks", "_template.md"),
	}
	if len(created) != len(want) {
		t.Fatalf("created %v, want %v", created, want)
	}
	for i, w := range want {
		if created[i] != w {
			t.Errorf("created[%d] = %q, want %q", i, created[i], w)
		}
		if _, err := os.Stat(filepath.Join(root, w)); err != nil {
			t.Errorf("expected file %s on disk: %v", w, err)
		}
	}

	// Re-running must not overwrite; every file is kept, none created.
	created2, skipped2, err := Scaffold(root)
	if err != nil {
		t.Fatalf("second Scaffold: %v", err)
	}
	if len(created2) != 0 {
		t.Errorf("second scaffold created %v, want none", created2)
	}
	if len(skipped2) != len(want) {
		t.Errorf("second scaffold skipped %d, want %d", len(skipped2), len(want))
	}
}

func TestScaffoldPreservesUserEdits(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	logPath := filepath.Join(root, "sdd", "log.md")
	custom := []byte("# Log\n\n- 2026-07-25 — Decision 001 approved.\n")
	if err := os.WriteFile(logPath, custom, 0o644); err != nil {
		t.Fatalf("write custom log: %v", err)
	}
	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("re-Scaffold: %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(got) != string(custom) {
		t.Errorf("log.md was overwritten:\n got: %q\nwant: %q", got, custom)
	}
}

func TestReadStatus(t *testing.T) {
	root := t.TempDir()

	// Absent knowledge base is not an error.
	st, err := ReadStatus(root)
	if err != nil {
		t.Fatalf("ReadStatus (absent): %v", err)
	}
	if st.Present {
		t.Fatalf("Present = true for missing sdd/, want false")
	}

	if _, _, err := Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	// Templates (_template.md) must be excluded from counts.
	writeArtifact(t, root, "decisions/001-auth.md", "Decision", "Auth", "approved")
	writeArtifact(t, root, "tasks/001-auth-signup-ui.md", "Task", "Signup UI", "done")
	writeArtifact(t, root, "tasks/001-auth-guard.md", "Task", "Route guard", "pending")

	st, err = ReadStatus(root)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if !st.Present {
		t.Fatal("Present = false, want true")
	}
	if st.Decisions != 1 {
		t.Errorf("Decisions = %d, want 1 (templates excluded)", st.Decisions)
	}
	if len(st.Tasks) != 2 {
		t.Fatalf("Tasks = %d, want 2 (templates excluded)", len(st.Tasks))
	}
	// Sorted by name: guard before signup-ui.
	if st.Tasks[0].Name != "001-auth-guard" || st.Tasks[0].Status != "pending" {
		t.Errorf("Tasks[0] = %+v, want name=001-auth-guard status=pending", st.Tasks[0])
	}
	if st.Tasks[1].Title != "Signup UI" || st.Tasks[1].Status != "done" {
		t.Errorf("Tasks[1] = %+v, want title=Signup UI status=done", st.Tasks[1])
	}
}

func writeArtifact(t *testing.T, root, rel, typ, title, status string) {
	t.Helper()
	body := "---\ntype: " + typ + "\ntitle: " + title + "\nstatus: " + status + "\n---\n\n# " + typ + "\n"
	path := filepath.Join(root, "sdd", filepath.FromSlash(rel))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write artifact %s: %v", rel, err)
	}
}
