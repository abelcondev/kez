package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var sandboxUserCacheDir = os.UserCacheDir
var sandboxRuntimeNow = time.Now

const (
	sandboxRuntimeMaxAge   = 30 * 24 * time.Hour
	sandboxRuntimeMaxRoots = 64
)

var fallbackSandboxRuntimes = struct {
	sync.Mutex
	roots map[string]string
}{roots: make(map[string]string)}

type SandboxRuntime struct {
	Root   string `json:"root,omitempty"`
	Home   string `json:"home,omitempty"`
	Cache  string `json:"cache,omitempty"`
	Config string `json:"config,omitempty"`
	Data   string `json:"data,omitempty"`
	State  string `json:"state,omitempty"`
	Temp   string `json:"temp,omitempty"`
	// GitHubToken, when set, is injected as GH_TOKEN (plus a github.com git
	// credential helper) into the sandboxed command's environment so gh and
	// git-over-HTTPS authenticate against the host's active GitHub account without
	// a real HOME or keychain access. json:"-" keeps this secret out of every
	// serialized profile/execution report.
	GitHubToken string `json:"-"`
}

func prepareSandboxRuntime(workspaceRoot string) (SandboxRuntime, func(), error) {
	workspaceRoot = filepath.Clean(strings.TrimSpace(workspaceRoot))
	if workspaceRoot == "" || workspaceRoot == "." {
		return SandboxRuntime{}, nil, errors.New("sandbox runtime requires a workspace root")
	}
	cacheRoot, err := sandboxUserCacheDir()
	if err != nil {
		return SandboxRuntime{}, nil, fmt.Errorf("resolve user cache directory: %w", err)
	}
	cacheRoot = filepath.Clean(strings.TrimSpace(cacheRoot))
	if cacheRoot == "" || cacheRoot == "." {
		return SandboxRuntime{}, nil, errors.New("user cache directory is unavailable")
	}
	digest := sha256.Sum256([]byte(workspaceRoot))
	root := filepath.Join(cacheRoot, "kez", "runtime", "v1", hex.EncodeToString(digest[:8]))
	if pathWithinRoot(workspaceRoot, root) {
		root, err = fallbackSandboxRuntimeRoot(workspaceRoot)
		if err != nil {
			return SandboxRuntime{}, nil, err
		}
	}
	lease, err := prepareSandboxRuntimeLease(root)
	if err != nil {
		root, err = fallbackSandboxRuntimeRoot(workspaceRoot)
		if err != nil {
			return SandboxRuntime{}, nil, err
		}
		lease, err = prepareSandboxRuntimeLease(root)
		if err != nil {
			return SandboxRuntime{}, nil, err
		}
	}
	prepared := false
	defer func() {
		if !prepared {
			lease.release()
		}
	}()
	runtimeState := SandboxRuntime{
		Root:   root,
		Home:   filepath.Join(root, "home"),
		Cache:  filepath.Join(root, "cache"),
		Config: filepath.Join(root, "config"),
		Data:   filepath.Join(root, "data"),
		State:  filepath.Join(root, "state"),
		Temp:   filepath.Join(root, "tmp"),
	}
	directories := []string{
		runtimeState.Root,
		runtimeState.Home,
		runtimeState.Cache,
		runtimeState.Config,
		runtimeState.Data,
		runtimeState.State,
		runtimeState.Temp,
		filepath.Join(runtimeState.Cache, "npm"),
		filepath.Join(runtimeState.Cache, "yarn"),
		filepath.Join(runtimeState.Cache, "corepack"),
		filepath.Join(runtimeState.Cache, "pip"),
		filepath.Join(runtimeState.Cache, "go-build"),
		filepath.Join(runtimeState.Data, "go-mod"),
		filepath.Join(runtimeState.Data, "cargo"),
	}
	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return SandboxRuntime{}, nil, fmt.Errorf("create sandbox runtime directory %s: %w", directory, err)
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return SandboxRuntime{}, nil, fmt.Errorf("secure sandbox runtime directory %s: %w", directory, err)
		}
	}
	now := sandboxRuntimeNow()
	if err := os.Chtimes(runtimeState.Root, now, now); err != nil {
		return SandboxRuntime{}, nil, fmt.Errorf("touch sandbox runtime root: %w", err)
	}
	cleanupSandboxRuntimeRoots(filepath.Dir(runtimeState.Root), runtimeState.Root, now)
	prepared = true
	return runtimeState, lease.release, nil
}

func prepareSandboxRuntimeLease(root string) (*sandboxRuntimeLease, error) {
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return nil, fmt.Errorf("create sandbox runtime parent: %w", err)
	}
	return acquireSandboxRuntimeLease(root)
}

// cleanupSandboxRuntimeRoots applies a conservative age/count policy. Cleanup
// is best-effort and never removes the runtime selected for the current
// command, so cache maintenance cannot turn a valid execution into a failure.
func cleanupSandboxRuntimeRoots(parent, current string, now time.Time) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	type candidate struct {
		path    string
		modTime time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		path := filepath.Join(parent, entry.Name())
		if !entry.IsDir() || filepath.Clean(path) == filepath.Clean(current) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if now.Sub(info.ModTime()) > sandboxRuntimeMaxAge {
			removeSandboxRuntimeRootIfUnused(path)
			continue
		}
		candidates = append(candidates, candidate{path: path, modTime: info.ModTime()})
	}
	if len(candidates) < sandboxRuntimeMaxRoots {
		return
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modTime.Before(candidates[j].modTime) })
	for len(candidates) >= sandboxRuntimeMaxRoots {
		removeSandboxRuntimeRootIfUnused(candidates[0].path)
		candidates = candidates[1:]
	}
}

