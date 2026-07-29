package agent

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/abelcondev/kez/internal/sandbox"
	"github.com/abelcondev/kez/internal/tools"
	"mvdan.cc/sh/v3/syntax"
)

func proposedCommandPrefix(toolName string, args map[string]any) []string {
	if !isShellCommandTool(toolName) {
		return nil
	}
	command, ok := firstStringArg(args, "command", "cmd", "script", "shell")
	if !ok {
		return nil
	}
	segments, ok := safeShellCommandSegments(command)
	if !ok {
		return nil
	}
	if requested, ok := requestedPrefixRule(args); ok {
		if safeRequestedPrefixForSegments(requested, segments) {
			return requested
		}
		return nil
	}
	// Only propose approving a prefix of one segment when every other segment
	// in the command is independently known-safe. Once a prefix is approved,
	// shellExecutionArgsForApproval escalates the whole command (every
	// segment, not just the approved one) to bypass the sandbox, so offering a
	// prefix that leaves an MSYS-prone (or otherwise unsafe) segment uncovered
	// would let that segment run unsandboxed without ever being reviewed.
	for index, tokens := range segments {
		if knownSafeCommandSegment(tokens) {
			continue
		}
		if !sandbox.ValidCommandPrefix(tokens) || !otherSegmentsKnownSafe(segments, index) {
			return nil
		}
		return append([]string(nil), tokens...)
	}
	if len(segments) == 0 || !sandbox.ValidCommandPrefix(segments[0]) {
		return nil
	}
	return append([]string(nil), segments[0]...)
}

// otherSegmentsKnownSafe reports whether every segment other than the one at
// skip is known-safe on its own.
func otherSegmentsKnownSafe(segments [][]string, skip int) bool {
	for index, tokens := range segments {
		if index == skip {
			continue
		}
		if !knownSafeCommandSegment(tokens) {
			return false
		}
	}
	return true
}

func matchCommandPrefix(toolName string, args map[string]any, options Options) (sandbox.CommandPrefixGrant, bool, bool) {
	if !isShellCommandTool(toolName) || options.Sandbox == nil {
		return sandbox.CommandPrefixGrant{}, false, false
	}
	if shellCommandAdditionalPermissionsRequested(args) {
		return sandbox.CommandPrefixGrant{}, false, false
	}
	command, ok := firstStringArg(args, "command", "cmd", "script", "shell")
	if !ok {
		return sandbox.CommandPrefixGrant{}, false, false
	}
	segments, ok := safeShellCommandSegments(command)
	if !ok {
		return sandbox.CommandPrefixGrant{}, false, false
	}
	var matched sandbox.CommandPrefixGrant
	matchedAny := false
	matchedSession := false
	for _, tokens := range segments {
		if grant, ok := options.Sandbox.LookupCommandPrefix(toolName, tokens); ok {
			if !matchedAny {
				matched = grant
			}
			matchedAny = true
			continue
		}
		if grant, ok := options.Sandbox.LookupCommandPrefixForSession(toolName, tokens); ok {
			if !matchedAny {
				matched = grant
			}
			matchedAny = true
			matchedSession = true
			continue
		}
		if knownSafeCommandSegment(tokens) {
			continue
		}
		return sandbox.CommandPrefixGrant{}, false, false
	}
	if matchedAny {
		return matched, true, matchedSession
	}
	return sandbox.CommandPrefixGrant{}, false, false
}

func shellExecutionArgsForApproval(toolName string, args map[string]any, action PermissionDecisionAction, options Options) map[string]any {
	if !isShellCommandTool(toolName) || !shellPrefixApprovalBypassesSandbox(action) {
		return args
	}
	if options.Sandbox == nil || !options.Sandbox.UnsandboxedExecutionAllowed() {
		return args
	}
	if shellCommandAdditionalPermissionsRequested(args) || shellCommandRequiresEscalated(args) {
		return args
	}
	planned := cloneArgs(args)
	if planned == nil {
		planned = map[string]any{}
	}
	planned["sandbox_permissions"] = string(tools.SandboxPermissionsRequireEscalated)
	return planned
}

