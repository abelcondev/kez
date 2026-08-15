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
	doc.WriteString("title: ")
	doc.WriteString(title)
	doc.WriteString("\n")
	doc.WriteString("description: ")
	doc.WriteString(description)
	doc.WriteString("\n")
	tags := strings.TrimSpace(fm["tags"])
	if tags == "" {
		tags = "[]"
	}
	doc.WriteString("tags: ")
	doc.WriteString(tags)
	doc.WriteString("\nstatus: approved\n")
	doc.WriteString("timestamp: ")
	doc.WriteString(now.UTC().Format(time.RFC3339))
	doc.WriteString("\n")
	doc.WriteString("supersedes: []\n")
	doc.WriteString("---\n\n")
	doc.WriteString(strings.TrimSpace(relabelFirstHeading(body, "Proposal", "Decision")))
	doc.WriteString("\n")
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
// The tier is left to inference (no frontmatter override).
func AddTask(root, decisionRef, title string, now time.Time) (string, error) {
	return AddTaskWithTier(root, decisionRef, title, "", now)
}

// AddTaskWithTier is AddTask with an explicit tier override written to the task's
// frontmatter. An empty tier writes no `tier:` line, leaving it to inference.
func AddTaskWithTier(root, decisionRef, title string, tier Tier, now time.Time) (string, error) {
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
	doc.WriteString("title: ")
	doc.WriteString(title)
	doc.WriteString("\n")
	doc.WriteString("description: \n")
	doc.WriteString("decision: ")
	doc.WriteString(ref)
	doc.WriteString("\n")
	doc.WriteString("tags: []\n")
	doc.WriteString("status: pending\n")
	if t, ok := parseTier(string(tier)); ok {
		doc.WriteString("tier: ")
		doc.WriteString(string(t))
		doc.WriteString("\n")
	}
	doc.WriteString("timestamp: ")
	doc.WriteString(now.UTC().Format(time.RFC3339))
	doc.WriteString("\n")
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

// designBody is the starting checklist for a design artifact. A product screen is
// coded hi-fi directly (no placeholder workbench build), so the design doc is a
// lightweight spec — references + an ASCII wireframe + which base components
// compose it — that the human approves before any UI code is written.
const designBody = `# Design

The spec for a product screen: gather references, sketch the layout as a
wireframe, and record which base components compose it. The screen is then coded
hi-fi **directly** — no placeholder build. The human approves this doc first.

## References

What the UI should look like — ask the user before sketching and record whatever
they provide (save images under this design's folder ` + "`sdd/designs/<NNN>/`" + `):

- Figma: <url> (if given, pull frames/tokens with the figma MCP)
- Image / Pencil / screenshot: <path>
- Site or product to emulate: <url>
- Or a written description of the intended look & feel.

If the user has no reference, note that and describe the intended look here.

## Wireframe

An ASCII/line sketch of each screen and key state — layout, regions, and where
each component sits. One block per state (default, empty, loading, error):

` + "```" + `
┌───────────────────────────┐
│  <sketch the layout>       │
└───────────────────────────┘
` + "```" + `

## Composition

- Base components used (from the ` + "`/design-system`" + ` workbench): <Button, Card, …>.
- Product components coded directly for this screen: <name each>.
- Missing base primitive to add to the workbench first (if any): <name>.

## Behavior & states

Interactions, empty/loading/error handling, and edge cases the reviewer must
check. Behavior itself is covered by code tests in ` + "`sdd-test`" + `; the human
verifies the live screen on localhost.
`

// CompleteTask marks a task done and appends a line to log.md, atomically closing
// it so callers don't hand-edit frontmatter. taskRef accepts "tasks/NNN-slug.md",
// "NNN-slug.md", or "NNN-slug". Returns the task path relative to root.
func CompleteTask(root, taskRef string, now time.Time) (string, error) {
	base := filepath.Join(root, DirName)
	path, err := resolveArtifactPath(base, "tasks", taskRef)
	if err != nil {
		return "", err
	}
	fm := readFrontmatter(path)
	if err := setFrontmatterStatus(path, "done"); err != nil {
		return "", err
	}
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	logLine := fmt.Sprintf("- %s — Task %s done.", now.UTC().Format("2006-01-02"), name)
	if title := strings.TrimSpace(fm["title"]); title != "" {
		logLine = fmt.Sprintf("- %s — Task %s (%s) done.", now.UTC().Format("2006-01-02"), name, title)
	}
	if err := appendLine(filepath.Join(base, "log.md"), logLine); err != nil {
		return "", err
	}
	return filepath.Join(DirName, "tasks", name+".md"), nil
}

// AddDesign scaffolds designs/NNN-slug.md, an in-review design linked to
// decisionRef. It is the UI gate's artifact: a task on a UI decision may not be
// implemented until its design is approved. Returns the path relative to root.
func AddDesign(root, decisionRef, title string, now time.Time) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", fmt.Errorf("a design title is required")
	}
	base := filepath.Join(root, DirName)
	designsDir := filepath.Join(base, "designs")
	if err := os.MkdirAll(designsDir, 0o755); err != nil {
		return "", err
	}
	num, err := nextNumber(designsDir)
	if err != nil {
		return "", err
	}
	fileName := num + "-" + slugify(title) + ".md"
	relPath := filepath.Join(DirName, "designs", fileName)

	ref := strings.TrimSpace(decisionRef)
	if ref == "" {
		ref = "decisions/NNN-name.md"
	}

	var doc strings.Builder
	doc.WriteString("---\ntype: Design\ntitle: ")
	doc.WriteString(title)
	doc.WriteString("\ndescription: \ndecision: ")
	doc.WriteString(ref)
	doc.WriteString("\ntags: [ui]\nstatus: in-review\ntimestamp: ")
	doc.WriteString(now.UTC().Format(time.RFC3339))
	doc.WriteString("\n---\n\n")
	doc.WriteString(designBody)
	if err := os.WriteFile(filepath.Join(root, relPath), []byte(doc.String()), 0o644); err != nil {
		return "", err
	}
	return relPath, nil
}

