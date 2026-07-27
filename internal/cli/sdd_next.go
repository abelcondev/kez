package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/abelcondev/kez/internal/sdd"
)

// runSDDNext prints the single recommended next step of the SDD loop, derived
// from disk state plus the current git branch. It is the resumable entry point:
// instead of re-reading a long workflow to recover "where am I", a caller runs
// this and gets exactly one action.
func runSDDNext(root string, stdout io.Writer, stderr io.Writer) int {
	state, err := sdd.ReadLoopState(root, currentGitBranch(root))
	if err != nil {
		return writeAppError(stderr, err.Error(), exitCrash)
	}
	printNextAction(stdout, state.Next())
	return exitSuccess
}

// printNextAction renders a NextAction as a "next:" line plus an optional
// command hint. Gate steps are flagged so a human review is not mistaken for
// automated work.
func printNextAction(stdout io.Writer, action sdd.NextAction) {
	label := "next"
	if action.Gate {
		label = "next (your call)"
	}
	fmt.Fprintf(stdout, "  %s: %s\n", label, action.Summary)
	if action.Command != "" {
		fmt.Fprintf(stdout, "         %s\n", action.Command)
	}
	if action.Skill != "" {
		fmt.Fprintf(stdout, "         skill: %s\n", action.Skill)
	}
}

// currentGitBranch returns the checked-out branch name by reading <root>/.git/HEAD,
// or "" when detached, outside a repo, or in a form it does not parse (e.g. a
// worktree gitdir pointer). Best-effort and dependency-free by design.
func currentGitBranch(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	const prefix = "ref: refs/heads/"
	if strings.HasPrefix(head, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(head, prefix))
	}
	return ""
}
