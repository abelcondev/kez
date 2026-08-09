package agent

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/abelcondev/kez/internal/branchguard"
	"github.com/abelcondev/kez/internal/config"
	"github.com/abelcondev/kez/internal/repomap"
	"github.com/abelcondev/kez/internal/sdd"
	"github.com/abelcondev/kez/internal/workspaceseed"
)

// coreSystemPrompt is the de-branded coding-craft instruction set: identity,
// autonomy, workflow, editing discipline, the testing gate, tool use, and
// communication style.
//
//go:embed system_prompt.md
var coreSystemPrompt string

// confirmationPolicy is the de-branded safety policy appended to the system
// prompt so the model self-polices before risky actions. The sandbox enforces a
// subset of these rules, but the model applies judgement first.
//
//go:embed confirmation_policy.md
var confirmationPolicy string

// fallbackSystemPrompt is used only if the embedded core prompt is somehow empty
// (it never should be) so a run always has a non-empty system turn.
const fallbackSystemPrompt = "You are Zero, a terminal coding agent. Help with the current workspace and use tools when needed."

// projectContextFiles are workspace docs injected into the system prompt so the
// agent honors project-specific conventions (mirrors AGENTS.md / CLAUDE.md).
// The first match at each directory level wins; the loader walks the chain
// from the git root down to the cwd and injects the matches in
// general-to-specific order.
var projectContextFiles = []string{"AGENTS.md", "ZERO.md", ".kez/AGENTS.md"}

// userContextFile is the per-user instruction file, resolved under
// config.UserConfigDir()/zero/ alongside the rest of Zero's per-user config
// (config.json, commands, specialists) so users can keep personal guidance out
// of individual repositories.
const userContextFile = "ZERO.md"

var userConfigDirForPrompt = config.UserConfigDir

// maxProjectContextBytes caps how much of a single project doc is injected so
// a large guidelines file can't blow the context budget.
const maxProjectContextBytes = 8 << 10 // 8 KiB per file

// maxProjectContextTotalBytes caps the total bytes across all matched
// project guideline files. With path-walking, multiple files can match (root
// + sub-tree), so a per-file cap alone is not enough.
const maxProjectContextTotalBytes = 32 << 10 // 32 KiB total

// maxRepoMapContextBytes keeps the repository map useful but compact enough to
// remain stable across normal agent turns.
const maxRepoMapContextBytes = 4 << 10 // 4 KiB

const (
	workspaceSeedMaxLines = 12
	workspaceSeedWidth    = 100
)

// buildSystemPrompt assembles the full system prompt for a run: the core
// coding-craft instructions, dynamic workspace context (cwd, git branch, project
// guidelines), and the safety confirmation policy. It is built once per run so
// every turn shares one (cacheable) system turn.
func buildSystemPrompt(options Options) string {
	return buildSystemPromptParts(options).prompt
}

// systemPromptParts retains both the exact joined prompt and the diagnostic
// sections used by prompt-prefix tracing. Building these together prevents the
// tracing path from re-reading workspace state and guarantees that the hash is
// computed from the same prompt bytes placed in the request.
type systemPromptParts struct {
	prompt             string
	baseInstructions   string
	confirmationPolicy string
	projectContext     string
	skills             string
}

