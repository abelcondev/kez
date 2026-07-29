package doctor

import (
	"os/exec"
	"strings"

	"github.com/abelcondev/kez/internal/config"
)

// githubCheck verifies that GitHub operations will work from kez. Trusted forge
// commands (gh, git push/fetch/pull/clone/ls-remote) auto-escalate OUT of the
// sandbox and run on the host, so the host's `gh auth status` and credential
// setup are exactly what they will see — which is what this check inspects.
//
// It only applies when the workspace has a github.com remote; otherwise it is a
// pass with a skip note. It never fails (GitHub auth is environmental, not a kez
// defect): a missing/unauthenticated setup is a warning with an actionable
// remedy so the agent learns about it up front instead of mid-task.
func githubCheck(lookup func(string) (string, error), run func(string, ...string) (string, error), workspaceRoot string, sandboxConfig config.SandboxConfig) Check {
	if lookup == nil {
		lookup = exec.LookPath
	}
	if run == nil {
		run = defaultDoctorCommand
	}

	remoteURL, hasGitHubRemote := githubRemoteURL(run, workspaceRoot)
	if !hasGitHubRemote {
		return check("github.access", "GitHub access", StatusPass, "No github.com remote in this workspace; GitHub checks skipped.", nil)
	}
	usesSSH := strings.HasPrefix(remoteURL, "git@") || strings.HasPrefix(remoteURL, "ssh://")

	details := map[string]any{"remote": remoteURL}
	if usesSSH {
		details["protocol"] = "ssh"
	} else {
		details["protocol"] = "https"
	}

	// When auto-escalation is off, forge commands stay sandboxed with a virtual
	// HOME and scrubbed tokens, so gh/git-over-https will fail or (for git)
	// dead-end on a now-fast-failing credential prompt.
	if !sandboxConfig.AutoEscalateVCSEnabled() {
		details["autoEscalateVcs"] = false
		details["remedy"] = "Remove `sandbox.autoEscalateVcs: false` so gh and git push/fetch/pull/clone/ls-remote reach the host credential store."
		return check("github.access", "GitHub access", StatusWarn,
			"Git-forge auto-escalation is disabled, so gh and git-over-HTTPS run sandboxed without credentials and will fail.", details)
	}
	details["autoEscalateVcs"] = true

	// gh is only required for gh-based operations (PRs, releases, api). Plain git
	// push/fetch over SSH or an https credential helper does not need it.
	ghPath, ghErr := lookup("gh")
	if ghErr != nil {
		if usesSSH {
			return check("github.access", "GitHub access", StatusPass, "GitHub SSH remote; git operations use ssh-agent. `gh` is not installed (only needed for PRs/releases).", details)
		}
		details["remedy"] = "Install GitHub CLI (`gh`) for PR/release operations, and ensure a git credential helper is configured for HTTPS."
		return check("github.access", "GitHub access", StatusWarn, "`gh` is not installed; PR/release operations will fail and HTTPS git relies on a credential helper.", details)
	}
	details["ghPath"] = ghPath

	output, statusErr := run("gh", "auth", "status")
	if statusErr != nil || !strings.Contains(output, "Logged in to") {
		details["remedy"] = "Run `gh auth login` on the host so auto-escalated gh/git commands can authenticate."
		return check("github.access", "GitHub access", StatusWarn,
			"GitHub CLI is installed but not authenticated on the host; auto-escalated gh/git commands will fail.", details)
	}

	return check("github.access", "GitHub access", StatusPass,
		"GitHub CLI is authenticated; forge commands auto-escalate to the host credential store.", details)
}

// githubRemoteURL returns the first github.com remote URL in the workspace, if
// any. A non-git workspace or a git failure reports no remote (the check then
// skips) rather than erroring.
func githubRemoteURL(run func(string, ...string) (string, error), workspaceRoot string) (string, bool) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return "", false
	}
	output, err := run("git", "-C", workspaceRoot, "remote", "-v")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		url := fields[1]
		if strings.Contains(url, "github.com") {
			return url, true
		}
	}
	return "", false
}

func defaultDoctorCommand(name string, args ...string) (string, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	return string(output), err
}
