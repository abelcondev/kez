package sdd

import (
	"os"
	"path/filepath"
	"strings"
)

// protectedBranches are the branches the feature-branch policy refuses code
// writes on; the loop advisor tells you to branch off them before implementing.
var protectedBranches = map[string]bool{"main": true, "master": true}

// IsProtectedBranch reports whether branch is one the feature-branch policy
// protects — the default branch a proposal must be branched off before its doc,
// approval, or code is written. Exported so the `propose` entrypoint can decide
// whether to open the proposal's branch.
func IsProtectedBranch(branch string) bool {
	return protectedBranches[strings.TrimSpace(branch)]
}

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
	StackDecided   bool   // some decision carries an architecture/stack tag
	PendingTasks   []TaskInfo
	Branch         string

	// Design gate. A UI-bearing decision must have an approved design artifact
	// under sdd/designs/ before its tasks may be implemented in code. The design is
	// the components rendered live in an isolated /design-system workbench route —
	// the code-first replacement for external mockup tools — not a static mockup.
	DesignInReview     string // a design artifact awaiting the gate ("designs/NNN-slug"), or ""
	FirstTaskDecision  string // decision ref PendingTasks[0] links to (verbatim), or ""
	FirstTaskNeedsUI   bool   // that decision is tagged UI, so a design is required first
	FirstTaskHasDesign bool   // an approved design already exists for that decision

	// Design-system foundations. Right after the stack is chosen and before any
	// feature UI, the loop nudges building the base components in an isolated
	// /design-system workbench route and reviewing them live at its gate.
	DesignSystemDecision string // ref of the decision tagged design-system, or ""
	DesignSystemReady    bool   // that decision has an approved workbench design
}

// SkillImplement names the phase skill for the implement step — the long
// TDD (red → green → refactor) + review cycle. Exported so callers can detect
// this phase (e.g. to widen the turn budget) without duplicating the literal.
const SkillImplement = "sdd-implement"

// NextAction is the one recommended step given a LoopState. Gate marks a step
// that hands control to the human (review/approval) rather than doing work.
// Skill names the built-in phase skill the agent should load before acting on
// this step (the deterministic router that makes skill-loading predictable); it
// is "" for mechanical steps (init, branch) and human gates, which need no skill.
type NextAction struct {
	Summary string
	Command string
	Gate    bool
	Skill   string
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
	for _, path := range decisions {
		fm := readFrontmatter(path)
		if hasTag(fm["tags"], "architecture") || hasTag(fm["tags"], "stack") {
			st.StackDecided = true
			break
		}
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
			Name:     strings.TrimSuffix(filepath.Base(path), ".md"),
			Title:    fm["title"],
			Status:   status,
			Decision: strings.TrimSpace(fm["decision"]),
			Tier:     ResolveTier(fm),
		})
	}

	// A design awaiting its gate blocks regardless of pending tasks; an approved
	// design for the first pending task's decision satisfies the UI gate.
	designInReview, approvedByDecision := scanDesigns(base)
	st.DesignInReview = designInReview
	if len(st.PendingTasks) > 0 {
		st.FirstTaskDecision = st.PendingTasks[0].Decision
		if st.FirstTaskDecision != "" {
			st.FirstTaskNeedsUI = decisionIsUI(base, st.FirstTaskDecision)
			st.FirstTaskHasDesign = approvedByDecision[normalizeRef(st.FirstTaskDecision)]
		}
	}

	// Design-system foundations: locate the decision tagged design-system (if any)
	// and report whether its workbench design has been approved yet.
	for _, path := range decisions {
		if hasTag(readFrontmatter(path)["tags"], "design-system") {
			ref := "decisions/" + filepath.Base(path)
			st.DesignSystemDecision = ref
			st.DesignSystemReady = approvedByDecision[normalizeRef(ref)]
			break
		}
	}
	return st, nil
}

// scanDesigns reports the first design artifact awaiting the gate (status
// in-review) and the set of decisions (normalized refs) that already have an
// approved design. A missing designs/ dir yields no gate and an empty set.
func scanDesigns(base string) (inReview string, approvedByDecision map[string]bool) {
	approvedByDecision = map[string]bool{}
	paths, err := listArtifacts(filepath.Join(base, "designs"))
	if err != nil {
		return "", approvedByDecision
	}
	for _, p := range paths {
		fm := readFrontmatter(p)
		name := strings.TrimSuffix(filepath.Base(p), ".md")
		switch strings.TrimSpace(fm["status"]) {
		case "approved", "done", "completed":
			if dec := normalizeRef(fm["decision"]); dec != "" {
				approvedByDecision[dec] = true
			}
		case "in-review":
			if inReview == "" {
				inReview = "designs/" + name
			}
		}
	}
	return inReview, approvedByDecision
}

// decisionIsUI reports whether the decision named by decisionRef is UI-bearing —
// either `ui: true` in its frontmatter or a "ui" tag — so the loop requires an
// approved design before its tasks are implemented.
func decisionIsUI(base, decisionRef string) bool {
	name := normalizeRef(decisionRef)
	if name == "" {
		return false
	}
	fm := readFrontmatter(filepath.Join(base, "decisions", name+".md"))
	if isTruthy(fm["ui"]) {
		return true
	}
	return hasTag(fm["tags"], "ui")
}

