package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Promote turns the in-review proposal.md into a numbered, approved decision:
// it writes decisions/NNN-slug.md (frontmatter status=approved, stamped now),
// appends a line to log.md, adds the decision to index.md's Decisions list, and
// resets proposal.md to the empty template. titleOverride wins over the
// proposal's frontmatter title when non-empty. Returns the created decision's
// path relative to root.
func Promote(root, titleOverride string, now time.Time) (string, error) {
	base := filepath.Join(root, DirName)
	proposalPath := filepath.Join(base, "proposal.md")
	raw, err := os.ReadFile(proposalPath)
	if err != nil {
		return "", fmt.Errorf("read proposal: %w", err)
	}
	fm, body := splitFrontmatter(string(raw))

	title := strings.TrimSpace(titleOverride)
	if title == "" {
		title = strings.TrimSpace(fm["title"])
	}
	// A title override always wins; otherwise a placeholder/empty proposal title
	// means there is nothing to approve yet.
	if title == "" || title == "(no active proposal)" {
		return "", fmt.Errorf("no active proposal to approve; draft sdd/proposal.md first (or pass --title)")
	}
	description := strings.TrimSpace(fm["description"])

	decisionsDir := filepath.Join(base, "decisions")
	if err := os.MkdirAll(decisionsDir, 0o755); err != nil {
		return "", err
	}
	num, err := nextNumber(decisionsDir)
	if err != nil {
		return "", err
	}
	fileName := num + "-" + slugify(title) + ".md"
	relPath := filepath.Join(DirName, "decisions", fileName)

	var doc strings.Builder
	doc.WriteString("---\n")
	doc.WriteString("type: Decision\n")
	doc.WriteString("title: " + title + "\n")
	doc.WriteString("description: " + description + "\n")
	doc.WriteString("tags: []\n")
	doc.WriteString("status: approved\n")
	doc.WriteString("timestamp: " + now.UTC().Format(time.RFC3339) + "\n")
	doc.WriteString("supersedes: []\n")
	doc.WriteString("---\n\n")
	doc.WriteString(strings.TrimSpace(relabelFirstHeading(body, "Proposal", "Decision")) + "\n")
	if err := os.WriteFile(filepath.Join(root, relPath), []byte(doc.String()), 0o644); err != nil {
		return "", err
	}

	logLine := fmt.Sprintf("- %s — Decision %s: %s.", now.UTC().Format("2006-01-02"), num, title)
	if description != "" {
		logLine += " " + description
	}
	if err := appendLine(filepath.Join(base, "log.md"), logLine); err != nil {
		return "", err
	}

	if err := addDecisionToIndex(filepath.Join(base, "index.md"), num, title, fileName, description); err != nil {
		return "", err
	}

	// Reset proposal.md to the empty template so the next proposal starts clean.
	if tmpl, err := templatesFS.ReadFile("templates/proposal.md"); err == nil {
		_ = os.WriteFile(proposalPath, tmpl, 0o644)
	}

	return relPath, nil
}

// AddTask scaffolds tasks/NNN-slug.md, a pending task linked to decisionRef
// (e.g. "decisions/003-auth.md"). Returns the new task's path relative to root.
func AddTask(root, decisionRef, title string, now time.Time) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", fmt.Errorf("task title is required")
	}
	base := filepath.Join(root, DirName)
	tasksDir := filepath.Join(base, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return "", err
	}
	num, err := nextNumber(tasksDir)
	if err != nil {
		return "", err
	}
	fileName := num + "-" + slugify(title) + ".md"
	relPath := filepath.Join(DirName, "tasks", fileName)

	ref := strings.TrimSpace(decisionRef)
	if ref == "" {
		ref = "decisions/NNN-name.md"
	}

	var doc strings.Builder
	doc.WriteString("---\n")
	doc.WriteString("type: Task\n")
	doc.WriteString("title: " + title + "\n")
	doc.WriteString("description: \n")
	doc.WriteString("decision: " + ref + "\n")
	doc.WriteString("tags: []\n")
	doc.WriteString("status: pending\n")
	doc.WriteString("timestamp: " + now.UTC().Format(time.RFC3339) + "\n")
	doc.WriteString("---\n\n")
	doc.WriteString("# Acceptance criteria\n\n")
	doc.WriteString("```gherkin\n")
	doc.WriteString("Scenario: <describe the behavior>\n")
	doc.WriteString("  Given <precondition>\n")
	doc.WriteString("  When <action>\n")
	doc.WriteString("  Then <expected outcome>\n")
	doc.WriteString("```\n\n")
	doc.WriteString("# Dependencies\n\n")
	doc.WriteString("List any tasks or decisions this depends on.\n")
	if err := os.WriteFile(filepath.Join(root, relPath), []byte(doc.String()), 0o644); err != nil {
		return "", err
	}
	return relPath, nil
}

// nextNumber returns the next zero-padded 3-digit sequence for dir, based on the
// highest NNN- prefix among its *.md files. An empty or missing dir yields "001".
func nextNumber(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "001", nil
		}
		return "", err
	}
	highest := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		i := 0
		for i < len(name) && name[i] >= '0' && name[i] <= '9' {
			i++
		}
		if i == 0 {
			continue
		}
		n, convErr := strconv.Atoi(name[:i])
		if convErr == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("%03d", highest+1), nil
}

// slugify lowercases title and reduces it to a filename-safe [a-z0-9-] slug.
func slugify(title string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '.':
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return out
}

// splitFrontmatter separates a leading YAML frontmatter block (--- … ---) from
// the markdown body. Returns a flat key→value map of top-level scalars and the
// remaining body. A document without frontmatter yields an empty map and the
// whole content as body.
func splitFrontmatter(content string) (map[string]string, string) {
	fm := map[string]string{}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return fm, content
	}
	lines := strings.Split(normalized, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return fm, content
	}
	for i := 1; i < end; i++ {
		key, val, ok := strings.Cut(lines[i], ":")
		if !ok {
			continue
		}
		if key = strings.TrimSpace(key); key != "" {
			fm[key] = strings.TrimSpace(val)
		}
	}
	return fm, strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\n")
}

// relabelFirstHeading rewrites the first "# from" heading line to "# to".
func relabelFirstHeading(body, from, to string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "# "+from {
			lines[i] = "# " + to
			break
		}
	}
	return strings.Join(lines, "\n")
}

// appendLine appends line + "\n" to path, creating it if absent.
func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// addDecisionToIndex inserts a decision bullet at the end of index.md's
// "## Decisions" section (newest last). If the section is absent it is appended.
func addDecisionToIndex(path, num, title, fileName, description string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	bullet := fmt.Sprintf("- [%s — %s](decisions/%s)", num, title, fileName)
	if description != "" {
		bullet += " — " + description
	}

	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Decisions" {
			start = i
			break
		}
	}
	if start == -1 {
		out := strings.TrimRight(string(raw), "\n") + "\n\n## Decisions\n\n" + bullet + "\n"
		return os.WriteFile(path, []byte(out), 0o644)
	}

	// End of the Decisions section: the next "## " heading, the English-footer
	// line, or EOF — whichever comes first.
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "Everything here is written") {
			end = i
			break
		}
	}
	// Insert after the last non-blank line of the section (keeps trailing blanks).
	ins := end
	for ins-1 > start && strings.TrimSpace(lines[ins-1]) == "" {
		ins--
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:ins]...)
	out = append(out, bullet)
	out = append(out, lines[ins:]...)
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}
