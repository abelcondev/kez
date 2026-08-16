package tui

import (
	"context"
	"testing"

	"github.com/abelcondev/kez/internal/tools"
	"github.com/abelcondev/kez/internal/zeroruntime"
)

// stagePNGImages attaches n one-byte PNG images through the real clipboard-attach
// path, each of which drops an inline [Image #k] token into the composer.
func stagePNGImages(t *testing.T, m model, n int) model {
	t.Helper()
	png := []byte{0x89, 'P', 'N', 'G'}
	for i := 0; i < n; i++ {
		m = m.attachClipboardImage(png, "image/png")
	}
	if len(m.pendingImages) != n {
		t.Fatalf("setup: want %d staged images, got %d (vision gate?)", n, len(m.pendingImages))
	}
	return m
}

// imageChipLabels returns the rendered "[Image #k]" labels for the composer's
// current image tokens, in reading order.
func imageChipLabels(m model) []string {
	var labels []string
	for _, p := range validComposerPastePreviews(m.currentComposerState(), m.composerPastePreviews) {
		if p.imageID != 0 {
			labels = append(labels, p.label)
		}
	}
	return labels
}

// TestInlineImageTokenAllowsTypingBothSides is the core of the user's request: the
// [Image #1] chip is a real token in the buffer, so text can be typed before AND
// after it. On submit the sentinel is stripped and the image threads through.
func TestInlineImageTokenAllowsTypingBothSides(t *testing.T) {
	m := newModel(context.Background(), Options{ModelName: "gpt-4.1"})
	m = stagePNGImages(t, m, 1)

	m = typeRunes(t, m, "after") // cursor sits after the token
	state := m.currentComposerState()
	state.cursor = 0 // jump before the token
	m.setComposerState(state)
	m = typeRunes(t, m, "before ")

	text, images, _ := m.resolveComposerImageTokens()
	if text != "before after" {
		t.Fatalf("resolved text = %q, want %q", text, "before after")
	}
	if len(images) != 1 {
		t.Fatalf("want 1 image threaded, got %d", len(images))
	}
}

// TestInlineImageTokenRenumbersAfterDelete deletes a middle chip as a unit and
// checks the survivors renumber to 1..N-1 and the dropped image is pruned at
// submit, keeping labels and attached images consistent — the Claude Code feel.
func TestInlineImageTokenRenumbersAfterDelete(t *testing.T) {
	m := newModel(context.Background(), Options{ModelName: "gpt-4.1"})
	m = stagePNGImages(t, m, 3)

	if got := imageChipLabels(m); len(got) != 3 || got[0] != "[Image #1]" || got[2] != "[Image #3]" {
		t.Fatalf("labels = %v, want three numbered chips", got)
	}

	// Delete the FIRST token as a unit (cursor just past it → backspace-preview).
	state := m.currentComposerState()
	state.cursor = 1
	nextState, nextPreviews, ok := deleteComposerPastePreviewBefore(state, m.composerPastePreviews)
	if !ok {
		t.Fatal("expected the first image token to delete as a unit")
	}
	m.setComposerState(nextState)
	m.composerPastePreviews = nextPreviews

	// The two survivors renumber to #1, #2.
	if got := imageChipLabels(m); len(got) != 2 || got[0] != "[Image #1]" || got[1] != "[Image #2]" {
		t.Fatalf("after delete labels = %v, want [[Image #1] [Image #2]]", got)
	}
	// Submit resolution drops the deleted image, keeping the two survivors in order.
	_, images, _ := m.resolveComposerImageTokens()
	if len(images) != 2 {
		t.Fatalf("resolve should keep 2 surviving images, got %d", len(images))
	}
}

// TestInlineImageTokenImageOnlySubmits guards that a chips-only prompt (no words)
// still starts a run rather than being swallowed as an empty submit.
func TestInlineImageTokenImageOnlySubmits(t *testing.T) {
	root := t.TempDir()
	provider := &fakeProvider{events: []zeroruntime.StreamEvent{
		{Type: zeroruntime.StreamEventText, Content: "ok"},
		{Type: zeroruntime.StreamEventDone},
	}}
	m := newModel(context.Background(), Options{
		Cwd:          root,
		ProviderName: "openai",
		ModelName:    "gpt-4.1",
		Provider:     provider,
		Registry:     tools.NewRegistry(),
		SessionStore: testSessionStore(t),
	})
	m.agentOptions.OnText = func(string) {}

	captured := make(chan []zeroruntime.ImageBlock, 1)
	m.captureRunImages = func(imgs []zeroruntime.ImageBlock) { captured <- imgs }

	m = stagePNGImages(t, m, 1) // composer holds only the [Image #1] chip

	updated, cmd := m.handleSubmit()
	next := updated.(model)
	if cmd == nil {
		t.Fatal("an image-only submit should still start a run")
	}
	if len(next.pendingImages) != 0 {
		t.Fatalf("submit must clear pending images, got %d", len(next.pendingImages))
	}

	execCmd(cmd)
	select {
	case imgs := <-captured:
		if len(imgs) != 1 {
			t.Fatalf("image-only run should receive 1 image, got %d", len(imgs))
		}
	default:
		t.Fatal("expected the run to receive the staged image")
	}
}
