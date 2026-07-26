package sdd

import (
	"os"
	"path/filepath"
	"strings"
)

// protectedBranches are the branches the feature-branch policy refuses code
// writes on; the loop advisor tells you to branch off them before implementing.
var protectedBranches = map[string]bool{"main": true, "master": true}

// LoopState is the resumable position of the SDD loop: everything the advisor
// needs to name the single next action. It is derived purely from files on disk
// plus the current git branch, so it is cheap to recompute every turn — the
// intended replacement for re-reading a long workflow skill to recover context.
type LoopState struct {
	Present        bool
	ProposalActive bool
	ProposalTitle  string
	Decisions      int
	LatestDecision string // e.g. "decisions/002-architecture.md", or "" if none
	PendingTasks   []TaskInfo
	Branch         string
}

// NextAction is the one recommended step given a LoopState. Gate marks a step
// that hands control to the human (review/approval) rather than doing work.
type NextAction struct {
	Summary string
	Command string
	Gate    bool
}

// ReadLoopState inspects <root>/sdd plus the given git branch (pass "" if
// unknown or outside a repo) and reports the loop position.
func ReadLoopState(root, branch string) (LoopState, error) {
	st := LoopState{Branch: strings.TrimSpace(branch)}
	base := filepath.Join(root, DirName)
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return LoopState{}, err
	}
	st.Present = true

	if raw, err := os.ReadFile(filepath.Join(base, "proposal.md")); err == nil {
		fm, _ := splitFrontmatter(string(raw))
		if status := strings.TrimSpace(fm["status"]); status != "" && status != "empty" {
			st.ProposalActive = true
			st.ProposalTitle = strings.TrimSpace(fm["title"])
		}
	}

	decisions, err := listArtifacts(filepath.Join(base, "decisions"))
	if err != nil {
		return LoopState{}, err
	}
	st.Decisions = len(decisions)
	if len(decisions) > 0 {
		latest := filepath.Base(decisions[len(decisions)-1])
		st.LatestDecision = filepath.ToSlash(filepath.Join("decisions", latest))
	}

	tasks, err := listArtifacts(filepath.Join(base, "tasks"))
	if err != nil {
		return LoopState{}, err
	}
	for _, path := range tasks {
		fm := readFrontmatter(path)
		status := strings.TrimSpace(fm["status"])
		if status == "done" || status == "completed" {
			continue
		}
		st.PendingTasks = append(st.PendingTasks, TaskInfo{
			Name:   strings.TrimSuffix(filepath.Base(path), ".md"),
			Title:  fm["title"],
			Status: status,
		})
	}
	return st, nil
}

// Next returns the single recommended action for the current loop position. The
// decision tree is priority-ordered: seed the knowledge base, honor an open
// review gate, then either start the first proposal, branch for pending work,
// implement it, or open the next cycle.
func (st LoopState) Next() NextAction {
	if !st.Present {
		return NextAction{
			Summary: "No SDD knowledge base yet. Seed it, then start the loop.",
			Command: "kez sdd init",
		}
	}
	if st.ProposalActive {
		title := st.ProposalTitle
		if title == "" {
			title = "the draft"
		}
		return NextAction{
			Summary: "A proposal is in review: " + title + ". Review sdd/proposal.md, then approve it.",
			Command: `kez sdd approve --title "` + title + `"`,
			Gate:    true,
		}
	}
	if st.Decisions == 0 {
		return NextAction{
			Summary: "Nothing recorded yet. Draft the first proposal (start with discovery — the what & why).",
			Command: `kez sdd propose "Discovery: <who uses it, the one flow that must not fail, constraints>"`,
		}
	}
	if len(st.PendingTasks) > 0 {
		task := st.PendingTasks[0]
		if protectedBranches[st.Branch] {
			return NextAction{
				Summary: "Pending task " + task.Name + " but HEAD is on " + st.Branch + ". Branch before writing code.",
				Command: "git checkout -b feat/" + featureSlug(task.Name),
			}
		}
		label := task.Name
		if task.Title != "" {
			label += " — " + task.Title
		}
		return NextAction{
			Summary: "Implement pending task " + label + " (TDD: red → green), then open a PR.",
		}
	}
	ref := st.LatestDecision
	if ref == "" {
		ref = "decisions/NNN-name.md"
	}
	return NextAction{
		Summary: "No open work. Add a task to the latest decision, or propose the next thing.",
		Command: `kez sdd task ` + ref + ` "<task title>"`,
	}
}

// featureSlug turns a task file name (e.g. "002-owner-auth") into a branch slug,
// dropping the leading NNN- sequence prefix so branches read feat/owner-auth.
func featureSlug(taskName string) string {
	name := taskName
	if i := strings.IndexByte(name, '-'); i > 0 {
		if _, err := parseLeadingNumber(name[:i]); err == nil {
			name = name[i+1:]
		}
	}
	if name == "" {
		return slugify(taskName)
	}
	return slugify(name)
}

// parseLeadingNumber reports whether s is all digits (a sequence prefix).
func parseLeadingNumber(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(r-'0')
	}
	if s == "" {
		return 0, os.ErrInvalid
	}
	return n, nil
}
