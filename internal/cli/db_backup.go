package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/backups"
	"github.com/luem2/sqlkit/internal/sqlserver"
	"github.com/luem2/sqlkit/internal/ssdt"
)

func newDBBackupCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a copy-only database backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateUserDatabaseName(flags.database); err != nil {
				return err
			}
			if err := guardProd(flags); err != nil {
				return err
			}

			started := time.Now()
			moveToHost := shouldMoveBackupToHost(flags)
			fileName := backupFileName(flags.database, started)
			hostPath := ""
			container := ""
			if moveToHost {
				hostPath = resolveBackupHostPath(app, flags.output, flags.env, flags.database, started, fileName)
				container = backupContainer(app, flags)
				if container == "" {
					return fmt.Errorf("local backups are moved to host by default, but no Docker container is configured; run from the SQL repo, pass --repo /path/to/sql, configure paths.sqlserver_container, or pass --container")
				}
			}

			backupFile := strings.TrimSpace(flags.serverOutput)
			if backupFile == "" {
				if moveToHost {
					backupFile = backups.DatedPath(resolveSQLServerBakDir(app, flags.bakDir), flags.env, flags.database, "manual", started, fileName)
				} else {
					backupFile = strings.TrimSpace(flags.output)
					if backupFile == "" {
						backupFile = backups.DatedPath(resolveSQLServerBakDir(app, flags.bakDir), flags.env, flags.database, "manual", started, fileName)
					}
				}
			}
			directoryFile := backupFile
			if container != "" {
				directoryFile = dockerBackupSource(app, backupFile)
			}
			if err := prepareBackupDirectory(cmd, app, container, directoryFile); err != nil {
				return err
			}
			sql, err := sqlserver.BackupDatabaseSQL(flags.database, backupFile)
			if err != nil {
				return err
			}
			if err := runBackupRestoreSQL(cmd, app, flags, sql, "Backup", backupFile); err != nil {
				return err
			}
			if moveToHost {
				hostPath, err := moveBackupToHost(cmd, app, container, backupFile, hostPath)
				if err != nil {
					return err
				}
				successf(cmd.OutOrStdout(), "Moved to host: %s", hostPath)
			}
			return nil
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "host backup file or directory when moving to host; defaults to ./data/backups/<env>/<database>/manual/YYYY/MM/DD; SQL Server host path otherwise")
	cmd.Flags().StringVar(&flags.serverOutput, "server-output", "", "backup file path on the SQL Server host")
	cmd.Flags().StringVar(&flags.bakDir, "bak-dir", "", "backup directory on the SQL Server host; defaults to paths.sqlserver_backup_dir or backups")
	cmd.Flags().BoolVar(&flags.moveToHost, "move-to-host", false, "force moving the generated backup from a Docker SQL Server container to the host")
	cmd.Flags().StringVar(&flags.container, "container", "", "Docker SQL Server container name; defaults to paths.sqlserver_container")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	return cmd
}

func runBackupRestoreSQL(cmd *cobra.Command, app *appContext, flags *dbFlags, statement sqlserver.Statement, label string, file string) error {
	db, err := newDBService(cmd, app, flags.env)
	if err != nil {
		return err
	}

	if err := db.execIn("master", statement); err != nil {
		return err
	}

	successf(cmd.OutOrStdout(), "%s OK: %s", label, file)
	return nil
}

func resolveSQLServerBakDir(app *appContext, value string) string {
	return firstNonEmpty(value, app.cfg.Paths["sqlserver_backup_dir"], "backups")
}

func resolveBakPath(app *appContext, value string, bakDir string) string {
	value = strings.TrimSpace(value)
	if value == "" || hasSQLServerPathSeparator(value) {
		return value
	}
	return joinSQLServerPath(resolveSQLServerBakDir(app, bakDir), sqlServerPathBase(value))
}

func shouldMoveBackupToHost(flags *dbFlags) bool {
	if flags.moveToHost {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(flags.env), "local")
}

func backupContainer(app *appContext, flags *dbFlags) string {
	return firstNonEmpty(flags.container, app.cfg.Paths["sqlserver_container"])
}

