package cli

import (
	"bytes"
	"testing"

	"github.com/abelcondev/kez/internal/sdd"
)

func TestSDDTaskTierOverrideAndInference(t *testing.T) {
	root := t.TempDir()
	if _, _, err := sdd.Scaffold(root); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	// Explicit override.
	var out, errBuf bytes.Buffer
	if code := runSDDTask(root, []string{"decisions/001-x.md", "Tweak list layout", "--tier", "critical"}, &out, &errBuf); code != exitSuccess {
		t.Fatalf("task --tier = %d, stderr=%s", code, errBuf.String())
	}
	// Inferred from an auth keyword, no flag.
	if code := runSDDTask(root, []string{"decisions/001-x.md", "PIN sign-in issuing token"}, &out, &errBuf); code != exitSuccess {
		t.Fatalf("task infer = %d, stderr=%s", code, errBuf.String())
	}

	st, err := sdd.ReadStatus(root)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	byTitle := map[string]sdd.Tier{}
	for _, task := range st.Tasks {
		byTitle[task.Title] = task.Tier
	}
	if byTitle["Tweak list layout"] != sdd.TierCritical {
		t.Errorf("override tier = %q, want critical", byTitle["Tweak list layout"])
	}
	if byTitle["PIN sign-in issuing token"] != sdd.TierCritical {
		t.Errorf("inferred tier = %q, want critical", byTitle["PIN sign-in issuing token"])
	}
}
