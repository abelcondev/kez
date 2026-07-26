package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxSeedTitleLen bounds the title derived from a free-text description so the
// frontmatter title (and the resulting decision slug) stays reasonable.
const maxSeedTitleLen = 72

// SeedProposal writes sdd/proposal.md as an in-review skeleton seeded from a
// natural-language description, without invoking any model. It is the graceful
// fallback for `kez sdd propose` when no provider is configured: the human (or a
// later agent turn) fills in the Context/Acceptance sections, then approves.
//
// It runs Scaffold first so a missing sdd/ is created, and refuses to clobber a
// proposal that is already in review — surfacing that instead of overwriting.
// Returns the proposal path relative to root.
func SeedProposal(root, description string, now time.Time) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return "", fmt.Errorf("a proposal description is required")
	}
	if _, _, err := Scaffold(root); err != nil {
		return "", fmt.Errorf("scaffold sdd/: %w", err)
	}

	base := filepath.Join(root, DirName)
	proposalPath := filepath.Join(base, "proposal.md")
	if raw, err := os.ReadFile(proposalPath); err == nil {
		fm, _ := splitFrontmatter(string(raw))
		if status := strings.TrimSpace(fm["status"]); status != "" && status != "empty" {
			return "", fmt.Errorf("sdd/proposal.md is already in review (%q); approve or clear it before drafting a new one", status)
		}
	}

	title := deriveTitle(description)
	var doc strings.Builder
	doc.WriteString("---\n")
	doc.WriteString("type: Proposal\n")
	doc.WriteString("title: " + title + "\n")
	doc.WriteString("description: \n")
	doc.WriteString("tags: []\n")
	doc.WriteString("status: in-review\n")
	doc.WriteString("timestamp: " + now.UTC().Format(time.RFC3339) + "\n")
	doc.WriteString("---\n\n")
	doc.WriteString("# Proposal\n\n")
	doc.WriteString(description + "\n\n")
	doc.WriteString("_Seeded skeleton — expand the what/why above, then fill Context and Acceptance below._\n\n")
	doc.WriteString("# Context\n\n")
	doc.WriteString("The forces, constraints, and alternatives considered.\n\n")
	doc.WriteString("# Acceptance\n\n")
	doc.WriteString("What must be true for this proposal to be approved and promoted to `decisions/NNN-name.md`.\n")

	relPath := filepath.Join(DirName, "proposal.md")
	if err := os.WriteFile(proposalPath, []byte(doc.String()), 0o644); err != nil {
		return "", err
	}
	return relPath, nil
}

// deriveTitle turns the first sentence/line of a description into a concise
// frontmatter title, trimmed to maxSeedTitleLen on a word boundary.
func deriveTitle(description string) string {
	first := description
	if idx := strings.IndexAny(first, ".\n"); idx > 0 {
		first = first[:idx]
	}
	first = strings.TrimSpace(strings.Join(strings.Fields(first), " "))
	if first == "" {
		return "Untitled proposal"
	}
	if len(first) <= maxSeedTitleLen {
		return first
	}
	clipped := first[:maxSeedTitleLen]
	if sp := strings.LastIndex(clipped, " "); sp > 0 {
		clipped = clipped[:sp]
	}
	return strings.TrimSpace(clipped) + "…"
}
