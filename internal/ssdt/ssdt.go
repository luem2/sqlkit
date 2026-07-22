package ssdt

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/luem2/sqlkit/internal/config"
)

const DefaultConfiguration = "Debug"

type PackageAction string

const (
	ActionPublish PackageAction = "Publish"
	ActionScript  PackageAction = "Script"
	ActionExport  PackageAction = "Export"
	ActionImport  PackageAction = "Import"
)

func BuildArgs(project string, configuration string) []string {
	args := []string{"build", project}
	if strings.TrimSpace(configuration) != "" {
		args = append(args, "--configuration", configuration)
	}
	return args
}

func DefaultDacpacPath(project string, configuration string) string {
	config := strings.TrimSpace(configuration)
	if config == "" {
		config = DefaultConfiguration
	}

	base := strings.TrimSuffix(filepath.Base(project), filepath.Ext(project))
	return filepath.Join(filepath.Dir(project), "bin", config, base+".dacpac")
}

func ScriptOutputPath(artifactsDir string, database string) string {
	return filepath.Join(artifactsDir, "publish", SafeFileName(database)+".sql")
}

func SQLPackageArgs(action PackageAction, dacpac string, conn *config.SQLConnection, database string, output string) []string {
	server := SQLPackageServerName(conn.Server)
	args := []string{
		"/Action:" + string(action),
		"/SourceFile:" + dacpac,
		"/TargetServerName:" + server,
		"/TargetDatabaseName:" + database,
		"/TargetUser:" + conn.User,
		"/TargetPassword:" + conn.Password,
		"/TargetEncryptConnection:" + packageEncryptConnection(conn.Encrypt),
		"/TargetTrustServerCertificate:" + packageTrustServerCertificate(conn),
	}

	if action == ActionScript && strings.TrimSpace(output) != "" {
		args = append(args, "/OutputPath:"+output)
	}

	return args
}

func BacpacExportArgs(conn *config.SQLConnection, database string, output string) []string {
	server := SQLPackageServerName(conn.Server)
	return []string{
		"/Action:" + string(ActionExport),
		"/TargetFile:" + output,
		"/SourceServerName:" + server,
		"/SourceDatabaseName:" + database,
		"/SourceUser:" + conn.User,
		"/SourcePassword:" + conn.Password,
		"/SourceEncryptConnection:" + packageEncryptConnection(conn.Encrypt),
		"/SourceTrustServerCertificate:" + packageTrustServerCertificate(conn),
		"/p:VerifyExtraction=False",
	}
}

func BacpacImportArgs(conn *config.SQLConnection, database string, bacpac string) []string {
	server := SQLPackageServerName(conn.Server)
	return []string{
		"/Action:" + string(ActionImport),
		"/SourceFile:" + bacpac,
		"/TargetServerName:" + server,
		"/TargetDatabaseName:" + database,
		"/TargetUser:" + conn.User,
		"/TargetPassword:" + conn.Password,
		"/TargetEncryptConnection:" + packageEncryptConnection(conn.Encrypt),
		"/TargetTrustServerCertificate:" + packageTrustServerCertificate(conn),
	}
}

func PublishProfileArgs(profile string, dacpac string, conn *config.SQLConnection, targetDatabase string, allowDrop bool) []string {
	server := SQLPackageServerName(conn.Server)
	args := []string{
		"/Action:" + string(ActionPublish),
		"/Profile:" + profile,
		"/SourceFile:" + dacpac,
		"/TargetServerName:" + server,
		"/TargetDatabaseName:" + targetDatabase,
		"/TargetUser:" + conn.User,
		"/TargetPassword:" + conn.Password,
		"/TargetEncryptConnection:" + packageEncryptConnection(conn.Encrypt),
		"/TargetTrustServerCertificate:" + packageTrustServerCertificate(conn),
	}

	if allowDrop {
		args = append(args,
			"/p:DropObjectsNotInSource=True",
			"/p:DoNotDropObjectTypes=Users;DatabaseRoles;RoleMembership;Permissions",
			"/p:DropRoleMembersNotInSource=False",
			"/p:DropPermissionsNotInSource=False",
		)
	}

	args = append(args,
		"/v:SqlServer="+server,
		"/v:TargetDb="+targetDatabase,
		"/v:TargetDB="+targetDatabase,
		"/v:SqlUser="+conn.User,
		"/v:SqlPassword="+conn.Password,
	)

	return args
}

func SQLPackageServerName(server string) string {
	trimmed := strings.TrimSpace(server)
	if runtime.GOOS == "windows" && strings.EqualFold(trimmed, "localhost") {
		return "127.0.0.1"
	}
	return server
}

var unsafeFileNameRegex = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func SafeFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "database"
	}
	value = unsafeFileNameRegex.ReplaceAllString(value, "_")
	value = strings.Trim(value, "._-")
	if value == "" {
		return "database"
	}
	return value
}

func packageEncryptConnection(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return "Strict"
	case "true":
		return "True"
	default:
		return "False"
	}
}

func packageTrustServerCertificate(conn *config.SQLConnection) string {
	if strings.TrimSpace(conn.Encrypt) == "" {
		return "True"
	}
	return packageBool(conn.TrustServerCertificate)
}

func packageBool(value bool) string {
	if value {
		return "True"
	}
	return "False"
}
