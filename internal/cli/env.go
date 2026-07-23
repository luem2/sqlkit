package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/config"
)

func newEnvCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Inspect configured SQL environments",
	}

	cmd.AddCommand(newEnvListCommand(app))
	cmd.AddCommand(newEnvCheckCommand(app))

	return cmd
}

func newEnvListCommand(app *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List known environments and user config status",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, envName := range config.ValidEnvNames {
				status := "missing"
				if envCfg, ok := app.cfg.Envs[envName]; ok {
					status = "partial"
					if envCfg.Server != "" && envCfg.User != "" && envCfg.PasswordKey != "" {
						status = "configured"
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %s\n", envName, status)
			}
			return nil
		},
	}
}

func newEnvCheckCommand(app *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "check <env>",
		Short: "Validate connection settings for an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := loadConnection(app, args[0])
			if err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Environment OK: %s", args[0])
			infof(cmd.OutOrStdout(), "Server: %s", conn.Server)
			infof(cmd.OutOrStdout(), "User: %s", conn.User)
			infof(cmd.OutOrStdout(), "Password: configured")
			infof(cmd.OutOrStdout(), "Encrypt: %s", conn.Encrypt)
			infof(cmd.OutOrStdout(), "Trust server certificate: %t", conn.TrustServerCertificate)
			return nil
		},
	}
}
