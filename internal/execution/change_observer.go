package execution

import (
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	maxObservedFiles    = 20_000
	maxHashedFileBytes  = 1 << 20
	maxTotalHashedBytes = 32 << 20
	// maxReportedIndividualChanges bounds how many per-file changes a single
	// command may surface. Beyond this the individual entries collapse into
	// per-top-level-directory aggregates, so a command run against a workspace
	// with a noisy background writer (a dev server rewriting generated files, a
	// bulk checkout/format) can never flood the tool result — and the FILES
	// sidebar that re-scans every changed path each frame — with tens of
	// thousands of rows.
	maxReportedIndividualChanges = 500
)

type ChangeKind string

const (
	ChangeCreated  ChangeKind = "created"
	ChangeModified ChangeKind = "modified"
	ChangeDeleted  ChangeKind = "deleted"
)

type Change struct {
	Path       string     `json:"path"`
	Kind       ChangeKind `json:"kind"`
	Aggregated bool       `json:"aggregated,omitempty"`
	Count      int        `json:"count,omitempty"`
}

type fileFingerprint struct {
	Mode    fs.FileMode
	Size    int64
	ModTime int64
	Digest  [sha256.Size]byte
	Hashed  bool
	// Aggregated marks a bounded fingerprint for a generated directory. The
	// observer never enumerates that tree into individual changes.
	Aggregated bool
	Count      int
}

// ChangeObserver records a bounded workspace snapshot around a command. It
// deliberately skips Zero/repository control metadata and large generated
// dependency trees; those paths must neither be read as command evidence nor
// flood the Files panel.
type ChangeObserver struct {
	root   string
	ignore *ignoreMatcher
	before map[string]fileFingerprint
	valid  bool
}

func NewChangeObserver(root string) *ChangeObserver {
	root = filepath.Clean(strings.TrimSpace(root))
	// Load the ignore rules once and reuse them for both the before and after
	// snapshots so a mid-command .gitignore edit can't desync the two passes.
	ignore := loadIgnoreMatcher(root)
	before, ok := snapshotWorkspace(root, ignore)
	return &ChangeObserver{root: root, ignore: ignore, before: before, valid: ok}
}

