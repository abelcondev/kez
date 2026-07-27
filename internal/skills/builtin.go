package skills

import (
	"embed"
	"sort"
)

// builtinFS holds the skills compiled into the kez binary. Unlike disk skills
// (installed under ~/.local/share/kez/skills), built-ins ship inside the binary
// itself, so any curated workflow is always available in a fresh checkout with no
// skills directory and cannot be lost by a missing or unconfigured skills dir.
//
// The set is currently empty: the new-app workflow was retired in favor of the
// always-on SDD loop advisor (`kez sdd next`) plus the guidance seeded into each
// project's sdd/index.md. The `all:` embed keeps compiling with only the .keep
// placeholder present, so a future built-in can be dropped into builtin/ with no
// other change.
//
//go:embed all:builtin
var builtinFS embed.FS

// Builtin returns the skills baked into the binary, parsed like disk skills and
// sorted by name. Each carries a "builtin:<name>" Path so its provenance is
// clear in listings; the skill body is served from memory, never re-read from
// disk. A malformed or unreadable embedded entry is skipped rather than failing
// the whole set (matching the disk loader's fail-open behavior).
func Builtin() []Skill {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil
	}
	out := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest := "builtin/" + entry.Name() + "/" + skillFileName
		data, err := builtinFS.ReadFile(manifest)
		if err != nil {
			continue
		}
		out = append(out, parseSkill(entry.Name(), "builtin:"+entry.Name(), string(data)))
	}
	sort.Slice(out, func(left int, right int) bool {
		return out[left].Name < out[right].Name
	})
	return out
}

// MergeBuiltins overlays the built-in skills onto a disk-discovered list. Disk
// skills WIN a name clash — a user can override a built-in by installing their
// own skill of the same name — so built-ins are the lowest-priority source. When
// keepContent is false, every returned skill has its body stripped (for listing
// callers); pass true to retain bodies (for the skill tool). The result is
// sorted by name.
//
// This is deliberately a separate helper rather than being folded into
// LoadFromRoots/ListFromRoots: those loaders stay pure so their exact-count
// tests hold, and only the production wiring (plugin skill merge, `kez skills
// list`) opts into built-ins.
func MergeBuiltins(disk []Skill, keepContent bool) []Skill {
	seen := make(map[string]bool, len(disk))
	out := make([]Skill, 0, len(disk)+len(builtinNames()))
	for _, skill := range disk {
		seen[skill.Name] = true
		if !keepContent {
			skill.Content = ""
		}
		out = append(out, skill)
	}
	for _, skill := range Builtin() {
		if seen[skill.Name] {
			continue
		}
		if !keepContent {
			skill.Content = ""
		}
		out = append(out, skill)
	}
	sort.Slice(out, func(left int, right int) bool {
		return out[left].Name < out[right].Name
	})
	return out
}

// builtinNames is a tiny helper so MergeBuiltins can size its slice without
// re-parsing bodies twice; it returns just the built-in skill names.
func builtinNames() []string {
	built := Builtin()
	names := make([]string, len(built))
	for index, skill := range built {
		names[index] = skill.Name
	}
	return names
}