func buildSystemPromptParts(options Options) systemPromptParts {
	core := strings.TrimSpace(options.SystemPrompt)
	if core == "" {
		core = strings.TrimSpace(coreSystemPrompt)
	}
	if core == "" {
		core = fallbackSystemPrompt
	}
	sections := []string{core}
	if addendum := modelPromptAddendum(options.Model); addendum != "" {
		sections = append(sections, addendum)
	}
	if session := sessionRuntimeContext(options); session != "" {
		sections = append(sections, session)
	}
	if prefixes := approvedCommandPrefixContext(options); prefixes != "" {
		sections = append(sections, prefixes)
	}
	if seed := workspaceSeedContext(options.Cwd); seed != "" {
		sections = append(sections, seed)
	}
	// User guidelines are injected before workspace/project guidelines so the
	// project's AGENTS.md/ZERO.md is the later, more specific instruction
	// block. See userGuidelines for the explicit precedence note carried in
	// the section text itself.
	if user := userGuidelines(); user != "" {
		sections = append(sections, user)
	}
	project := workspaceContext(options.Cwd)
	if project != "" {
		sections = append(sections, project)
	}
	// SDD (OKF) knowledge base: injected after project guidelines so the agent
	// reads sdd/index.md at the start of every session, as the index declares.
	if sddBlock := sddContext(options.Cwd); sddBlock != "" {
		sections = append(sections, sddBlock)
	}
	if delegation := specialistDelegationContext(options); delegation != "" {
		sections = append(sections, delegation)
	}
	// Deterministic implementation-route advisor (Direct / Delegated / SDD) and
	// the content-bound review-receipt status, injected each turn from disk.
	if routeBlock := implementationRouteContext(options.Cwd); routeBlock != "" {
		sections = append(sections, routeBlock)
	}
	if receiptBlock := reviewReceiptContext(options.Cwd); receiptBlock != "" {
		sections = append(sections, receiptBlock)
	}
	skillsBlock := skillsContext(options)
	if skillsBlock != "" {
		sections = append(sections, skillsBlock)
	}
	if style := responseStyleContext(options); style != "" {
		sections = append(sections, style)
	}
	policy := strings.TrimSpace(confirmationPolicy)
	if policy != "" {
		sections = append(sections, policy)
	}
	return systemPromptParts{
		prompt:             strings.Join(sections, "\n\n"),
		baseInstructions:   core,
		confirmationPolicy: policy,
		projectContext:     project,
		skills:             skillsBlock,
	}
}

// responseStyleContext renders the operator-selected reply style (TUI /style) as
// a system-prompt directive so the choice actually shapes responses. "balanced"
// (the default) and unknown/empty values add nothing, keeping the prompt
// byte-identical to the pre-style behavior.
func responseStyleContext(options Options) string {
	switch strings.ToLower(strings.TrimSpace(options.ResponseStyle)) {
	case "concise":
		return "Response style: concise. Lead with the result and keep answers short and direct; omit preamble, restating the question, and any explanation not needed to be correct."
	case "explanatory":
		return "Response style: explanatory. Explain the reasoning behind your answers and changes, surface relevant trade-offs, and add brief context a learner would want — while staying on task and not padding."
	case "review":
		return "Response style: review. Work like a critical reviewer: call out risks, edge cases, and unstated assumptions, flag anything questionable, and prefer precise, evidence-backed statements over reassurance."
	default:
		return ""
	}
}

func approvedCommandPrefixContext(options Options) string {
	if options.Sandbox == nil {
		return ""
	}
	prefixes := options.Sandbox.ApprovedCommandPrefixes()
	if len(prefixes) == 0 {
		return ""
	}
	lines := make([]string, 0, len(prefixes))
	for _, grant := range prefixes {
		if len(grant.Prefix) == 0 {
			continue
		}
		lines = append(lines, "- "+grant.ToolName+": "+strings.Join(grant.Prefix, " "))
	}
	if len(lines) == 0 {
		return ""
	}
	return "## Approved Command Prefixes\n\nThe following command prefixes have already been approved and do not need another permission prompt:\n" + strings.Join(lines, "\n")
}

