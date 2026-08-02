package route

import (
	"strings"
	"testing"
)

func TestRecommendNeverSelectsSDDFromSize(t *testing.T) {
	if got := Recommend(false); got != RouteDirect {
		t.Fatalf("with no active proposal, want Direct default, got %q", got)
	}
	if got := Recommend(true); got != RouteSDD {
		t.Fatalf("with an active proposal, want SDD, got %q", got)
	}
}

func TestAdvisoryEncodesHardRuleWhenNoProposal(t *testing.T) {
	text := Advisory(false, 7)
	for _, want := range []string{"Direct", "Delegated", "Task", "never select SDD"} {
		if !strings.Contains(text, want) {
			t.Fatalf("advisory missing %q:\n%s", want, text)
		}
	}
	// The changed-file count is context only and must not be phrased as a
	// trigger to enter SDD.
	if !strings.Contains(text, "7 file(s)") {
		t.Fatalf("advisory should surface the changed-file count as context:\n%s", text)
	}
}

func TestAdvisoryDefersToSDDLoopWhenActive(t *testing.T) {
	text := Advisory(true, 0)
	if !strings.Contains(text, "SDD proposal is active") || !strings.Contains(text, "kez sdd next") {
		t.Fatalf("active-proposal advisory should defer to the SDD loop:\n%s", text)
	}
}
