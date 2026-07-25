package tui

import (
	"strings"

	"github.com/abelcondev/kez/internal/config"
)

// themeMode is the operator's palette preference.
type themeMode string

const (
	themeAuto  themeMode = "auto" // detect terminal background (default)
	themeDark  themeMode = "dark"
	themeLight themeMode = "light"
)

// themeModes lists the values /theme accepts, in picker order: `auto` first, then
// every registered theme (theme_palettes.go). It is the single ordered source
// feeding both the picker and the /theme state list — adding a registry entry
// extends it automatically.
var themeModes = append([]string{string(themeAuto)}, themeNames()...)

// resolveThemeMode picks the first accepted preference from candidates in
// precedence order — the caller passes them highest-first: the --theme flag, then
// KEZ_THEME, then the persisted config theme. A value is accepted if it is `auto`
// or names a registered theme; unrecognized/blank values are skipped, and an empty
// list (or all-unrecognized) falls back to auto.
func resolveThemeMode(candidates ...string) themeMode {
	for _, v := range candidates {
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "" {
			continue
		}
		if s == string(themeAuto) {
			return themeAuto
		}
		if _, ok := lookupTheme(s); ok {
			return themeMode(s)
		}
	}
	return themeAuto
}

// validThemeMode reports whether s names a theme mode (for /theme validation):
// `auto` or any registered theme.
func validThemeMode(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == string(themeAuto) {
		return true
	}
	_, ok := lookupTheme(s)
	return ok
}

// ValidThemeArg reports whether s is an acceptable --theme / KEZ_THEME value
// (`auto` or a registered theme name). Exported so the CLI flag validator shares
// this one source of truth instead of hardcoding the theme list.
func ValidThemeArg(s string) bool { return validThemeMode(s) }

// applyTheme swaps the active palette (zeroTheme) and the globals derived from it
// — the streaming-fade ramp and the static render cache — so a switch repaints
// every subsequent render. For themeAuto it resolves to dark/light from
// hasDarkBackground; explicit dark/light ignore it. Returns the concrete mode
// applied (never auto). Must run on the Bubble Tea update goroutine (or before the
// program starts), like every other zeroTheme access.
func applyTheme(mode themeMode, hasDarkBackground bool) themeMode {
	resolved := mode
	if mode == themeAuto {
		resolved = themeDark
		if !hasDarkBackground {
			resolved = themeLight
		}
	}
	// Resolve the (now concrete) mode to its registered palette; an unknown name
	// falls back to the dark built-in so a bad value can never leave zeroTheme unset.
	entry, ok := lookupTheme(string(resolved))
	if !ok {
		entry, _ = lookupTheme(string(themeDark))
	}
	zeroTheme = buildTheme(entry.Palette)
	rebuildStreamingFadePalette()
	if defaultRenderCache != nil {
		defaultRenderCache.clear() // old-palette entries must not be reused
	}
	return resolved
}

// previewSelectedTheme makes the live palette match the theme picker's current
// state: the highlighted mode when a row is selectable, or the committed
// m.themeMode when the filter matches nothing. Called on every change to the
// picker's selection or filter (arrow/wheel moves, mouse, and query typing) so the
// whole UI — and the overlay itself — always renders the mode the popup points at,
// and never strands on a stale preview. It only swaps the global zeroTheme via
// applyTheme; m.themeMode keeps the committed preference so Esc can restore it. A
// no-op unless a theme picker is open. Runs on the Update goroutine, like every
// zeroTheme access.
func (m model) previewSelectedTheme() {
	if m.picker == nil || m.picker.kind != pickerTheme {
		return
	}
	if item, ok := m.picker.current(); ok {
		applyTheme(themeMode(item.Value), m.hasDarkBg)
		return
	}
	// No selectable row (the filter matched nothing): fall back to the committed
	// theme rather than leaving the previous preview applied.
	m.restoreCommittedTheme()
}

// restoreCommittedTheme re-applies m.themeMode after a /theme preview is dismissed
// without choosing (Esc). Preview never wrote m.themeMode, so it still holds the
// real preference; re-applying repaints back to it.
func (m model) restoreCommittedTheme() {
	applyTheme(m.themeMode, m.hasDarkBg)
}

// handleThemeCommand implements /theme [name]: `list` shows state, a registered
// theme name (or `auto`) switches the active palette live. Bare `/theme` opens the
// picker at the dispatch layer, so it never reaches here empty. Mirrors handleStyleCommand.
func (m model) handleThemeCommand(args string) (model, string) {
	arg := strings.ToLower(strings.TrimSpace(args))
	if arg == "" || arg == "list" {
		return m, m.themeStateText()
	}
	if !validThemeMode(arg) {
		return m, "Theme\nUnknown theme: " + arg + " (use /theme with no argument to pick from the list)"
	}
	m.themeMode = themeMode(arg)
	resolved := applyTheme(m.themeMode, m.hasDarkBg)
	active := arg
	if m.themeMode == themeAuto {
		active = "auto (" + string(resolved) + ")"
	}
	lines := []string{
		"Theme",
		"active theme: " + active,
		"Already-printed scrollback keeps its previous colors; new output uses the new theme.",
	}
	// Commit path (both /theme <name> and the picker route through here): persist the
	// choice so it survives restart. Previews call applyTheme directly and never reach here.
	if note := m.persistThemePreference(); note != "" {
		lines = append(lines, note)
	}
	return m, strings.Join(lines, "\n")
}

// persistThemePreference writes the committed theme to user config so it is applied
// again at startup (via Options.SavedTheme -> resolveThemeMode). Best-effort: returns
// a short note to surface on failure, or "" on success / when there is no config
// path (e.g. tests). Never called from the live-preview path.
func (m model) persistThemePreference() string {
	if strings.TrimSpace(m.userConfigPath) == "" {
		return ""
	}
	if _, err := config.SetTheme(m.userConfigPath, string(m.themeMode)); err != nil {
		return "note: could not save theme preference (" + err.Error() + ")"
	}
	return ""
}

// themeStateText renders the /theme state view.
func (m model) themeStateText() string {
	active := string(m.themeMode)
	if m.themeMode == themeAuto {
		bg := "light"
		if m.hasDarkBg {
			bg = "dark"
		}
		active = "auto (" + bg + ")"
	}
	return renderCommandOutput(commandOutput{
		Title:  "Theme",
		Status: commandStatusOK,
		Sections: []commandSection{{
			Title: "State",
			Lines: []string{
				"active theme: " + active,
				"available: " + strings.Join(themeModes, ", "),
			},
		}},
		Hints: []string{"run /theme with no argument to open the picker, or /theme <name> to switch directly"},
	})
}
