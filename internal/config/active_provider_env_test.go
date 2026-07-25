package config

import (
	"os"
	"testing"
)

// SetActiveProviderEnv must export the provider name so a child process (whose
// resolution reads the inherited environment via the nil-map -> os.Getenv path in
// applyEnv) resolves the SAME active provider. A blank name CLEARS it.
func TestSetActiveProviderEnvRoundTrips(t *testing.T) {
	t.Setenv(ActiveProviderEnv, "") // register cleanup; restores the original after

	if err := os.Unsetenv(ActiveProviderEnv); err != nil {
		t.Fatalf("unset: %v", err)
	}
	SetActiveProviderEnv("") // blank with nothing set stays unset
	if got := os.Getenv(ActiveProviderEnv); got != "" {
		t.Fatalf("empty name set the env to %q, want unset", got)
	}

	// Regression: switching from a named provider back to an unnamed/default profile
	// must CLEAR a previously exported value, not leave it stale for children.
	if err := os.Setenv(ActiveProviderEnv, "stale-provider"); err != nil {
		t.Fatalf("seed stale: %v", err)
	}
	SetActiveProviderEnv("") // blank must unset the stale value
	if got := os.Getenv(ActiveProviderEnv); got != "" {
		t.Fatalf("blank name left stale env %q, want cleared", got)
	}
	cfgStale := &FileConfig{}
	applyEnv(cfgStale, nil)
	if cfgStale.ActiveProvider != "" {
		t.Fatalf("applyEnv still resolved stale provider %q after clear", cfgStale.ActiveProvider)
	}

	SetActiveProviderEnv("ollama-cloud")
	if got := os.Getenv(ActiveProviderEnv); got != "ollama-cloud" {
		t.Fatalf("env = %q, want ollama-cloud", got)
	}

	// applyEnv with a nil map is the exact path a child uses: it falls back to
	// os.Getenv, so the inherited value selects the active provider.
	cfg := &FileConfig{}
	applyEnv(cfg, nil)
	if cfg.ActiveProvider != "ollama-cloud" {
		t.Fatalf("applyEnv ActiveProvider = %q, want ollama-cloud (inherited env not read)", cfg.ActiveProvider)
	}
}
