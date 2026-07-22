package ssdt

import "testing"

func TestExplainSQLPackageErrorForTLS(t *testing.T) {
	got := ExplainSQLPackageError("The certificate chain was issued by an authority that is not trusted.")
	if got == "" {
		t.Fatal("expected TLS hint")
	}
}

func TestExplainSQLPackageErrorForLogin(t *testing.T) {
	got := ExplainSQLPackageError("Login failed for user 'sa'.")
	if got == "" {
		t.Fatal("expected login hint")
	}
}

func TestExplainSQLPackageErrorUnknown(t *testing.T) {
	if got := ExplainSQLPackageError("unexpected"); got != "" {
		t.Fatalf("got %q, want empty hint", got)
	}
}
