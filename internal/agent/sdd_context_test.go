package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDDContextInjectsIndexAndPendingTasks(t *testing.T) {
	root := t.TempDir()
	tasksDir := filepath.Join(root, "sdd", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(root, "sdd", "index.md"), "# Knowledge Index\n\nRead me first.\n")
	writeFile(t, filepath.Join(tasksDir, "001-done.md"), "---\ntype: Task\ntitle: Done one\nstatus: done\n---\n")
	writeFile(t, filepath.Join(tasksDir, "002-open.md"), "---\ntype: Task\ntitle: Open one\nstatus: pending\n---\n")

	out := sddContext(root)
	if out == "" {
		t.Fatal("sddContext returned empty, want an SDD section")
	}
	for _, want := range []string{
		"Spec-Driven Development (OKF)",
		"Read me first.",
		"Pending tasks",
		"002-open — Open one",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "001-done") {
		t.Errorf("completed task must not appear in pending list:\n%s", out)
	}
}

func TestSDDContextAbsentIsEmpty(t *testing.T) {
	if out := sddContext(t.TempDir()); out != "" {
		t.Errorf("sddContext with no sdd/ = %q, want empty", out)
	}
	if out := sddContext(""); out != "" {
		t.Errorf("sddContext(\"\") = %q, want empty", out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
