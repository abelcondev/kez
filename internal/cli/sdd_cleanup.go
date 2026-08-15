package cli

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// runSDDCleanup deletes local proposal branches (sdd/prop-* or feat/*) that are
// already merged into the default branch. It uses `git update-ref -d`, which
// removes the ref WITHOUT pruning the `branch.<name>` section from `.git/config`
// — the config write that a plain `git branch -d` attempts and that the sandbox
// refuses ("could not write config file .git/config: Operation not permitted"),
// leaving branch cleanup half-done after a merge. `--dry-run` lists candidates
// without deleting:
//
//	kez sdd cleanup [--dry-run]
func runSDDCleanup(root string, args []string, stdout io.Writer, stderr io.Writer) int {
	dryRun := hasFlag(args, "--dry-run")
	def := gitDefaultBranch(root)
	if def == "" {
		return writeAppError(stderr, "could not determine the default branch (main/master); run from inside the repo", exitCrash)
	}
	merged, err := gitMergedBranches(root, def)
	if err != nil {
		return writeAppError(stderr, "could not list merged branches: "+err.Error(), exitCrash)
	}
	candidates := cleanupCandidates(merged, def, currentGitBranch(root))
	if len(candidates) == 0 {
		fmt.Fprintf(stdout, "No merged proposal branches to clean up (default: %s).\n", def)
		return exitSuccess
	}
	if dryRun {
		fmt.Fprintf(stdout, "Merged proposal branches (dry run — nothing deleted):\n")
		for _, b := range candidates {
			fmt.Fprintf(stdout, "  - %s\n", b)
		}
		return exitSuccess
	}
	fmt.Fprintf(stdout, "Deleting merged proposal branches (default: %s):\n", def)
	failed := 0
	for _, b := range candidates {
		if err := exec.Command("git", "-C", root, "update-ref", "-d", "refs/heads/"+b).Run(); err != nil {
			fmt.Fprintf(stderr, "  ! %s: %v\n", b, err)
			failed++
			continue
		}
		fmt.Fprintf(stdout, "  - %s (deleted)\n", b)
	}
	if failed > 0 {
		return writeAppError(stderr, fmt.Sprintf("%d branch(es) could not be deleted", failed), exitCrash)
	}
	return exitSuccess
}

// cleanupCandidates filters merged branch names to the proposal branches safe to
// delete: never the default branch, never the currently checked-out branch, and
// only the loop's own naming shapes (sdd/prop-*, feat/*) so a user's unrelated
// feature branch is left alone.
func cleanupCandidates(merged []string, defaultBranch, current string) []string {
	var out []string
	for _, b := range merged {
		b = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(b), "* "))
		if b == "" || b == defaultBranch || b == current {
			continue
		}
		if strings.HasPrefix(b, "sdd/prop-") || strings.HasPrefix(b, "feat/") {
			out = append(out, b)
		}
	}
	return out
}

// gitDefaultBranch resolves the repo's default branch: origin's HEAD when known,
// otherwise whichever of main/master exists locally. Returns "" outside a repo.
func gitDefaultBranch(root string) string {
	if out, err := exec.Command("git", "-C", root, "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		if i := strings.LastIndex(ref, "/"); i >= 0 {
			ref = ref[i+1:]
		}
		if ref != "" {
			return ref
		}
	}
	for _, b := range []string{"main", "master"} {
		if exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "refs/heads/"+b).Run() == nil {
			return b
		}
	}
	return ""
}

// gitMergedBranches returns the local branches already merged into defaultBranch.
func gitMergedBranches(root, defaultBranch string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "branch", "--merged", defaultBranch, "--format", "%(refname:short)").Output()
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			branches = append(branches, s)
		}
	}
	return branches, nil
}

// hasFlag reports whether args contains the exact flag token.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
