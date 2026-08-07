package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/dataseed"
)

type dbDataScriptFlags struct {
	env       string
	manifest  string
	group     string
	tables    []string
	output    string
	sourceDB  string
	allowProd bool
}

const defaultBDSistemaSeedManifest = "BD_SISTEMA/seeds/data-seeds.manifest.toml"

func newDBDataScriptCommand(app *appContext) *cobra.Command {
	flags := &dbDataScriptFlags{}
	cmd := &cobra.Command{
		Use:   "data-script",
		Short: "Generate versionable seed data SQL from a manifest group",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDBDataScript(cmd, app, flags)
		},
	}
	cmd.Flags().StringVar(&flags.env, "env", "", "source environment; defaults to manifest defaults.source_env")
	cmd.Flags().StringVar(&flags.manifest, "manifest", defaultBDSistemaSeedManifest, "seed manifest path")
	cmd.Flags().StringVar(&flags.group, "group", "", "manifest group to generate")
	cmd.Flags().StringArrayVar(&flags.tables, "table", nil, "table from the group to generate; repeatable and requires --output")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "override output path")
	cmd.Flags().StringVar(&flags.sourceDB, "source-database", "", "source database override")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow generation from prod or prod-legacy")
	_ = cmd.MarkFlagRequired("group")
	return cmd
}

func bdSistemaSeedManifestPath(app *appContext) string {
	if app != nil && app.cfg != nil {
		return firstNonEmpty(app.cfg.Paths["bd_sistema_seed_manifest"], defaultBDSistemaSeedManifest)
	}
	return defaultBDSistemaSeedManifest
}

func runDBDataScript(cmd *cobra.Command, app *appContext, flags *dbDataScriptFlags) error {
	manifestPath := flags.manifest
	if !cmd.Flags().Changed("manifest") {
		manifestPath = bdSistemaSeedManifestPath(app)
	}
	manifest, err := dataseed.LoadManifest(app.cfg.Root, manifestPath)
	if err != nil {
		return err
	}
	group, err := manifest.Group(flags.group)
	if err != nil {
		return err
	}
	if len(flags.tables) > 0 && strings.TrimSpace(flags.output) == "" {
		return fmt.Errorf("--output is required when using --table to avoid overwriting the full group script")
	}
	group, err = dataseed.FilterGroupTables(group, flags.tables)
	if err != nil {
		return err
	}
	if strings.TrimSpace(flags.output) != "" {
		group.Output = flags.output
	}
	if strings.TrimSpace(flags.sourceDB) != "" {
		group.SourceDatabase = strings.TrimSpace(flags.sourceDB)
	}

	env := firstNonEmpty(flags.env, manifest.Defaults.SourceEnv)
	if strings.TrimSpace(env) == "" {
		return fmt.Errorf("--env is required when manifest defaults.source_env is empty")
	}
	if isProdLike(env) && !flags.allowProd {
		return fmt.Errorf("--allow-prod is required for source env %q", env)
	}

	db, err := newDBService(cmd, app, env)
	if err != nil {
		return err
	}
	client, err := db.open(group.SourceDatabase)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	output, err := dataseed.Generate(commandContext(cmd), client, dataseed.Options{
		RepoRoot: app.cfg.Root,
		Group:    group,
	})
	if err != nil {
		return err
	}

	successf(cmd.OutOrStdout(), "Data seed script: %s", output)
	return nil
}
