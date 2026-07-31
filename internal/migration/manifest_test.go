package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectStepsRange(t *testing.T) {
	manifest := &Manifest{Steps: []Step{
		{Name: "a", Order: 1, Script: "a.sql"},
		{Name: "b", Order: 2, Script: "b.sql"},
		{Name: "c", Order: 3, Script: "c.sql"},
	}}

	steps, err := manifest.SelectSteps("", "b", "c", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[0].Name != "b" || steps[1].Name != "c" {
		t.Fatalf("unexpected steps: %#v", steps)
	}
}

func TestSelectStepsRequiresOneMode(t *testing.T) {
	manifest := &Manifest{Steps: []Step{{Name: "a", Order: 1, Script: "a.sql"}}}
	if _, err := manifest.SelectSteps("", "", "", false); err == nil {
		t.Fatal("expected selection error")
	}
	if _, err := manifest.SelectSteps("a", "", "", true); err == nil {
		t.Fatal("expected conflicting selection error")
	}
}

func TestResolveStepScripts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("SELECT 1;"), 0600); err != nil {
		t.Fatal(err)
	}

	scripts, err := ResolveStepScripts(dir, []Step{{Name: "a", Script: "a.sql"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 || !filepath.IsAbs(scripts[0]) {
		t.Fatalf("unexpected scripts: %#v", scripts)
	}
}
