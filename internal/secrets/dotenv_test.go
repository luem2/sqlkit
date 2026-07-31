package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.env")
	if err := os.WriteFile(path, []byte(`
# comment
export SQLKIT_ENV_LOCAL_PASSWORD="local pwd"
DBA_LOGIN_PASSWORD='dba pwd'
PLAIN=value
`), 0600); err != nil {
		t.Fatal(err)
	}

	values, err := LoadDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if values["SQLKIT_ENV_LOCAL_PASSWORD"] != "local pwd" {
		t.Fatalf("SQLKIT_ENV_LOCAL_PASSWORD = %q", values["SQLKIT_ENV_LOCAL_PASSWORD"])
	}
	if values["DBA_LOGIN_PASSWORD"] != "dba pwd" {
		t.Fatalf("DBA_LOGIN_PASSWORD = %q", values["DBA_LOGIN_PASSWORD"])
	}
	if values["PLAIN"] != "value" {
		t.Fatalf("PLAIN = %q", values["PLAIN"])
	}
}
