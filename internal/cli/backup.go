package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/backups"
	"github.com/luem2/sqlkit/internal/sqlserver"
)

type backupFlags struct {
	env               string
	typ               string
	policy            string
	database          string
	at                string
	localRoot         string
	sqlServerRoot     string
	allowProd         bool
	yes               bool
	dryRun            bool
	skipS3            bool
	all               bool
	checkDB           bool
	fail              bool
	disableRemoteCopy bool
}

func newBackupCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Operational backup policy commands",
	}
	cmd.AddCommand(newBackupRunCommand(app))
	cmd.AddCommand(newBackupVerifyCommand(app))
	cmd.AddCommand(newBackupPruneCommand(app))
	cmd.AddCommand(newBackupStatusCommand(app))
	cmd.AddCommand(newBackupHealthCommand(app))
	cmd.AddCommand(newBackupRestoreDrillCommand(app))
	return cmd
}

func newBackupHealthCommand(app *appContext) *cobra.Command {
	flags := &backupFlags{}
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Emit machine-readable backup health for monitoring",
		RunE: func(cmd *cobra.Command, args []string) error {
			policy, err := loadBackupPolicy(app, flags)
			if err != nil {
				return err
			}
			manifests, err := backups.LoadManifests(policy.LocalRoot)
			if err != nil {
				return err
			}
			health := backups.EvaluateHealth(policy, manifests, time.Now())
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(health); err != nil {
				return err
			}
			if flags.fail && !health.OK {
				return fmt.Errorf("backup health check failed")
			}
			return nil
		},
	}
	addBackupCommonFlags(cmd, flags)
	cmd.Flags().BoolVar(&flags.fail, "fail", false, "return a non-zero exit code when backup health is not OK")
	return cmd
}

func newBackupRunCommand(app *appContext) *cobra.Command {
	flags := &backupFlags{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run operational full, diff or log backups from a policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			policy, err := loadBackupPolicy(app, flags)
			if err != nil {
				return err
			}
			backupType, err := backups.NormalizeType(flags.typ)
			if err != nil {
				return err
			}
			if !policy.Enabled {
				return fmt.Errorf("backup policy for %s is disabled", policy.Environment)
			}
			if err := guardBackupProd(flags); err != nil {
				return err
			}

			db, err := newDBService(cmd, app, flags.env)
			if err != nil {
				return err
			}
			client, err := db.open("master")
			if err != nil {
				return err
			}
			defer client.Close()

			databases, err := selectedDatabases(policy, flags.database)
			if err != nil {
				return err
			}
			var failures []string
			for _, database := range databases {
				if err := runOperationalBackup(cmd, app, db, client, policy, database, backupType, flags.skipS3); err != nil {
					failures = append(failures, database+": "+err.Error())
					warnf(cmd.ErrOrStderr(), "Backup failed for %s: %v", database, err)
					continue
				}
			}
			if len(failures) > 0 {
				return fmt.Errorf("backup run finished with failures: %s", strings.Join(failures, "; "))
			}
			return nil
		},
	}
	addBackupCommonFlags(cmd, flags)
	cmd.Flags().StringVar(&flags.typ, "type", "", "backup type: full, diff or log")
	cmd.Flags().StringVar(&flags.database, "database", "", "single database from the policy to back up")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().BoolVar(&flags.skipS3, "skip-s3", false, "skip upload to S3 even when the policy has s3_prefix")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func newBackupVerifyCommand(app *appContext) *cobra.Command {
	flags := &backupFlags{}
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run RESTORE VERIFYONLY for successful manifests",
		RunE: func(cmd *cobra.Command, args []string) error {
			policy, err := loadBackupPolicy(app, flags)
			if err != nil {
				return err
			}
			db, err := newDBService(cmd, app, flags.env)
			if err != nil {
				return err
			}
			client, err := db.open("master")
			if err != nil {
				return err
			}
			defer client.Close()

			manifests, err := backups.LoadManifests(policy.LocalRoot)
			if err != nil {
				return err
			}
			if _, err := selectedDatabases(policy, flags.database); err != nil {
				return err
			}
			targets := manifestsForVerification(policy, manifests, flags)
			if len(targets) == 0 {
				return fmt.Errorf("no successful manifests found to verify")
			}
			for _, manifest := range targets {
				serverFile, cleanup, err := stageBackupFile(cmd, app, policy, manifest.File)
				if err != nil {
					return err
				}
				sql, err := sqlserver.VerifyBackupSQL(serverFile)
				if err != nil {
					cleanup()
					return err
				}
				if err := db.exec(client, sql); err != nil {
					cleanup()
					return fmt.Errorf("verify %s: %w", manifest.File, err)
				}
				cleanup()
				successf(cmd.OutOrStdout(), "Verified %s %s: %s", manifest.Database, manifest.Type, manifest.File)
			}
			return nil
		},
	}
	addBackupCommonFlags(cmd, flags)
	cmd.Flags().StringVar(&flags.database, "database", "", "single database from the policy to verify")
	cmd.Flags().BoolVar(&flags.all, "all", false, "verify all successful manifests instead of only the latest per database/type")
	return cmd
}

