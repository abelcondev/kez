package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abelcondev/kez/internal/config"
	"github.com/abelcondev/kez/internal/sdd"
)

// runSDD implements `kez sdd`: Kez's native Open Knowledge Format (OKF)
// Spec-Driven Development knowledge base. Unlike the ephemeral spec-draft flow,
// these artifacts are persistent, versioned files under <workspace>/sdd.
func runSDD(args []string, stdout io.Writer, stderr io.Writer, deps appDeps) int {
	if len(args) == 0 || wantsHelp(args) {
		_, _ = io.WriteString(stdout, sddHelp)
		return exitSuccess
	}

	root := sddWorkspaceRoot(args, deps)
	if root == "" {
		return writeAppError(stderr, "could not resolve a workspace directory", exitCrash)
	}

	switch args[0] {
	case "init":
		return runSDDInit(root, stdout, stderr)
	case "status":
		return runSDDStatus(root, stdout, stderr)
	case "propose":
		return runSDDPropose(root, args[1:], stdout, stderr, deps)
	case "approve":
		return runSDDApprove(root, args[1:], stdout, stderr)
	case "design":
		return runSDDDesign(root, args[1:], stdout, stderr)
	case "approve-design":
		return runSDDApproveDesign(root, args[1:], stdout, stderr)
	case "task":
		return runSDDTask(root, args[1:], stdout, stderr)
	case "done":
		return runSDDDone(root, args[1:], stdout, stderr)
	case "next":
		return runSDDNext(root, stdout, stderr)
	default:
		return writeExecUsageError(stderr, fmt.Sprintf("unknown sdd command %q. Use `kez sdd --help`.", args[0]))
	}
}

// sddWorkspaceRoot resolves the workspace the same way exec/init do, falling
// back to the process cwd so `kez sdd` works even outside a git repo.
func sddWorkspaceRoot(args []string, deps appDeps) string {
	cwd := initCwdFromArgs(args)
	if cwd == "" {
		if wd, err := deps.getwd(); err == nil {
			cwd = wd
		}
	}
	if root, err := resolveWorkspaceRoot(cwd, deps); err == nil && root != "" {
		return root
	}
	return cwd
}

func runSDDInit(root string, stdout io.Writer, stderr io.Writer) int {
	created, skipped, err := sdd.Scaffold(root)
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	base := filepath.Join(root, sdd.DirName)
	if len(created) == 0 {
		fmt.Fprintf(stdout, "SDD knowledge base already present at %s (nothing to create).\n", base)
	} else {
		fmt.Fprintf(stdout, "Scaffolded OKF SDD knowledge base at %s:\n", base)
		for _, c := range created {
			fmt.Fprintf(stdout, "  + %s\n", c)
		}
	}
	for _, s := range skipped {
		fmt.Fprintf(stdout, "  = %s (kept)\n", s)
	}

	// Turn on the branch guard from the seed so foundations and every later pass
	// land via a feature branch + PR (one PR per proposal), and the system prompt
	// states the policy from the first turn. The marker travels with the repo.
	if wrote, err := writeRequireBranchMarker(root); err != nil {
		fmt.Fprintf(stderr, "warning: could not enable the branch guard: %v\n", err)
	} else if wrote {
		fmt.Fprintf(stdout, "  + %s (branch guard on: feature branch + PR required)\n", filepath.Join(".kez", "require-branch"))
	}

	if len(created) > 0 {
		fmt.Fprintln(stdout, "\nNext: draft a proposal in sdd/proposal.md, then promote it to sdd/decisions/NNN-name.md on approval.")
	}
	return exitSuccess
}

