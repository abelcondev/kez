package cli

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/abelcondev/kez/internal/sdd"
)

// shipProbes are the external git/gh queries the ship pre-flight makes. They are
// function fields so tests can supply hermetic fakes instead of touching the
// network or a real `gh` install. Overridable as a package var for the same
// reason.
type shipProbes struct {
	branch          func(root string) string
	remoteURL       func(root string) (string, error)
	remoteReachable func(root string) error
	ghAuth          func() (account string, err error)
}

// activeShipProbes is the probe set the ship/preflight commands use. Tests swap
// it for fakes; production uses the real git/gh calls below.
var activeShipProbes = shipProbes{
	branch:          currentGitBranch,
	remoteURL:       gitRemoteURL,
	remoteReachable: gitRemoteReachable,
	ghAuth:          ghAuthAccount,
}

// preflightCheck is one line of the ship pre-flight report. A failing hard check
// aborts the ship before the task is closed; a soft check only warns.
type preflightCheck struct {
	name   string
	ok     bool
	detail string
	hard   bool
}

// preflight verifies the branch/remote/auth preconditions that used to only
// surface *after* a task was closed and the commit built — the push would then
// fail on a renamed/deleted remote or the wrong authenticated account, stranding
// finished work. Running it first makes that failure loud and early. It returns
// the per-check results and whether every hard check passed.
func preflight(root string, p shipProbes) (checks []preflightCheck, ok bool) {
	branch := p.branch(root)
	switch {
	case branch == "":
		checks = append(checks, preflightCheck{"branch", false, "not on a branch (detached HEAD or not a git repo) — ship from the proposal's feature branch", true})
	case sdd.IsProtectedBranch(branch):
		checks = append(checks, preflightCheck{"branch", false, "on protected branch " + branch + " — ship from the proposal's feature branch, never the default branch", true})
	default:
		checks = append(checks, preflightCheck{"branch", true, branch, false})
	}

	url, err := p.remoteURL(root)
	if err != nil {
		checks = append(checks, preflightCheck{"remote", false, "no 'origin' remote configured — set one with `git remote add origin <url>`", true})
	} else {
		checks = append(checks, preflightCheck{"remote", true, "origin → " + url, false})
		// Only worth probing reachability once we know a remote exists.
		if rerr := p.remoteReachable(root); rerr != nil {
			checks = append(checks, preflightCheck{"reachable", false, "origin unreachable: " + rerr.Error() + " (repo renamed/deleted, or the authenticated account lacks access)", true})
		} else {
			checks = append(checks, preflightCheck{"reachable", true, "origin responds to ls-remote", false})
		}
	}

	account, err := p.ghAuth()
	if err != nil {
		checks = append(checks, preflightCheck{"gh auth", false, "gh not authenticated: " + err.Error() + " — run `gh auth login`", true})
	} else {
		detail := "authenticated"
		if account != "" {
			detail = "authenticated as " + account
		}
		checks = append(checks, preflightCheck{"gh auth", true, detail, false})
	}

	ok = true
	for _, c := range checks {
		if !c.ok && c.hard {
			ok = false
		}
	}
	return checks, ok
}

// printPreflight renders the pre-flight report as ✓/✗ lines.
func printPreflight(stdout io.Writer, checks []preflightCheck) {
	fmt.Fprintln(stdout, "Ship pre-flight:")
	for _, c := range checks {
		mark := "✓"
		if !c.ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s: %s\n", mark, c.name, c.detail)
	}
}

// runSDDPreflight runs the ship pre-flight on its own and reports pass/fail,
// exiting non-zero if any hard check fails. Useful as a standalone gate.
func runSDDPreflight(root string, stdout io.Writer, stderr io.Writer) int {
	checks, ok := preflight(root, activeShipProbes)
	printPreflight(stdout, checks)
	if !ok {
		return writeAppError(stderr, "pre-flight failed — fix the ✗ above before pushing", exitCrash)
	}
	fmt.Fprintln(stdout, "Pre-flight passed — safe to push.")
	return exitSuccess
}