// specialistDelegationContext nudges the orchestrator to offload read-heavy or
// parallelizable work to a specialist sub-agent via the Task tool, keeping large
// tool outputs out of the main context. It renders only when specialists are
// known (which is only where the Task tool is actually registered), so a run with
// no delegatable specialists produces the previous prompt unchanged.
func specialistDelegationContext(options Options) string {
	if len(options.Specialists) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<specialists>\n")
	b.WriteString("Delegate focused or read-heavy work to a specialist sub-agent with the Task tool instead of doing it inline. ")
	b.WriteString("When a request matches a specialist's purpose, delegate to it proactively — you do not need the user to ask first. ")
	b.WriteString("This keeps large tool outputs — searches, file dumps, multi-step exploration — out of your own context, so you stay fast and token-efficient. ")
	b.WriteString("Prefer delegating codebase search and exploration; for independent subtasks, launch several specialists in parallel. Handle small, direct edits yourself.\n")
	b.WriteString("Available specialists (call Task with the matching name when the task fits its purpose):\n")
	for _, info := range options.Specialists {
		name := strings.TrimSpace(info.Name)
		if name == "" {
			continue
		}
		if desc := strings.TrimSpace(info.WhenToUse); desc != "" {
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString(": ")
			b.WriteString(desc)
			b.WriteString("\n")
		} else {
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
	}
	b.WriteString("</specialists>")
	return b.String()
}

// skillsContext lists the reusable skills the model can pull in on demand via the
// skill tool, so it invokes the right one on the first try instead of guessing a
// name, failing, and reading the error. It mirrors specialistDelegationContext:
// names + one-line descriptions only (never skill bodies), and it renders nothing
// when no skills are installed, so a skill-less run reproduces the previous prompt
// byte-for-byte.
// skillsContextListBudget bounds the bytes spent listing individual skills so a
// workspace with an extreme number of them can't bloat every turn's prompt.
// Skills past the budget are summarized as a count rather than dropped — the
// model can still load any of them by name (an unknown name returns the full
// list from the skill tool).
//
// The budget is deliberately generous — roughly 18 skills with maximally long
// (200-rune, ASCII) descriptions, or 40+ with typical terser ones; non-ASCII
// descriptions consume it faster since the budget counts bytes. This list is
// the model's ONLY discovery surface, so a skill squeezed out of it effectively
// never triggers. The previous 640-byte cap silently broke invocation for
// anyone with more than ~6 skills — the model cannot match a request against a
// skill it cannot see.
const skillsContextListBudget = 4096

func skillsContext(options Options) string {
	if len(options.Skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<available_skills>\n")
	b.WriteString("Reusable, on-demand instruction sets you can load with the skill tool. Before acting on a request, scan this list; when the request matches a skill's name or description, call skill with its exact name FIRST and follow the loaded guidance — do not guess names, do not skip a matching skill, and do not substitute your own approach for its instructions.\n")
	listed, spent, omitted := 0, 0, 0
	for _, info := range options.Skills {
		name := strings.TrimSpace(info.Name)
		if name == "" {
			continue
		}
		line := "- " + name
		if desc := strings.TrimSpace(info.Description); desc != "" {
			line += ": " + truncateForSkillLine(desc)
		}
		line += "\n"
		// Always list at least one skill; past the budget, summarize the remainder
		// as a count instead of silently dropping it.
		if listed > 0 && spent+len(line) > skillsContextListBudget {
			omitted++
			continue
		}
		b.WriteString(line)
		spent += len(line)
		listed++
	}
	if omitted > 0 {
		b.WriteString("- …and ")
		b.WriteString(strconv.Itoa(omitted))
		b.WriteString(" more (call skill with a name; an unknown name lists them all)\n")
	}
	b.WriteString("</available_skills>")
	return b.String()
}

// truncateForSkillLine keeps a skill's one-line description short so a single
// verbose description can't dominate the skills-list budget. The cap is roomy on
// purpose: the description carries the skill's trigger conditions ("use when…"),
// and truncating those is what stops the model from ever invoking the skill.
func truncateForSkillLine(desc string) string {
	const maxDescRunes = 200
	runes := []rune(desc)
	if len(runes) <= maxDescRunes {
		return desc
	}
	return strings.TrimSpace(string(runes[:maxDescRunes])) + "…"
}

func sessionRuntimeContext(options Options) string {
	provider := strings.TrimSpace(options.ProviderName)
	model := strings.TrimSpace(options.Model)
	if provider == "" && model == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("<session>\n")
	if provider != "" {
		b.WriteString("Active provider: ")
		b.WriteString(provider)
		b.WriteString("\n")
	}
	if model != "" {
		b.WriteString("Active model: ")
		b.WriteString(model)
		b.WriteString("\n")
	}
	b.WriteString("Use the active provider/model above when answering questions about what is currently running. Persisted config commands may show saved defaults that differ from this live run/session.\n")
	b.WriteString("</session>")
	return b.String()
}

// workspaceContext returns an <environment> block describing the working
// directory, git branch, and any project guideline doc, so the model grounds its
// work in the actual repo. Returns "" when cwd is unset (keeps headless/test
// runs deterministic).
func workspaceContext(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("<environment>\n")
	b.WriteString("Working directory: ")
	b.WriteString(cwd)
	b.WriteString("\n")
	b.WriteString("Operating system: ")
	b.WriteString(runtime.GOOS)
	b.WriteString("\n")
	if runtime.GOOS == "windows" {
		b.WriteString("Shell syntax: Windows cmd.exe syntax for exec_command/bash tools. To put | & > < etc inside an arg value, use double quotes around the value, not single quotes (single quotes do not protect metachars in cmd.exe): gh --jq \".a | b\", go test -run \"A|B\". Do not pipe to or invoke POSIX coreutils from Git for Windows (usr\\bin head/grep/tail/cat/...): they are MSYS binaries and fail under the write-restricted sandbox; use native Zero tools (grep, read_file, list_directory, glob) or cmd.exe findstr/more instead, or sandbox_permissions require_escalated only when host-level execution is truly required. Prefer the workdir/cwd argument over cd when changing directories.\n")
	} else {
		b.WriteString("Shell syntax: /bin/sh syntax for exec_command/bash tools; prefer the workdir/cwd argument instead of cd when changing directories.\n")
	}
	if branch := gitBranchForPrompt(cwd); branch != "" {
		b.WriteString("Git branch: ")
		b.WriteString(branch)
		b.WriteString("\n")
	}
	b.WriteString("</environment>")

	b.WriteString(projectGuidelines(cwd, FindProjectGitRoot(cwd)))
	if repoMap := repoMapContext(cwd); repoMap != "" {
		b.WriteString("\n\n## Repo map\n\n")
		b.WriteString(repoMap)
	}
	return b.String()
}

// maxSDDContextBytes caps how much of the SDD index is injected so a large
// knowledge base can't blow the context budget.
const maxSDDContextBytes = 8 << 10 // 8 KiB

// sddContext injects the project's native SDD (OKF) knowledge base into the
// system prompt so the agent reads sdd/index.md at the start of every session —
// as the index itself declares — and surfaces the compact list of not-yet-done
// tasks so the agent knows the current unit of work. It resolves the knowledge
// base at the git root (falling back to cwd) and returns "" when the workspace
// has no sdd/ base, keeping headless/test runs deterministic.
// sddImplementPhase reports whether the SDD loop's single next action for the
// workspace at cwd is the implement phase (sdd.SkillImplement) — the long
// TDD + review cycle. It mirrors the advisor the system prompt renders (same
// root/branch resolution and ReadLoopState call), so a turn-budget bump keyed
// on it always agrees with the on-screen "Next step". Any error, or a workspace
// with no sdd/ base, reports false.
func sddImplementPhase(cwd string) bool {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return false
	}
	root := FindProjectGitRoot(cwd)
	if root == "" {
		root = cwd
	}
	ls, err := sdd.ReadLoopState(root, gitBranchForPrompt(root))
	if err != nil {
		return false
	}
	return ls.Next().Skill == sdd.SkillImplement
}

func sddContext(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	root := FindProjectGitRoot(cwd)
	if root == "" {
		root = cwd
	}

	data, err := os.ReadFile(filepath.Join(root, sdd.DirName, "index.md"))
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}
	content = truncateGuidelineContent(content, maxSDDContextBytes)

	var b strings.Builder
	b.WriteString("## Spec-Driven Development (OKF)\n\n")
	b.WriteString("This project keeps a native SDD knowledge base under `")
	b.WriteString(sdd.DirName)
	b.WriteString("/`, read first every session. Ground your work in it: consult the relevant `decisions/` entry before implementing, honor each task's Gherkin acceptance criteria, and never change architecture without an approved decision. When you finish a task, close it with `kez sdd done <task>` (it flips the status and appends to `")
	b.WriteString(sdd.DirName)
	b.WriteString("/log.md`). Resume by asking the repo — run `kez sdd next` for the single next step; do only that step and stop at every gate.\n\n")

	b.WriteString("### UI work\n\n")
	b.WriteString("The design system is built in code, in an isolated `/design-system` workbench route, and reviewed live — not mocked up in an external tool (Figma/Penpot). Right after the stack is chosen, build the base components there first. A task that ships user-facing UI must have an approved design first (`")
	b.WriteString(sdd.DirName)
	b.WriteString("/designs/NNN`, the components rendered in the `/design-system` workbench and approved with `kez sdd approve-design`) — do not jump straight to code. Build screens from that workbench as recorded in the index's \"UI conventions\"; do not hand-roll with raw CSS or utility classes when a component exists. If a primitive is missing, add it to the workbench first.\n\n")

	// When the repo requires PRs (a .kez/require-branch marker or KEZ_REQUIRE_BRANCH),
	// state the branch/PR policy up front so the agent branches BEFORE editing —
	// the compiled branch guard will otherwise refuse code writes on the protected
	// branch and the turn wastes a round-trip discovering that.
	if branchguard.Resolve(root, os.Getenv).Enabled {
		b.WriteString("### Branch & PR policy\n\n")
		b.WriteString("This repo requires every change to go through a feature branch and a pull request; the default branch is protected. Before writing or editing code, create/switch to a feature branch tied to the task (`git checkout -b feat/<task-slug>`) — a compiled guard refuses code writes on a protected branch. When the task's acceptance criteria pass, push the branch and prepare a `gh pr create` command for the user to run; never merge or push to the protected branch yourself.\n\n")
	}

	b.WriteString("### ")
	b.WriteString(sdd.DirName)
	b.WriteString("/index.md\n\n")
	b.WriteString(content)

	if st, err := sdd.ReadStatus(root); err == nil {
		var pending []sdd.TaskInfo
		for _, task := range st.Tasks {
			switch task.Status {
			case "done", "completed":
			default:
				pending = append(pending, task)
			}
		}
		if len(pending) > 0 {
			b.WriteString("\n\n### Pending tasks")
			for _, task := range pending {
				b.WriteString("\n- ")
				b.WriteString(task.Name)
				if task.Title != "" {
					b.WriteString(" — ")
					b.WriteString(task.Title)
				}
				if task.Status != "" {
					b.WriteString(" (")
					b.WriteString(task.Status)
					b.WriteString(")")
				}
			}
		}
	}

	// Surface the deterministic loop advisor's single next step so the agent
	// resumes from disk state without re-deriving "where am I" each turn — this is
	// the same result as `kez sdd next`, computed inline.
	if ls, err := sdd.ReadLoopState(root, gitBranchForPrompt(root)); err == nil {
		action := ls.Next()
		b.WriteString("\n\n### Next step (from `kez sdd next`)\n\n")
		b.WriteString(action.Summary)
		if action.Gate {
			// Human approval gate. Keep it friendly: never make the user type the
			// CLI command. Tell them what to review, ask for a plain-language
			// approval, and run the command yourself once they give it — so the
			// user's whole job at the gate is to say "aprobado".
			b.WriteString("\n\nThis is a human approval gate — stop and let the user decide. Do NOT ask them to run a command. In your reply, point them at the file to review and ask them to respond with a short approval (e.g. \"aprobado\", \"approve\", \"dale\", \"ok\") or the changes they want, then wait.")
			if action.Command != "" {
				b.WriteString("\n\nWhen the user approves, run this command yourself on their behalf — it records the approval — then continue the loop:\n\n`")
				b.WriteString(action.Command)
				b.WriteString("`")
			}
			b.WriteString("\n\nIf they ask for changes instead, make them and re-present the gate.")
		} else if action.Command != "" {
			b.WriteString("\n\n`")
			b.WriteString(action.Command)
			b.WriteString("`")
		}
		if action.Skill != "" {
			b.WriteString("\n\nLoad the `")
			b.WriteString(action.Skill)
			b.WriteString("` skill FIRST (call the skill tool with that exact name) and follow its guidance for this phase — it carries the dense, phase-specific rules that this summary only points at.")
		}
	}
	return b.String()
}

