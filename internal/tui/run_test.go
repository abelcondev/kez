package tui

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestUseAltScreenForInteractiveChat(t *testing.T) {
	if !useAltScreen(Options{}) {
		t.Fatal("normal chat should use the alternate screen")
	}
	if !useAltScreen(Options{Setup: SetupOptions{Visible: true}}) {
		t.Fatal("setup takeover should also use the alternate screen")
	}
}

// TestRunRejectsNonTTYStdin pins that the interactive shell fails fast with a
// non-zero code when stdin is not a terminal, instead of blocking forever in the
// Bubble Tea event loop (e.g. `echo "" | zero`). The guard runs before any model
// construction, so empty Options are fine.
func TestRunRejectsNonTTYStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	w.Close() // a pipe read-end is not a character device

	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old; r.Close() }()

	done := make(chan int, 1)
	go func() { done <- Run(context.Background(), Options{}) }()

	select {
	case code := <-done:
		if code != 2 {
			t.Fatalf("Run with non-TTY stdin returned %d; want exit code 2 from the stdin TTY guard", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run blocked on non-TTY stdin instead of failing fast")
	}
}
