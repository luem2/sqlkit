package sqlserver

import (
	"testing"

	"github.com/luem2/sqlkit/internal/config"
)

func TestConnectionStringUsesURLWithoutQuotedHost(t *testing.T) {
	got := connectionString(&config.SQLConnection{
		Server:   "localhost",
		User:     "sa",
		Password: "Password.123",
	}, "P_BD_SISTEMA")

	mustContain(t, got, "sqlserver://sa:Password.123@localhost?")
	mustNotContain(t, got, `"localhost"`)
	mustContain(t, got, "database=P_BD_SISTEMA")
	mustContain(t, got, "encrypt=disable")
	mustContain(t, got, "TrustServerCertificate=true")
}

func TestConnectionStringConvertsCommaPort(t *testing.T) {
	got := connectionString(&config.SQLConnection{
		Server:   "localhost,1433",
		User:     "sa",
		Password: "p@w",
	}, "master")

	mustContain(t, got, "@localhost:1433?")
	if got == "sqlserver://sa:p@w@localhost:1433?TrustServerCertificate=true&database=master&encrypt=disable" {
		t.Fatalf("expected password to be URL-escaped, got %q", got)
	}
}

func TestConnectionStringUsesConfiguredTLSOptions(t *testing.T) {
	got := connectionString(&config.SQLConnection{
		Server:                 "localhost",
		User:                   "sa",
		Password:               "Password.123",
		Encrypt:                "true",
		TrustServerCertificate: false,
	}, "master")

	mustContain(t, got, "encrypt=true")
	mustContain(t, got, "TrustServerCertificate=false")
}

func TestDadbodURLUsesSQLServerConnectionURL(t *testing.T) {
	got := DadbodURL(&config.SQLConnection{
		Server:   "localhost,1433",
		User:     "tester",
		Password: "p@w",
	}, "GRUPO_CENTRAL")

	mustContain(t, got, "sqlserver://tester:p%40w@localhost:1433?")
	mustContain(t, got, "database=GRUPO_CENTRAL")
	mustContain(t, got, "encrypt=disable")
	mustContain(t, got, "TrustServerCertificate=true")
}
