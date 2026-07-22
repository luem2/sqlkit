package deps

import "testing"

func TestPlanInstallSqlPackage(t *testing.T) {
	plan, err := PlanInstall("sqlpackage")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Name != "sqlpackage" {
		t.Fatalf("Name = %q, want sqlpackage", plan.Name)
	}
	if len(plan.Commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(plan.Commands))
	}
	if got := plan.Commands[0][0]; got != "dotnet" {
		t.Fatalf("command = %q, want dotnet", got)
	}
}

func TestPlanInstallUnsupportedTool(t *testing.T) {
	if _, err := PlanInstall("unknown"); err == nil {
		t.Fatal("expected unsupported dependency error")
	}
}
