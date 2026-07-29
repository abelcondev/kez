package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/abelcondev/kez/internal/config"
)

// stubRun returns canned output per first argument (git / gh).
func stubRun(remote string, remoteErr error, ghStatus string, ghErr error) func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		switch name {
		case "git":
			return remote, remoteErr
		case "gh":
			return ghStatus, ghErr
		default:
			return "", errors.New("unexpected command " + name)
		}
	}
}

func autoEscalateOn() config.SandboxConfig {
	enabled := true
	return config.SandboxConfig{AutoEscalateVCS: &enabled}
}
func autoEscalateOff() config.SandboxConfig {
	disabled := false
	return config.SandboxConfig{AutoEscalateVCS: &disabled}
}

func TestGithubCheckSkipsWithoutGitHubRemote(t *testing.T) {
	run := stubRun("origin\thttps://gitlab.com/x/y.git (fetch)\n", nil, "", nil)
	got := githubCheck(stubLookup("gh", "git"), run, "/ws", autoEscalateOn())
	if got.Status != StatusPass || !strings.Contains(got.Message, "skipped") {
		t.Fatalf("no-remote check = %#v, want pass+skipped", got)
	}
}

func TestGithubCheckPassesWhenAuthenticated(t *testing.T) {
	run := stubRun(
		"origin\thttps://github.com/abelcondev/kez.git (fetch)\n",
		nil,
		"github.com\n  ✓ Logged in to github.com account abelcondev\n",
		nil,
	)
	got := githubCheck(stubLookup("gh", "git"), run, "/ws", autoEscalateOn())
	if got.Status != StatusPass || !strings.Contains(got.Message, "authenticated") {
		t.Fatalf("authenticated check = %#v, want pass+authenticated", got)
	}
}

func TestGithubCheckWarnsWhenGhNotAuthenticated(t *testing.T) {
	run := stubRun(
		"origin\thttps://github.com/abelcondev/kez.git (fetch)\n",
		nil,
		"You are not logged into any GitHub hosts.",
		errors.New("exit 1"),
	)
	got := githubCheck(stubLookup("gh", "git"), run, "/ws", autoEscalateOn())
	if got.Status != StatusWarn || !strings.Contains(got.Message, "not authenticated") {
		t.Fatalf("unauth check = %#v, want warn+not authenticated", got)
	}
}

func TestGithubCheckWarnsWhenAutoEscalateDisabled(t *testing.T) {
	run := stubRun("origin\thttps://github.com/abelcondev/kez.git (fetch)\n", nil, "", nil)
	got := githubCheck(stubLookup("gh", "git"), run, "/ws", autoEscalateOff())
	if got.Status != StatusWarn || !strings.Contains(got.Message, "auto-escalation is disabled") {
		t.Fatalf("disabled check = %#v, want warn+disabled", got)
	}
}

func TestGithubCheckSSHRemoteWithoutGhStillPasses(t *testing.T) {
	run := stubRun("origin\tgit@github.com:abelcondev/kez.git (fetch)\n", nil, "", nil)
	got := githubCheck(stubLookup("git"), run, "/ws", autoEscalateOn())
	if got.Status != StatusPass {
		t.Fatalf("ssh-without-gh check = %#v, want pass", got)
	}
}

func TestGithubCheckWarnsHTTPSWithoutGh(t *testing.T) {
	run := stubRun("origin\thttps://github.com/abelcondev/kez.git (fetch)\n", nil, "", nil)
	got := githubCheck(stubLookup("git"), run, "/ws", autoEscalateOn())
	if got.Status != StatusWarn || !strings.Contains(got.Message, "not installed") {
		t.Fatalf("https-without-gh check = %#v, want warn+not installed", got)
	}
}