func newBackupPruneCommand(app *appContext) *cobra.Command {
	flags := &backupFlags{}
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune local and S3 backups older than policy retention",
		RunE: func(cmd *cobra.Command, args []string) error {
			policy, err := loadBackupPolicy(app, flags)
			if err != nil {
				return err
			}
			if err := guardBackupProd(flags); err != nil {
				return err
			}
			if !flags.dryRun && !flags.yes {
				return fmt.Errorf("--yes is required unless --dry-run is set")
			}
			manifests, err := backups.LoadManifests(policy.LocalRoot)
			if err != nil {
				return err
			}
			candidates := backups.PruneCandidates(policy, manifests, time.Now(), !flags.skipS3)
			for _, candidate := range candidates {
				infof(cmd.OutOrStdout(), "Prune %s %s %s", candidate.Database, candidate.Type, candidate.Path)
				if flags.dryRun {
					continue
				}
				if candidate.DeleteS3 {
					for _, uri := range []string{candidate.S3URI, candidate.S3ManifestURI} {
						if strings.TrimSpace(uri) == "" {
							continue
						}
						result := newProcessService(cmd, app).Run(app.cfg.ToolPath("aws"), "s3", "rm", uri)
						if result.ExitCode != 0 {
							return fmt.Errorf("aws s3 rm failed for %s: %s", uri, firstNonEmpty(result.Stderr, result.Stdout))
						}
					}
				}
				if err := backups.DeleteLocalCandidate(candidate, policy.LocalRoot); err != nil {
					return err
				}
			}
			successf(cmd.OutOrStdout(), "Prune candidates: %d", len(candidates))
			return nil
		},
	}
	addBackupCommonFlags(cmd, flags)
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "delete candidates selected by policy retention")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "show prune candidates without deleting")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().BoolVar(&flags.skipS3, "skip-s3", false, "skip deleting S3 objects")
	return cmd
}

func newBackupStatusCommand(app *appContext) *cobra.Command {
	flags := &backupFlags{}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show latest backup manifests by database and type",
		RunE: func(cmd *cobra.Command, args []string) error {
			policy, err := loadBackupPolicy(app, flags)
			if err != nil {
				return err
			}
			manifests, err := backups.LoadManifests(policy.LocalRoot)
			if err != nil {
				return err
			}
			latest := backups.LatestByDatabaseAndType(manifests)
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "DATABASE\tTYPE\tFINISHED_AT\tFILE\tS3")
			for _, database := range policy.Databases {
				for _, backupType := range []string{backups.TypeFull, backups.TypeDiff, backups.TypeLog} {
					manifest, ok := latest[database][backupType]
					if !ok {
						fmt.Fprintf(writer, "%s\t%s\tmissing\t\t\n", database, backupType)
						continue
					}
					fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%t\n", database, backupType, manifest.FinishedAt.Format(time.RFC3339), manifest.File, manifest.S3Uploaded)
				}
			}
			return writer.Flush()
		},
	}
	addBackupCommonFlags(cmd, flags)
	return cmd
}

