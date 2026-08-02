package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/abelcondev/kez/internal/receipt"
	"github.com/abelcondev/kez/internal/route"
	"github.com/abelcondev/kez/internal/sdd"
	"github.com/abelcondev/kez/internal/tools"
	"github.com/abelcondev/kez/internal/verify"
)

// receiptVerifyTimeoutMS bounds each objective check the commit gate runs so a
// hung test suite cannot stall a commit indefinitely.
const receiptVerifyTimeoutMS = 120000

// gitDeliveryIntent reports whether a shell command delivers content — a git
// commit and/or a git push. It reuses auto-escalation's tolerant AST parser, so
// an unparseable command (substitutions, redirects) yields (false,false) and is
// left to the always-on zerogit gate; the bash layer fails open rather than
// blocking a command it cannot safely read.
func gitDeliveryIntent(toolName string, args map[string]any) (commit, push bool) {
	if !isShellCommandTool(toolName) {
		return false, false
	}
	command, ok := firstStringArg(args, "command", "cmd", "script", "shell")
	if !ok {
		return false, false
	}
	segments, ok := forgeCommandSegments(command)
	if !ok {
		return false, false
	}
	for _, tokens := range segments {
		if len(tokens) == 0 || commandName(tokens[0]) != "git" {
			continue
		}
		switch gitLeadingSubcommand(tokens) {
		case "commit":
			commit = true
		case "push":
			push = true
		}
	}
	return commit, push
}

