package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpecEnvNamesAndRequired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secretspec.toml")
	if err := os.WriteFile(path, []byte(`
[profiles.default]
DBA_LOGIN_PASSWORD = { sqlkit_name = "dba-login-password", env = "DBA_LOGIN_PASSWORD", required = false }
SQLKIT_ENV_LOCAL_PASSWORD = { sqlkit_name = "env/local/password", env = "SQLKIT_ENV_LOCAL_PASSWORD", required = false }

[profiles.local]
SQLKIT_ENV_LOCAL_PASSWORD = { required = true }
`), 0600); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	envNames := spec.EnvNames("dba-login-password")
	if len(envNames) != 1 || envNames[0] != "DBA_LOGIN_PASSWORD" {
		t.Fatalf("EnvNames = %#v", envNames)
	}
	required := spec.Required("local")
	if len(required) != 1 || required[0].Name != "SQLKIT_ENV_LOCAL_PASSWORD" {
		t.Fatalf("Required = %#v", required)
	}
	if required[0].SQLKitName != "env/local/password" || required[0].Env != "SQLKIT_ENV_LOCAL_PASSWORD" {
		t.Fatalf("Required did not inherit default fields: %#v", required[0])
	}
}
