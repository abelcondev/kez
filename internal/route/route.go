// Package route implements kez's deterministic implementation-route advisor.
// Each turn it recommends the *smallest useful* route for the work — do it
// inline, delegate a narrow slice, or continue the active SDD proposal — and
// injects the rule into the system prompt so route selection stops being an
// implicit guess.
//
// The load-bearing rule, adapted from gentle-ai's organic routing: size, file
// count, and perceived risk NEVER select SDD on their own. SDD is durable
// planning state; it is entered only by an explicit human request or an already
// accepted proposal, never because a change "looks big".
package route

import "fmt"

// Route is one of the three implementation routes.
type Route string

const (
	// RouteDirect handles small, understood work inline (no delegation, no SDD).
	RouteDirect Route = "direct"
	// RouteDelegated hands a narrow slice — broad exploration or one focused
	// writer — to the Task tool, WITHOUT creating any SDD state.
	RouteDelegated Route = "delegated"
	// RouteSDD continues an already-active SDD proposal's tasks.
	RouteSDD Route = "sdd"
)

// Recommend returns the default route from the only signal that can be derived
// deterministically: whether an SDD proposal is already active. When one is,
// work continues under SDD; otherwise the default is Direct, and the agent
// escalates to Delegated by judgment (see Advisory). SDD is never selected here
// from size or risk.
func Recommend(sddActive bool) Route {
	if sddActive {
		return RouteSDD
	}
	return RouteDirect
}

// Advisory renders the per-turn routing guidance block for the system prompt.
// changedFiles is the count of already-changed files in the working tree, shown
// only as context — it never forces SDD.
func Advisory(sddActive bool, changedFiles int) string {
	if sddActive {
		return "### Implementation route\n\n" +
			"An SDD proposal is active. Continue its tasks under the SDD loop " +
			"(`kez sdd next`); do not start parallel ad-hoc work."
	}
	return fmt.Sprintf("### Implementation route\n\n"+
		"No SDD proposal is active (%d file(s) currently changed). Pick the "+
		"smallest useful route:\n"+
		"- **Direct (default):** 1–3 files of understood work — edit inline yourself.\n"+
		"- **Delegated:** 4+ files, broad exploration, or running tests/build/adversarial review — "+
		"delegate a narrow slice with the `Task` tool. This creates NO SDD state.\n"+
		"- **SDD:** only after the user explicitly asks for it or a proposal is accepted.\n\n"+
		"Hard rule: size, file count, and risk never select SDD on their own. "+
		"Never run `kez sdd propose` just because a change looks big.", changedFiles)
}
