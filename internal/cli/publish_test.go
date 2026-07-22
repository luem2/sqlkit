package cli

import "testing"

func TestCompanyTargetDatabase(t *testing.T) {
	got, err := companyTargetDatabase(" p ", "_FACTURACION")
	if err != nil {
		t.Fatal(err)
	}
	if got != "P_FACTURACION" {
		t.Fatalf("companyTargetDatabase() = %q, want P_FACTURACION", got)
	}
}

func TestCompanyTargetDatabaseRequiresCompany(t *testing.T) {
	if _, err := companyTargetDatabase(" ", "_BD_SISTEMA"); err == nil {
		t.Fatal("expected error for empty company")
	}
}
