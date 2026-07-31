package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/confirm"
	"github.com/luem2/sqlkit/internal/ssdt"
)

type publishFlags struct {
	env          string
	company      string
	database     string
	dacpac       string
	profile      string
	allowProd    bool
	skipSecurity bool
}

func newPublishCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Project publish workflows",
	}

	cmd.AddCommand(newPublishBDSistemaCommand(app))
	cmd.AddCommand(newPublishGrupoCentralCommand(app))
	cmd.AddCommand(newPublishFacturacionCommand(app))

	return cmd
}

func newPublishBDSistemaCommand(app *appContext) *cobra.Command {
	flags := &publishFlags{}
	cmd := &cobra.Command{
		Use:   "bd-sistema",
		Short: "Publish BD_SISTEMA for a company",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedEnv := strings.ToLower(strings.TrimSpace(flags.env))
			if normalizedEnv == "prod-legacy" {
				return fmt.Errorf("BD_SISTEMA cannot be published to prod-legacy. Use 'local' or 'prod'")
			}
			if strings.TrimSpace(flags.company) == "" {
				return fmt.Errorf("--company is required")
			}
			normalizedCompany, err := companyCode(flags.company)
			if err != nil {
				return err
			}

			targetDB, err := companyTargetDatabase(flags.company, "_BD_SISTEMA")
			if err != nil {
				return err
			}
			if strings.TrimSpace(flags.database) != "" {
				targetDB = strings.TrimSpace(flags.database)
			}
			dacpac := resolveRepoPath(app, firstNonEmpty(flags.dacpac, app.cfg.Paths["bd_sistema_dacpac"], "BD_SISTEMA/bin/Debug/BD_SISTEMA.dacpac"))
			profile := resolvePublishProfile(app, flags.profile, profileKey("bd_sistema", flags.env), defaultBDSistemaProfile(flags.env))
			logFile := filepath.Join(resolveLogsDir(app, "publish"), fmt.Sprintf("BD_SISTEMA-%s.log", normalizedEnv))

			return runProfilePublish(cmd, app, flags, publishProfileRequest{
				systemName: "BD_SISTEMA",
				targetDB:   targetDB,
				dacpac:     dacpac,
				profile:    profile,
				logFile:    logFile,
				company:    normalizedCompany,
				sqlcmdVariables: map[string]string{
					"Company": normalizedCompany,
				},
				allowDrop:   publishProfileEnv(flags.env) == "test",
				blockProd:   false,
				requireAWS:  true,
				envForLog:   strings.ToUpper(normalizedEnv),
				description: "BD_SISTEMA",
				security: publishSecurityRequest{
					databaseScriptPathKey:  "bd_sistema_security_script",
					databaseScriptFallback: "_infra/security/bd-sistema.sql",
				},
			})
		},
	}
	addPublishEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.company, "company", "", "company code used to resolve target database")
	cmd.Flags().StringVar(&flags.database, "database", "", "target database override")
	cmd.Flags().StringVar(&flags.dacpac, "dacpac", "", "dacpac path")
	cmd.Flags().StringVar(&flags.profile, "profile", "", "publish profile path")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().BoolVar(&flags.skipSecurity, "skip-security", false, "skip post-publish SQL security scripts")
	_ = cmd.MarkFlagRequired("company")
	return cmd
}

func newPublishGrupoCentralCommand(app *appContext) *cobra.Command {
	flags := &publishFlags{}
	cmd := &cobra.Command{
		Use:   "grupo-central",
		Short: "Publish GRUPO_CENTRAL",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedEnv := strings.ToLower(strings.TrimSpace(flags.env))
			dacpac := resolveRepoPath(app, firstNonEmpty(flags.dacpac, app.cfg.Paths["grupo_central_dacpac"], "GRUPO_CENTRAL/bin/Debug/GRUPO_CENTRAL.dacpac"))
			profile := resolvePublishProfile(app, flags.profile, profileKey("grupo_central", flags.env), defaultGrupoCentralProfile(flags.env))
			logFile := filepath.Join(resolveLogsDir(app, "publish"), fmt.Sprintf("GRUPO_CENTRAL-%s.log", normalizedEnv))

			return runProfilePublish(cmd, app, flags, publishProfileRequest{
				systemName:  "GRUPO_CENTRAL",
				targetDB:    "GRUPO_CENTRAL",
				dacpac:      dacpac,
				profile:     profile,
				logFile:     logFile,
				allowDrop:   publishProfileEnv(flags.env) == "test",
				requireAWS:  true,
				envForLog:   strings.ToUpper(normalizedEnv),
				description: "GRUPO_CENTRAL",
				security: publishSecurityRequest{
					databaseScriptPathKey:  "grupo_central_security_script",
					databaseScriptFallback: "_infra/security/grupo-central.sql",
				},
			})
		},
	}
	addPublishEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.dacpac, "dacpac", "", "dacpac path")
	cmd.Flags().StringVar(&flags.profile, "profile", "", "publish profile path")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().BoolVar(&flags.skipSecurity, "skip-security", false, "skip post-publish SQL security scripts")
	return cmd
}