func newBackupRestoreDrillCommand(app *appContext) *cobra.Command {
	flags := &backupFlags{}
	cmd := &cobra.Command{
		Use:   "restore-drill",
		Short: "Restore latest valid backup chains into temporary drill databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			policy, err := loadBackupPolicy(app, flags)
			if err != nil {
				return err
			}
			if !policy.RestoreDrill.Enabled {
				return fmt.Errorf("restore drill is disabled for policy %s", policy.Environment)
			}
			if err := guardBackupProd(flags); err != nil {
				return err
			}
			targetEnv := firstNonEmpty(policy.RestoreDrill.TargetEnv, flags.env)
			db, err := newDBService(cmd, app, targetEnv)
			if err != nil {
				return err
			}
			client, err := db.open("master")
			if err != nil {
				return err
			}
			defer client.Close()

			manifests, err := backups.LoadManifests(policy.LocalRoot)
			if err != nil {
				return err
			}
			at, err := parseBackupTime(flags.at)
			if err != nil {
				return err
			}
			checkDB := policy.RestoreDrill.CheckDB || flags.checkDB
			databases, err := selectedDatabases(policy, flags.database)
			if err != nil {
				return err
			}
			for _, database := range databases {
				plan, err := backups.PlanRestore(database, manifests, at)
				if err != nil {
					return err
				}
				fullFile, diffFile, logFiles, cleanup, err := stageRestorePlan(cmd, app, policy, plan)
				if err != nil {
					return err
				}
				target := database + policy.RestoreDrill.DatabaseSuffix
				sql, err := sqlserver.RestoreDrillSQL(target, fullFile, diffFile, strings.Join(logFiles, "\n"), checkDB)
				if err != nil {
					cleanup()
					return err
				}
				if err := db.exec(client, sql); err != nil {
					cleanup()
					return fmt.Errorf("restore drill %s: %w", database, err)
				}
				cleanup()
				successf(cmd.OutOrStdout(), "Restore drill OK for %s into %s", database, target)
			}
			return nil
		},
	}
	addBackupCommonFlags(cmd, flags)
	cmd.Flags().StringVar(&flags.database, "database", "", "single database from the policy to drill")
	cmd.Flags().StringVar(&flags.at, "at", "", "restore point upper bound in RFC3339 format; defaults to latest")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().BoolVar(&flags.checkDB, "checkdb", false, "force DBCC CHECKDB even if disabled in policy")
	return cmd
}

func addBackupCommonFlags(cmd *cobra.Command, flags *backupFlags) {
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	cmd.Flags().StringVar(&flags.policy, "policy", "", "backup policy TOML path")
	cmd.Flags().StringVar(&flags.localRoot, "local-root", "", "override policy local_root for this run")
	cmd.Flags().StringVar(&flags.sqlServerRoot, "sqlserver-root", "", "override policy sqlserver_root for this run")
	cmd.Flags().BoolVar(&flags.disableRemoteCopy, "disable-remote-copy", false, "ignore policy remote_copy and use direct local filesystem access")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("policy")
}

func loadBackupPolicy(app *appContext, flags *backupFlags) (*backups.Policy, error) {
	path := resolveRepoPath(app, flags.policy)
	policy, err := backups.DecodePolicy(path)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(policy.Environment, flags.env) {
		return nil, fmt.Errorf("policy environment %q does not match --env %q", policy.Environment, flags.env)
	}
	applyBackupPolicyOverrides(policy, flags)
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if !isAbsoluteBackupRoot(policy.LocalRoot) {
		policy.LocalRoot = resolveRepoPath(app, policy.LocalRoot)
	}
	if strings.TrimSpace(policy.RemoteCopy.IdentityFile) != "" && !filepath.IsAbs(policy.RemoteCopy.IdentityFile) {
		policy.RemoteCopy.IdentityFile = resolveRepoPath(app, policy.RemoteCopy.IdentityFile)
	}
	return policy, nil
}

