package ssdt

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/luem2/sqlkit/internal/config"
)

func TestBuildArgs(t *testing.T) {
	got := BuildArgs("BD/BD.sqlproj", "Release")
	want := []string{"build", "BD/BD.sqlproj", "--configuration", "Release"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestDefaultDacpacPath(t *testing.T) {
	got := DefaultDacpacPath(filepath.Join("BD", "BD.sqlproj"), "Release")
	want := filepath.Join("BD", "bin", "Release", "BD.dacpac")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSQLPackageArgs(t *testing.T) {
	conn := &config.SQLConnection{Server: "localhost", User: "sa", Password: "secret"}
	server := SQLPackageServerName(conn.Server)
	got := SQLPackageArgs(ActionScript, "db.dacpac", conn, "TargetDB", "out.sql")
	want := []string{
		"/Action:Script",
		"/SourceFile:db.dacpac",
		"/TargetServerName:" + server,
		"/TargetDatabaseName:TargetDB",
		"/TargetUser:sa",
		"/TargetPassword:secret",
		"/TargetEncryptConnection:False",
		"/TargetTrustServerCertificate:True",
		"/OutputPath:out.sql",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestBacpacArgs(t *testing.T) {
	conn := &config.SQLConnection{Server: "localhost", User: "sa", Password: "secret"}
	server := SQLPackageServerName(conn.Server)

	exportArgs := BacpacExportArgs(conn, "SourceDB", "out.bacpac")
	mustContainArg(t, exportArgs, "/Action:Export")
	mustContainArg(t, exportArgs, "/TargetFile:out.bacpac")
	mustContainArg(t, exportArgs, "/SourceServerName:"+server)
	mustContainArg(t, exportArgs, "/SourceDatabaseName:SourceDB")
	mustContainArg(t, exportArgs, "/SourceEncryptConnection:False")
	mustContainArg(t, exportArgs, "/SourceTrustServerCertificate:True")
	mustContainArg(t, exportArgs, "/p:VerifyExtraction=False")

	importArgs := BacpacImportArgs(conn, "TargetDB", "in.bacpac")
	mustContainArg(t, importArgs, "/Action:Import")
	mustContainArg(t, importArgs, "/SourceFile:in.bacpac")
	mustContainArg(t, importArgs, "/TargetServerName:"+server)
	mustContainArg(t, importArgs, "/TargetDatabaseName:TargetDB")
	mustContainArg(t, importArgs, "/TargetEncryptConnection:False")
	mustContainArg(t, importArgs, "/TargetTrustServerCertificate:True")
}

func TestPublishProfileArgs(t *testing.T) {
	conn := &config.SQLConnection{Server: "localhost", User: "sa", Password: "secret"}
	server := SQLPackageServerName(conn.Server)
	args := PublishProfileArgs("profile.xml", "db.dacpac", conn, "A_BD_SISTEMA", true, map[string]string{
		"Company": "A",
	})

	mustContainArg(t, args, "/Action:Publish")
	mustContainArg(t, args, "/Profile:profile.xml")
	mustContainArg(t, args, "/TargetServerName:"+server)
	mustContainArg(t, args, "/TargetDatabaseName:A_BD_SISTEMA")
	mustContainArg(t, args, "/TargetUser:sa")
	mustContainArg(t, args, "/TargetPassword:secret")
	mustContainArg(t, args, "/TargetEncryptConnection:False")
	mustContainArg(t, args, "/TargetTrustServerCertificate:True")
	mustContainArg(t, args, "/p:DropObjectsNotInSource=True")
	mustContainArg(t, args, "/v:SqlServer="+server)
	mustContainArg(t, args, "/v:TargetDb=A_BD_SISTEMA")
	mustContainArg(t, args, "/v:Company=A")
	mustContainArg(t, args, "/v:SqlPassword=secret")
}

func TestSQLPackageArgsUsesConfiguredTLSOptions(t *testing.T) {
	conn := &config.SQLConnection{
		Server:                 "localhost",
		User:                   "sa",
		Password:               "secret",
		Encrypt:                "strict",
		TrustServerCertificate: false,
	}
	args := SQLPackageArgs(ActionPublish, "db.dacpac", conn, "TargetDB", "")

	mustContainArg(t, args, "/TargetEncryptConnection:Strict")
	mustContainArg(t, args, "/TargetTrustServerCertificate:False")
}

func TestSafeFileName(t *testing.T) {
	got := SafeFileName("P BD/SISTEMA")
	if got != "P_BD_SISTEMA" {
		t.Fatalf("got %q", got)
	}
}

func mustContainArg(t *testing.T, args []string, want string) {
	t.Helper()
	for _, arg := range args {
		if arg == want {
			return
		}
	}
	t.Fatalf("expected %#v to contain %q", args, want)
}
