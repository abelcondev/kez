//go:build darwin

package tui

import (
	"os/exec"
	"strings"
)

// osDarkAppearance reports the macOS system appearance. macOS exposes it through
// the global `AppleInterfaceStyle` default, which holds the string "Dark" in Dark
// mode and is ABSENT in Light mode (the read then exits non-zero). We rely on this
// instead of the terminal's OSC 11 background probe because some terminals — Zed's
// integrated terminal among them — never answer that query, leaving `auto` stuck on
// its dark default. ok is true whenever we could reach `defaults` at all (a missing
// key is a definitive "Light", not an error), so callers can treat the OS as the
// authoritative light/dark source on macOS.
func osDarkAppearance() (dark bool, ok bool) {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		// Non-zero exit is the documented Light-mode case: the key only exists in
		// Dark mode. (A truly broken `defaults` is indistinguishable and also maps
		// to Light, which is the safe, readable default on a light terminal.)
		return false, true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(string(out))), "dark"), true
}
