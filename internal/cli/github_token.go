package cli

import (
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/abelcondev/kez/internal/config"
)

// githubTokenTTL bounds how often the provider shells out to `gh auth token`.
// Short enough that a `gh auth switch` on the host is picked up within seconds
// (so the injected token tracks the active account), long enough that a burst of
// sandboxed commands does not spawn a gh process each time.
const githubTokenTTL = 5 * time.Second

// newGitHubTokenProvider returns a provider that resolves the host's ACTIVE
// GitHub token for injection into sandboxed commands, or nil when injection is
// disabled by config or `gh` is not installed (in which case the engine injects
// nothing and falls back to auto-escalation for forge commands).
//
// It reads `gh auth token -h github.com` — which reflects the account selected by
// `gh auth switch` — behind a short TTL cache, so frequent account switching on
// the host is honored without a gh process per command.
func newGitHubTokenProvider(resolved config.ResolvedConfig) func() string {
	if !resolved.Sandbox.InjectGitHubTokenEnabled() {
		return nil
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return nil
	}
	var (
		mu       sync.Mutex
		cached   string
		fetched  time.Time
		hasValue bool
	)
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		if hasValue && time.Since(fetched) < githubTokenTTL {
			return cached
		}
		cached = readActiveGitHubToken()
		fetched = time.Now()
		hasValue = true
		return cached
	}
}

// readActiveGitHubToken runs gh on the host to get the active account's token.
// Any error (not authenticated, gh failure) yields "" so the engine injects
// nothing rather than a stale or partial value.
func readActiveGitHubToken() string {
	output, err := exec.Command("gh", "auth", "token", "-h", "github.com").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
