package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abelcondev/kez/internal/config"
	"github.com/abelcondev/kez/internal/zeroruntime"
)

// TestRunExecDropsImagesOnNonVisionModelWithWarning drives a full exec run with
// an --image attachment against a custom (catalog-unknown) model id. The vision
// gate cannot confirm vision support for an unknown id, so it must warn on
// stderr and proceed text-only (exit 0), never erroring the run.
func TestRunExecDropsImagesOnNonVisionModelWithWarning(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "shot.png"), pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	exitCode, _, stderr := runExecWithEcho(t, []string{
		"exec", "--cwd", root,
		"--model", "my-custom-vision-less-model",
		"--image", "shot.png",
		"describe the screenshot",
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0 (drop+warn, never error), got %d: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "does not support image input") {
		t.Fatalf("expected non-vision warning on stderr, got %q", stderr)
	}
}

// TestRunExecKeepsImagesWhenProviderDeclaresVision drives a full exec run with
// an --image attachment against a catalog-unknown "k3" model whose provider
// profile declares supportsVision: true. The config override must win over the
// heuristic, so the image is kept (no drop warning) and the run succeeds.
func TestRunExecKeepsImagesWhenProviderDeclaresVision(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "shot.png"), pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	yes := true
	var stdout, stderr bytes.Buffer
	exitCode := runWithDeps([]string{
		"exec", "--cwd", root,
		"--model", "k3",
		"--image", "shot.png",
		"describe the screenshot",
	}, &stdout, &stderr, appDeps{
		getwd: func() (string, error) { return root, nil },
		resolveConfig: func(_ string, overrides config.Overrides) (config.ResolvedConfig, error) {
			model := "k3"
			if overrides.Provider.Model != "" {
				model = overrides.Provider.Model
			}
			return config.ResolvedConfig{
				ActiveProvider: "custom",
				Provider: config.ProviderProfile{
					Name:           "custom",
					ProviderKind:   config.ProviderKindOpenAICompatible,
					BaseURL:        "http://127.0.0.1/v1",
					Model:          model,
					SupportsVision: &yes,
				},
				MaxTurns: 3,
			}, nil
		},
		newProvider: func(config.ProviderProfile) (zeroruntime.Provider, error) {
			return echoExecProvider{}, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}
	if strings.Contains(stderr.String(), "does not support image input") {
		t.Fatalf("declared-vision provider must not drop the image, got stderr %q", stderr.String())
	}
}

// TestRunExecRejectsUnsupportedImageType confirms the usage-error path is wired
// into the run (a .txt sniffs as text -> unsupported, exit 2).
func TestRunExecRejectsUnsupportedImageType(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("not an image at all"), 0o600); err != nil {
		t.Fatal(err)
	}

	exitCode, _, stderr := runExecWithEcho(t, []string{
		"exec", "--cwd", root,
		"--image", "notes.txt",
		"look",
	})

	if exitCode != 2 {
		t.Fatalf("expected usage exit code 2, got %d: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "unsupported image type") {
		t.Fatalf("expected unsupported-image-type error on stderr, got %q", stderr)
	}
}