// writeRequireBranchMarker creates <root>/.kez/require-branch (empty) to opt the
// repo into the feature-branch guard. It reports whether it created the marker;
// an already-present marker is left untouched.
func writeRequireBranchMarker(root string) (bool, error) {
	dir := filepath.Join(root, ".kez")
	marker := filepath.Join(dir, "require-branch")
	if _, err := os.Stat(marker); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func runSDDStatus(root string, stdout io.Writer, stderr io.Writer) int {
	st, err := sdd.ReadStatus(root)
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	if !st.Present {
		fmt.Fprintf(stdout, "No SDD knowledge base found. Run `kez sdd init` to scaffold %s/.\n", sdd.DirName)
		return exitSuccess
	}

	var done, pending, other int
	for _, t := range st.Tasks {
		switch t.Status {
		case "done", "completed":
			done++
		case "pending", "todo", "":
			pending++
		default:
			other++
		}
	}

	fmt.Fprintf(stdout, "SDD (OKF) — %s/\n", sdd.DirName)
	fmt.Fprintf(stdout, "  decisions: %d\n", st.Decisions)
	fmt.Fprintf(stdout, "  tasks:     %d (%d done, %d pending, %d other)\n", len(st.Tasks), done, pending, other)
	for _, t := range st.Tasks {
		title := t.Title
		if title != "" {
			title = " — " + title
		}
		fmt.Fprintf(stdout, "    [%s] %s%s\n", statusOrDash(t.Status), t.Name, title)
	}

	// Loop position: the single next gate, so callers (and agents) can resume
	// from disk state instead of re-reading the workflow from scratch.
	if state, err := sdd.ReadLoopState(root, currentGitBranch(root)); err == nil {
		if state.Branch != "" {
			fmt.Fprintf(stdout, "  branch:    %s\n", state.Branch)
		}
		printNextAction(stdout, state.Next())
	}
	return exitSuccess
}

func statusOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// runSDDApprove promotes sdd/proposal.md into a numbered, approved decision,
// updating log.md and index.md and resetting the proposal. `--title <text>`
// overrides the proposal's frontmatter title.
func runSDDApprove(root string, args []string, stdout io.Writer, stderr io.Writer) int {
	title, err := flagValue(args, "--title")
	if err != nil {
		return writeExecUsageError(stderr, err.Error())
	}
	rel, err := sdd.Promote(root, title, time.Now())
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	fmt.Fprintf(stdout, "Approved. Wrote decision %s, appended sdd/log.md, updated sdd/index.md, reset sdd/proposal.md.\n", rel)
	return exitSuccess
}

// runSDDTask scaffolds a pending task linked to a decision:
//
//	kez sdd task <decision-ref> <title...>
func runSDDTask(root string, args []string, stdout io.Writer, stderr io.Writer) int {
	positional := nonFlagArgs(args)
	if len(positional) < 2 {
		return writeExecUsageError(stderr, "usage: kez sdd task <decision-ref> <title...>")
	}
	decisionRef := positional[0]
	title := strings.Join(positional[1:], " ")
	rel, err := sdd.AddTask(root, decisionRef, title, time.Now())
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	fmt.Fprintf(stdout, "Created task %s (pending, linked to %s).\n", rel, decisionRef)
	return exitSuccess
}

// runSDDDesign scaffolds an in-review UI design linked to a decision:
//
//	kez sdd design <decision-ref> <title...>
func runSDDDesign(root string, args []string, stdout io.Writer, stderr io.Writer) int {
	positional := nonFlagArgs(args)
	if len(positional) < 2 {
		return writeExecUsageError(stderr, "usage: kez sdd design <decision-ref> <title...>")
	}
	decisionRef := positional[0]
	title := strings.Join(positional[1:], " ")
	rel, err := sdd.AddDesign(root, decisionRef, title, time.Now())
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	fmt.Fprintf(stdout, "Created design %s (in-review, linked to %s). Build it in Penpot/Figma, fill in the frames + screenshots, then run `kez sdd approve-design %s`.\n", rel, decisionRef, strings.TrimSuffix(filepath.Base(rel), ".md"))
	return exitSuccess
}

// runSDDApproveDesign flips a design from in-review to approved, clearing the UI
// gate for its decision's tasks:
//
//	kez sdd approve-design <design-ref>
func runSDDApproveDesign(root string, args []string, stdout io.Writer, stderr io.Writer) int {
	positional := nonFlagArgs(args)
	if len(positional) < 1 {
		return writeExecUsageError(stderr, "usage: kez sdd approve-design <design-ref>")
	}
	rel, err := sdd.ApproveDesign(root, positional[0], time.Now())
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	fmt.Fprintf(stdout, "Approved design %s, appended sdd/log.md. UI tasks for its decision are unblocked.\n", rel)
	return exitSuccess
}

// runSDDDone marks a task done and appends a line to log.md in one step:
//
//	kez sdd done <task-ref>
func runSDDDone(root string, args []string, stdout io.Writer, stderr io.Writer) int {
	positional := nonFlagArgs(args)
	if len(positional) < 1 {
		return writeExecUsageError(stderr, "usage: kez sdd done <task-ref>")
	}
	rel, err := sdd.CompleteTask(root, positional[0], time.Now())
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	fmt.Fprintf(stdout, "Marked %s done, appended sdd/log.md.\n", rel)
	return exitSuccess
}

// runSDDPropose drafts sdd/proposal.md from a natural-language description by
// forwarding a seeded prompt to the normal exec machinery (provider, tools,
// sandbox). The agent writes the proposal; it is told not to write code.
//
// When no provider can be resolved (the exec path would otherwise crash with a
// provider error), it degrades gracefully: it writes a seeded proposal skeleton
// with the description in place and tells the caller to fill it in and approve —
// so the loop keeps moving without a model.
func runSDDPropose(root string, args []string, stdout io.Writer, stderr io.Writer, deps appDeps) int {
	description := strings.TrimSpace(strings.Join(nonFlagArgs(args), " "))
	if description == "" {
		return writeExecUsageError(stderr, `usage: kez sdd propose "<what you want to build>"`)
	}

	if _, err := deps.resolveConfig(root, config.Overrides{}); err != nil {
		rel, seedErr := sdd.SeedProposal(root, description, time.Now())
		if seedErr != nil {
			return writeAppError(stderr, seedErr.Error(), exitCrash)
		}
		fmt.Fprintf(stdout, "No usable provider (%s).\nWrote a seeded proposal skeleton to %s — expand it, then run `kez sdd approve --title \"…\"`.\n", err.Error(), rel)
		return exitSuccess
	}

	prompt := "Draft an OKF Spec-Driven Development proposal for the request below and write it to sdd/proposal.md " +
		"(run the equivalent of `kez sdd init` first if sdd/ does not exist). Follow the existing proposal.md frontmatter " +
		"(type: Proposal, a concise title, a one-line description, status: in-review) and the # Proposal / # Context / # Acceptance " +
		"sections. If the work involves user-facing screens, set `tags: [ui]` so the loop requires an approved design before UI code. " +
		"Describe the WHAT and WHY, not the HOW. Do NOT write or modify any code — only sdd/proposal.md.\n\nRequest: " + description
	// Forward flags (e.g. --model, -C) unchanged; append the built prompt last.
	execArgs := append(passthroughFlags(args), prompt)
	return runExec(execArgs, stdout, stderr, deps)
}

// flagValue returns the value of --flag / --flag=value from args, or "" if
// absent. It errors if the flag is present without a value.
func flagValue(args []string, flag string) (string, error) {
	for i, a := range args {
		if a == flag {
			if i+1 < len(args) {
				return args[i+1], nil
			}
			return "", fmt.Errorf("%s requires a value", flag)
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"="), nil
		}
	}
	return "", nil
}