// projectGuidelines walks the directory chain from the git root to cwd
// (inclusive), finds the first matching project context file at each level
// (case-insensitive basename match), reads it, and returns the joined
// content in general-to-specific order — the most general file (at the git
// root) appears first, the most specific (at cwd) last. Each file is capped
// at maxProjectContextBytes; the total across all files is capped at
// maxProjectContextTotalBytes. Returns "" when no file matches.
func projectGuidelines(cwd, gitRoot string) string {
	dirs := projectGuidelineDirs(cwd, gitRoot)
	if len(dirs) == 0 {
		return ""
	}
	var b strings.Builder
	totalUsed := 0
	for _, dir := range dirs {
		if totalUsed >= maxProjectContextTotalBytes {
			break
		}
		match := findProjectContextFile(dir)
		if match == "" {
			continue
		}
		data, err := os.ReadFile(match)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		// Per-file cap is the lesser of the configured per-file limit and
		// whatever remains of the total budget, so a single file can never
		// blow past either bound.
		limit := maxProjectContextBytes
		if remaining := maxProjectContextTotalBytes - totalUsed; remaining < limit {
			limit = remaining
		}
		content = truncateGuidelineContent(content, limit)
		label := projectGuidelineLabel(match, gitRoot)
		b.WriteString("\n\n## Project guidelines (")
		b.WriteString(label)
		b.WriteString(")\n\n")
		b.WriteString(content)
		totalUsed += len(content)
	}
	return b.String()
}

