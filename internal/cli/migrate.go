package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/migration"
)

type migrateFlags struct {
	env       string
	company   string
	companyID int
	database  string
	manifest  string
	step      string
	from      string
	to        string
	all       bool
	allowProd bool
}

func newMigrateCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Operational migration workflows",
	}

	cmd.AddCommand(newMigrateBDSistemaCommand(app))
	return cmd
}

func newMigrateBDSistemaCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bd-sistema",
		Short: "Run BD_SISTEMA migration steps",
	}

	cmd.AddCommand(newMigrateBDSistemaListCommand(app))
	cmd.AddCommand(newMigrateBDSistemaRunCommand(app))
	return cmd
}

func newMigrateBDSistemaListCommand(app *appContext) *cobra.Command {
	flags := &migrateFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List BD_SISTEMA migration steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := loadBDSistemaMigrationManifest(app, flags.manifest)
			if err != nil {
				return err
			}
			for _, step := range manifest.Steps {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%02d  %-28s  %s\n", step.Order, step.Name, step.Description); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.manifest, "manifest", "", "migration manifest path")
	return cmd
}

func newMigrateBDSistemaRunCommand(app *appContext) *cobra.Command {
	flags := &migrateFlags{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run BD_SISTEMA migration steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedCompany, err := companyCode(flags.company)
			if err != nil {
				return err
			}
			if strings.TrimSpace(flags.database) == "" {
				return fmt.Errorf("--database is required")
			}

			manifest, err := loadBDSistemaMigrationManifest(app, flags.manifest)
			if err != nil {
				return err
			}
			steps, err := manifest.SelectSteps(flags.step, flags.from, flags.to, flags.all)
			if err != nil {
				return err
			}
			companyID := flags.companyID
			if companyID <= 0 {
				var ok bool
				companyID, ok = manifest.CompanyID(normalizedCompany)
				if !ok {
					return fmt.Errorf("--company-id is required; manifest has no id_empresa for company %s", normalizedCompany)
				}
			}

			scripts, err := migration.ResolveStepScripts(app.cfg.Root, steps)
			if err != nil {
				return err
			}
			sqlFlags := &sqlFlags{
				env:       flags.env,
				database:  strings.TrimSpace(flags.database),
				allowProd: flags.allowProd,
				variables: []string{
					"Company=" + normalizedCompany,
					"CompanyId=" + strconv.Itoa(companyID),
				},
			}
			if err := runSQLScripts(cmd, app, sqlFlags, scripts); err != nil {
				return fmt.Errorf("migrate BD_SISTEMA: %w", err)
			}
			successf(cmd.OutOrStdout(), "Migration OK: %d step(s)", len(steps))
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	cmd.Flags().StringVar(&flags.company, "company", "", "company code")
	cmd.Flags().IntVar(&flags.companyID, "company-id", 0, "legacy id_empresa override")
	cmd.Flags().StringVar(&flags.database, "database", "", "target database")
	cmd.Flags().StringVar(&flags.manifest, "manifest", "", "migration manifest path")
	cmd.Flags().StringVar(&flags.step, "step", "", "single migration step name")
	cmd.Flags().StringVar(&flags.from, "from", "", "first migration step name for an inclusive range")
	cmd.Flags().StringVar(&flags.to, "to", "", "last migration step name for an inclusive range")
	cmd.Flags().BoolVar(&flags.all, "all", false, "run all migration steps")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("company")
	_ = cmd.MarkFlagRequired("database")
	return cmd
}

func loadBDSistemaMigrationManifest(app *appContext, explicit string) (*migration.Manifest, error) {
	path := resolveRepoPath(app, firstNonEmpty(explicit, app.cfg.Paths["bd_sistema_migration_manifest"], "BD_SISTEMA/migration/bd-sistema.toml"))
	return migration.Load(path)
}