// runSDDShip is the safe close entrypoint: it runs the pre-flight FIRST, and only
// if it passes does it close the task (the same path as `kez sdd done`, including
// --residual follow-ups). This front-loads the remote/auth failure that `done`
// alone let slip through to a post-commit push, and keeps the risky `gh pr`
// judgment (reuse vs create, draft vs ready) in the resumable sdd-ship skill:
//
//	kez sdd ship <task-ref> [--residual "..."]
func runSDDShip(root string, args []string, stdout io.Writer, stderr io.Writer) int {
	positional := nonFlagArgs(args)
	if len(positional) < 1 {
		return writeExecUsageError(stderr, `usage: kez sdd ship <task-ref> [--residual "..."]`)
	}

	checks, ok := preflight(root, activeShipProbes)
	printPreflight(stdout, checks)
	if !ok {
		return writeAppError(stderr, "pre-flight failed — fix the ✗ above before shipping. The task was NOT closed.", exitCrash)
	}
	fmt.Fprintln(stdout, "Pre-flight passed.")

	// Resumable: a prior turn may have already closed this task. Don't re-close it
	// (that would duplicate the log line); just reprint the proposal's PR state.
	if taskIsDone(root, positional[0]) {
		rel := "sdd/tasks/" + refStem(positional[0]) + ".md"
		fmt.Fprintf(stdout, "\n%s is already closed — nothing to re-close.\n", rel)
		printProposalProgress(root, rel, stdout)
		printLoopNext(root, stdout)
		return exitSuccess
	}
	return runSDDDone(root, args, stdout, stderr)
}

// taskIsDone reports whether the referenced task's frontmatter status is closed.
// Best-effort: an unreadable base or an unknown ref reports false.
func taskIsDone(root, taskRef string) bool {
	st, err := sdd.ReadStatus(root)
	if err != nil {
		return false
	}
	want := refStem(taskRef)
	for _, t := range st.Tasks {
		if refStem(t.Name) == want {
			return isDoneStatus(t.Status)
		}
	}
	return false
}

// gitRemoteURL returns the URL of the `origin` remote, or an error if none.
func gitRemoteURL(root string) (string, error) {
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("no origin remote")
	}
	return strings.TrimSpace(string(out)), nil
}

// gitRemoteReachable probes the `origin` remote with a short-timeout ls-remote —
// the same query a push resolves first, so it catches a renamed/deleted repo or a
// no-access token before any local work is committed.
func gitRemoteReachable(root string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-remote", "--exit-code", "origin", "HEAD")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timed out contacting origin")
	}
	if err != nil {
		return fmt.Errorf("%s", firstNonEmptyLine(string(out), err.Error()))
	}
	return nil
}

// ghAuthAccount returns the active github.com account from `gh auth status`, or an
// error if gh is missing or not authenticated. gh writes the status to stderr, so
// combined output is parsed.
func ghAuthAccount() (string, error) {
	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", firstNonEmptyLine(string(out), err.Error()))
	}
	return parseGhAccount(string(out)), nil
}

// parseGhAccount extracts the account name from a `gh auth status` report line
// like "✓ Logged in to github.com account abeljams (keyring)". Returns "" if no
// such line is found (the caller still treats a zero-error status as authenticated).
func parseGhAccount(status string) string {
	for _, line := range strings.Split(status, "\n") {
		if !strings.Contains(line, "Logged in") {
			continue
		}
		i := strings.Index(line, "account ")
		if i < 0 {
			continue
		}
		rest := strings.TrimSpace(line[i+len("account "):])
		if rest == "" {
			continue
		}
		return strings.Fields(rest)[0]
	}
	return ""
}

// firstNonEmptyLine returns the first non-blank line of s, falling back to def
// when s has none.
func firstNonEmptyLine(s, def string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return def
}