// userGuidelines returns the per-user ZERO.md instructions block, if present.
// The file lives in config.UserConfigDir()/zero/ next to Zero's other
// per-user config; the basename match is case-insensitive so a file saved as
// zero.md still resolves on case-sensitive filesystems, mirroring the project
// guideline loader. The section carries an explicit precedence note because
// this is a global, personal preferences file: it is injected earlier in the
// prompt than the project's AGENTS.md/ZERO.md (see buildSystemPrompt), and
// the note keeps that precedence unambiguous even if a model weighs later
// context more heavily than section order alone implies.
func userGuidelines() string {
	configDir, err := userConfigDirForPrompt()
	if err != nil {
		return ""
	}
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return ""
	}
	match := findCaseInsensitiveFile(filepath.Join(configDir, "kez"), userContextFile)
	if match == "" {
		return ""
	}
	data, err := os.ReadFile(match)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}
	content = truncateGuidelineContent(content, maxProjectContextBytes)
	return "## User guidelines (" + filepath.Base(match) + ")\n\n" +
		"These are the operator's personal preferences, not project policy. " +
		"Where they conflict with a repository's project guidelines below (AGENTS.md/ZERO.md), the project guidelines take precedence.\n\n" +
		content
}

// truncationMarker is appended to guideline content that was cut short.
const truncationMarker = "\n… (truncated)"

