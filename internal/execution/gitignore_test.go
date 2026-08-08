package execution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatcherPatterns(t *testing.T) {
	matcher := &ignoreMatcher{}
	for _, line := range []string{
		"node_modules/", // unanchored dir, matches at any depth
		"*.log",         // unanchored file glob
		"/local-data",   // root-anchored, dir or file
		"dist-custom",   // unanchored name
		"!keep.log",     // re-include (last match wins)
	} {
		rule, ok := parseIgnoreLine(line)
		if !ok {
			t.Fatalf("expected %q to parse", line)
		}
		matcher.rules = append(matcher.rules, rule)
	}
	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"node_modules", true, true},
		{"packages/web/node_modules", true, true},
		{"node_modules", false, false}, // dir-only pattern must not match a file
		{"app.log", false, true},
		{"logs/app.log", false, true},
		{"keep.log", false, false}, // negation wins over *.log
		{"local-data", true, true},
		{"local-data/db.sqlite", false, true}, // under an anchored ignored path
		{"src/local-data", true, false},       // anchored: only at root
		{"dist-custom", true, true},
		{"src/app.ts", false, false},
	}
	for _, tc := range cases {
		if got := matcher.ignored(tc.rel, tc.isDir); got != tc.want {
			t.Errorf("ignored(%q, dir=%v) = %v, want %v", tc.rel, tc.isDir, got, tc.want)
		}
	}
}

func TestLoadIgnoreMatcherReadsGitignoreAndExclude(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte("secret/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	matcher := loadIgnoreMatcher(root)
	if matcher == nil {
		t.Fatal("expected a matcher from .gitignore + exclude")
	}
	if !matcher.ignored("app.log", false) {
		t.Error(".gitignore rule *.log not applied")
	}
	if !matcher.ignored("secret", true) {
		t.Error(".git/info/exclude rule secret/ not applied")
	}
}

func TestLoadIgnoreMatcherNilWithoutRules(t *testing.T) {
	if got := loadIgnoreMatcher(t.TempDir()); got != nil {
		t.Errorf("expected nil matcher when no ignore files exist, got %#v", got)
	}
	// A nil matcher must be safe to query.
	var nilMatcher *ignoreMatcher
	if nilMatcher.ignored("anything", true) {
		t.Error("nil matcher must never report a path ignored")
	}
}
