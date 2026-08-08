# Zero build/test/lint targets. AGENTS.md says "Build with `make`" and "Run `make
# lint` before opening a PR" — these targets back those instructions.
.DEFAULT_GOAL := build
.PHONY: build build-all install test test-race vet fmt fmt-check lint tidy clean baseline help

# Build the main CLI binary into ./zero.
build:
	go build -o kez ./cmd/kez

# Build from source and install the CLI to INSTALL_DIR (default ~/.local/bin).
# This is the supported way to run your local changes: scripts/install.sh pulls
# prebuilt binaries from GitHub Releases, so it will NOT contain unreleased local
# fixes. On macOS the binary is re-signed ad-hoc so Sequoia's code-signing
# monitor does not SIGKILL it ("zsh: killed kez").
INSTALL_DIR ?= $(HOME)/.local/bin
install:
	@mkdir -p "$(INSTALL_DIR)"
	go build -o "$(INSTALL_DIR)/kez" ./cmd/kez
	@if [ "$$(uname)" = "Darwin" ]; then codesign --force --sign - "$(INSTALL_DIR)/kez" >/dev/null 2>&1 && echo "re-signed for macOS"; fi
	@echo "installed kez -> $(INSTALL_DIR)/kez"

# Build every command in cmd/.
build-all:
	go build ./...

# Run the full test suite with the race detector (matches CI expectations).
test:
	go test ./... -race -count=1

# Faster, no race detector.
test-quick:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $(shell git ls-files '*.go')

# Fail if any tracked Go file is not gofmt-clean.
fmt-check:
	@out="$$(gofmt -l $$(git ls-files '*.go'))"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Lint = formatting check + vet (no extra tooling required).
lint: fmt-check vet

tidy:
	go mod tidy

clean:
	rm -f kez
	go clean ./...

# Run the per-turn benchmark harness over the checked-in baseline manifest and
# write the JSON result to internal/perfbench/reports/baseline.json. Requires a
# built `zero` binary and a model; set KEZ_BENCH_MODEL (required) and
# KEZ_BENCH_BINARY (defaults to ./zero) to configure the run. The report is
# machine-specific and regenerated, not hand-edited.
baseline: build
	@if [ -z "$(KEZ_BENCH_MODEL)" ]; then echo "Set KEZ_BENCH_MODEL (and optionally KEZ_BENCH_BINARY) before running 'make baseline'"; exit 2; fi
	@KEZ_BIN="$${KEZ_BENCH_BINARY:-./zero}"; \
	go run ./cmd/zero-perf-bench turn \
		--suite internal/perfbench/manifests/baseline.json \
		--model $(KEZ_BENCH_MODEL) \
		--binary "$$KEZ_BIN" \
		--output internal/perfbench/reports/baseline.json

help:
	@echo "Targets: build (default), build-all, test, test-quick, vet, fmt, fmt-check, lint, tidy, clean, baseline"