// ApproveDesign flips a design artifact from in-review to approved and logs it,
// clearing the UI gate for the tasks of its decision. designRef accepts the same
// forms as CompleteTask. Returns the design path relative to root.
func ApproveDesign(root, designRef string, now time.Time) (string, error) {
	base := filepath.Join(root, DirName)
	path, err := resolveArtifactPath(base, "designs", designRef)
	if err != nil {
		return "", err
	}
	fm := readFrontmatter(path)
	if err := setFrontmatterStatus(path, "approved"); err != nil {
		return "", err
	}
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	logLine := fmt.Sprintf("- %s — Design %s approved.", now.UTC().Format("2006-01-02"), name)
	if title := strings.TrimSpace(fm["title"]); title != "" {
		logLine = fmt.Sprintf("- %s — Design %s (%s) approved.", now.UTC().Format("2006-01-02"), name, title)
	}
	if err := appendLine(filepath.Join(base, "log.md"), logLine); err != nil {
		return "", err
	}
	return filepath.Join(DirName, "designs", name+".md"), nil
}

// resolveArtifactPath resolves a user-supplied artifact reference to an existing
// file under <base>/<subdir>, accepting "<subdir>/NNN-slug.md", "NNN-slug.md", or
// "NNN-slug". It errors if the reference is empty or the file is absent.
func resolveArtifactPath(base, subdir, ref string) (string, error) {
	name := normalizeRef(ref)
	if name == "" {
		return "", fmt.Errorf("a %s reference is required", strings.TrimSuffix(subdir, "s"))
	}
	path := filepath.Join(base, subdir, name+".md")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s not found: %s", strings.TrimSuffix(subdir, "s"), filepath.ToSlash(filepath.Join(subdir, name+".md")))
	}
	return path, nil
}

// setFrontmatterStatus rewrites the `status:` line inside a markdown file's
// frontmatter in place, preserving every other line. It errors if the file has
// no frontmatter or no status field.
func setFrontmatterStatus(path, status string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("no frontmatter in %s", path)
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "status:") {
			lines[i] = "status: " + status
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	return fmt.Errorf("no status field in %s", path)
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
	if _, err = f.WriteString(line); err != nil {
		return err
	}
	_, err = f.WriteString("\n")
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
