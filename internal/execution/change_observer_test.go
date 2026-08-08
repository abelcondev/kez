package execution

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestChangeObserverReportsCreateModifyAndDelete(t *testing.T) {
	root := t.TempDir()
	modified := filepath.Join(root, "modified.txt")
	deleted := filepath.Join(root, "deleted.txt")
	if err := os.WriteFile(modified, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deleted, []byte("delete"), 0o644); err != nil {
		t.Fatal(err)
	}
	observer := NewChangeObserver(root)
	if err := os.WriteFile(modified, []byte("after!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(deleted); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "created.ts"), []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []Change{
		{Path: "deleted.txt", Kind: ChangeDeleted},
		{Path: "modified.txt", Kind: ChangeModified},
		{Path: "src/created.ts", Kind: ChangeCreated},
	}
	if got := observer.Changes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Changes() = %#v, want %#v", got, want)
	}
}

func TestChangeObserverSkipsControlMetadataAndSummarizesGeneratedTrees(t *testing.T) {
	root := t.TempDir()
	observer := NewChangeObserver(root)
	for _, path := range []string{
		filepath.Join(root, ".kez", "config.json"),
		filepath.Join(root, ".agents", "skills", "x"),
		filepath.Join(root, ".git", "config"),
		filepath.Join(root, "node_modules", "pkg", "index.js"),
		filepath.Join(root, "dist", "bundle.js"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("generated"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := []Change{
		{Path: "dist/", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: "node_modules/", Kind: ChangeCreated, Aggregated: true, Count: 1},
	}
	if got := observer.Changes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Changes() = %#v, want generated summaries without control metadata %#v", got, want)
	}
}

// TestChangeObserverAggregatesDevServerAndBuildTrees pins the fix for the TUI
// freeze: a running dev server (SvelteKit/Next/Vite/etc.) continuously rewrites
// files under its generated dirs, which must be reported as one aggregated tree
// change each — never enumerated file-by-file — so they can't flood the tool
// result and the FILES sidebar. A real source file alongside them still surfaces.
func TestChangeObserverAggregatesDevServerAndBuildTrees(t *testing.T) {
	root := t.TempDir()
	observer := NewChangeObserver(root)
	for _, path := range []string{
		filepath.Join(root, ".svelte-kit", "generated", "client.js"),
		filepath.Join(root, ".next", "server", "page.js"),
		filepath.Join(root, ".vite", "deps", "chunk.js"),
		filepath.Join(root, "build", "index.html"),
		filepath.Join(root, "target", "debug", "app"),
		filepath.Join(root, "__pycache__", "mod.pyc"),
		filepath.Join(root, ".wrangler", "state", "v3", "d1", "db.sqlite"),
		filepath.Join(root, "src", "app.ts"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("generated"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := []Change{
		{Path: ".next/", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: ".svelte-kit/", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: ".vite/", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: ".wrangler/", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: "__pycache__/", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: "build/", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: "src/app.ts", Kind: ChangeCreated},
		{Path: "target/", Kind: ChangeCreated, Aggregated: true, Count: 1},
	}
	if got := observer.Changes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Changes() = %#v, want dev-server/build trees aggregated %#v", got, want)
	}
}

// TestBoundChangesPassThroughUnderBudget: at or below the cap the change list is
// returned verbatim, individual entries intact.
func TestBoundChangesPassThroughUnderBudget(t *testing.T) {
	changes := []Change{
		{Path: "a.ts", Kind: ChangeModified},
		{Path: "b.ts", Kind: ChangeCreated},
	}
	if got := boundChanges(changes); !reflect.DeepEqual(got, changes) {
		t.Fatalf("boundChanges under budget = %#v, want unchanged %#v", got, changes)
	}
}

// TestBoundChangesAggregatesOverBudget: past the cap the individual changes
// collapse into one aggregated entry per top-level directory (root files under
// "./"), so the reported count tracks touched directories, not touched files.
func TestBoundChangesAggregatesOverBudget(t *testing.T) {
	var changes []Change
	for i := 0; i < maxReportedIndividualChanges+50; i++ {
		changes = append(changes, Change{Path: "app/f" + strconv.Itoa(i) + ".ts", Kind: ChangeModified})
	}
	changes = append(changes, Change{Path: "root.txt", Kind: ChangeCreated})
	// A pre-existing aggregated tree summary must survive untouched.
	changes = append(changes, Change{Path: "node_modules/", Kind: ChangeCreated, Aggregated: true, Count: 3})
	want := []Change{
		{Path: "./", Kind: ChangeCreated, Aggregated: true, Count: 1},
		{Path: "app/", Kind: ChangeModified, Aggregated: true, Count: maxReportedIndividualChanges + 50},
		{Path: "node_modules/", Kind: ChangeCreated, Aggregated: true, Count: 3},
	}
	if got := boundChanges(changes); !reflect.DeepEqual(got, want) {
		t.Fatalf("boundChanges over budget = %#v, want per-top-dir aggregates %#v", got, want)
	}
}