func moveBackupToHost(cmd *cobra.Command, app *appContext, container string, backupFile string, hostPath string) (string, error) {
	hostDir := filepath.Dir(hostPath)
	if err := os.MkdirAll(hostDir, 0755); err != nil {
		return "", err
	}

	sourcePath := dockerBackupSource(app, backupFile)
	source := container + ":" + sourcePath
	proc := newProcessService(cmd, app)
	result := proc.Run(app.cfg.ToolPath("docker"), "cp", source, hostPath)
	if result.ExitCode != 0 {
		return "", fmt.Errorf("docker cp failed with exit code %d\n%s", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}

	result = proc.Run(app.cfg.ToolPath("docker"), "exec", container, "rm", "-f", sourcePath)
	if result.ExitCode != 0 {
		return "", fmt.Errorf("backup copied to %s, but docker rm failed with exit code %d\n%s", hostPath, result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	return hostPath, nil
}

func copyRestoreBackupToContainer(cmd *cobra.Command, app *appContext, container string, hostPath string, database string) (string, func() error, error) {
	containerPath := restoreContainerTempPath(app, database, hostPath, time.Now())
	destination := container + ":" + containerPath
	proc := newProcessService(cmd, app)

	result := proc.Run(app.cfg.ToolPath("docker"), "cp", hostPath, destination)
	if result.ExitCode != 0 {
		return "", nil, fmt.Errorf("docker cp failed with exit code %d\n%s", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	infof(cmd.OutOrStdout(), "Staged restore backup in container: %s", containerPath)

	cleanup := func() error {
		result := proc.Run(app.cfg.ToolPath("docker"), "exec", container, "rm", "-f", containerPath)
		if result.ExitCode != 0 {
			return fmt.Errorf("docker rm failed with exit code %d\n%s", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
		}
		infof(cmd.OutOrStdout(), "Removed staged restore backup from container: %s", containerPath)
		return nil
	}

	return containerPath, cleanup, nil
}

func resolveRestoreHostBakPath(app *appContext, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}

	candidates := []string{value}
	if !filepath.IsAbs(value) {
		candidates = append([]string{resolveRepoPath(app, value)}, candidates...)
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return candidate, true
		}
		return absolute, true
	}

	return "", false
}

func restoreContainerTempPath(app *appContext, database string, hostPath string, at time.Time) string {
	extension := filepath.Ext(hostPath)
	if strings.TrimSpace(extension) == "" {
		extension = ".bak"
	}
	fileName := fmt.Sprintf("sqlkit-restore-%s-%d%s", ssdt.SafeFileName(database), at.UnixNano(), extension)
	return joinSQLServerPath(firstNonEmpty(app.cfg.Paths["sqlserver_data"], "/var/opt/mssql/data"), fileName)
}

func resolveBackupHostPath(app *appContext, output string, environment string, database string, at time.Time, fileName string) string {
	output = strings.TrimSpace(output)
	if output != "" {
		if strings.HasSuffix(output, "/") || strings.HasSuffix(output, `\`) {
			return absolutePath(filepath.Join(output, fileName))
		}
		if strings.EqualFold(filepath.Ext(output), ".bak") {
			return absolutePath(output)
		}
		if info, err := os.Stat(output); err == nil && info.IsDir() {
			return absolutePath(filepath.Join(output, fileName))
		}
		return absolutePath(output)
	}
	return backups.DatedPath(resolveManualBackupRoot(app), environment, database, "manual", at, fileName)
}

func resolveManualBackupRoot(app *appContext) string {
	if value := strings.TrimSpace(app.cfg.Paths["backups"]); value != "" {
		return resolveRepoPath(app, value)
	}
	return resolveRepoPath(app, filepath.Join("data", "backups"))
}

func absolutePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolute
}

func dockerBackupSource(app *appContext, backupFile string) string {
	if isAbsoluteSQLServerPath(backupFile) {
		return backupFile
	}
	dataDir := firstNonEmpty(app.cfg.Paths["sqlserver_data"], "/var/opt/mssql/data")
	return joinSQLServerPath(dataDir, backupFile)
}

func backupFileName(database string, at time.Time) string {
	return fmt.Sprintf("%s_MANUAL_%s_%09d.bak", ssdt.SafeFileName(database), at.Format("20060102_150405"), at.Nanosecond())
}

func joinSQLServerPath(dir string, file string) string {
	dir = strings.TrimRight(strings.TrimSpace(dir), `/\`)
	file = strings.TrimLeft(strings.TrimSpace(file), `/\`)
	separator := "/"
	if strings.Contains(dir, `\`) {
		separator = `\`
	}
	if dir == "" {
		return file
	}
	if file == "" {
		return dir
	}
	return dir + separator + file
}

func hasSQLServerPathSeparator(value string) bool {
	return strings.Contains(value, "/") || strings.Contains(value, `\`)
}

func isAbsoluteSQLServerPath(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) {
		return true
	}
	if len(value) >= 3 && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	return false
}

func sqlServerPathBase(value string) string {
	value = strings.ReplaceAll(value, `\`, "/")
	return filepath.Base(value)
}
