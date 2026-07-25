package skills

import "testing"

func findSkill(list []Skill, name string) (Skill, bool) {
	for _, skill := range list {
		if skill.Name == name {
			return skill, true
		}
	}
	return Skill{}, false
}

func TestBuiltinIncludesNewApp(t *testing.T) {
	built := Builtin()
	newApp, ok := findSkill(built, "new-app")
	if !ok {
		t.Fatalf("Builtin() must include new-app, got %#v", built)
	}
	if newApp.Description == "" {
		t.Error("new-app must carry a description so the model can trigger it")
	}
	if newApp.Content == "" {
		t.Error("new-app must carry a body served from the embedded FS")
	}
	if newApp.Path != "builtin:new-app" {
		t.Errorf("new-app Path = %q, want builtin:new-app", newApp.Path)
	}
}

func TestMergeBuiltinsAddsWhenAbsent(t *testing.T) {
	merged := MergeBuiltins(nil, true)
	if _, ok := findSkill(merged, "new-app"); !ok {
		t.Fatalf("MergeBuiltins(nil) must surface built-ins, got %#v", merged)
	}
}

func TestMergeBuiltinsDiskWinsNameClash(t *testing.T) {
	disk := []Skill{{Name: "new-app", Description: "user override", Content: "disk body", Path: "/disk/new-app/SKILL.md"}}
	merged := MergeBuiltins(disk, true)

	got, ok := findSkill(merged, "new-app")
	if !ok {
		t.Fatal("new-app missing after merge")
	}
	if got.Path != "/disk/new-app/SKILL.md" || got.Content != "disk body" {
		t.Errorf("disk skill must win the name clash, got %#v", got)
	}
	// No duplicate entry for the shadowed built-in.
	count := 0
	for _, skill := range merged {
		if skill.Name == "new-app" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one new-app after override, got %d", count)
	}
}

func TestMergeBuiltinsStripsContentWhenListing(t *testing.T) {
	merged := MergeBuiltins(nil, false)
	for _, skill := range merged {
		if skill.Content != "" {
			t.Errorf("keepContent=false must strip bodies; %s kept content", skill.Name)
		}
	}
}

func TestMergeBuiltinsSortedByName(t *testing.T) {
	disk := []Skill{{Name: "zzz", Path: "/z"}, {Name: "aaa", Path: "/a"}}
	merged := MergeBuiltins(disk, false)
	for i := 1; i < len(merged); i++ {
		if merged[i-1].Name > merged[i].Name {
			t.Fatalf("MergeBuiltins result not sorted: %q before %q", merged[i-1].Name, merged[i].Name)
		}
	}
}
