package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunBuildsMarkdownFromEnvironment(t *testing.T) {
	env := []string{
		"KEZ_PR_NUMBER=50",
		"KEZ_REVIEW_HEAD_SHA=abcdef1234567890",
		"KEZ_REVIEW_DIFF_CHECK=success",
		"KEZ_REVIEW_TEST=success",
		"KEZ_REVIEW_BUILD=success",
		"KEZ_REVIEW_SMOKE=success",
		"KEZ_CHANGED_FILES=b.go\na.go",
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(nil, env, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"<!-- zero-auto-review -->",
		"Verdict: **No blockers found**",
		"Head: `abcdef123456`",
		"Changed files (2): `a.go`, `b.go`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q:\n%s", want, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--help"}, nil, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}
	for _, want := range []string{"zero-pr-review", "KEZ_REVIEW_DIFF_CHECK", "KEZ_CHANGED_FILES"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected help to contain %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunRejectsUnknownArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--json"}, nil, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown flag "--json"`) {
		t.Fatalf("expected unknown flag error, got %q", stderr.String())
	}
}
