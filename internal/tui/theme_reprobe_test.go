package tui

import (
	"context"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// cmdReProbesBackground reports whether cmd re-detects the terminal background —
// either tea.RequestBackgroundColor directly, or (the auto case) a tea.Batch that
// includes it alongside the OS-appearance detection command.
func cmdReProbesBackground(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	want := reflect.ValueOf(tea.RequestBackgroundColor).Pointer()
	if reflect.ValueOf(cmd).Pointer() == want {
		return true
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		return false
	}
	for _, c := range batch {
		if c != nil && reflect.ValueOf(c).Pointer() == want {
			return true
		}
	}
	return false
}

// TestThemeAutoReProbesBackground guards the M17 fix at the command-dispatch level
// (not just the handleThemeCommand helper): selecting `/theme auto` must return
// tea.RequestBackgroundColor so the terminal background is re-detected live, while
// a fixed theme must NOT re-probe. A regression that reverts the dispatch to
// `return m, nil` would otherwise pass the whole suite.
func TestThemeAutoReProbesBackground(t *testing.T) {
	// applyTheme mutates global palette state; restore it afterward.
	defer applyTheme(themeDark, true)

	m := newModel(context.Background(), Options{ModelName: "gpt-4"})
	m.input.SetValue("/theme auto")
	_, cmd := m.handleSubmit()
	if cmd == nil {
		t.Fatal("/theme auto must return a non-nil cmd (background re-probe)")
	}
	if !cmdReProbesBackground(cmd) {
		t.Error("/theme auto cmd must re-probe via tea.RequestBackgroundColor")
	}

	m2 := newModel(context.Background(), Options{ModelName: "gpt-4"})
	m2.input.SetValue("/theme dark")
	if _, cmd2 := m2.handleSubmit(); cmd2 != nil {
		t.Error("/theme dark must not return a background-color request")
	}
}
