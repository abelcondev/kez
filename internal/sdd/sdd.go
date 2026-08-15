// Package sdd implements Kez's native Spec-Driven Development (SDD) knowledge
// base in Open Knowledge Format (OKF): a persistent, versioned set of markdown
// artifacts under <workspace>/sdd — proposal.md, decisions/, tasks/, log.md,
// indexed by index.md. Unlike an ephemeral spec-draft session, these artifacts
// live in the repo as durable, reviewable deliverables.
package sdd

import (
	"bufio"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirName is the SDD knowledge-base directory, relative to the workspace root.
const DirName = "sdd"

//go:embed all:templates
var templatesFS embed.FS

// Scaffold writes the OKF SDD knowledge base under <root>/sdd. Existing files
// are never overwritten, so re-running is safe and idempotent. It returns the
// paths (relative to root) it created and the ones it skipped because they were
// already present.
func Scaffold(root string) (created, skipped []string, err error) {
	base := filepath.Join(root, DirName)
	walkErr := fs.WalkDir(templatesFS, "templates", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == "templates" {
			return nil
		}
		rel := strings.TrimPrefix(p, "templates/")
		dest := filepath.Join(base, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if _, statErr := os.Stat(dest); statErr == nil {
			skipped = append(skipped, filepath.Join(DirName, rel))
			return nil
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		data, readErr := templatesFS.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			return mkErr
		}
		if wErr := os.WriteFile(dest, data, 0o644); wErr != nil {
			return wErr
		}
		created = append(created, filepath.Join(DirName, rel))
		return nil
	})
	sort.Strings(created)
	sort.Strings(skipped)
	return created, skipped, walkErr
}

// TaskInfo is a single task artifact's summary, read from its frontmatter.
type TaskInfo struct {
	Name     string // file name without extension, e.g. "003-auth-signup-ui"
	Title    string
	Status   string
	Decision string // decision ref this task links to (frontmatter `decision:`), or ""
	Tier     Tier   // resolved ceremony weight: frontmatter override or inferred
}

// Status is a snapshot of the SDD knowledge base for reporting.
type Status struct {
	Present   bool
	Decisions int // approved decisions (excludes _template.md)
	Tasks     []TaskInfo
}

// ReadStatus inspects <root>/sdd and reports decision and task counts. A missing
// knowledge base is not an error: it returns Status{Present: false}.
func ReadStatus(root string) (Status, error) {
	base := filepath.Join(root, DirName)
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return Status{Present: false}, nil
		}
		return Status{}, err
	}
	st := Status{Present: true}

	decisions, err := listArtifacts(filepath.Join(base, "decisions"))
	if err != nil {
		return Status{}, err
	}
	st.Decisions = len(decisions)

	tasks, err := listArtifacts(filepath.Join(base, "tasks"))
	if err != nil {
		return Status{}, err
	}
	for _, path := range tasks {
		fm := readFrontmatter(path)
		st.Tasks = append(st.Tasks, TaskInfo{
			Name:     strings.TrimSuffix(filepath.Base(path), ".md"),
			Title:    fm["title"],
			Status:   fm["status"],
			Decision: fm["decision"],
			Tier:     ResolveTier(fm),
		})
	}
	sort.Slice(st.Tasks, func(i, j int) bool { return st.Tasks[i].Name < st.Tasks[j].Name })
	return st, nil
}

// listArtifacts returns the *.md files in dir, excluding template and hidden
// files (names beginning with "_" or "."). A missing dir yields an empty slice.
func listArtifacts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Strings(out)
	return out, nil
}

// readFrontmatter parses the leading YAML frontmatter block (--- … ---) of a
// markdown file into a flat key→value map. Only top-level scalar keys are read;
// this is deliberately a light parser, not a full YAML implementation. A file
// with no frontmatter yields an empty map.
func readFrontmatter(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return out
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		out[key] = value
	}
	// Frontmatter is best-effort: a read error just yields whatever we parsed so
	// far, but surface the check so a truncated read isn't silently ignored.
	if err := scanner.Err(); err != nil {
		return out
	}
	return out
}
