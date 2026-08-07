package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/dataseed"
)

type bootstrapFlags struct {
	env                     string
	company                 string
	database                string
	script                  string
	seedManifest            string
	skipSensitive           bool
	sensitiveSourceEnv      string
	sensitiveSourceDatabase string
	allowProd               bool
}

func newBootstrapCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap a freshly published database",
	}

	cmd.AddCommand(newBootstrapBDSistemaCommand(app))
	return cmd
}

func newBootstrapBDSistemaCommand(app *appContext) *cobra.Command {
	flags := &bootstrapFlags{}
	cmd := &cobra.Command{
		Use:   "bd-sistema",
		Short: "Apply BD_SISTEMA bootstrap data",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flags.skipSensitive && strings.TrimSpace(flags.script) != "" {
				return fmt.Errorf("--script requires --skip-sensitive; default bootstrap copies sensitive data using the split runners")
			}
			normalizedCompany, err := companyCode(flags.company)
			if err != nil {
				return err
			}

			targetDB, err := companyTargetDatabase(normalizedCompany, "_BD_SISTEMA")
			if err != nil {
				return err
			}
			if strings.TrimSpace(flags.database) != "" {
				targetDB = strings.TrimSpace(flags.database)
			}

			sqlFlags := &sqlFlags{
				env:       flags.env,
				database:  targetDB,
				allowProd: flags.allowProd,
				variables: []string{
					"Company=" + normalizedCompany,
				},
			}
			if !flags.skipSensitive {
				coreScript := resolveRepoPath(app, firstNonEmpty(app.cfg.Paths["bd_sistema_bootstrap_core_script"], "BD_SISTEMA/bootstrap/bootstrap-core.sql"))
				afterUsersScript := resolveRepoPath(app, firstNonEmpty(app.cfg.Paths["bd_sistema_bootstrap_after_users_script"], "BD_SISTEMA/bootstrap/bootstrap-after-users.sql"))
				if err := runSQLScripts(cmd, app, sqlFlags, []string{coreScript}); err != nil {
					return fmt.Errorf("bootstrap BD_SISTEMA core: %w", err)
				}
				sensitiveScript, err := generateSensitiveBootstrapScript(cmd, app, flags, normalizedCompany)
				if err != nil {
					return err
				}
				if err := runSQLScripts(cmd, app, sqlFlags, []string{sensitiveScript}); err != nil {
					return fmt.Errorf("bootstrap BD_SISTEMA sensitive: %w", err)
				}
				if err := runSQLScripts(cmd, app, sqlFlags, []string{afterUsersScript}); err != nil {
					return fmt.Errorf("bootstrap BD_SISTEMA after users: %w", err)
				}
			} else {
				script := resolveRepoPath(app, firstNonEmpty(flags.script, app.cfg.Paths["bd_sistema_bootstrap_script"], "BD_SISTEMA/bootstrap/bootstrap.sql"))
				if err := runSQLScripts(cmd, app, sqlFlags, []string{script}); err != nil {
					return fmt.Errorf("bootstrap BD_SISTEMA: %w", err)
				}
			}
			successf(cmd.OutOrStdout(), "Bootstrap OK: %s", targetDB)
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	cmd.Flags().StringVar(&flags.company, "company", "", "company code")
	cmd.Flags().StringVar(&flags.database, "database", "", "target database override")
	cmd.Flags().StringVar(&flags.script, "script", "", "bootstrap runner script path")
	cmd.Flags().StringVar(&flags.seedManifest, "seed-manifest", defaultBDSistemaSeedManifest, "seed manifest path")
	cmd.Flags().BoolVar(&flags.skipSensitive, "skip-sensitive", false, "do not generate/apply bootstrap-sensitive data; users must already exist if referenced by bootstrap data")
	cmd.Flags().StringVar(&flags.sensitiveSourceEnv, "sensitive-source-env", "", "source environment for sensitive bootstrap; defaults to --env")
	cmd.Flags().StringVar(&flags.sensitiveSourceDatabase, "sensitive-source-database", "", "source database override for sensitive bootstrap")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("company")
	return cmd
}

func generateSensitiveBootstrapScript(cmd *cobra.Command, app *appContext, flags *bootstrapFlags, company string) (string, error) {
	manifestPath := flags.seedManifest
	if !cmd.Flags().Changed("seed-manifest") {
		manifestPath = bdSistemaSeedManifestPath(app)
	}
	manifest, err := dataseed.LoadManifest(app.cfg.Root, manifestPath)
	if err != nil {
		return "", err
	}
	groupName := "company-sensitive-" + company
	group, err := manifest.Group(groupName)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(group.Phase), "bootstrap-sensitive") {
		return "", fmt.Errorf("manifest group %q must use phase bootstrap-sensitive", groupName)
	}
	if strings.TrimSpace(flags.sensitiveSourceDatabase) != "" {
		group.SourceDatabase = strings.TrimSpace(flags.sensitiveSourceDatabase)
	}

	sourceEnv := firstNonEmpty(flags.sensitiveSourceEnv, flags.env)
	if isProdLike(sourceEnv) && !flags.allowProd {
		return "", fmt.Errorf("--allow-prod is required for sensitive source env %q", sourceEnv)
	}

	infof(cmd.OutOrStdout(), "Generating sensitive bootstrap %s from %s/%s", groupName, sourceEnv, group.SourceDatabase)
	db, err := newDBService(cmd, app, sourceEnv)
	if err != nil {
		return "", err
	}
	client, err := db.open(group.SourceDatabase)
	if err != nil {
		return "", err
	}
	defer func() { _ = client.Close() }()

	output, err := dataseed.Generate(commandContext(cmd), client, dataseed.Options{
		RepoRoot: app.cfg.Root,
		Group:    group,
	})
	if err != nil {
		return "", err
	}
	successf(cmd.OutOrStdout(), "Sensitive bootstrap script: %s", output)
	return output, nil
}