// gitLeadingSubcommand returns a git invocation's subcommand, skipping leading
// global options the same way gitSubcommand/gitNetworkSubcommand do, but for any
// subcommand (not just the read-only allowlist).
func gitLeadingSubcommand(command []string) string {
	for index := 1; index < len(command); index++ {
		arg := command[index]
		if gitOptionConsumesValue(arg) {
			index++
			continue
		}
		if gitOptionHasInlineValue(arg) || arg == "--" || strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

// ensureReceiptForCommit is the commit gate. It never blocks — WIP is allowed —
// but guarantees that before content becomes a commit, a content-bound receipt
// covering exactly the current working tree exists, carrying the verdict of
// objective verification. If a matching verified receipt already exists (the
// common case after an end-of-turn or prior-commit freeze), it is a no-op; only
// otherwise does it run verification. Best-effort: outside a repo, or on any
// error, it silently does nothing rather than obstruct the commit.
func ensureReceiptForCommit(ctx context.Context, options Options) {
	root, ok := receipt.Root(ctx, options.Cwd)
	if !ok {
		return
	}
	if existing, found, err := receipt.Read(ctx, root); err == nil && found && existing.Verified {
		if match, err := receipt.MatchesWorkingTree(ctx, root, existing); err == nil && match {
			return
		}
	}
	verified, summary := runReceiptVerification(ctx, root)
	_, _ = receipt.Freeze(ctx, root, verified, summary)
}

// receiptPushGate is the push gate. It blocks a push whose HEAD content is not
// covered by a review receipt — i.e. HEAD was amended, rebased, or committed
// outside kez's reviewed path — so unreviewed content cannot silently reach a
// remote. It only guards a push that does not itself commit first (a compound
// `git commit && git push` establishes coverage as it runs). Best-effort on its
// own errors: it never blocks a push because the gate itself failed to read.
func receiptPushGate(ctx context.Context, options Options, call ToolCall) (ToolResult, bool) {
	root, ok := receipt.Root(ctx, options.Cwd)
	if !ok {
		return ToolResult{}, false
	}
	existing, found, err := receipt.Read(ctx, root)
	if err != nil {
		return ToolResult{}, false
	}
	if !found {
		return receiptBlockedResult(call,
			"no review receipt exists for the current HEAD. Commit through kez so the change is verified and content-bound before pushing."), true
	}
	covered, err := receipt.HeadTreeMatches(ctx, root, existing)
	if err != nil {
		return ToolResult{}, false
	}
	if !covered {
		return receiptBlockedResult(call,
			"HEAD's content is not covered by a review receipt (it was amended, rebased, or committed outside kez's reviewed path). Re-commit through kez to re-freeze and verify the change before pushing."), true
	}
	return ToolResult{}, false
}

// emitTurnReceipt freezes a content-bound receipt at the end of an agent turn so
// the advisor and commit fast-path always reflect the current working tree. It
// is cheap: it runs no verification (that is the commit gate's job) and does not
// clobber an existing receipt whose tree still matches — preserving a prior
// verified verdict. Best-effort; failures are silent.
func emitTurnReceipt(ctx context.Context, options Options) {
	root, ok := receipt.Root(ctx, options.Cwd)
	if !ok {
		return
	}
	hash, _, err := receipt.WorkingTreeHash(ctx, root)
	if err != nil {
		return
	}
	if existing, found, err := receipt.Read(ctx, root); err == nil && found && existing.TreeHash == hash {
		return
	}
	_, _ = receipt.Freeze(ctx, root, false, "pending verification (frozen at end of turn)")
}

// runReceiptVerification runs the repo's detected objective checks and reports
// whether they passed plus a short summary. A repo with no detectable checks is
// treated as passing (nothing to run), so the receipt still binds content.
func runReceiptVerification(ctx context.Context, root string) (bool, string) {
	plan, err := verify.DetectPlan(root)
	if err != nil {
		return false, "verification unavailable: " + err.Error()
	}
	if len(plan.Checks) == 0 {
		return true, "no checks detected"
	}
	report := verify.Run(ctx, plan, verify.RunOptions{TimeoutMS: receiptVerifyTimeoutMS})
	summary := fmt.Sprintf("%d passed, %d failed, %d errored",
		report.Summary.Passed, report.Summary.Failed, report.Summary.Errors)
	return report.OK, summary
}

// implementationRouteContext renders the per-turn routing advisory: pick the
// smallest useful route and never let size or risk select SDD on their own.
func implementationRouteContext(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	root := FindProjectGitRoot(cwd)
	if root == "" {
		root = cwd
	}
	sddActive := false
	if ls, err := sdd.ReadLoopState(root, gitBranchForPrompt(root)); err == nil {
		sddActive = ls.ProposalActive
	}
	changed := 0
	if files, err := receipt.ChangedFiles(context.Background(), root); err == nil {
		changed = len(files)
	}
	return route.Advisory(sddActive, changed)
}

// reviewReceiptContext surfaces the current content-bound receipt so the agent
// knows whether the working tree is covered and verified before it delivers.
func reviewReceiptContext(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	root, ok := receipt.Root(context.Background(), cwd)
	if !ok {
		return ""
	}
	r, found, err := receipt.Read(context.Background(), root)
	if err != nil || !found {
		return "### Review receipt\n\nNo content-bound receipt yet. kez freezes one at " +
			"the end of each turn and verifies it at commit; a git push is refused until " +
			"HEAD's content is covered by a receipt."
	}
	status := "unverified"
	if r.Verified {
		status = "verified"
	}
	return fmt.Sprintf("### Review receipt\n\nA content-bound receipt (%s) covers the last "+
		"frozen working tree on `%s`. If you change files after review, the commit gate "+
		"re-freezes and re-verifies; a git push is refused when HEAD's content is not covered "+
		"(amend/rebase/opaque commits).", status, r.Branch)
}

// receiptBlockedResult builds the model-facing tool result for a delivery
// blocked by the receipt gate.
func receiptBlockedResult(call ToolCall, reason string) ToolResult {
	return ToolResult{
		ToolCallID:   call.ID,
		Name:         call.Name,
		Status:       tools.StatusError,
		Output:       "Error: push blocked by the content-bound receipt gate: " + reason,
		DenialReason: DenialReceiptBlocked,
	}
}