// truncateGuidelineContent caps content at limit bytes without splitting a
// UTF-8 rune, appending a truncation marker when anything was cut. Space for
// the marker is reserved before choosing the cut point. When limit is too
// small to fit the marker itself, the marker is dropped instead, so the
// returned string never exceeds limit for any non-negative limit.
func truncateGuidelineContent(content string, limit int) string {
	if len(content) <= limit {
		return content
	}
	if limit <= 0 {
		return ""
	}
	cut := limit - len(truncationMarker)
	if cut < 0 {
		cut = 0
	}
	for cut > 0 && !utf8.RuneStart(content[cut]) {
		cut--
	}
	if truncated := content[:cut] + truncationMarker; len(truncated) <= limit {
		return truncated
	}
	// limit is smaller than the marker itself: fall back to a hard,
	// rune-safe cut at limit bytes with no marker rather than exceeding it.
	end := limit
	for end > 0 && !utf8.RuneStart(content[end]) {
		end--
	}
	return content[:end]
}

// findCaseInsensitiveFile returns the on-disk path of the regular file in dir
// whose name matches basename case-insensitively, or "" when absent.
func findCaseInsensitiveFile(dir, basename string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), basename) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// projectGuidelineDirs returns the directory chain from gitRoot to cwd
// (inclusive), in root-to-leaf order. If gitRoot is empty or unreachable
// from cwd, the chain collapses to [cwd].
func projectGuidelineDirs(cwd, gitRoot string) []string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return nil
	}
	gitRoot = strings.TrimSpace(gitRoot)
	if gitRoot == "" {
		return []string{cwd}
	}
	rel, err := filepath.Rel(gitRoot, cwd)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return []string{cwd}
	}
	if rel == "." {
		return []string{gitRoot}
	}
	dirs := []string{gitRoot}
	cur := gitRoot
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if seg == "" || seg == "." {
			continue
		}
		cur = filepath.Join(cur, seg)
		dirs = append(dirs, cur)
	}
	return dirs
}

// projectGuidelineLabel renders the path of match as a short label relative
// to gitRoot. When gitRoot is empty, the basename is returned.
func projectGuidelineLabel(match, gitRoot string) string {
	if gitRoot == "" {
		return filepath.Base(match)
	}
	rel, err := filepath.Rel(gitRoot, match)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Base(match)
	}
	return rel
}