func removeSandboxRuntimeRootIfUnused(root string) {
	lease, inUse, err := tryAcquireSandboxRuntimeCleanupLease(root)
	if err != nil || inUse {
		return
	}
	defer lease.release()
	_ = os.RemoveAll(root)
}

func combineSandboxCleanups(cleanups ...func()) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, cleanup := range cleanups {
				if cleanup != nil {
					cleanup()
				}
			}
		})
	}
}

func fallbackSandboxRuntimeRoot(workspaceRoot string) (string, error) {
	fallbackSandboxRuntimes.Lock()
	defer fallbackSandboxRuntimes.Unlock()
	if root := fallbackSandboxRuntimes.roots[workspaceRoot]; root != "" {
		return root, nil
	}
	parent, err := os.MkdirTemp("", "zero-runtime-")
	if err != nil {
		return "", fmt.Errorf("create fallback sandbox runtime: %w", err)
	}
	root := filepath.Join(parent, "runtime")
	if pathWithinRoot(workspaceRoot, root) {
		_ = os.RemoveAll(parent)
		return "", fmt.Errorf("fallback sandbox runtime root %q must be outside workspace %q", root, workspaceRoot)
	}
	fallbackSandboxRuntimes.roots[workspaceRoot] = root
	return root, nil
}

func sandboxRuntimeEnvironment(env []string, runtimeState *SandboxRuntime) []string {
	if runtimeState == nil || strings.TrimSpace(runtimeState.Root) == "" {
		return env
	}
	overrides := []string{
		"HOME=" + runtimeState.Home,
		"XDG_CACHE_HOME=" + runtimeState.Cache,
		"XDG_CONFIG_HOME=" + runtimeState.Config,
		"XDG_DATA_HOME=" + runtimeState.Data,
		"XDG_STATE_HOME=" + runtimeState.State,
		// A sandboxed command runs with a virtual HOME and no credential store, so
		// any git operation over HTTPS that needs a username/password would block
		// forever on an interactive terminal prompt (a silent hang, not an error).
		// GIT_TERMINAL_PROMPT=0 turns that into an immediate, actionable failure;
		// GIT_ASKPASS="" neutralizes any GUI askpass helper inherited from the host
		// so it can't hijack the prompt either. Trusted forge commands (gh, git
		// push/fetch/…) auto-escalate out of the sandbox and never reach this env.
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"TMPDIR=" + runtimeState.Temp,
		"TMP=" + runtimeState.Temp,
		"TEMP=" + runtimeState.Temp,
		"npm_config_cache=" + filepath.Join(runtimeState.Cache, "npm"),
		"NPM_CONFIG_USERCONFIG=" + filepath.Join(runtimeState.Config, "npmrc"),
		"YARN_CACHE_FOLDER=" + filepath.Join(runtimeState.Cache, "yarn"),
		"COREPACK_HOME=" + filepath.Join(runtimeState.Cache, "corepack"),
		"PIP_CACHE_DIR=" + filepath.Join(runtimeState.Cache, "pip"),
		"GOCACHE=" + filepath.Join(runtimeState.Cache, "go-build"),
		"GOMODCACHE=" + filepath.Join(runtimeState.Data, "go-mod"),
		"CARGO_HOME=" + filepath.Join(runtimeState.Data, "cargo"),
	}
	overrides = append(overrides, githubTokenEnvironment(runtimeState.GitHubToken)...)
	return upsertEnvList(env, overrides...)
}

// githubTokenEnvironment returns the env overrides that make gh and git-over-HTTPS
// authenticate with the injected token. It runs AFTER credential scrubbing (via
// upsertEnvList in sandboxRuntimeEnvironment), so the injected GH_TOKEN survives.
// The git credential helper is scoped to github.com and reads the token from the
// GH_TOKEN it sets, so no secret is written to disk or embedded in git config on
// the host. Empty token → no overrides (feature off / no host auth).
func githubTokenEnvironment(token string) []string {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return []string{
		"GH_TOKEN=" + token,
		"GITHUB_TOKEN=" + token,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.https://github.com.helper",
		`GIT_CONFIG_VALUE_0=!f() { echo username=x-access-token; echo "password=$GH_TOKEN"; }; f`,
	}
}

func permissionProfileWithRuntime(profile PermissionProfile, runtimeState SandboxRuntime) PermissionProfile {
	profile.Runtime = &runtimeState
	if profile.FileSystem.Kind != FileSystemRestricted || runtimeState.Root == "" {
		return profile
	}
	for _, root := range profile.FileSystem.WriteRoots {
		if filepath.Clean(root.Root) == filepath.Clean(runtimeState.Root) {
			return profile
		}
	}
	profile.FileSystem.WriteRoots = append(profile.FileSystem.WriteRoots, WritableRoot{Root: runtimeState.Root})
	return profile
}
