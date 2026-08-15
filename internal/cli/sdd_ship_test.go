package cli

import (
	"bytes"
	"strings"
	"testing"
)

// okProbes returns a probe set where every check passes, on a feature branch.
func okProbes() shipProbes {
	return shipProbes{
		branch:          func(string) string { return "sdd/prop-thing" },
		remoteURL:       func(string) (string, error) { return "https://github.com/acme/app.git", nil },
		remoteReachable: func(string) error { return nil },
		ghAuth:          func() (string, error) { return "acme-bot", nil },
	}
}

func checkByName(checks []preflightCheck, name string) (preflightCheck, bool) {
	for _, c := range checks {
		if c.name == name {
			return c, true
		}
	}
	return preflightCheck{}, false
}

func TestPreflightAllGreen(t *testing.T) {
	checks, ok := preflight("", okProbes())
	if !ok {
		t.Fatalf("preflight not ok with all-green probes: %+v", checks)
	}
	for _, name := range []string{"branch", "remote", "reachable", "gh auth"} {
		c, found := checkByName(checks, name)
		if !found {
			t.Fatalf("missing check %q", name)
		}
		if !c.ok {
			t.Errorf("check %q should pass: %s", name, c.detail)
		}
	}
}

func TestPreflightFailsOnProtectedBranch(t *testing.T) {
	p := okProbes()
	p.branch = func(string) string { return "main" }
	checks, ok := preflight("", p)
	if ok {
		t.Fatalf("preflight should fail on a protected branch")
	}
	c, _ := checkByName(checks, "branch")
	if c.ok || !strings.Contains(c.detail, "protected") {
		t.Errorf("branch check should fail with a protected-branch reason: %+v", c)
	}
}

func TestPreflightFailsWhenRemoteMissing(t *testing.T) {
	p := okProbes()
	p.remoteURL = func(string) (string, error) { return "", errNoRemote }
	checks, ok := preflight("", p)
	if ok {
		t.Fatalf("preflight should fail with no origin remote")
	}
	// With no remote, reachability is not even probed.
	if _, found := checkByName(checks, "reachable"); found {
		t.Errorf("reachability should be skipped when the remote is missing")
	}
}

func TestPreflightFailsWhenRemoteUnreachable(t *testing.T) {
	p := okProbes()
	p.remoteReachable = func(string) error { return errUnreachable }
	checks, ok := preflight("", p)
	if ok {
		t.Fatalf("preflight should fail when origin is unreachable")
	}
	c, _ := checkByName(checks, "reachable")
	if c.ok {
		t.Errorf("reachability check should fail")
	}
}

func TestPreflightFailsWhenGhNotAuthed(t *testing.T) {
	p := okProbes()
	p.ghAuth = func() (string, error) { return "", errNoAuth }
	_, ok := preflight("", p)
	if ok {
		t.Fatalf("preflight should fail when gh is not authenticated")
	}
}

func TestShipAbortsWithoutClosingWhenPreflightFails(t *testing.T) {
	root, task1, task2 := setupProposalWithTasks(t)

	restore := activeShipProbes
	t.Cleanup(func() { activeShipProbes = restore })
	p := okProbes()
	p.remoteReachable = func(string) error { return errUnreachable }
	activeShipProbes = p

	var out, errBuf bytes.Buffer
	if code := runSDDShip(root, []string{task1}, &out, &errBuf); code == exitSuccess {
		t.Fatalf("ship should fail when pre-flight fails")
	}
	// The task must NOT have been closed — a failing push must not strand a closed task.
	if taskIsDone(root, task1) {
		t.Errorf("task %s was closed despite a failed pre-flight", task1)
	}
	_ = task2
	if !strings.Contains(out.String(), "✗ reachable") {
		t.Errorf("ship output missing the failing check:\n%s", out.String())
	}
}

func TestShipClosesTaskWhenPreflightPasses(t *testing.T) {
	root, task1, _ := setupProposalWithTasks(t)

	restore := activeShipProbes
	t.Cleanup(func() { activeShipProbes = restore })
	activeShipProbes = okProbes()

	var out, errBuf bytes.Buffer
	if code := runSDDShip(root, []string{task1}, &out, &errBuf); code != exitSuccess {
		t.Fatalf("ship = %d, stderr=%s", code, errBuf.String())
	}
	if !taskIsDone(root, task1) {
		t.Errorf("task %s should be closed after a passing ship", task1)
	}
	got := out.String()
	if !strings.Contains(got, "Pre-flight passed") || !strings.Contains(got, "Proposal tasks (this PR):") {
		t.Errorf("ship output missing pre-flight/progress:\n%s", got)
	}
}

func TestShipIsResumableWhenTaskAlreadyClosed(t *testing.T) {
	root, task1, _ := setupProposalWithTasks(t)

	restore := activeShipProbes
	t.Cleanup(func() { activeShipProbes = restore })
	activeShipProbes = okProbes()

	var sink bytes.Buffer
	if code := runSDDShip(root, []string{task1}, &sink, &sink); code != exitSuccess {
		t.Fatalf("first ship = stderr=%s", sink.String())
	}

	var out, errBuf bytes.Buffer
	if code := runSDDShip(root, []string{task1}, &out, &errBuf); code != exitSuccess {
		t.Fatalf("second ship = %d, stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "already closed") {
		t.Errorf("re-shipping a closed task should report it is already closed:\n%s", out.String())
	}
}

func TestParseGhAccount(t *testing.T) {
	status := "github.com\n  ✓ Logged in to github.com account abeljams (keyring)\n  - Active account: true\n"
	if got := parseGhAccount(status); got != "abeljams" {
		t.Errorf("parseGhAccount = %q, want abeljams", got)
	}
	if got := parseGhAccount("no account here"); got != "" {
		t.Errorf("parseGhAccount with no match = %q, want \"\"", got)
	}
}

// sentinel errors for probe fakes.
var (
	errNoRemote    = &probeErr{"no origin remote"}
	errUnreachable = &probeErr{"Repository not found"}
	errNoAuth      = &probeErr{"not logged in"}
)

type probeErr struct{ msg string }

func (e *probeErr) Error() string { return e.msg }