// normalizeRef reduces an artifact reference to its bare stem: "decisions/002-x.md",
// "002-x.md", and "002-x" all normalize to "002-x".
func normalizeRef(ref string) string {
	ref = strings.TrimSpace(filepath.ToSlash(strings.TrimSpace(ref)))
	if ref == "" {
		return ""
	}
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	return strings.TrimSuffix(ref, ".md")
}

// hasTag reports whether a frontmatter tags value like "[ui, caja]" contains want.
func hasTag(tags, want string) bool {
	for _, t := range strings.Split(strings.Trim(strings.TrimSpace(tags), "[]"), ",") {
		if strings.EqualFold(strings.TrimSpace(t), want) {
			return true
		}
	}
	return false
}

// isTruthy reports whether a scalar frontmatter value reads as boolean true.
func isTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1", "on":
		return true
	}
	return false
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
	if st.DesignInReview != "" {
		return NextAction{
			Summary: "A design is in review: " + st.DesignInReview + ". Review it before any UI code — the base components live in the /design-system workbench, or the wireframe + references for a product screen — then approve it.",
			Command: "kez sdd approve-design " + st.DesignInReview,
			Gate:    true,
		}
	}
	if st.Decisions == 0 {
		return NextAction{
			Summary: "Nothing recorded yet. Draft the first proposal (start with discovery — the what & why).",
			Command: `kez sdd propose "Discovery: <who uses it, the one flow that must not fail, constraints>"`,
			Skill:   "sdd-discovery",
		}
	}
	if len(st.PendingTasks) > 0 {
		task := st.PendingTasks[0]
		if st.FirstTaskNeedsUI && !st.FirstTaskHasDesign {
			return NextAction{
				Summary: "Task " + task.Name + " is UI work with no approved design. Ask the user for visual references, sketch the layout as an ASCII wireframe in the design doc, get it approved, then code the screen hi-fi directly.",
				Command: `kez sdd design ` + st.FirstTaskDecision + ` "<screen or flow>"`,
				Skill:   "sdd-design",
			}
		}
		if protectedBranches[st.Branch] {
			return NextAction{
				Summary: "Pending task " + task.Name + " but HEAD is on " + st.Branch + ". Branch once per proposal before writing code.",
				Command: "git checkout -b " + proposalBranch(st.FirstTaskDecision, task.Name),
			}
		}
		label := task.Name
		if task.Title != "" {
			label += " — " + task.Title
		}
		tier := task.Tier
		if tier == "" {
			tier = TierStandard
		}
		return NextAction{
			Summary: "Implement pending task " + label + " [tier: " + string(tier) + "] (TDD: red → green), review at that tier, then ship it with `kez sdd ship " + task.Name + "`. One PR per proposal.",
			Skill:   SkillImplement,
		}
	}
	// A product decision exists but no stack/architecture decision does yet: the
	// stack is the deliberate next step, not a task. Route to the stack phase,
	// which asks the user which technologies they want before deciding.
	if !st.StackDecided {
		return NextAction{
			Summary: "Decision recorded, but the stack isn't chosen yet. Ask the user which technologies they want (framework, UI, tests), research the open pieces, then record it as an architecture decision.",
			Command: `kez sdd propose "Architecture: <stack chosen with the user>"`,
			Skill:   "sdd-stack",
		}
	}
	// Foundations: the design system comes right after the stack and before any
	// feature UI. Its components are built and reviewed live in an isolated
	// /design-system workbench route — the code-first replacement for external
	// mockup tools — rather than mocked up in a separate design app.
	if !st.DesignSystemReady {
		if st.DesignSystemDecision == "" {
			return NextAction{
				Summary: "Stack chosen, but there's no design system yet. Propose one: the base components (tokens, button, input, layout, states) built in an isolated /design-system workbench route, reviewed live before any feature UI.",
				Command: `kez sdd propose "Design system: base components in an isolated /design-system workbench route"`,
				Skill:   "sdd-design-system",
			}
		}
		if protectedBranches[st.Branch] {
			return NextAction{
				Summary: "Design-system decision approved but HEAD is on " + st.Branch + ". Branch before building the workbench.",
				Command: "git checkout -b " + proposalBranch(st.DesignSystemDecision, ""),
			}
		}
		return NextAction{
			Summary: "Build the /design-system workbench for the base components — each in every state (empty, loading, error, hover, variants) — then record it with `kez sdd design` and stop at the gate so the human reviews them live.",
			Command: `kez sdd design ` + st.DesignSystemDecision + ` "Design system workbench"`,
			Skill:   "sdd-design-system",
		}
	}
	ref := st.LatestDecision
	if ref == "" {
		ref = "decisions/NNN-name.md"
	}
	return NextAction{
		Summary: "No open work. Add a task to the latest decision, or propose the next thing.",
		Command: `kez sdd task ` + ref + ` "<task title>"`,
		Skill:   "sdd-task",
	}
}

// proposalBranch names the single feature branch a proposal's work lands on: one
// PR per proposal. When the task links to a decision, the branch carries the
// decision's stem so every task of that decision shares it (feat/002-owner-auth).
// A task with no decision link falls back to a branch off its own slug.
func proposalBranch(decisionRef, taskName string) string {
	if slug := normalizeRef(decisionRef); slug != "" {
		return "feat/" + slug
	}
	return "feat/" + featureSlug(taskName)
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
