package config

import (
	"os"
	"testing"
)

// SetMaxTurnsEnv must export the per-run turn budget so a child process (whose
// resolution reads the inherited environment via the nil-map -> os.Getenv path in
// applyEnv) runs with the SAME budget the user set via /turns. n <= 0 is a no-op,
// and a non-numeric/zero env value must not clobber the configured budget.
func TestSetMaxTurnsEnvRoundTrips(t *testing.T) {
	t.Setenv(MaxTurnsEnv, "") // register cleanup; restores the original after

	if err := os.Unsetenv(MaxTurnsEnv); err != nil {
		t.Fatalf("unset: %v", err)
	}
	SetMaxTurnsEnv(0) // non-positive must not set anything
	if got := os.Getenv(MaxTurnsEnv); got != "" {
		t.Fatalf("n<=0 set the env to %q, want unset", got)
	}

	SetMaxTurnsEnv(150)
	if got := os.Getenv(MaxTurnsEnv); got != "150" {
		t.Fatalf("env = %q, want 150", got)
	}

	// applyEnv with a nil map is the exact path a child uses: it falls back to
	// os.Getenv, so the inherited value sets the budget.
	cfg := &FileConfig{MaxTurns: defaultMaxTurns}
	applyEnv(cfg, nil)
	if cfg.MaxTurns != 150 {
		t.Fatalf("applyEnv MaxTurns = %d, want 150 (inherited env not read)", cfg.MaxTurns)
	}

	// A garbage env value must leave the configured budget untouched.
	if err := os.Setenv(MaxTurnsEnv, "not-a-number"); err != nil {
		t.Fatalf("set: %v", err)
	}
	cfg2 := &FileConfig{MaxTurns: defaultMaxTurns}
	applyEnv(cfg2, nil)
	if cfg2.MaxTurns != defaultMaxTurns {
		t.Fatalf("garbage env clobbered MaxTurns to %d, want default %d", cfg2.MaxTurns, defaultMaxTurns)
	}

	// An over-ceiling env value (e.g. a raw shell export that bypasses the /turns
	// clamp) must be clamped to MaxTurnsCeiling at the read site.
	if err := os.Setenv(MaxTurnsEnv, "999999"); err != nil {
		t.Fatalf("set: %v", err)
	}
	cfg3 := &FileConfig{MaxTurns: defaultMaxTurns}
	applyEnv(cfg3, nil)
	if cfg3.MaxTurns != MaxTurnsCeiling {
		t.Fatalf("applyEnv MaxTurns = %d, want clamped to ceiling %d", cfg3.MaxTurns, MaxTurnsCeiling)
	}
}
