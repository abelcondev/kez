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

func TestBuiltinIsEmptyByDefault(t *testing.T) {
	// new-app was retired; the loop guidance now lives in the SDD advisor
	// (`kez sdd next`) plus each project's sdd/index.md. The built-in set is empty
	// but the machinery still compiles (see the .keep placeholder in builtin/).
	if built := Builtin(); len(built) != 0 {
		t.Fatalf("Builtin() = %#v, want empty", built)
	}
}

func TestMergeBuiltinsPassesThroughDiskSkills(t *testing.T) {
	disk := []Skill{{Name: "custom", Description: "user skill", Content: "disk body", Path: "/disk/custom/SKILL.md"}}
	merged := MergeBuiltins(disk, true)

	got, ok := findSkill(merged, "custom")
	if !ok {
		t.Fatalf("MergeBuiltins dropped the disk skill, got %#v", merged)
	}
	if got.Path != "/disk/custom/SKILL.md" || got.Content != "disk body" {
		t.Errorf("disk skill must pass through unchanged, got %#v", got)
	}
	if len(merged) != len(disk) {
		t.Errorf("MergeBuiltins added %d entries, want only the %d disk skills", len(merged)-len(disk), len(disk))
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
