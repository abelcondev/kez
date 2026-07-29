package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedProposalWritesInReviewSkeleton(t *testing.T) {
	root := t.TempDir()
	rel, err := SeedProposal(root, "Build a POS for a single pollería. Offline-first cash flow.", fixedTime(t))
	if err != nil {
		t.Fatalf("SeedProposal: %v", err)
	}
	if rel != filepath.Join("sdd", "proposal.md") {
		t.Fatalf("rel = %q", rel)
	}
	body := readFile(t, filepath.Join(root, rel))
	for _, want := range []string{
		"status: in-review",
		"title: Build a POS for a single pollería",
		"# Proposal",
		"Offline-first cash flow",
		"# Acceptance",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("proposal missing %q:\n%s", want, body)
		}
	}
}

func TestSeedProposalScaffoldsWhenMissing(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedProposal(root, "First idea", fixedTime(t)); err != nil {
		t.Fatalf("SeedProposal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, DirName, "index.md")); err != nil {
		t.Fatalf("expected sdd/ scaffolded: %v", err)
	}
}

func TestSeedProposalRefusesToClobberActiveProposal(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedProposal(root, "First idea", fixedTime(t)); err != nil {
		t.Fatalf("seed first: %v", err)
	}
	_, err := SeedProposal(root, "Second idea", fixedTime(t))
	if err == nil {
		t.Fatalf("expected refusal on already in-review proposal")
	}
	if !strings.Contains(err.Error(), "already in review") {
		t.Fatalf("error = %v", err)
	}
}

func TestSeedProposalRequiresDescription(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedProposal(root, "   ", fixedTime(t)); err == nil {
		t.Fatalf("expected error for empty description")
	}
}

func TestDeriveTitleTruncatesOnWordBoundary(t *testing.T) {
	long := strings.Repeat("word ", 40)
	got := deriveTitle(long)
	if len(got) > maxSeedTitleLen+len("…") {
		t.Fatalf("title too long (%d): %q", len(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}

func TestProposalBranchName(t *testing.T) {
	cases := map[string]string{
		"SaaS multi-tenant para pollerías en Perú. Offline.": "sdd/prop-saas-multi-tenant-para-polleras-en-per",
		"Architecture with TypeScript and InstantDB":         "sdd/prop-architecture-with-typescript-and-instantdb",
		"   ":                     "sdd/prop-untitled-proposal",
		"Fix vuelto rounding bug": "sdd/prop-fix-vuelto-rounding-bug",
	}
	for desc, want := range cases {
		if got := ProposalBranchName(desc); got != want {
			t.Fatalf("ProposalBranchName(%q) = %q, want %q", desc, got, want)
		}
	}
}