// nonFlagArgs returns positional args, dropping flags and the value that
// immediately follows a space-separated flag.
func nonFlagArgs(args []string) []string {
	var out []string
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			if !strings.Contains(a, "=") {
				skipNext = true
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

// passthroughFlags returns only the flag args (and their space-separated
// values), dropping positionals — used to forward exec flags to a sub-run.
func passthroughFlags(args []string) []string {
	var out []string
	takeNext := false
	for _, a := range args {
		if takeNext {
			out = append(out, a)
			takeNext = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			out = append(out, a)
			if !strings.Contains(a, "=") {
				takeNext = true
			}
		}
	}
	return out
}

const sddHelp = `kez sdd — native OKF Spec-Driven Development knowledge base.

Persistent, versioned spec artifacts under <workspace>/sdd:
  proposal.md   the current, in-review proposal (transient; cleared on approval)
  decisions/    approved, numbered architectural decisions (historical)
  designs/      approved UI designs (frames + screenshots) — the gate before UI code
  tasks/        units of work with Gherkin acceptance criteria
  log.md        append-only history

Lifecycle: propose → approve (→ decision) → [design → approve-design, for UI] →
task → branch → implement → done, tracked in log.md. The same loop covers
everything — discovery, architecture, foundations, and each feature are all passes
through it. init turns on the feature-branch guard (one PR per proposal). Run
"kez sdd next" any time to see the one recommended next step from disk state.

Usage:
  kez sdd init                        Scaffold sdd/ + enable the branch guard (idempotent)
  kez sdd propose "<what & why>"      Draft sdd/proposal.md via the agent (writes no code)
  kez sdd approve [--title <text>]    Promote proposal.md → decisions/NNN, update log + index
  kez sdd design <decision-ref> <t…>  Scaffold an in-review UI design linked to a decision
  kez sdd approve-design <design-ref> Approve a design, unblocking its decision's UI tasks
  kez sdd task <decision-ref> <t…>    Scaffold a pending task (Gherkin) linked to a decision
  kez sdd done <task-ref>             Mark a task done and append to log.md
  kez sdd status                      Report decisions, tasks, and the current loop position
  kez sdd next                        Print the single recommended next step (resumable)

Flags:
  -C, --cwd <dir>   Operate on the workspace containing <dir> (default: current directory)
  -h, --help        Show this help
`
