// Package swarm adds a multi-agent SWARM on top of zero's single sub-agent
// mechanism (internal/specialist): an orchestrator can spawn and coordinate
// MULTIPLE specialist members that run concurrently, communicate via per-agent
// mailboxes, and hand work off. It composes internal/specialist (to launch each
// member), internal/daemon (pooled workers when a daemon is running) and
// internal/background. Everything is additive and opt-in — the existing single
// "task" tool is unchanged; swarm tools are only active when invoked.
//
// Every member runs under the SAME sandbox + risk/autonomy policy as the
// orchestrator: the swarm never grants a member more authority than its parent.
package swarm

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

// ErrUnknownAgentType is returned when a definition lookup misses.
var ErrUnknownAgentType = errors.New("swarm: unknown agent type")

// modelInherit is the sentinel meaning "use the orchestrator's model" — members
// never silently pick a different (e.g. more capable) model.
const modelInherit = "inherit"

// Definition is one entry in the agent roster: how a member of a given type
// behaves (agent type, when to use it, model — "inherit" by default —,
// permission mode, and a system-prompt builder).
type Definition struct {
	AgentType      string
	WhenToUse      string
	Model          string // "inherit" => use the orchestrator's model
	PermissionMode string
	// SystemPrompt returns the member's system prompt for the given task context.
	// It is a func so a definition can fold the task briefing in.
	SystemPrompt func(ctx PromptContext) string
}

// PromptContext is the minimal context handed to a definition's SystemPrompt.
type PromptContext struct {
	Team string
	Task string
}

// Registry is the agent roster: definitions looked up by agent type. Built-ins
// are seeded at construction; user-defined definitions extend or override them.
type Registry struct {
	mu   sync.RWMutex
	defs map[string]Definition
}

// NewRegistry builds a registry seeded with the built-in roster.
func NewRegistry() *Registry {
	r := &Registry{defs: map[string]Definition{}}
	for _, def := range builtinDefinitions() {
		r.defs[def.AgentType] = def
	}
	return r
}

// Register adds or overrides a definition (user-defined agents extend the
// built-ins). An empty AgentType is rejected.
func (r *Registry) Register(def Definition) error {
	agentType := strings.TrimSpace(def.AgentType)
	if agentType == "" {
		return errors.New("swarm: definition requires an agentType")
	}
	if def.Model == "" {
		def.Model = modelInherit
	}
	def.AgentType = agentType
	r.mu.Lock()
	r.defs[agentType] = def
	r.mu.Unlock()
	return nil
}

// Lookup returns the definition for agentType, or ErrUnknownAgentType.
func (r *Registry) Lookup(agentType string) (Definition, error) {
	r.mu.RLock()
	def, ok := r.defs[strings.TrimSpace(agentType)]
	r.mu.RUnlock()
	if !ok {
		return Definition{}, ErrUnknownAgentType
	}
	return def, nil
}

// AgentTypes returns the registered agent types, sorted, for status/help.
func (r *Registry) AgentTypes() []string {
	r.mu.RLock()
	types := make([]string, 0, len(r.defs))
	for t := range r.defs {
		types = append(types, t)
	}
	r.mu.RUnlock()
	sort.Strings(types)
	return types
}

