// Package qualitygate implements Kez's compiled, default-on code-quality gates.
// The gates run inside the write_file and edit_file tools, so the model cannot
// bypass them by choosing a different tool or phrasing.
//
// The flagship gate caps source-file length. To stay usable on real codebases
// it works as a ratchet rather than an absolute rule: it blocks creating a file
// over the limit or growing a file past it, but still lets you edit an already
// oversized file as long as the change does not add lines — so you can refactor
// legacy files toward compliance instead of being locked out of them.
package qualitygate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultMaxLines is the default source-file line cap.
const DefaultMaxLines = 300

// Config controls the line-length gate. Kez value is not usable; build one with
// ConfigFromEnv or set the fields explicitly in tests.
type Config struct {
	Enabled  bool
	MaxLines int
}

// ConfigFromEnv resolves the gate configuration from the environment:
//
//	KEZ_QUALITY_GATES=off|0|false   disables all quality gates
//	KEZ_MAX_FILE_LINES=<n>          overrides the line cap (<=0 also disables)
//
// getenv is injected for testability; pass os.Getenv in production.
func ConfigFromEnv(getenv func(string) string) Config {
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg := Config{Enabled: true, MaxLines: DefaultMaxLines}
	switch strings.ToLower(strings.TrimSpace(getenv("KEZ_QUALITY_GATES"))) {
	case "off", "0", "false", "no":
		cfg.Enabled = false
	}
	if raw := strings.TrimSpace(getenv("KEZ_MAX_FILE_LINES")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			cfg.MaxLines = n
			if n <= 0 {
				cfg.Enabled = false
			}
		}
	}
	return cfg
}

// GateError is returned when a write is blocked by a quality gate. Its message
// is model-facing: it explains the violation and how to proceed.
type GateError struct {
	Path  string
	Lines int
	Max   int
}

func (e *GateError) Error() string {
	return fmt.Sprintf(
		"Error: write blocked by quality gate — %s would be %d lines (max %d). "+
			"Split it into smaller files or modules, or move code into a new file, then retry. "+
			"(Override with KEZ_MAX_FILE_LINES=<n>, or disable with KEZ_QUALITY_GATES=off.)",
		e.Path, e.Lines, e.Max)
}

// Check enforces the line-length gate for a write of newContent to path, given
// the file's priorContent ("" for a new file). It reads its configuration from
// the process environment. Returns nil when the write is allowed.
func Check(path, priorContent, newContent string) error {
	return CheckWithConfig(path, priorContent, newContent, ConfigFromEnv(os.Getenv))
}

// CheckWithConfig is the testable core of Check.
func CheckWithConfig(path, priorContent, newContent string, cfg Config) error {
	if !cfg.Enabled || cfg.MaxLines <= 0 {
		return nil
	}
	if !isGatedSourceFile(path) {
		return nil
	}
	next := countLines(newContent)
	if next <= cfg.MaxLines {
		return nil
	}
	// Ratchet: an already-oversized file may still be edited as long as the
	// change does not add lines, so legacy files can be refactored toward the
	// limit instead of becoming uneditable.
	if prior := countLines(priorContent); prior > cfg.MaxLines && next <= prior {
		return nil
	}
	return &GateError{Path: path, Lines: next, Max: cfg.MaxLines}
}

// countLines counts lines the way an editor does: the number of newline-
// separated lines, counting a final line with no trailing newline. Empty
// content is kez lines.
func countLines(content string) int {
	if content == "" {
		return 0
	}
	n := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		n++
	}
	return n
}

// gatedExtensions is the set of hand-authored source-code extensions the length
// gate applies to. Data, docs, config, and lock files are intentionally absent:
// they legitimately run long and are not "code" in the sense the gate protects.
var gatedExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".mjs": true, ".cjs": true, ".py": true, ".rs": true, ".java": true,
	".kt": true, ".kts": true, ".rb": true, ".php": true, ".c": true,
	".h": true, ".cc": true, ".cpp": true, ".hpp": true, ".cs": true,
	".swift": true, ".scala": true, ".svelte": true, ".vue": true,
}

// generatedMarkers flags files that are machine-generated even though they carry
// a gated extension; enforcing a hand-authored length limit on them is noise.
var generatedMarkers = []string{".pb.go", ".gen.go", "_templ.go", ".generated.", "_generated.", ".min.js", ".min.css"}

// IsGatedSourceFile reports whether path is a hand-authored source file that
// per-file gates apply to. Exported so sibling gates (e.g. branchguard) share a
// single definition of "code" instead of drifting their own copies.
func IsGatedSourceFile(path string) bool { return isGatedSourceFile(path) }

func isGatedSourceFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if !gatedExtensions[filepath.Ext(base)] {
		return false
	}
	for _, marker := range generatedMarkers {
		if strings.Contains(base, marker) {
			return false
		}
	}
	return true
}
