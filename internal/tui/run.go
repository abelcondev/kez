package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
)

// signalWatchdogGrace is how long the clean Bubble Tea shutdown gets after the
// first termination signal before the watchdog force-exits. Bubble Tea catches
// SIGINT/SIGTERM and turns them into a QuitMsg sent on an internal channel with
// a BLOCKING send (tea.go handleSignals); if the event loop is wedged (a CPU
// spin or message flood) that send never lands, so the signal is swallowed and
// the process can only be killed with SIGKILL. A short grace lets a healthy TUI
// quit cleanly (flush session state, restore the terminal) while guaranteeing a
// wedged one still dies promptly.
const signalWatchdogGrace = 3 * time.Second

// Run starts the Zero Bubble Tea shell and returns a process-style exit code.
func Run(ctx context.Context, options Options) int {
	// The interactive shell needs a real terminal on stdin: with piped or
	// redirected input Bubble Tea blocks forever waiting for events that never
	// arrive (e.g. `echo "" | zero`). Fail fast with guidance toward the headless
	// path instead of hanging. term.IsTerminal is a true TTY check (it rejects
	// pipes, regular files, and non-terminal char devices like /dev/null) and
	// fails closed — anything that is not a verified terminal blocks the shell.
	if !term.IsTerminal(os.Stdin.Fd()) {
		fmt.Fprintln(os.Stderr, "zero: the interactive shell needs a terminal (stdin is not a TTY). For non-interactive use, run: zero exec \"<prompt>\"")
		return 2
	}

	externalSink := options.RuntimeMessageSink
	var program *tea.Program
	forward := func(msg tea.Msg) {
		if externalSink != nil {
			externalSink(msg)
		}
		if program != nil {
			program.Send(msg)
		}
	}
	// Coalesce streamed assistant-text deltas to ~one frame each so a fast provider
	// can't drive a full Update→View per token; every other message flushes pending
	// text first, keeping order intact.
	options.RuntimeMessageSink = newTextCoalescer(forward).send
	options.AltScreen = useAltScreen(options)

	// Guarantee the process can always be terminated. Bubble Tea catches
	// SIGINT/SIGTERM and turns them into a QuitMsg it sends on an unbuffered
	// channel; if the event loop is wedged (a CPU spin, or the input reader
	// spinning on a closed PTY after the terminal is closed) that send blocks
	// forever, so the signal is swallowed and only SIGKILL works — the "kez at
	// 120% CPU that ignores SIGTERM" zombie. SIGHUP (terminal closed) is not
	// caught by Bubble Tea at all. The watchdog cancels the program context so
	// its goroutines unwind, and force-exits if a wedged/orphaned loop can't quit
	// within the grace window.
	ctx, stopWatchdog := withTerminationWatchdog(ctx)
	defer stopWatchdog()

	programOpts := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
		tea.WithFilter(mouseEventFilter()),
	}
	// Honor the no-color.org spec ourselves: NO_COLOR set to ANY non-empty value
	// disables color. bubbletea/colorprofile only respects strconv.ParseBool-style
	// values, so NO_COLOR=yes / NO_COLOR=foo would otherwise leave the UI in full
	// color. Force the Ascii (no-color, bold-still-allowed) profile. (AUDIT-M3)
	if noColorRequested(os.Getenv) {
		programOpts = append(programOpts, tea.WithColorProfile(colorprofile.Ascii))
	}
	initialModel := newModel(ctx, options)
	if initialModel.wantsMouseCapture() {
		initialModel.mouseCapture = true
	}
	program = tea.NewProgram(initialModel, programOpts...)

	if _, err := program.Run(); err != nil {
		// Surface the failure: exiting 1 with zero diagnostics left users
		// guessing why the default chat surface died.
		fmt.Fprintln(os.Stderr, "zero: tui error:", err)
		return 1
	}
	return 0
}

// watchdogExit is os.Exit, indirected so tests can observe the force-exit
// without terminating the test process.
var watchdogExit = os.Exit

// withTerminationWatchdog derives a child context that is cancelled on a
// termination signal (SIGINT/SIGTERM/SIGHUP) and starts a goroutine that
// force-exits the process if the program does not return within
// signalWatchdogGrace of the first signal (or immediately on a second signal).
// The returned stop func must be called when the program exits normally; it
// releases the signal handler, cancels the context, and stops the watchdog so a
// clean quit never triggers a force-exit.
func withTerminationWatchdog(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	done := make(chan struct{})
	go runTerminationWatchdog(sig, done, signalWatchdogGrace, cancel, watchdogExit)

	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			signal.Stop(sig)
			close(done)
			cancel()
		})
	}
}

// runTerminationWatchdog is the watchdog's testable core. It waits for the first
// termination signal, cancels the program context so Bubble Tea can unwind, and
// then force-exits if the program has not signalled done within grace — or
// immediately on a second signal. Returns without exiting when done fires first
// (a clean shutdown).
func runTerminationWatchdog(sig <-chan os.Signal, done <-chan struct{}, grace time.Duration, cancel func(), exit func(int)) {
	select {
	case <-done:
		return
	case <-sig:
	}
	// First signal: ask the program to unwind, but don't trust a wedged loop to
	// process it. 130 = 128 + SIGINT, the conventional "terminated by signal" code.
	cancel()
	select {
	case <-done:
	case <-sig:
		exit(130)
	case <-time.After(grace):
		exit(130)
	}
}

func useAltScreen(_ Options) bool {
	return true
}

// noColorRequested reports whether the no-color.org spec is in effect: NO_COLOR set
// to any non-empty value. Checked here rather than via the colorprofile dependency,
// whose strconv.ParseBool gate ignores common values like NO_COLOR=yes. (AUDIT-M3)
func noColorRequested(getenv func(string) string) bool {
	return getenv("NO_COLOR") != ""
}
