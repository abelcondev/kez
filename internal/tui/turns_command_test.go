package tui

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestHandleTurnsCommand(t *testing.T) {
	// Isolate the inherited-by-children budget env this command exports.
	t.Setenv("KEZ_MAX_TURNS", "")

	t.Run("sets a valid budget and exports it for children", func(t *testing.T) {
		got, out := model{}.handleTurnsCommand("150")
		if got.agentOptions.MaxTurns != 150 {
			t.Fatalf("MaxTurns = %d, want 150", got.agentOptions.MaxTurns)
		}
		if !strings.Contains(out, "150") {
			t.Fatalf("output should report 150, got %q", out)
		}
		if env := os.Getenv("KEZ_MAX_TURNS"); env != "150" {
			t.Fatalf("KEZ_MAX_TURNS = %q, want 150 (so sub-agents inherit it)", env)
		}
	})

	t.Run("clamps above the ceiling (env too)", func(t *testing.T) {
		got, _ := model{}.handleTurnsCommand("99999")
		if got.agentOptions.MaxTurns != maxTurnsCeiling {
			t.Fatalf("MaxTurns = %d, want clamp to %d", got.agentOptions.MaxTurns, maxTurnsCeiling)
		}
		if env := os.Getenv("KEZ_MAX_TURNS"); env != strconv.Itoa(maxTurnsCeiling) {
			t.Fatalf("KEZ_MAX_TURNS = %q, want clamped %d", env, maxTurnsCeiling)
		}
	})

	t.Run("invalid / non-positive input is rejected without changing the budget", func(t *testing.T) {
		for _, in := range []string{"abc", "0", "-5", "3.5"} {
			m := model{}
			m.agentOptions.MaxTurns = 42
			got, out := m.handleTurnsCommand(in)
			if got.agentOptions.MaxTurns != 42 {
				t.Fatalf("input %q changed MaxTurns to %d, want unchanged (42)", in, got.agentOptions.MaxTurns)
			}
			if !strings.Contains(out, "Usage") {
				t.Fatalf("input %q should show usage, got %q", in, out)
			}
		}
	})

	t.Run("empty / status shows the current budget without changing it", func(t *testing.T) {
		m := model{}
		m.agentOptions.MaxTurns = 42
		got, out := m.handleTurnsCommand("")
		if got.agentOptions.MaxTurns != 42 {
			t.Fatalf("status changed MaxTurns to %d, want 42", got.agentOptions.MaxTurns)
		}
		if !strings.Contains(out, "42") {
			t.Fatalf("status should report current 42, got %q", out)
		}
	})
}