func applyBackupPolicyOverrides(policy *backups.Policy, flags *backupFlags) {
	if strings.TrimSpace(flags.localRoot) != "" {
		policy.LocalRoot = strings.TrimSpace(flags.localRoot)
	}
	if strings.TrimSpace(flags.sqlServerRoot) != "" {
		policy.SQLServerRoot = strings.TrimSpace(flags.sqlServerRoot)
	}
	if flags.disableRemoteCopy {
		policy.RemoteCopy.Enabled = false
	}
}

func isAbsoluteBackupRoot(value string) bool {
	value = strings.TrimSpace(value)
	if filepath.IsAbs(value) || strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) {
		return true
	}
	return len(value) >= 3 && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

func guardBackupProd(flags *backupFlags) error {
	if isProdLike(flags.env) && !flags.allowProd {
		return fmt.Errorf("--allow-prod is required for env %q", flags.env)
	}
	return nil
}

func selectedDatabases(policy *backups.Policy, database string) ([]string, error) {
	database = strings.TrimSpace(database)
	if database == "" {
		return policy.Databases, nil
	}
	for _, candidate := range policy.Databases {
		if strings.EqualFold(candidate, database) {
			return []string{candidate}, nil
		}
	}
	return nil, fmt.Errorf("database %q is not listed in backup policy %s", database, policy.Environment)
}

func runOperationalBackup(cmd *cobra.Command, app *appContext, db *dbService, client *sqlserver.Client, policy *backups.Policy, database string, backupType string, skipS3 bool) error {
	started := time.Now()
	backupFile := backups.BackupPath(policy, database, backupType, started)
	serverFile := backups.SQLServerBackupPath(policy, database, backupType, started)
	manifestFile := backups.ManifestPath(policy, database, backupType, started)
	s3URI := backups.S3URI(policy, database, backupType, started, backupFile)
	s3ManifestURI := backups.S3ManifestURI(policy, database, backupType, started, manifestFile)
	manifest := &backups.Manifest{
		Environment:   policy.Environment,
		Database:      database,
		Type:          backupType,
		StartedAt:     started,
		File:          backupFile,
		ServerFile:    serverFile,
		ManifestFile:  manifestFile,
		SQLChecksum:   true,
		S3URI:         s3URI,
		S3ManifestURI: s3ManifestURI,
		Status:        "failed",
	}
	if err := backups.EnsureParent(backupFile); err != nil {
		return err
	}
	if err := prepareOperationalBackupDirectory(cmd, app, policy, serverFile); err != nil {
		manifest.Error = err.Error()
		return err
	}
	defer func() {
		if manifest.FinishedAt.IsZero() {
			manifest.FinishedAt = time.Now()
		}
		if err := backups.WriteManifest(manifest); err != nil {
			warnf(cmd.ErrOrStderr(), "Could not write manifest %s: %v", manifest.ManifestFile, err)
		}
	}()

	sql, err := sqlserver.OperationalBackupSQL(database, serverFile, backupType)
	if err != nil {
		manifest.Error = err.Error()
		return err
	}
	if err := db.exec(client, sql); err != nil {
		manifest.Error = err.Error()
		return err
	}
	manifest.FinishedAt = time.Now()
	if metadata, err := backupMetadata(db, client, serverFile); err == nil {
		backups.ApplyMetadata(manifest, metadata)
	}
	if err := collectServerBackup(cmd, app, policy, serverFile, backupFile); err != nil {
		manifest.Error = err.Error()
		return err
	}
	if size, err := backups.FileSize(backupFile); err == nil {
		manifest.SizeBytes = size
	}
	if hash, err := backups.FileSHA256(backupFile); err == nil {
		manifest.SHA256 = hash
	}
	if s3URI != "" && !skipS3 {
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("aws"), "s3", "cp", backupFile, s3URI)
		if result.ExitCode != 0 {
			manifest.Error = "s3 upload failed: " + firstNonEmpty(result.Stderr, result.Stdout)
			return fmt.Errorf("%s", manifest.Error)
		}
		manifest.S3Uploaded = true
	}
	manifest.Status = "ok"
	manifest.Error = ""
	if err := backups.WriteManifest(manifest); err != nil {
		return err
	}
	if s3URI != "" && !skipS3 {
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("aws"), "s3", "cp", manifest.ManifestFile, s3ManifestURI)
		if result.ExitCode != 0 {
			return fmt.Errorf("s3 manifest upload failed: %s", firstNonEmpty(result.Stderr, result.Stdout))
		}
	}
	successf(cmd.OutOrStdout(), "Backup OK %s %s: %s", database, backupType, backupFile)
	return nil
}