func newPublishFacturacionCommand(app *appContext) *cobra.Command {
	flags := &publishFlags{}
	cmd := &cobra.Command{
		Use:   "facturacion",
		Short: "Publish FACTURACION for a company",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedEnv := strings.ToLower(strings.TrimSpace(flags.env))
			targetDB, err := companyTargetDatabase(flags.company, "_FACTURACION")
			if err != nil {
				return err
			}

			dacpac := resolveRepoPath(app, firstNonEmpty(flags.dacpac, app.cfg.Paths["facturacion_dacpac"], "FACTURACION/bin/Debug/FACTURACION.dacpac"))
			profile := resolvePublishProfile(app, flags.profile, profileKey("facturacion", flags.env), defaultFacturacionProfile(flags.env))
			logFile := filepath.Join(resolveLogsDir(app, "publish"), fmt.Sprintf("FACTURACION-%s.log", normalizedEnv))

			return runProfilePublish(cmd, app, flags, publishProfileRequest{
				systemName:  "FACTURACION",
				targetDB:    targetDB,
				dacpac:      dacpac,
				profile:     profile,
				logFile:     logFile,
				company:     strings.TrimSpace(flags.company),
				allowDrop:   publishProfileEnv(flags.env) == "test",
				blockProd:   false,
				requireAWS:  true,
				envForLog:   strings.ToUpper(normalizedEnv),
				description: "FACTURACION",
			})
		},
	}
	addPublishEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.company, "company", "", "company code used to resolve target database")
	cmd.Flags().StringVar(&flags.dacpac, "dacpac", "", "dacpac path")
	cmd.Flags().StringVar(&flags.profile, "profile", "", "publish profile path")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().BoolVar(&flags.skipSecurity, "skip-security", false, "skip post-publish SQL security scripts")
	_ = cmd.MarkFlagRequired("company")
	return cmd
}

type publishProfileRequest struct {
	systemName      string
	targetDB        string
	dacpac          string
	profile         string
	logFile         string
	company         string
	allowDrop       bool
	blockProd       bool
	requireAWS      bool
	envForLog       string
	description     string
	security        publishSecurityRequest
	sqlcmdVariables map[string]string
}

type publishSecurityRequest struct {
	databaseScriptPathKey  string
	databaseScriptFallback string
}

