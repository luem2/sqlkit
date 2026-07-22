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
	env       string
	company   string
	dacpac    string
	profile   string
	allowProd bool
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

			targetDB, err := companyTargetDatabase(flags.company, "_BD_SISTEMA")
			if err != nil {
				return err
			}
			dacpac := resolveRepoPath(app, firstNonEmpty(flags.dacpac, app.cfg.Paths["bd_sistema_dacpac"], "BD_SISTEMA/bin/Debug/BD_SISTEMA.dacpac"))
			profile := resolvePublishProfile(app, flags.profile, profileKey("bd_sistema", flags.env), defaultBDSistemaProfile(flags.env))
			logFile := filepath.Join(resolveLogsDir(app, "publish"), fmt.Sprintf("BD_SISTEMA-%s.log", normalizedEnv))

			return runProfilePublish(cmd, app, flags, publishProfileRequest{
				systemName:  "BD_SISTEMA",
				targetDB:    targetDB,
				dacpac:      dacpac,
				profile:     profile,
				logFile:     logFile,
				company:     flags.company,
				allowDrop:   publishProfileEnv(flags.env) == "test",
				blockProd:   false,
				requireAWS:  true,
				envForLog:   strings.ToUpper(normalizedEnv),
				description: "BD_SISTEMA",
			})
		},
	}
	addPublishEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.company, "company", "", "company code used to resolve target database")
	cmd.Flags().StringVar(&flags.dacpac, "dacpac", "", "dacpac path")
	cmd.Flags().StringVar(&flags.profile, "profile", "", "publish profile path")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
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
			})
		},
	}
	addPublishEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.dacpac, "dacpac", "", "dacpac path")
	cmd.Flags().StringVar(&flags.profile, "profile", "", "publish profile path")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
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
	_ = cmd.MarkFlagRequired("company")
	return cmd
}

type publishProfileRequest struct {
	systemName  string
	targetDB    string
	dacpac      string
	profile     string
	logFile     string
	company     string
	allowDrop   bool
	blockProd   bool
	requireAWS  bool
	envForLog   string
	description string
}

func runProfilePublish(cmd *cobra.Command, app *appContext, flags *publishFlags, req publishProfileRequest) error {
	if isProdLike(flags.env) && !flags.allowProd {
		return fmt.Errorf("--allow-prod is required for env %q", flags.env)
	}

	conn, err := loadConnection(app, flags.env)
	if err != nil {
		return err
	}

	args := ssdt.PublishProfileArgs(req.profile, req.dacpac, conn, req.targetDB, req.allowDrop)
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
	return nil
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
	normalized := strings.TrimSpace(company)
	if normalized == "" {
		return "", fmt.Errorf("--company is required")
	}
	firstLetter := []rune(normalized)[0]
	return strings.ToUpper(string(firstLetter)) + suffix, nil
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