func shellPrefixApprovalBypassesSandbox(action PermissionDecisionAction) bool {
	return action == PermissionDecisionAllowPrefix || action == PermissionDecisionAlwaysAllowPrefix
}

func shellCommandRequiresEscalated(args map[string]any) bool {
	raw, ok := args["sandbox_permissions"]
	if !ok || raw == nil {
		return false
	}
	value, ok := raw.(string)
	if !ok {
		value = fmt.Sprint(raw)
	}
	return strings.TrimSpace(value) == string(tools.SandboxPermissionsRequireEscalated)
}

func isShellCommandTool(toolName string) bool {
	return toolName == "bash" || toolName == "exec_command"
}

// autoEscalateVCSCommand reports whether a shell command is a well-known
// version-control forge network operation that should auto-escalate to run
// unsandboxed, so it reaches the real HOME + credential store (macOS keychain,
// git credential helpers, gh auth) that the sandbox hides behind a virtual HOME
// and scrubbed tokens. It is conservative: every segment must be either a
// forge-network op (gh, git push/fetch/pull/clone/ls-remote) or an independently
// known-safe segment, and at least one must be a forge op. So `git push` and
// `gh pr create` escalate, but `git push && npm run deploy` or `git fetch | evil`
// do not — the unsafe segment keeps the whole command on the normal sandboxed
// path. Commands that already carry sandbox_permissions are left untouched.
func autoEscalateVCSCommand(toolName string, args map[string]any) bool {
	if !isShellCommandTool(toolName) {
		return false
	}
	if shellCommandAdditionalPermissionsRequested(args) || shellCommandRequiresEscalated(args) {
		return false
	}
	command, ok := firstStringArg(args, "command", "cmd", "script", "shell")
	if !ok {
		return false
	}
	segments, ok := forgeCommandSegments(command)
	if !ok {
		return false
	}
	matchedForge := false
	for _, tokens := range segments {
		if vcsForgeNetworkSegment(tokens) {
			matchedForge = true
			continue
		}
		if knownSafeCommandSegment(tokens) {
			continue
		}
		return false
	}
	return matchedForge
}

// forgeCommandSegments parses a command into simple-command segments for the
// auto-escalation check. It shares safeShellCommandSegments' structural safety
// (no redirects, background, negation, coprocess; only &&/||/| chaining and
// plain calls — never a subshell or block that could hide a command) but reads
// tokens tolerantly: quoted string literals are accepted because they are inert
// data passed to gh/git (a jq filter `-q '.title'`, a PR `--body "text"`),
// whereas safeShellCommandSegments rejects them and thus never escalated any real
// `gh pr create`. Any command/parameter/arithmetic/process substitution still
// fails the parse — those could execute or inject something into what would run
// unsandboxed.
func forgeCommandSegments(command string) ([][]string, bool) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, false
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil || len(file.Stmts) == 0 {
		return nil, false
	}
	segments := make([][]string, 0, len(file.Stmts))
	for _, stmt := range file.Stmts {
		if !collectForgeStatement(stmt, &segments) {
			return nil, false
		}
	}
	if len(segments) == 0 {
		return nil, false
	}
	return segments, true
}

func collectForgeStatement(stmt *syntax.Stmt, segments *[][]string) bool {
	if stmt == nil || stmt.Negated || stmt.Background || stmt.Coprocess || stmt.Disown || len(stmt.Redirs) > 0 {
		return false
	}
	return collectForgeCommand(stmt.Cmd, segments)
}

func collectForgeCommand(cmd syntax.Command, segments *[][]string) bool {
	switch node := cmd.(type) {
	case *syntax.CallExpr:
		tokens, ok := forgeCallTokens(node)
		if !ok {
			return false
		}
		*segments = append(*segments, tokens)
		return true
	case *syntax.BinaryCmd:
		switch node.Op {
		case syntax.AndStmt, syntax.OrStmt, syntax.Pipe:
		default:
			return false
		}
		return collectForgeStatement(node.X, segments) && collectForgeStatement(node.Y, segments)
	default:
		return false
	}
}

