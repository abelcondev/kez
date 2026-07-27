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

// sddPhaseSkills is the set of built-in skills that back the SDD loop phases.
// The advisor (`kez sdd next` / NextAction.Skill) names one of these per phase,
// so the router and this set must stay in sync.
var sddPhaseSkills = []string{
	"sdd-design",
	"sdd-discovery",
	"sdd-implement",
	"sdd-review",
	"sdd-ship",
	"sdd-stack",
	"sdd-task",
	"sdd-test",
}

func TestBuiltinShipsSDDPhaseSkills(t *testing.T) {
	built := Builtin()
	if len(built) != len(sddPhaseSkills) {
		t.Fatalf("Builtin() has %d skills, want %d (%v)", len(built), len(sddPhaseSkills), sddPhaseSkills)
	}
	for _, name := range sddPhaseSkills {
		skill, ok := findSkill(built, name)
		if !ok {
			t.Fatalf("built-in %q missing from Builtin()", name)
		}
		if skill.Description == "" {
			t.Errorf("built-in %q has no description; it is the model's only trigger surface", name)
		}
		if skill.Content == "" {
			t.Errorf("built-in %q has no body", name)
		}
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
	// The disk skill passes through and the built-in SDD phase skills are overlaid.
	if len(merged) != len(disk)+len(Builtin()) {
		t.Errorf("MergeBuiltins = %d entries, want %d disk + %d built-in", len(merged), len(disk), len(Builtin()))
	}
	if _, ok := findSkill(merged, "sdd-discovery"); !ok {
		t.Errorf("MergeBuiltins must overlay built-in phase skills; sdd-discovery absent")
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
