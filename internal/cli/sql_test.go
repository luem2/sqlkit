package cli

import "testing"

func TestParseSQLCMDVariables(t *testing.T) {
	got, err := parseSQLCMDVariables([]string{"BackupLogin=sqlkit_backup", "Empty="})
	if err != nil {
		t.Fatal(err)
	}
	if got["BackupLogin"] != "sqlkit_backup" || got["Empty"] != "" {
		t.Fatalf("unexpected variables: %#v", got)
	}
}

func TestParseSQLCMDVariablesRejectsInvalidAndDuplicateNames(t *testing.T) {
	for _, values := range [][]string{
		{"missing-separator"},
		{"bad-name=value"},
		{"Name=one", "Name=two"},
	} {
		if _, err := parseSQLCMDVariables(values); err == nil {
			t.Fatalf("expected error for %#v", values)
		}
	}
}

func TestEscapeSQLStringLiteral(t *testing.T) {
	if got := escapeSQLStringLiteral("pa'ss"); got != "pa''ss" {
		t.Fatalf("escapeSQLStringLiteral = %q", got)
	}
}