func forgeCallTokens(call *syntax.CallExpr) ([]string, bool) {
	if call == nil || len(call.Assigns) > 0 || len(call.Args) == 0 {
		return nil, false
	}
	tokens := make([]string, 0, len(call.Args))
	for _, word := range call.Args {
		value, ok := literalWordValue(word)
		if !ok {
			return nil, false
		}
		tokens = append(tokens, value)
	}
	return tokens, true
}

// literalWordValue returns the static text of a word when it carries no runtime
// expansion — bare literals plus single- and double-quoted string literals. It
// fails on any command/parameter/arithmetic/process substitution (e.g. $(...),
// $VAR, `...`), including inside double quotes, so an escalated command can never
// smuggle in something that runs or expands at execution time.
func literalWordValue(word *syntax.Word) (string, bool) {
	if word == nil || len(word.Parts) == 0 {
		return "", false
	}
	var builder strings.Builder
	for _, part := range word.Parts {
		switch node := part.(type) {
		case *syntax.Lit:
			builder.WriteString(node.Value)
		case *syntax.SglQuoted:
			builder.WriteString(node.Value)
		case *syntax.DblQuoted:
			for _, inner := range node.Parts {
				lit, ok := inner.(*syntax.Lit)
				if !ok {
					return "", false
				}
				builder.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return builder.String(), true
}

// vcsForgeNetworkSegment reports whether a single command segment is a git-forge
// network operation that needs credential access: any gh subcommand, or git
// push/fetch/pull/clone/ls-remote.
func vcsForgeNetworkSegment(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	switch commandName(tokens[0]) {
	case "gh":
		// Bare `gh` just prints help; require a subcommand so an empty invocation
		// does not needlessly escalate.
		return len(tokens) >= 2
	case "git":
		return gitNetworkSubcommand(tokens)
	default:
		return false
	}
}

// gitNetworkSubcommand reports whether a git invocation is a network subcommand
// (push/fetch/pull/clone/ls-remote), skipping leading global options the same way
// gitSubcommand does. It first rejects any --upload-pack/--receive-pack option
// (which can name an arbitrary program to run for local-path remotes), wherever
// it appears, so auto-escalation cannot be abused to run something unsandboxed
// without review.
func gitNetworkSubcommand(command []string) bool {
	for _, arg := range command[1:] {
		if strings.HasPrefix(arg, "--upload-pack") || strings.HasPrefix(arg, "--receive-pack") {
			return false
		}
	}
	for index := 1; index < len(command); index++ {
		arg := command[index]
		if gitOptionConsumesValue(arg) {
			index++
			continue
		}
		if gitOptionHasInlineValue(arg) || arg == "--" || strings.HasPrefix(arg, "-") {
			continue
		}
		switch arg {
		case "push", "fetch", "pull", "clone", "ls-remote":
			return true
		default:
			return false
		}
	}
	return false
}

func firstStringArg(args map[string]any, names ...string) (string, bool) {
	for _, name := range names {
		if raw, ok := args[name].(string); ok {
			value := strings.TrimSpace(raw)
			if value != "" {
				return value, true
			}
		}
	}
	return "", false
}

func requestedPrefixRule(args map[string]any) ([]string, bool) {
	raw, ok := args["prefix_rule"]
	if !ok {
		return nil, false
	}
	switch value := raw.(type) {
	case []string:
		return cleanPrefixRule(value), true
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			part, ok := item.(string)
			if !ok {
				return nil, true
			}
			out = append(out, part)
		}
		return cleanPrefixRule(out), true
	default:
		return nil, true
	}
}

func cleanPrefixRule(prefix []string) []string {
	cleaned := make([]string, 0, len(prefix))
	for _, part := range prefix {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil
		}
		cleaned = append(cleaned, part)
	}
	return cleaned
}

func safeRequestedPrefixForSegments(prefix []string, segments [][]string) bool {
	if len(prefix) == 0 || !sandbox.ValidCommandPrefix(prefix) {
		return false
	}
	matched := false
	for _, command := range segments {
		if len(prefix) > len(command) {
			if knownSafeCommandSegment(command) {
				continue
			}
			return false
		}
		if hasStringPrefix(command, prefix) {
			matched = true
			continue
		}
		if knownSafeCommandSegment(command) {
			continue
		}
		return false
	}
	return matched
}

func safeShellCommandSegments(command string) ([][]string, bool) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, false
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil || len(file.Stmts) == 0 {
		return nil, false
	}
	segments := make([][]string, 0, len(file.Stmts))
	for _, stmt := range file.Stmts {
		if !collectSafeShellStatement(stmt, &segments) {
			return nil, false
		}
	}
	if len(segments) == 0 {
		return nil, false
	}
	return segments, true
}

