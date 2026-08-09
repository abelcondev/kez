package tui

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestTerminationWatchdogForceExitsOnSignal: a termination signal cancels the
// context, and when the program never signals done, the watchdog force-exits
// after the grace window.
func TestTerminationWatchdogForceExitsOnSignal(t *testing.T) {
	sig := make(chan os.Signal, 2)
	done := make(chan struct{})
	var cancelled atomic.Bool
	exited := make(chan int, 1)
	go runTerminationWatchdog(sig, done, 20*time.Millisecond,
		func() { cancelled.Store(true) },
		func(code int) { exited <- code },
	)
	sig <- os.Interrupt
	select {
	case code := <-exited:
		if code != 130 {
			t.Fatalf("force-exit code = %d, want 130", code)
		}
	case <-time.After(time.Second):
		t.Fatal("watchdog did not force-exit a wedged program")
	}
	if !cancelled.Load() {
		t.Fatal("watchdog did not cancel the context on the first signal")
	}
}

// TestTerminationWatchdogSecondSignalExitsImmediately: a second signal during
// the grace window force-exits without waiting for the timer.
func TestTerminationWatchdogSecondSignalExitsImmediately(t *testing.T) {
	sig := make(chan os.Signal, 2)
	done := make(chan struct{})
	exited := make(chan int, 1)
	go runTerminationWatchdog(sig, done, time.Hour, // long grace: only a 2nd signal should exit
		func() {},
		func(code int) { exited <- code },
	)
	sig <- os.Interrupt
	sig <- os.Interrupt
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("second signal did not force an immediate exit")
	}
}

// TestTerminationWatchdogCleanShutdownDoesNotExit: when the program signals done
// (a clean quit) the watchdog returns without ever force-exiting.
func TestTerminationWatchdogCleanShutdownDoesNotExit(t *testing.T) {
	sig := make(chan os.Signal, 2)
	done := make(chan struct{})
	exited := make(chan int, 1)
	finished := make(chan struct{})
	go func() {
		runTerminationWatchdog(sig, done, time.Hour,
			func() {},
			func(code int) { exited <- code },
		)
		close(finished)
	}()
	close(done) // program exited cleanly before any signal
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("watchdog did not return after a clean shutdown")
	}
	select {
	case code := <-exited:
		t.Fatalf("watchdog force-exited (%d) on a clean shutdown", code)
	default:
	}
}

// TestTerminationWatchdogCleanShutdownDuringGraceDoesNotExit: a done signalled
// AFTER the first signal but within the grace window still avoids force-exit.
func TestTerminationWatchdogCleanShutdownDuringGraceDoesNotExit(t *testing.T) {
	sig := make(chan os.Signal, 2)
	done := make(chan struct{})
	exited := make(chan int, 1)
	finished := make(chan struct{})
	go func() {
		runTerminationWatchdog(sig, done, time.Hour,
			func() {},
			func(code int) { exited <- code },
		)
		close(finished)
	}()
	sig <- os.Interrupt // first signal starts the grace window
	close(done)         // program then unwinds cleanly within grace
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("watchdog did not return after a clean shutdown within grace")
	}
	select {
	case code := <-exited:
		t.Fatalf("watchdog force-exited (%d) despite a clean shutdown within grace", code)
	default:
	}
}
