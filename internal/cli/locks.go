package cli

import (
	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/sqlserver"
)

type locksFlags struct {
	env string
}

func newLocksCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "locks",
		Short: "Diagnose SQL Server locks",
	}

	cmd.AddCommand(newLocksDiagnoseCommand(app))

	return cmd
}

func newLocksDiagnoseCommand(app *appContext) *cobra.Command {
	flags := &locksFlags{}
	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "Show active blocking sessions and locked objects",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := newDBService(cmd, app, flags.env)
			if err != nil {
				return err
			}

			resultSets, err := db.queryIn("master", sqlserver.LocksDiagnoseSQL())
			if err != nil {
				return err
			}
			printResultSets(cmd.OutOrStdout(), resultSets)
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	_ = cmd.MarkFlagRequired("env")
	return cmd
}