// builtinDefinitions returns the seeded roster — the two generic members
// (teammate, subagent) plus the SDD phase specialists (explorer, planner, coder,
// reviewer) that let the orchestrator delegate a loop phase to a fresh-context
// member. Each phase prompt is self-contained: swarm members run with the
// read/edit/execute/plan toolset but NOT the skill tool, so the prompt carries
// the phase's rules inline and points the member at the on-disk SDD artifacts for
// grounding rather than telling it to load a skill. User-defined agents extend
// this via Register.
func builtinDefinitions() []Definition {
	return []Definition{
		{
			AgentType: "teammate",
			WhenToUse: "In-process teammate for parallel task execution; delegate work to run alongside the orchestrator.",
			Model:     modelInherit,
			// Empty PermissionMode => inherit the orchestrator's mode (never widened).
			SystemPrompt: func(ctx PromptContext) string {
				return "You are a teammate agent collaborating with an orchestrator on team " +
					displayTeam(ctx.Team) + ". Complete your assigned task and report results.\n\nTask: " + ctx.Task
			},
		},
		{
			AgentType: "subagent",
			WhenToUse: "General-purpose subagent for an isolated, delegated task; starts with zero prior context.",
			Model:     modelInherit,
			SystemPrompt: func(ctx PromptContext) string {
				return "You are a subagent spawned to complete a specific task. You start with zero context — " +
					"the briefing below is all you know.\n\nTask: " + ctx.Task
			},
		},
		{
			AgentType: "explorer",
			WhenToUse: "Fresh-context, read-only investigation — map a subsystem, find where/how something works, gather facts. Returns a conclusion; writes nothing.",
			Model:     modelInherit,
			SystemPrompt: func(ctx PromptContext) string {
				return "You are an explorer: a fresh-context, READ-ONLY investigator on team " +
					displayTeam(ctx.Team) + ". You start with zero prior context — the briefing below is all you know. " +
					"Do not edit files or run mutating commands; find and report, do not change.\n\n" +
					"Ground yourself first: if an `sdd/index.md` exists, read it, then read the files the task points at. " +
					"Use grep/glob/read to map the code. Return a tight, factual conclusion — what you found and where (file:line) — " +
					"not a narrative of your search.\n\nTask: " + ctx.Task
			},
		},
		{
			AgentType: "planner",
			WhenToUse: "Fresh-context planning — draft a proposal, choose a stack with the user's constraints, or break work into tasks with acceptance criteria. Produces a plan/spec, never code.",
			Model:     modelInherit,
			SystemPrompt: func(ctx PromptContext) string {
				return "You are a planner: a fresh-context specialist that turns a goal into a plan or spec, NOT code, on team " +
					displayTeam(ctx.Team) + ". You start with zero prior context.\n\n" +
					"Ground yourself: read `sdd/index.md` and the relevant `sdd/decisions/`, `sdd/tasks/`, and `sdd/designs/` entries. " +
					"Produce the requested artifact content — a proposal (what & why), an architecture/stack decision (name the " +
					"technologies and the trade-offs), or a task breakdown with Given/When/Then acceptance criteria. " +
					"Be concrete and scoped; write no implementation code. Everything you output is in English.\n\nTask: " + ctx.Task
			},
		},
		{
			AgentType: "coder",
			WhenToUse: "Fresh-context implementation of a single, fully-specified SDD task with TDD. Best for independent, parallelizable tasks whose acceptance criteria are already written.",
			Model:     modelInherit,
			SystemPrompt: func(ctx PromptContext) string {
				return "You are a coder: a fresh-context specialist implementing ONE fully-specified SDD task with test-driven " +
					"discipline, on team " + displayTeam(ctx.Team) + ". You start with zero prior context.\n\n" +
					"Ground yourself: read `sdd/index.md` (stack, UI conventions, test conventions) and the task file plus its " +
					"linked decision and design. Then:\n" +
					"1. TDD — write the failing test for each Given/When/Then, then the minimal code to pass.\n" +
					"2. Smallest diff in the surrounding style; no speculative abstraction, no unrelated refactors.\n" +
					"3. Run the validators (tests, typecheck, lint, build); do not finish while they fail.\n" +
					"Stay on the current feature branch; never write on main/master. For UI, compose from the `/design-system` " +
					"workbench — do not hand-roll a component that already exists. Report the files you changed and the validator " +
					"result.\n\nTask: " + ctx.Task
			},
		},
		{
			AgentType: "reviewer",
			WhenToUse: "Fresh-context reviewer — audits a diff for correctness, security, and craft/maintainability. Independent by construction; returns structured findings and fixes nothing.",
			Model:     modelInherit,
			SystemPrompt: func(ctx PromptContext) string {
				return "You are a reviewer: a fresh-context auditor on team " + displayTeam(ctx.Team) +
					". Your independence is the point — you did NOT write this code, so do not rationalize it. " +
					"You start with zero prior context.\n\n" +
					"Review the DIFF, not the whole repo (use `git diff` and `git diff --stat` against the base branch). " +
					"Read `sdd/index.md` for the project's conventions. Judge three lenses:\n" +
					"- Correctness: bugs, wrong logic, missed edge cases.\n" +
					"- Security: secrets, injection, authz, unsafe I/O.\n" +
					"- Craft/maintainability: files past ~300–400 lines, over-long functions, poor wiring or coupling, layering " +
					"violations (for UI: presentation vs behavior leaked, or hand-rolled where a workbench component exists), " +
					"duplicated logic that should be shared, dead code, weak naming.\n\n" +
					"Return findings as a list of { file, lines, lens, severity: high|medium|low, why, concrete_fix }. " +
					"High = a real bug, a security hole, broken wiring, a layering violation, or an unmanageable file. " +
					"Be specific and honest; if the diff is clean, say so. Do NOT fix anything — report only, so the orchestrator " +
					"can remediate.\n\nTask: " + ctx.Task
			},
		},
	}
}

func displayTeam(team string) string {
	if strings.TrimSpace(team) == "" {
		return "default"
	}
	return team
}
