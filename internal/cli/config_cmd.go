package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/luem2/sqlkit/internal/config"
)

type configFlags struct {
	force                  bool
	server                 string
	user                   string
	password               string
	encrypt                string
	trustServerCertificate string
}

func newConfigCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage sqlkit configuration",
	}

	cmd.AddCommand(newConfigInitCommand())
	cmd.AddCommand(newConfigSetRepoCommand())
	cmd.AddCommand(newConfigPathCommand())
	cmd.AddCommand(newConfigEnvCommand(app))
	cmd.AddCommand(newConfigSecretCommand(app))

	return cmd
}

func newConfigSecretCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage generic user secrets in keyring",
	}
	cmd.AddCommand(newConfigSecretSetCommand(app))
	cmd.AddCommand(newConfigSecretGetCommand())
	return cmd
}

func newConfigSecretGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Print a named secret from keyring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			secret, err := config.LoadNamedSecret(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), secret)
			return err
		},
	}
}

func newConfigSecretSetCommand(app *appContext) *cobra.Command {
	flags := &configFlags{}
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Store a named secret in keyring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("secret name is required")
			}
			secret := flags.password
			var err error
			if strings.TrimSpace(secret) == "" {
				if err := requireInteractive(app, "--password"); err != nil {
					return err
				}
				secret, err = promptPassword(cmd, "Secret")
				if err != nil {
					return err
				}
			}

			key := config.SecretKey(name)
			if err := config.SetSecret(key, secret); err != nil {
				return fmt.Errorf("store secret in keyring: %w", err)
			}

			cfg, err := config.LoadUserConfig()
			if err != nil {
				return err
			}
			cfg.Secrets[name] = key
			path, err := config.WriteUserConfig(cfg)
			if err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Secret configured: %s", name)
			infof(cmd.OutOrStdout(), "Config: %s", path)
			infof(cmd.OutOrStdout(), "Secret: keyring:%s", key)
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.password, "password", "", "secret value; stored in keyring")
	return cmd
}

func newConfigInitCommand() *cobra.Command {
	flags := &configFlags{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the user sqlkit config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.UserConfigPath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); err == nil && !flags.force {
				return fmt.Errorf("%s already exists; use --force to overwrite it", path)
			} else if err != nil && !os.IsNotExist(err) {
				return err
			}

			cfg := config.DefaultUserConfig()
			if _, err := config.WriteUserConfig(cfg); err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Created: %s", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flags.force, "force", false, "overwrite existing user config")
	return cmd
}

func newConfigSetRepoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-repo <path>",
		Short: "Set the default SQL repo for commands run outside a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			info, err := os.Stat(repo)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", repo)
			}

			path, err := config.WriteUserDefaultRepo(repo)
			if err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Default repo: %s", repo)
			infof(cmd.OutOrStdout(), "Config: %s", path)
			return nil
		},
	}
	return cmd
}

func newConfigPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the user sqlkit config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.UserConfigPath()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
}

func newConfigEnvCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage user SQL environment config",
	}
	cmd.AddCommand(newConfigEnvSetCommand(app))
	return cmd
}

func newConfigEnvSetCommand(app *appContext) *cobra.Command {
	flags := &configFlags{}
	cmd := &cobra.Command{
		Use:   "set <env>",
		Short: "Configure an environment and store its password in keyring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName, err := config.NormalizeEnvName(args[0])
			if err != nil {
				return err
			}

			cfg, err := config.LoadUserConfig()
			if err != nil {
				return err
			}
			envCfg := cfg.Envs[envName]

			server := strings.TrimSpace(flags.server)
			if server == "" {
				if app.nonInteractive {
					server = envCfg.Server
					if strings.TrimSpace(server) == "" {
						return requireInteractive(app, "--server")
					}
				} else {
					server, err = promptDefault(cmd, "SQL Server", envCfg.Server)
					if err != nil {
						return err
					}
				}
			}
			user := strings.TrimSpace(flags.user)
			if user == "" {
				if app.nonInteractive {
					user = envCfg.User
					if strings.TrimSpace(user) == "" {
						return requireInteractive(app, "--user")
					}
				} else {
					user, err = promptDefault(cmd, "SQL User", envCfg.User)
					if err != nil {
						return err
					}
				}
			}
			password := flags.password
			passwordKey := envCfg.PasswordKey
			if strings.TrimSpace(password) == "" {
				if app.nonInteractive {
					if strings.TrimSpace(passwordKey) == "" {
						return requireInteractive(app, "--password")
					}
				} else {
					password, err = promptPassword(cmd, "SQL Password")
					if err != nil {
						return err
					}
				}
			}

			if strings.TrimSpace(password) != "" {
				passwordKey = config.KeyringKey(envName)
				if err := config.SetSecret(passwordKey, password); err != nil {
					return fmt.Errorf("store password in keyring: %w", err)
				}
			}

			encrypt := envCfg.Encrypt
			if strings.TrimSpace(flags.encrypt) != "" {
				encrypt, err = config.NormalizeEncrypt(flags.encrypt)
				if err != nil {
					return err
				}
			}

			trustServerCertificate := envCfg.TrustServerCertificate
			if strings.TrimSpace(flags.trustServerCertificate) != "" {
				normalized, err := parseConfigBool(flags.trustServerCertificate)
				if err != nil {
					return err
				}
				trustServerCertificate = &normalized
			}

			cfg.Envs[envName] = config.EnvConfig{
				Server:                 server,
				User:                   user,
				PasswordKey:            passwordKey,
				Encrypt:                encrypt,
				TrustServerCertificate: trustServerCertificate,
			}

			path, err := config.WriteUserConfig(cfg)
			if err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Environment configured: %s", envName)
			infof(cmd.OutOrStdout(), "Config: %s", path)
			infof(cmd.OutOrStdout(), "Password: keyring:%s", passwordKey)
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.server, "server", "", "SQL Server")
	cmd.Flags().StringVar(&flags.user, "user", "", "SQL user")
	cmd.Flags().StringVar(&flags.password, "password", "", "SQL password; stored in keyring")
	cmd.Flags().StringVar(&flags.encrypt, "encrypt", "", "SQL encryption: disable, false, true or strict")
	cmd.Flags().StringVar(&flags.trustServerCertificate, "trust-server-certificate", "", "trust SQL Server certificate: true or false")
	return cmd
}

func parseConfigBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("boolean value must be true or false")
	}
}

func promptDefault(cmd *cobra.Command, label string, current string) (string, error) {
	if strings.TrimSpace(current) != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]: ", label, current); err != nil {
			return "", err
		}
	} else {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: ", label); err != nil {
			return "", err
		}
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = current
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

func promptPassword(cmd *cobra.Command, label string) (string, error) {
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: ", label); err != nil {
		return "", err
	}
	if file, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		password, err := term.ReadPassword(int(file.Fd()))
		if _, printErr := fmt.Fprintln(cmd.OutOrStdout()); printErr != nil {
			return "", printErr
		}
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(string(password))
		if value == "" {
			return "", fmt.Errorf("%s is required", label)
		}
		return value, nil
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}
