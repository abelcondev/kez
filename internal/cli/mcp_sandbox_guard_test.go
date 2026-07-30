package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// markInsideKezSandbox sets the ambient markers IsAlreadySandboxed checks, so the
// MCP config guard behaves as if this process were a wrapped sandbox command.
func markInsideKezSandbox(t *testing.T) {
	t.Helper()
	t.Setenv("KEZ_SANDBOXED", "1")
	t.Setenv("KEZ_SANDBOX_BACKEND", "seatbelt")
}

func TestMCPAddRefusedInsideSandbox(t *testing.T) {
	markInsideKezSandbox(t)
	configPath := filepath.Join(t.TempDir(), "kez", "config.json")

	var stdout, stderr bytes.Buffer
	exitCode := runWithDeps([]string{"mcp", "add", "penpot", "--url", "https://example.com/mcp"}, &stdout, &stderr, appDeps{
		userConfigPath: func() (string, error) { return configPath, nil },
	})

	if exitCode != exitUsage {
		t.Fatalf("exitCode = %d, want exitUsage; stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "inside a kez sandbox") {
		t.Fatalf("stderr missing sandbox refusal: %s", stderr.String())
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config must not be written inside the sandbox; stat err=%v", err)
	}
}

func TestMCPAddAllowedOutsideSandbox(t *testing.T) {
	// Explicitly clear the markers so a host-run add proceeds and persists.
	t.Setenv("KEZ_SANDBOXED", "")
	t.Setenv("KEZ_SANDBOX_BACKEND", "")
	configPath := filepath.Join(t.TempDir(), "kez", "config.json")

	var stdout, stderr bytes.Buffer
	exitCode := runWithDeps([]string{"mcp", "add", "penpot", "--url", "https://example.com/mcp"}, &stdout, &stderr, appDeps{
		userConfigPath: func() (string, error) { return configPath, nil },
	})

	if exitCode != exitSuccess {
		t.Fatalf("exitCode = %d, want success; stderr=%s", exitCode, stderr.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config not written on host: %v", err)
	}
	if !strings.Contains(string(data), "penpot") {
		t.Fatalf("config missing added server: %s", string(data))
	}
}