func collectSafeShellStatement(stmt *syntax.Stmt, segments *[][]string) bool {
	if stmt == nil || stmt.Negated || stmt.Background || stmt.Coprocess || stmt.Disown || len(stmt.Redirs) > 0 {
		return false
	}
	return collectSafeShellCommand(stmt.Cmd, segments)
}

func collectSafeShellCommand(cmd syntax.Command, segments *[][]string) bool {
	switch node := cmd.(type) {
	case *syntax.CallExpr:
		tokens, ok := literalCallTokens(node)
		if !ok {
			return false
		}
		*segments = append(*segments, tokens)
		return true
	case *syntax.BinaryCmd:
		switch node.Op {
		case syntax.AndStmt, syntax.OrStmt, syntax.Pipe:
		default:
			return false
		}
		return collectSafeShellStatement(node.X, segments) && collectSafeShellStatement(node.Y, segments)
	default:
		return false
	}
}

func literalCallTokens(call *syntax.CallExpr) ([]string, bool) {
	if call == nil || len(call.Assigns) > 0 || len(call.Args) == 0 {
		return nil, false
	}
	tokens := make([]string, 0, len(call.Args))
	for _, word := range call.Args {
		if len(word.Parts) != 1 {
			return nil, false
		}
		lit, ok := word.Parts[0].(*syntax.Lit)
		if !ok || strings.ContainsAny(lit.Value, "*?[]{}") {
			return nil, false
		}
		tokens = append(tokens, lit.Value)
	}
	return tokens, true
}

func knownSafeCommandSegment(command []string) bool {
	if len(command) == 0 {
		return false
	}
	name := commandName(command[0])
	if runtime.GOOS == "windows" && tools.MsysProneCommandName(name) {
		return false
	}
	switch name {
	case "cat", "cd", "cut", "echo", "expr", "false", "grep", "head", "id",
		"ls", "nl", "paste", "pwd", "rev", "seq", "stat", "tail", "tr",
		"true", "uname", "uniq", "wc", "which", "whoami":
		return true
	case "base64":
		return safeBase64Command(command[1:])
	case "find":
		return safeFindCommand(command[1:])
	case "rg":
		return safeRipgrepCommand(command[1:])
	case "sed":
		return safeSedCommand(command[1:])
	case "git":
		return safeGitCommand(command)
	default:
		return false
	}
}

func safeBase64Command(args []string) bool {
	for _, arg := range args {
		if arg == "-o" || arg == "--output" || strings.HasPrefix(arg, "--output=") || (strings.HasPrefix(arg, "-o") && arg != "-o") {
			return false
		}
	}
	return true
}

func safeFindCommand(args []string) bool {
	unsafe := map[string]bool{
		"-exec": true, "-execdir": true, "-ok": true, "-okdir": true,
		"-delete": true,
		"-fls":    true, "-fprint": true, "-fprint0": true, "-fprintf": true,
	}
	for _, arg := range args {
		if unsafe[arg] {
			return false
		}
	}
	return true
}