// findProjectContextFile returns the first matching project guideline file
// in dir. Lookup is case-insensitive on the basename and on the actual
// filename returned, so a git-tracked file like AGENTS.md still resolves to
// its true filename on case-sensitive filesystems — the returned path is what
// should appear in the project guidelines label.
// Returns "" when nothing matches.
func findProjectContextFile(dir string) string {
	for _, name := range projectContextFiles {
		// Walk to the file's parent through dir with case-insensitive segment
		// matching, then find a regular file with the same basename. This works
		// on both case-sensitive and case-insensitive filesystems and always
		// returns the on-disk filename.
		parent, ok := resolveDirCaseInsensitive(filepath.Dir(filepath.Join(dir, name)), dir)
		if !ok {
			continue
		}
		if match := findCaseInsensitiveFile(parent, filepath.Base(name)); match != "" {
			return match
		}
	}
	return ""
}

// resolveDirCaseInsensitive walks from anchor to target, matching each
// segment case-insensitively. Returns the resolved absolute path and true on
// success, or "" and false if any segment cannot be found.
func resolveDirCaseInsensitive(target, anchor string) (string, bool) {
	if target == "" || target == "." {
		return anchor, true
	}
	rel, err := filepath.Rel(anchor, target)
	if err != nil {
		return "", false
	}
	if rel == "." {
		return anchor, true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	cur := anchor
	for _, p := range parts {
		if p == "" {
			continue
		}
		entries, err := os.ReadDir(cur)
		if err != nil {
			return "", false
		}
		found := false
		for _, e := range entries {
			if e.IsDir() && strings.EqualFold(e.Name(), p) {
				cur = filepath.Join(cur, e.Name())
				found = true
				break
			}
		}
		if !found {
			return "", false
		}
	}
	return cur, true
}

// FindProjectGitRoot returns the nearest ancestor of cwd that contains a
// .git entry (file or directory). Returns "" when no git root is found, so
// the caller can fall back to cwd-only lookup.
func FindProjectGitRoot(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	cur := cwd
	for {
		if HasGitMetadata(cur) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

func HasGitMetadata(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		_, err := os.Stat(filepath.Join(gitPath, "HEAD"))
		return err == nil
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(data)), "gitdir: ")
}

func workspaceSeedContext(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	seed, err := workspaceseed.BuildFromWorkspace(cwd, workspaceseed.GitInfo{
		Branch: gitBranchForPrompt(cwd),
	})
	if err != nil {
		return ""
	}
	rendered := strings.TrimSpace(workspaceseed.Render(seed, workspaceseed.RenderOptions{
		MaxLines: workspaceSeedMaxLines,
		Width:    workspaceSeedWidth,
	}))
	if rendered == "" {
		return ""
	}
	return "<workspace_seed>\n" + rendered + "\n</workspace_seed>"
}

func repoMapContext(cwd string) string {
	// repomap.Scan is best-effort supplemental context for the prompt. If it
	// fails, omit the repo map instead of failing the agent run; successful scans
	// are still capped by repomap.RenderPrompt and maxRepoMapContextBytes.
	snapshot, err := repomap.Scan(cwd, repomap.Options{
		MaxFiles: 300,
		MaxDepth: 5,
	})
	if err != nil {
		return ""
	}
	return repomap.RenderPrompt(snapshot, maxRepoMapContextBytes)
}

// gitBranchForPrompt reads the current branch (or short SHA when detached) for
// cwd, handling both a regular checkout (.git dir) and a worktree (.git file).
// Returns "" on any problem — the prompt simply omits the branch segment.
func gitBranchForPrompt(cwd string) string {
	gitRoot := FindProjectGitRoot(cwd)
	if gitRoot == "" {
		return ""
	}
	gitPath := filepath.Join(gitRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	headPath := filepath.Join(gitPath, "HEAD")
	if !info.IsDir() {
		data, err := os.ReadFile(gitPath)
		if err != nil {
			return ""
		}
		dir := strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir: ")
		if dir == "" {
			return ""
		}
		// In worktree mode the gitdir is often RELATIVE (e.g.
		// "gitdir: ../.git/worktrees/<name>") — resolve it against the worktree root (gitRoot), not the
		// process working directory, or HEAD lookup fails and we drop the branch.
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(gitRoot, dir)
		}
		headPath = filepath.Join(dir, "HEAD")
	}
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	ref := strings.TrimSpace(string(data))
	if strings.HasPrefix(ref, "ref: ") {
		return strings.TrimPrefix(strings.TrimPrefix(ref, "ref: "), "refs/heads/")
	}
	if len(ref) >= 7 {
		return ref[:7]
	}
	return ref
}
