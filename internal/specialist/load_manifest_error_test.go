package specialist

import (
	"strings"
	"testing"
)

// A model that invents a specialist name (e.g. "validator-runner", "file-writer")
// must get a corrective error listing the real specialists, so it self-corrects to a
// capable one instead of looping on made-up names that spawn doomed sub-agents.
func TestLoadManifestUnknownNameListsAvailable(t *testing.T) {
	executor := Executor{
		Load: func(LoadOptions) (LoadResult, error) {
			return LoadResult{Specialists: []Manifest{
				{Metadata: Metadata{Name: "worker"}},
				{Metadata: Metadata{Name: "explorer"}},
				{Metadata: Metadata{Name: "code-review"}},
			}}, nil
		},
	}

	_, err := executor.loadManifest("validator-runner")
	if err == nil {
		t.Fatal("an unknown specialist name must error")
	}
	msg := err.Error()
	for _, want := range []string{"validator-runner", "not found", "worker", "explorer", "code-review"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q (must steer the model to a real specialist)", msg, want)
		}
	}
}