func safeRipgrepCommand(args []string) bool {
	for _, arg := range args {
		if arg == "--search-zip" || arg == "-z" || arg == "--pre" || arg == "--hostname-bin" ||
			strings.HasPrefix(arg, "--pre=") || strings.HasPrefix(arg, "--hostname-bin=") {
			return false
		}
	}
	return true
}

func safeSedCommand(args []string) bool {
	if len(args) < 2 || len(args) > 3 || args[0] != "-n" {
		return false
	}
	return validSedPrintArg(args[1])
}

func validSedPrintArg(arg string) bool {
	if !strings.HasSuffix(arg, "p") {
		return false
	}
	body := strings.TrimSuffix(arg, "p")
	if body == "" {
		return false
	}
	for _, part := range strings.Split(body, ",") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func safeGitCommand(command []string) bool {
	subIndex, subcommand, ok := gitSubcommand(command)
	if !ok {
		return false
	}
	if gitHasUnsafeGlobalOption(command[1:subIndex]) {
		return false
	}
	args := command[subIndex+1:]
	switch subcommand {
	case "status", "log", "diff", "show":
		return gitArgsReadOnly(args)
	case "branch":
		return gitArgsReadOnly(args) && gitBranchReadOnly(args)
	default:
		return false
	}
}

func gitSubcommand(command []string) (int, string, bool) {
	for index := 1; index < len(command); index++ {
		arg := command[index]
		if gitOptionConsumesValue(arg) {
			index++
			continue
		}
		if gitOptionHasInlineValue(arg) || arg == "--" || strings.HasPrefix(arg, "-") {
			continue
		}
		switch arg {
		case "status", "log", "diff", "show", "branch":
			return index, arg, true
		default:
			return 0, "", false
		}
	}
	return 0, "", false
}

func gitOptionConsumesValue(arg string) bool {
	switch arg {
	case "-C", "-c", "--config-env", "--exec-path", "--git-dir", "--namespace", "--super-prefix", "--work-tree":
		return true
	default:
		return false
	}
}

func gitOptionHasInlineValue(arg string) bool {
	return strings.HasPrefix(arg, "--config-env=") ||
		strings.HasPrefix(arg, "--exec-path=") ||
		strings.HasPrefix(arg, "--git-dir=") ||
		strings.HasPrefix(arg, "--namespace=") ||
		strings.HasPrefix(arg, "--super-prefix=") ||
		strings.HasPrefix(arg, "--work-tree=") ||
		((strings.HasPrefix(arg, "-C") || strings.HasPrefix(arg, "-c")) && len(arg) > 2)
}

func gitHasUnsafeGlobalOption(args []string) bool {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--upload-pack"):
			return true
		case arg == "-C" || strings.HasPrefix(arg, "-C"):
			return true
		case gitOptionConsumesValue(arg):
			index++
		}
	}
	return false
}

func gitArgsReadOnly(args []string) bool {
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--output="), strings.HasPrefix(arg, "--exec="):
			return false
		case arg == "--output", arg == "--exec":
			return false
		}
	}
	return true
}

func gitBranchReadOnly(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--format=") {
			continue
		}
		switch arg {
		case "--list", "-l", "--show-current", "-a", "--all", "-r", "--remotes", "-v", "-vv", "--verbose":
			continue
		default:
			return false
		}
	}
	return true
}

func commandName(raw string) string {
	name := strings.TrimSpace(raw)
	if index := strings.LastIndexAny(name, `/\`); index >= 0 {
		name = name[index+1:]
	}
	name = strings.ToLower(name)
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".com"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

func hasStringPrefix(values []string, prefix []string) bool {
	if len(prefix) == 0 || len(prefix) > len(values) {
		return false
	}
	for index := range prefix {
		if values[index] != prefix[index] {
			return false
		}
	}
	return true
}
