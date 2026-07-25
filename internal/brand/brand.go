// Package brand holds the single source of truth for kez's user-facing name and
// identity. Every human-visible mention of the product — help banners, usage
// lines, warning prefixes, the release repository — routes through these
// constants so the brand can never drift across the tree the way scattered
// string literals do.
//
// IMPORTANT: these constants are for DISPLAY only. They are deliberately NOT
// used to rename Go identifiers, package names, or environment-variable keys
// (those carry their own compatibility contracts). Route a literal through brand
// only when a human reads it.
package brand

const (
	// Name is the binary / command name as typed at a shell prompt (lowercase).
	Name = "kez"

	// DisplayName is the capitalized product name for prose ("kez model registry").
	DisplayName = "Kez"

	// Tagline is the one-line product descriptor shown at the top of `kez help`.
	Tagline = "Kez terminal coding agent"

	// Repository is the canonical GitHub "owner/repo" the CLI checks for releases
	// and links to for downloads.
	Repository = "abelcondev/kez"
)

// LogPrefix is the bracketed tag prepended to CLI warnings and diagnostics on
// stderr (e.g. "[kez] Prompt required..."), matching the historical "[zero]"
// convention with the current brand.
const LogPrefix = "[" + Name + "]"
