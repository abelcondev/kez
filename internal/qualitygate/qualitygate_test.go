package qualitygate

import (
	"strings"
	"testing"
)

func lines(n int) string {
	return strings.Repeat("x\n", n)
}

func TestCheckWithConfig(t *testing.T) {
	cfg := Config{Enabled: true, MaxLines: 300}

	tests := []struct {
		name    string
		path    string
		prior   string
		next    string
		blocked bool
	}{
		{"new small file ok", "a.go", "", lines(299), false},
		{"new file at limit ok", "a.go", "", lines(300), false},
		{"new file over limit blocked", "a.go", "", lines(301), true},
		{"grow existing across limit blocked", "a.go", lines(250), lines(320), true},
		{"edit oversized without growing ok", "a.go", lines(400), lines(380), false},
		{"edit oversized down to equal ok", "a.go", lines(400), lines(400), false},
		{"grow an already-oversized file blocked", "a.go", lines(400), lines(420), true},
		{"non-code extension ignored", "README.md", "", lines(900), false},
		{"json data ignored", "data.json", "", lines(900), false},
		{"generated go ignored", "api.pb.go", "", lines(900), false},
		{"minified js ignored", "bundle.min.js", "", lines(900), false},
		{"svelte gated", "App.svelte", "", lines(400), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckWithConfig(tc.path, tc.prior, tc.next, cfg)
			if tc.blocked && err == nil {
				t.Errorf("expected block for %s, got nil", tc.path)
			}
			if !tc.blocked && err != nil {
				t.Errorf("expected allow for %s, got %v", tc.path, err)
			}
		})
	}
}

func TestCheckWithConfigDisabled(t *testing.T) {
	if err := CheckWithConfig("a.go", "", lines(9000), Config{Enabled: false, MaxLines: 300}); err != nil {
		t.Errorf("disabled gate must allow, got %v", err)
	}
	if err := CheckWithConfig("a.go", "", lines(9000), Config{Enabled: true, MaxLines: 0}); err != nil {
		t.Errorf("MaxLines<=0 must allow, got %v", err)
	}
}

func TestConfigFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	if c := ConfigFromEnv(env(nil)); !c.Enabled || c.MaxLines != DefaultMaxLines {
		t.Errorf("default = %+v, want enabled/300", c)
	}
	if c := ConfigFromEnv(env(map[string]string{"KEZ_QUALITY_GATES": "off"})); c.Enabled {
		t.Errorf("KEZ_QUALITY_GATES=off should disable")
	}
	if c := ConfigFromEnv(env(map[string]string{"KEZ_MAX_FILE_LINES": "120"})); c.MaxLines != 120 {
		t.Errorf("MaxLines = %d, want 120", c.MaxLines)
	}
	if c := ConfigFromEnv(env(map[string]string{"KEZ_MAX_FILE_LINES": "0"})); c.Enabled {
		t.Errorf("KEZ_MAX_FILE_LINES=0 should disable")
	}
}

func TestGateErrorMessageGuidesTheModel(t *testing.T) {
	err := CheckWithConfig("internal/big.go", "", lines(500), Config{Enabled: true, MaxLines: 300})
	if err == nil {
		t.Fatal("expected a GateError")
	}
	msg := err.Error()
	for _, want := range []string{"internal/big.go", "500", "300", "Split", "KEZ_MAX_FILE_LINES"} {
		if !strings.Contains(msg, want) {
			t.Errorf("gate message missing %q: %s", want, msg)
		}
	}
}
