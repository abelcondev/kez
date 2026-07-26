//go:build !darwin

package tui

// osDarkAppearance is only implemented on macOS. Elsewhere there is no portable OS
// appearance signal, so we report "unknown" (ok=false) and let the terminal's OSC
// 11 background-color probe drive `auto` as before.
func osDarkAppearance() (dark bool, ok bool) { return false, false }