func (observer *ChangeObserver) Changes() []Change {
	if observer == nil || !observer.valid {
		return nil
	}
	after, ok := snapshotWorkspace(observer.root, observer.ignore)
	if !ok {
		return nil
	}
	changes := make([]Change, 0)
	for path, current := range after {
		previous, existed := observer.before[path]
		if !existed {
			changes = append(changes, changeFromFingerprint(path, ChangeCreated, current))
			continue
		}
		if previous != current {
			changes = append(changes, changeFromFingerprint(path, ChangeModified, current))
		}
	}
	for path, previous := range observer.before {
		if _, exists := after[path]; !exists {
			changes = append(changes, changeFromFingerprint(path, ChangeDeleted, previous))
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return boundChanges(changes)
}

// boundChanges caps the number of individual (non-aggregated) file changes. Under
// the budget it returns the slice unchanged. Over it, the individual changes
// collapse into one aggregated entry per top-level directory (root files under
// "./"), so the reported count stays proportional to the number of touched
// directories, not the number of touched files. Pre-existing aggregated tree
// summaries pass through untouched.
func boundChanges(changes []Change) []Change {
	individual := 0
	for i := range changes {
		if !changes[i].Aggregated {
			individual++
		}
	}
	if individual <= maxReportedIndividualChanges {
		return changes
	}
	type aggregate struct {
		kind  ChangeKind
		count int
	}
	groups := map[string]*aggregate{}
	order := make([]string, 0)
	kept := make([]Change, 0, len(changes))
	for _, change := range changes {
		if change.Aggregated {
			kept = append(kept, change)
			continue
		}
		key := topLevelDirectory(change.Path)
		group, ok := groups[key]
		if !ok {
			group = &aggregate{kind: change.Kind}
			groups[key] = group
			order = append(order, key)
		}
		group.count++
	}
	for _, key := range order {
		group := groups[key]
		kept = append(kept, Change{Path: key, Kind: group.kind, Aggregated: true, Count: group.count})
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Path < kept[j].Path })
	return kept
}

// topLevelDirectory returns the first path segment with a trailing slash
// ("src/api/x.ts" → "src/"), or "./" for a file sitting directly in the
// workspace root. Paths use forward slashes (snapshotWorkspace normalizes them).
func topLevelDirectory(path string) string {
	path = filepath.ToSlash(path)
	if index := strings.IndexByte(path, '/'); index >= 0 {
		return path[:index+1]
	}
	return "./"
}

func snapshotWorkspace(root string, ignore *ignoreMatcher) (map[string]fileFingerprint, bool) {
	if root == "" || root == "." {
		return nil, false
	}
	files := make(map[string]fileFingerprint)
	hashedBytes := int64(0)
	ok := true
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil
		}
		slashRelative := filepath.ToSlash(relative)
		if entry.IsDir() {
			if protectedObservationDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			// A git-ignored directory is generated/ephemeral: prune it entirely so
			// a background dev server rewriting it can't attribute spurious changes
			// to every command. Unlike generatedObservationDirectory it is not even
			// recorded as an aggregated tree, since the user never tracks it.
			if ignore.ignored(slashRelative, true) {
				return filepath.SkipDir
			}
			if generatedObservationDirectory(entry.Name()) {
				files[slashRelative+"/"] = generatedTreeFingerprint(path)
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if ignore.ignored(slashRelative, false) {
			return nil
		}
		if len(files) >= maxObservedFiles {
			ok = false
			return fs.SkipAll
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		fingerprint := fileFingerprint{Mode: info.Mode(), Size: info.Size(), ModTime: info.ModTime().UnixNano()}
		if info.Size() <= maxHashedFileBytes && hashedBytes+info.Size() <= maxTotalHashedBytes {
			if digest, err := hashObservedFile(path); err == nil {
				fingerprint.Digest = digest
				fingerprint.Hashed = true
				hashedBytes += info.Size()
			}
		}
		files[slashRelative] = fingerprint
		return nil
	})
	if err != nil {
		return nil, false
	}
	return files, ok
}

func protectedObservationDirectory(name string) bool {
	switch name {
	case ".git", ".kez", ".agents":
		return true
	default:
		return false
	}
}

// generatedObservationDirectory reports whether a directory is machine-generated
// build output, a dependency install tree, or a dev-server/framework scratch
// dir. Those are recorded as a single aggregated tree fingerprint and never
// descended into: a running dev server (Vite/SvelteKit/Next/Nuxt/Astro/etc.)
// continuously rewrites files under these paths in the background, so
// enumerating them would attribute thousands of spurious per-file changes to
// every command the agent runs — flooding the tool result and the FILES sidebar
// that re-scans each changed path on every frame, which used to grind the whole
// TUI to a halt while a dev server was up.
func generatedObservationDirectory(name string) bool {
	switch name {
	case
		// Dependency install trees / package-manager caches.
		"node_modules", ".pnpm-store", ".yarn", ".npm", "bower_components",
		// JS/TS build output, framework caches, and dev-server scratch dirs.
		"dist", "build", "out", ".next", ".nuxt", ".svelte-kit", ".astro",
		".vite", ".turbo", ".parcel-cache", ".cache", ".output", ".vercel",
		".netlify", ".docusaurus", ".angular", ".expo",
		// Cloudflare Workers local runtime state: wrangler/miniflare persist KV,
		// D1 (SQLite), R2, cache, and durable-object storage under these and
		// rewrite them on every request — a running `wrangler dev` churns them
		// constantly, independent of what the agent edits.
		".wrangler", ".mf", ".miniflare",
		// Test / coverage artifacts.
		"coverage", ".nyc_output",
		// Other-language build output, caches, and virtualenvs.
		"target", ".gradle", ".terraform", ".serverless",
		"__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache", ".tox",
		".venv", "venv":
		return true
	default:
		return false
	}
}

// generatedTreeFingerprint samples only direct children and caps the work. It
// is intentionally qualitative: the UI needs to say that a generated tree
// changed, not inventory dependency contents.
func generatedTreeFingerprint(root string) fileFingerprint {
	directory, err := os.Open(root)
	if err != nil {
		return fileFingerprint{Aggregated: true, Count: -1}
	}
	defer directory.Close()
	const maxEntries = 256
	entries, err := directory.ReadDir(maxEntries + 1)
	if err != nil && err != io.EOF {
		return fileFingerprint{Aggregated: true, Count: -1}
	}
	hash := sha256.New()
	count := len(entries)
	if count > maxEntries {
		entries = entries[:maxEntries]
		count = -1
	}
	for _, entry := range entries {
		_, _ = io.WriteString(hash, entry.Name())
		if info, infoErr := entry.Info(); infoErr == nil {
			_, _ = io.WriteString(hash, info.Mode().String())
			_, _ = io.WriteString(hash, strconv.FormatInt(info.ModTime().UnixNano(), 10))
		}
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return fileFingerprint{Digest: digest, Hashed: true, Aggregated: true, Count: count}
}

func changeFromFingerprint(path string, kind ChangeKind, fingerprint fileFingerprint) Change {
	return Change{Path: path, Kind: kind, Aggregated: fingerprint.Aggregated, Count: fingerprint.Count}
}

func hashObservedFile(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}