func prepareOperationalBackupDirectory(cmd *cobra.Command, app *appContext, policy *backups.Policy, serverFile string) error {
	serverDir := serverPathDir(serverFile)
	if policy.RemoteCopy.Enabled {
		return makeRemoteDirectory(cmd, app, policy.RemoteCopy, serverDir)
	}
	if strings.TrimSpace(policy.Container) != "" {
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", policy.Container, "mkdir", "-p", serverDir)
		if result.ExitCode != 0 {
			return fmt.Errorf("create backup directory in container: %s", firstNonEmpty(result.Stderr, result.Stdout))
		}
		return nil
	}
	return os.MkdirAll(serverDir, 0750)
}

func prepareBackupDirectory(cmd *cobra.Command, app *appContext, container string, serverFile string) error {
	serverDir := serverPathDir(serverFile)
	if strings.TrimSpace(container) != "" {
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", container, "mkdir", "-p", serverDir)
		if result.ExitCode != 0 {
			return fmt.Errorf("create backup directory in container: %s", firstNonEmpty(result.Stderr, result.Stdout))
		}
		return nil
	}
	return os.MkdirAll(serverDir, 0750)
}

func collectServerBackup(cmd *cobra.Command, app *appContext, policy *backups.Policy, serverFile string, localFile string) error {
	if policy.RemoteCopy.Enabled {
		if err := copyRemoteBackup(cmd, app, policy.RemoteCopy, serverFile, localFile); err != nil {
			return err
		}
		if !policy.RemoteCopy.KeepRemote {
			if err := removeRemoteFile(cmd, app, policy.RemoteCopy, serverFile, policy.SQLServerRoot); err != nil {
				return fmt.Errorf("backup copied to %s, but remote cleanup failed: %w", localFile, err)
			}
		}
		return nil
	}
	if strings.TrimSpace(policy.Container) != "" {
		source := policy.Container + ":" + serverFile
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "cp", source, localFile)
		if result.ExitCode != 0 {
			return fmt.Errorf("copy backup from container: %s", firstNonEmpty(result.Stderr, result.Stdout))
		}
		result = newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", policy.Container, "rm", "-f", serverFile)
		if result.ExitCode != 0 {
			return fmt.Errorf("backup copied to %s, but container cleanup failed: %s", localFile, firstNonEmpty(result.Stderr, result.Stdout))
		}
		return nil
	}
	if filepath.Clean(serverFile) == filepath.Clean(localFile) {
		return nil
	}
	return copyFile(serverFile, localFile)
}

