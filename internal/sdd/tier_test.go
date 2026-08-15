package sdd

import "testing"

func TestInferTier(t *testing.T) {
	cases := []struct {
		text string
		want Tier
	}{
		{"PIN sign-in backend issuing staff tokens", TierCritical},
		{"Cobro de comanda con vuelto", TierCritical},
		{"Data migration: add workspaces table", TierCritical},
		{"Owner dashboard placeholder screen", TierTrivial},
		{"Rename helper for clarity", TierTrivial},
		{"Staff list ordering by name", TierStandard},
		{"", TierStandard},
	}
	for _, c := range cases {
		if got := InferTier(c.text); got != c.want {
			t.Errorf("InferTier(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestInferTierCriticalBeatsTrivial(t *testing.T) {
	// A placeholder that also touches auth must not be downgraded to trivial.
	if got := InferTier("auth login placeholder screen"); got != TierCritical {
		t.Errorf("critical signal must win over trivial, got %q", got)
	}
}

func TestResolveTierOverrideWins(t *testing.T) {
	// Frontmatter override beats inference, even when inference would say critical.
	fm := map[string]string{"title": "payment checkout flow", "tier": "trivial"}
	if got := ResolveTier(fm); got != TierTrivial {
		t.Errorf("override should win: got %q, want trivial", got)
	}
	// An invalid override falls back to inference.
	fm2 := map[string]string{"title": "payment checkout flow", "tier": "bogus"}
	if got := ResolveTier(fm2); got != TierCritical {
		t.Errorf("invalid override should fall back to inference: got %q, want critical", got)
	}
	// No override, no signal → standard.
	if got := ResolveTier(map[string]string{"title": "tweak list layout"}); got != TierStandard {
		t.Errorf("no signal should be standard, got %q", got)
	}
}
