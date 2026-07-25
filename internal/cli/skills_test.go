package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abelcondev/kez/internal/tui"
)

func writeSkillFixture(t *testing.T, dir string, name string, content string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", skillDir, err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func isolateCLIAgentsHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func TestRunSkillsListText(t *testing.T) {
	isolateCLIAgentsHome(t)
	dir := t.TempDir()
	writeSkillFixture(t, dir, "confirmation-policy", "---\nname: confirmation-policy\ndescription: Ask before risky actions.\n---\nbody")

	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skills", "list"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return dir },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"confirmation-policy", "Ask before risky actions."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunSkillsListWarnsOnDuplicateNames(t *testing.T) {
	isolateCLIAgentsHome(t)
	dir := t.TempDir()
	// Two directories declare the same frontmatter name; List keeps one and the
	// other is shadowed. The command must warn instead of silently dropping it.
	writeSkillFixture(t, dir, "alpha", "---\nname: shared\ndescription: First.\n---\nbody")
	writeSkillFixture(t, dir, "beta", "---\nname: shared\ndescription: Second.\n---\nbody")

	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skills", "list"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return dir },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), `duplicate skill "shared"`) {
		t.Fatalf("expected a duplicate-skill warning on stderr, got: %q", stderr.String())
	}
}

func TestRunSkillsDefaultsToList(t *testing.T) {
	isolateCLIAgentsHome(t)
	dir := t.TempDir()
	writeSkillFixture(t, dir, "demo", "body")

	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skills"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return dir },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "demo") {
		t.Fatalf("output missing demo:\n%s", stdout.String())
	}
}

func TestRunSkillsListJSON(t *testing.T) {
	isolateCLIAgentsHome(t)
	dir := t.TempDir()
	writeSkillFixture(t, dir, "demo", "---\nname: demo\ndescription: a demo\n---\nbody")

	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skills", "list", "--json"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return dir },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	var payload struct {
		Skills []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Path        string `json:"path"`
			Content     string `json:"content"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	// The disk "demo" skill plus the binary's built-in skills (e.g. new-app), so
	// locate the disk skill by name rather than assuming it is the only entry.
	var demo *struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}
	sawBuiltin := false
	for i := range payload.Skills {
		switch payload.Skills[i].Name {
		case "demo":
			demo = &payload.Skills[i]
		case "new-app":
			sawBuiltin = true
		}
	}
	if demo == nil {
		t.Fatalf("expected the demo skill in the list, got %#v", payload.Skills)
	}
	if demo.Description != "a demo" {
		t.Fatalf("unexpected demo skill: %#v", demo)
	}
	if demo.Path == "" {
		t.Fatalf("path should be present")
	}
	if !sawBuiltin {
		t.Fatalf("expected the built-in new-app skill in the list, got %#v", payload.Skills)
	}
}

// TestRunSkillsEmptyDirStillListsBuiltins verifies the whole point of built-in
// skills: with no disk skills installed at all, the binary's built-ins (e.g.
// new-app) are still listed, so the workflow is always available.
func TestRunSkillsEmptyDirStillListsBuiltins(t *testing.T) {
	isolateCLIAgentsHome(t)
	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skills", "list"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return filepath.Join(t.TempDir(), "missing") },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "new-app") {
		t.Fatalf("built-in new-app should be listed even with no disk skills, got:\n%s", stdout.String())
	}
}

func TestRunSkillsListIncludesAgentsOnlySkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	agents := filepath.Join(home, ".agents", "skills")
	writeSkillFixture(t, agents, "agents-only", "---\nname: agents-only\ndescription: Shared multi-agent skill.\n---\nbody")

	primary := t.TempDir()
	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skills", "list"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return primary },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "agents-only") {
		t.Fatalf("list should include agents-only skill:\n%s", out)
	}
	if !strings.Contains(out, agents) {
		t.Fatalf("list should show agents path:\n%s", out)
	}
}

func TestRunSkillsListPrimaryShadowsAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	agents := filepath.Join(home, ".agents", "skills")
	primary := t.TempDir()
	writeSkillFixture(t, primary, "shared", "---\nname: shared\ndescription: From primary.\n---\nbody")
	writeSkillFixture(t, agents, "shared", "---\nname: shared\ndescription: From agents.\n---\nbody")

	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skills", "list"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return primary },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "From primary") {
		t.Fatalf("primary should win list description:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), primary) {
		t.Fatalf("primary path should be shown:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), `duplicate skill "shared"`) {
		t.Fatalf("expected cross-root duplicate warning, got: %q", stderr.String())
	}
}

func TestRunSkillInfoResolvesAgentsOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	agents := filepath.Join(home, ".agents", "skills")
	writeSkillFixture(t, agents, "agents-info", "---\nname: agents-info\ndescription: Agents info.\n---\nbody")

	primary := t.TempDir()
	var stdout, stderr bytes.Buffer
	exit := runWithDeps([]string{"skill", "info", "agents-info"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return primary },
	})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "agents-info") || !strings.Contains(out, "Agents info.") {
		t.Fatalf("info missing agents skill:\n%s", out)
	}
	if strings.Contains(out, "source:") || strings.Contains(out, "hash:") {
		t.Fatalf("agents-only info must not invent lock metadata:\n%s", out)
	}
	if !strings.Contains(out, agents) {
		t.Fatalf("info should show agents path:\n%s", out)
	}
}

func TestRunSkillsDoesNotLaunchTUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	launchCalled := false
	_ = runWithDeps([]string{"skills"}, &stdout, &stderr, appDeps{
		skillsDir: func() string { return t.TempDir() },
		runTUI: func(ctx context.Context, options tui.Options) int {
			launchCalled = true
			return 0
		},
	})
	if launchCalled {
		t.Fatalf("TUI launcher should not be called for skills command")
	}
}