func makeRemoteDirectory(cmd *cobra.Command, app *appContext, remote backups.RemoteCopy, directory string) error {
	command := "mkdir -p -- " + shellQuote(directory) + " && chmod 0770 -- " + shellQuote(directory)
	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("ssh"), append(sshArgs(remote), command)...)
	if result.ExitCode != 0 {
		return fmt.Errorf("create remote backup directory: %s", firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func copyRemoteBackup(cmd *cobra.Command, app *appContext, remote backups.RemoteCopy, serverFile string, localFile string) error {
	if err := backups.EnsureParent(localFile); err != nil {
		return err
	}
	source := remoteSpec(remote, serverFile)
	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("scp"), append(scpArgs(remote), source, localFile)...)
	if result.ExitCode != 0 {
		return fmt.Errorf("copy backup from remote host: %s", firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func removeRemoteFile(cmd *cobra.Command, app *appContext, remote backups.RemoteCopy, file string, root string) error {
	command := remoteRemoveFileCommand(file, root)
	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("ssh"), append(sshArgs(remote), command)...)
	if result.ExitCode != 0 {
		return fmt.Errorf("%s", firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func remoteRemoveFileCommand(file string, root string) string {
	command := "rm -f -- " + shellQuote(file)
	root = strings.TrimRight(strings.TrimSpace(root), "/")
	if root == "" || strings.TrimSpace(file) == "" {
		return command
	}
	command += " && root=" + shellQuote(root)
	command += " && dir=$(dirname -- " + shellQuote(file) + ")"
	command += ` && case "$dir/" in "$root"/*) ;; *) exit 0 ;; esac`
	command += ` && while [ "$dir" != "$root" ] && [ "$dir" != "/" ] && [ -n "$dir" ]; do rmdir -- "$dir" 2>/dev/null || break; dir=$(dirname -- "$dir"); done`
	return command
}

func copyLocalBackupToRemote(cmd *cobra.Command, app *appContext, remote backups.RemoteCopy, localFile string, serverFile string) error {
	if err := makeRemoteDirectory(cmd, app, remote, serverPathDir(serverFile)); err != nil {
		return err
	}
	target := remoteSpec(remote, serverFile)
	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("scp"), append(scpArgs(remote), localFile, target)...)
	if result.ExitCode != 0 {
		return fmt.Errorf("stage backup file on remote host: %s", firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func sshArgs(remote backups.RemoteCopy) []string {
	args := []string{}
	if remote.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", remote.Port))
	}
	if strings.TrimSpace(remote.IdentityFile) != "" {
		args = append(args, "-i", remote.IdentityFile)
	}
	return append(args, remote.User+"@"+remote.Host)
}

func scpArgs(remote backups.RemoteCopy) []string {
	args := []string{}
	if remote.Port > 0 {
		args = append(args, "-P", fmt.Sprintf("%d", remote.Port))
	}
	if strings.TrimSpace(remote.IdentityFile) != "" {
		args = append(args, "-i", remote.IdentityFile)
	}
	return args
}

func remoteSpec(remote backups.RemoteCopy, path string) string {
	return remote.User + "@" + remote.Host + ":" + path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func copyFile(source string, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := backups.EnsureParent(target); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func serverPathDir(value string) string {
	normalized := strings.ReplaceAll(value, `\`, "/")
	index := strings.LastIndex(normalized, "/")
	if index < 0 {
		return "."
	}
	dir := normalized[:index]
	if strings.Contains(value, `\`) && !strings.Contains(value, "/") {
		return strings.ReplaceAll(dir, "/", `\`)
	}
	return dir
}

func stageRestorePlan(cmd *cobra.Command, app *appContext, policy *backups.Policy, plan backups.RestorePlan) (string, string, []string, func(), error) {
	if policy.RemoteCopy.Enabled {
		return stageRemoteRestorePlan(cmd, app, policy, plan)
	}
	if strings.TrimSpace(policy.Container) == "" {
		logFiles := make([]string, 0, len(plan.Logs))
		for _, manifest := range plan.Logs {
			logFiles = append(logFiles, manifest.File)
		}
		diffFile := ""
		if plan.Diff != nil {
			diffFile = plan.Diff.File
		}
		return plan.Full.File, diffFile, logFiles, func() {}, nil
	}

	var staged []string
	cleanup := func() {
		for _, path := range staged {
			result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", policy.Container, "rm", "-f", path)
			if result.ExitCode != 0 {
				warnf(cmd.ErrOrStderr(), "Could not remove staged restore file %s: %s", path, firstNonEmpty(result.Stderr, result.Stdout))
			}
		}
	}
	stage := func(localFile string) (string, error) {
		serverFile := backups.SQLServerDrillPath(policy, localFile)
		if err := prepareOperationalBackupDirectory(cmd, app, policy, serverFile); err != nil {
			return "", err
		}
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "cp", localFile, policy.Container+":"+serverFile)
		if result.ExitCode != 0 {
			return "", fmt.Errorf("stage restore file %s: %s", localFile, firstNonEmpty(result.Stderr, result.Stdout))
		}
		if err := fixContainerFileOwnership(cmd, app, policy.Container, serverFile); err != nil {
			return "", err
		}
		staged = append(staged, serverFile)
		return serverFile, nil
	}

	fullFile, err := stage(plan.Full.File)
	if err != nil {
		cleanup()
		return "", "", nil, func() {}, err
	}
	diffFile := ""
	if plan.Diff != nil {
		diffFile, err = stage(plan.Diff.File)
		if err != nil {
			cleanup()
			return "", "", nil, func() {}, err
		}
	}
	logFiles := make([]string, 0, len(plan.Logs))
	for _, manifest := range plan.Logs {
		logFile, err := stage(manifest.File)
		if err != nil {
			cleanup()
			return "", "", nil, func() {}, err
		}
		logFiles = append(logFiles, logFile)
	}
	return fullFile, diffFile, logFiles, cleanup, nil
}

func stageRemoteRestorePlan(cmd *cobra.Command, app *appContext, policy *backups.Policy, plan backups.RestorePlan) (string, string, []string, func(), error) {
	var staged []string
	cleanup := func() {
		for _, path := range staged {
			if err := removeRemoteFile(cmd, app, policy.RemoteCopy, path, policy.SQLServerRoot); err != nil {
				warnf(cmd.ErrOrStderr(), "Could not remove remote staged restore file %s: %v", path, err)
			}
		}
	}
	stage := func(localFile string) (string, error) {
		serverFile := backups.SQLServerDrillPath(policy, localFile)
		if err := copyLocalBackupToRemote(cmd, app, policy.RemoteCopy, localFile, serverFile); err != nil {
			return "", err
		}
		staged = append(staged, serverFile)
		return serverFile, nil
	}

	fullFile, err := stage(plan.Full.File)
	if err != nil {
		cleanup()
		return "", "", nil, func() {}, err
	}
	diffFile := ""
	if plan.Diff != nil {
		diffFile, err = stage(plan.Diff.File)
		if err != nil {
			cleanup()
			return "", "", nil, func() {}, err
		}
	}
	logFiles := make([]string, 0, len(plan.Logs))
	for _, manifest := range plan.Logs {
		logFile, err := stage(manifest.File)
		if err != nil {
			cleanup()
			return "", "", nil, func() {}, err
		}
		logFiles = append(logFiles, logFile)
	}
	return fullFile, diffFile, logFiles, cleanup, nil
}

func stageBackupFile(cmd *cobra.Command, app *appContext, policy *backups.Policy, localFile string) (string, func(), error) {
	if policy.RemoteCopy.Enabled {
		serverFile := backups.SQLServerDrillPath(policy, localFile)
		if err := copyLocalBackupToRemote(cmd, app, policy.RemoteCopy, localFile, serverFile); err != nil {
			return "", func() {}, err
		}
		cleanup := func() {
			if err := removeRemoteFile(cmd, app, policy.RemoteCopy, serverFile, policy.SQLServerRoot); err != nil {
				warnf(cmd.ErrOrStderr(), "Could not remove remote staged backup file %s: %v", serverFile, err)
			}
		}
		return serverFile, cleanup, nil
	}
	if strings.TrimSpace(policy.Container) == "" {
		return localFile, func() {}, nil
	}
	serverFile := backups.SQLServerDrillPath(policy, localFile)
	if err := prepareOperationalBackupDirectory(cmd, app, policy, serverFile); err != nil {
		return "", func() {}, err
	}
	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "cp", localFile, policy.Container+":"+serverFile)
	if result.ExitCode != 0 {
		return "", func() {}, fmt.Errorf("stage backup file %s: %s", localFile, firstNonEmpty(result.Stderr, result.Stdout))
	}
	if err := fixContainerFileOwnership(cmd, app, policy.Container, serverFile); err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", policy.Container, "rm", "-f", serverFile)
		if result.ExitCode != 0 {
			warnf(cmd.ErrOrStderr(), "Could not remove staged backup file %s: %s", serverFile, firstNonEmpty(result.Stderr, result.Stdout))
		}
	}
	return serverFile, cleanup, nil
}

func fixContainerFileOwnership(cmd *cobra.Command, app *appContext, container string, path string) error {
	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", "--user", "0", container, "chown", "mssql:mssql", path)
	if result.ExitCode != 0 {
		return fmt.Errorf("set staged file ownership: %s", firstNonEmpty(result.Stderr, result.Stdout))
	}
	result = newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", "--user", "0", container, "chmod", "0640", path)
	if result.ExitCode != 0 {
		return fmt.Errorf("set staged file permissions: %s", firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func backupMetadata(db *dbService, client *sqlserver.Client, backupFile string) (backups.BackupMetadata, error) {
	sql, err := sqlserver.BackupMetadataSQL(backupFile)
	if err != nil {
		return backups.BackupMetadata{}, err
	}
	results, err := db.query(client, sql)
	if err != nil {
		return backups.BackupMetadata{}, err
	}
	values := firstResultRow(results)
	return backups.BackupMetadata{
		FirstLSN:          values["first_lsn"],
		LastLSN:           values["last_lsn"],
		CheckpointLSN:     values["checkpoint_lsn"],
		DatabaseBackupLSN: values["database_backup_lsn"],
		BackupStartDate:   values["backup_start_date"],
		BackupFinishDate:  values["backup_finish_date"],
	}, nil
}

func firstResultRow(results []sqlserver.ResultSet) map[string]string {
	values := map[string]string{}
	for _, result := range results {
		if len(result.Rows) == 0 {
			continue
		}
		for index, column := range result.Columns {
			if index < len(result.Rows[0]) {
				values[column] = result.Rows[0][index]
			}
		}
		return values
	}
	return values
}

func manifestsForVerification(policy *backups.Policy, manifests []backups.Manifest, flags *backupFlags) []backups.Manifest {
	var filtered []backups.Manifest
	for _, manifest := range manifests {
		if manifest.Status != "ok" {
			continue
		}
		if strings.TrimSpace(flags.database) != "" && !strings.EqualFold(manifest.Database, flags.database) {
			continue
		}
		filtered = append(filtered, manifest)
	}
	if flags.all {
		return filtered
	}
	latest := backups.LatestByDatabaseAndType(filtered)
	var selected []backups.Manifest
	databases, err := selectedDatabases(policy, flags.database)
	if err != nil {
		return nil
	}
	for _, database := range databases {
		for _, backupType := range []string{backups.TypeFull, backups.TypeDiff, backups.TypeLog} {
			if manifest, ok := latest[database][backupType]; ok {
				selected = append(selected, manifest)
			}
		}
	}
	return selected
}

func parseBackupTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("--at must use RFC3339 format: %w", err)
	}
	return parsed, nil
}