func runProfilePublish(cmd *cobra.Command, app *appContext, flags *publishFlags, req publishProfileRequest) error {
	if isProdLike(flags.env) && !flags.allowProd {
		return fmt.Errorf("--allow-prod is required for env %q", flags.env)
	}

	conn, err := loadConnection(app, flags.env)
	if err != nil {
		return err
	}
	if err := validatePostPublishSecurity(app, flags, req); err != nil {
		return err
	}

	args := ssdt.PublishProfileArgs(req.profile, req.dacpac, conn, req.targetDB, req.allowDrop, req.sqlcmdVariables)
	if req.requireAWS && isAWSServer(conn.Server) {
		warnf(cmd.OutOrStdout(), "AWS target detected (%s).", conn.Server)
		if err := requireInteractive(app, "interactive AWS confirmation"); err != nil {
			return err
		}
		if err := confirm.ExactFold(cmd.InOrStdin(), cmd.OutOrStdout(), "AWS"); err != nil {
			return err
		}
	}

	header := publishLogHeader(req, conn.Server, conn.User)
	result := newProcessService(cmd, app).RunStreamingRedacted([]string{conn.Password}, app.cfg.ToolPath("sqlpackage"), args...)
	if err := writePublishLog(req.logFile, header, result.Stdout, result.Stderr); err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return sqlPackageError("publish", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	successf(cmd.OutOrStdout(), "Publish OK: %s", req.targetDB)
	if err := runPostPublishSecurity(cmd, app, flags, req); err != nil {
		return err
	}
	return nil
}

func validatePostPublishSecurity(app *appContext, flags *publishFlags, req publishProfileRequest) error {
	if flags.skipSecurity || !req.security.enabled() {
		return nil
	}
	for _, secretName := range publishSecuritySecretNames() {
		if _, err := resolveAppSecretValue(app, secretName); err != nil {
			return err
		}
	}
	for _, script := range []string{
		publishSecurityLoginsScript(app),
		publishSecurityDatabaseScript(app, req.security),
	} {
		if _, err := os.Stat(script); err != nil {
			return fmt.Errorf("security script not found %q: %w", script, err)
		}
	}
	return nil
}

func runPostPublishSecurity(cmd *cobra.Command, app *appContext, flags *publishFlags, req publishProfileRequest) error {
	if flags.skipSecurity {
		warnf(cmd.OutOrStdout(), "Skipping post-publish security.")
		return nil
	}
	if !req.security.enabled() {
		return nil
	}

	infof(cmd.OutOrStdout(), "Applying SQL security for %s.", req.targetDB)
	loginScript := publishSecurityLoginsScript(app)
	databaseScript := publishSecurityDatabaseScript(app, req.security)

	loginFlags := &sqlFlags{
		env:        flags.env,
		database:   "master",
		allowProd:  flags.allowProd,
		secretVars: publishSecuritySecretVars(),
	}
	if err := runSQLScripts(cmd, app, loginFlags, []string{loginScript}); err != nil {
		return fmt.Errorf("apply login security: %w", err)
	}

	databaseFlags := &sqlFlags{
		env:       flags.env,
		database:  req.targetDB,
		allowProd: flags.allowProd,
	}
	if err := runSQLScripts(cmd, app, databaseFlags, []string{databaseScript}); err != nil {
		return fmt.Errorf("apply database security: %w", err)
	}
	successf(cmd.OutOrStdout(), "Security OK: %s", req.targetDB)
	return nil
}

func (r publishSecurityRequest) enabled() bool {
	return strings.TrimSpace(r.databaseScriptPathKey) != "" || strings.TrimSpace(r.databaseScriptFallback) != ""
}

func publishSecurityLoginsScript(app *appContext) string {
	return resolveRepoPath(app, firstNonEmpty(app.cfg.Paths["security_logins_script"], "_infra/logins/apply.sql"))
}

func publishSecurityDatabaseScript(app *appContext, security publishSecurityRequest) string {
	return resolveRepoPath(app, firstNonEmpty(app.cfg.Paths[security.databaseScriptPathKey], security.databaseScriptFallback))
}

func publishSecuritySecretVars() []string {
	return []string{
		"DbaPassword=dba-login-password",
		"StkPassword=stk-login-password",
		"ErpPassword=erp-login-password",
		"TesterPassword=tester-login-password",
	}
}

func publishSecuritySecretNames() []string {
	return []string{
		"dba-login-password",
		"stk-login-password",
		"erp-login-password",
		"tester-login-password",
	}
}

func addPublishEnvFlag(cmd *cobra.Command, flags *publishFlags) {
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	_ = cmd.MarkFlagRequired("env")
}

func publishProfileEnv(envName string) string {
	switch strings.ToLower(strings.TrimSpace(envName)) {
	case "local":
		return "test"
	case "prod", "prod-legacy":
		return "prod"
	default:
		return ""
	}
}

func profileKey(prefix string, envName string) string {
	return prefix + "_" + publishProfileEnv(envName) + "_profile"
}

func defaultBDSistemaProfile(envName string) string {
	if publishProfileEnv(envName) == "prod" {
		return "BD_SISTEMA/Profiles/PROD.publish.xml"
	}
	return "BD_SISTEMA/Profiles/TEST.publish.xml"
}

func defaultGrupoCentralProfile(envName string) string {
	if publishProfileEnv(envName) == "prod" {
		return "GRUPO_CENTRAL/Profiles/PROD.publish.xml"
	}
	return "GRUPO_CENTRAL/Profiles/TEST.publish.xml"
}

func defaultFacturacionProfile(envName string) string {
	if publishProfileEnv(envName) == "prod" {
		return "FACTURACION/Profiles/PROD.publish.xml"
	}
	return "FACTURACION/Profiles/TEST.publish.xml"
}

func companyTargetDatabase(company string, suffix string) (string, error) {
	normalized, err := companyCode(company)
	if err != nil {
		return "", err
	}
	return normalized + suffix, nil
}

func companyCode(company string) (string, error) {
	normalized := strings.TrimSpace(company)
	if normalized == "" {
		return "", fmt.Errorf("--company is required")
	}
	firstLetter := []rune(normalized)[0]
	return strings.ToUpper(string(firstLetter)), nil
}

func resolvePublishProfile(app *appContext, explicit string, configKey string, fallback string) string {
	return resolveRepoPath(app, firstNonEmpty(explicit, app.cfg.Paths[configKey], fallback))
}

var awsServerPattern = regexp.MustCompile(`(?i)(^|[.-])aws([.-]|$)`)

func isAWSServer(server string) bool {
	return awsServerPattern.MatchString(server)
}

func publishLogHeader(req publishProfileRequest, server string, user string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Publish %s %s %s\n", req.systemName, req.envForLog, time.Now().Format(time.RFC3339))
	if req.company != "" {
		fmt.Fprintf(&builder, "Company=%s ", req.company)
	}
	fmt.Fprintf(&builder, "Db=%s Server=%s User=%s\n", req.targetDB, server, user)
	return builder.String()
}

func writePublishLog(path string, header string, stdout string, stderr string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := strings.TrimRight(header+"\n"+stdout+"\n"+stderr, "\r\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}
