package execution

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignoreMatcher matches workspace-relative paths against the repository's ignore
// rules, so the change observer never walks, hashes, or reports files git would
// ignore — the universal "generated / ephemeral" signal. It reads the top-level
// .gitignore and .git/info/exclude only: that captures the generated and
// dev-server-churned paths that flood a command's change list, without the cost
// and subtlety of a full per-directory gitignore engine. Rules from nested
// .gitignore files are not read; the change observer's hard-coded generated-dir
// list (generatedObservationDirectory) remains the by-basename safety net that
// catches known trees at any depth, so the two together cover the space.
//
// Supported syntax: blank/# comment lines, a leading ! negation, a trailing /
// directory-only marker, a leading / or embedded / anchoring to the workspace
// root, and *, ?, and [] globs (per path segment, via path.Match). ** is not
// specially handled and last-match-wins ordering is honored. Negations that
// re-include a child of an ignored directory are not resolved, because ignored
// directories are pruned from the walk wholesale; this is acceptable given the
// hard-coded safety net and how rarely generated trees carry re-includes.
type ignoreMatcher struct {
	rules []ignoreRule
}

type ignoreRule struct {
	pattern  string
	negate   bool
	dirOnly  bool
	anchored bool
}

// loadIgnoreMatcher builds a matcher from root/.gitignore and
// root/.git/info/exclude. A nil matcher (no rules, or an unreadable root) is
// safe: matches always return false, so the observer falls back to its
// hard-coded skips. Never errors — ignore rules are best-effort.
func loadIgnoreMatcher(root string) *ignoreMatcher {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	matcher := &ignoreMatcher{}
	matcher.appendFile(filepath.Join(root, ".gitignore"))
	matcher.appendFile(filepath.Join(root, ".git", "info", "exclude"))
	if len(matcher.rules) == 0 {
		return nil
	}
	return matcher
}

func (matcher *ignoreMatcher) appendFile(pathname string) {
	file, err := os.Open(pathname)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if rule, ok := parseIgnoreLine(scanner.Text()); ok {
			matcher.rules = append(matcher.rules, rule)
		}
	}
}

// parseIgnoreLine compiles one .gitignore line into a rule. ok is false for blank
// lines and comments.
func parseIgnoreLine(line string) (ignoreRule, bool) {
	line = strings.TrimRight(line, " \t\r")
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	rule := ignoreRule{}
	if strings.HasPrefix(line, "!") {
		rule.negate = true
		line = line[1:]
	}
	// A leading backslash escapes a literal '#' or '!'.
	if strings.HasPrefix(line, `\#`) || strings.HasPrefix(line, `\!`) {
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		rule.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		rule.anchored = true
		line = strings.TrimPrefix(line, "/")
	} else if strings.Contains(line, "/") {
		// A slash anywhere but the end anchors the pattern to the root, per
		// gitignore semantics.
		rule.anchored = true
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return ignoreRule{}, false
	}
	rule.pattern = filepath.ToSlash(line)
	return rule, true
}

// ignored reports whether the forward-slash workspace-relative path rel is
// ignored. Rules are evaluated in order and the last match wins, so a later
// negation can re-include an earlier ignore.
func (matcher *ignoreMatcher) ignored(rel string, isDir bool) bool {
	if matcher == nil || rel == "" {
		return false
	}
	rel = filepath.ToSlash(rel)
	matched := false
	for _, rule := range matcher.rules {
		if rule.dirOnly && !isDir {
			continue
		}
		if rule.matches(rel) {
			matched = !rule.negate
		}
	}
	return matched
}

func (rule ignoreRule) matches(rel string) bool {
	if rule.anchored {
		if ok, _ := path.Match(rule.pattern, rel); ok {
			return true
		}
		// An anchored directory pattern also matches everything beneath it.
		return strings.HasPrefix(rel, rule.pattern+"/")
	}
	// Unanchored: match the basename at any depth, and also allow the pattern to
	// match a trailing path segment run (e.g. "a/b" matching "x/a/b").
	if ok, _ := path.Match(rule.pattern, path.Base(rel)); ok {
		return true
	}
	if ok, _ := path.Match(rule.pattern, rel); ok {
		return true
	}
	if idx := strings.Index(rel, "/"+rule.pattern); idx >= 0 {
		tail := rel[idx+1:]
		if ok, _ := path.Match(rule.pattern, tail); ok {
			return true
		}
	}
	return false
}
