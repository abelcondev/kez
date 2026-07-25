package agent

import "testing"

// decodeToolArguments must (a) decode a normal single-object payload unchanged,
// (b) tolerate a weak model packing multiple concatenated top-level JSON objects
// into one arguments string by decoding the FIRST object and ignoring the rest
// (the minimax-m3 "invalid character '{' after top-level value" failure), and
// (c) still reject a genuinely malformed/truncated first object.
func TestDecodeToolArguments(t *testing.T) {
	t.Run("single object decodes normally", func(t *testing.T) {
		var a map[string]any
		if err := decodeToolArguments(`{"path":"x"}`, &a); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if a["path"] != "x" {
			t.Fatalf("decoded %v, want path=x", a)
		}
	})

	t.Run("concatenated objects decode the first, ignore trailing", func(t *testing.T) {
		var b map[string]any
		if err := decodeToolArguments(`{"path":"first"}{"path":"second"}`, &b); err != nil {
			t.Fatalf("multi-object should decode the first, got err = %v", err)
		}
		if b["path"] != "first" {
			t.Fatalf("decoded %v, want the first object (path=first)", b)
		}
	})

	t.Run("distinct tools' args concatenated (real minimax case): first object used", func(t *testing.T) {
		var c map[string]any
		if err := decodeToolArguments(`{"plan":[1,2]}{"cmd":"go version"}`, &c); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if _, ok := c["plan"]; !ok || c["cmd"] != nil {
			t.Fatalf("decoded %v, want only the first object's keys", c)
		}
	})

	t.Run("truncated first object still errors", func(t *testing.T) {
		var d map[string]any
		if err := decodeToolArguments(`{"path":`, &d); err == nil {
			t.Fatal("truncated JSON must still error")
		}
	})

	t.Run("non-JSON trailing garbage still errors (not masked)", func(t *testing.T) {
		var g map[string]any
		if err := decodeToolArguments(`{"x":1}xyz`, &g); err == nil {
			t.Fatal("a valid first object followed by non-JSON garbage must still error")
		}
	})

	t.Run("recoverableToolArguments: only whole-JSON trailing is tolerated", func(t *testing.T) {
		cases := []struct {
			in    string
			ok    bool
			first string
		}{
			{`{"a":1}`, true, `{"a":1}`},
			{`{"a":1}{"b":2}`, true, `{"a":1}`},
			{`{"a":1}  {"b":2}  `, true, `{"a":1}`},
			{`{"a":1}xyz`, false, ""},
			{`{"a":`, false, ""},
			{`   `, false, ""},
		}
		for _, c := range cases {
			first, ok := recoverableToolArguments(c.in)
			if ok != c.ok || (ok && first != c.first) {
				t.Fatalf("recoverableToolArguments(%q) = (%q,%v), want (%q,%v)", c.in, first, ok, c.first, c.ok)
			}
		}
	})

	t.Run("historySafeToolCalls recovers the first object, not {}", func(t *testing.T) {
		out := historySafeToolCalls([]ToolCall{
			{ID: "c1", Name: "read_file", Arguments: `{"path":"README.md"}{"path":"AGENTS.md"}`}, // concatenated -> first
			{ID: "c2", Name: "read_file", Arguments: `{"path":"x"}xyz`},                          // garbage -> {}
			{ID: "c3", Name: "read_file", Arguments: `{"path":"ok"}`},                            // valid -> unchanged
		})
		if out[0].Arguments != `{"path":"README.md"}` {
			t.Fatalf("concatenated args should record the first object, got %q", out[0].Arguments)
		}
		if out[1].Arguments != "{}" {
			t.Fatalf("garbage-trailing args should record {}, got %q", out[1].Arguments)
		}
		if out[2].Arguments != `{"path":"ok"}` {
			t.Fatalf("valid args should be unchanged, got %q", out[2].Arguments)
		}
	})

	t.Run("whitespace-only errors (caller guards empty)", func(t *testing.T) {
		var e map[string]any
		if err := decodeToolArguments("   ", &e); err == nil {
			t.Fatal("whitespace-only must error")
		}
	})
}
