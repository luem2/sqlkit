package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/config"
	"github.com/luem2/sqlkit/internal/secrets"
)

type appSecretSource struct {
	Value string
	Label string
}

func newSecretsCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Inspect secrets required by the SQL repo",
	}
	cmd.AddCommand(newSecretsListCommand(app))
	cmd.AddCommand(newSecretsCheckCommand(app))
	cmd.AddCommand(newSecretsTemplateCommand(app))
	return cmd
}

func newSecretsListCommand(app *appContext) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List secrets declared in secretspec.toml",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, path, err := loadAppSecretSpec(app)
			if err != nil {
				return err
			}
			infof(cmd.OutOrStdout(), "Spec: %s", path)
			items := spec.All()
			if strings.TrimSpace(profile) != "" {
				items = spec.Required(profile)
			}
			for _, item := range items {
				envName := firstNonEmpty(item.Env, item.Name)
				required := ""
				if item.Required {
					required = " required"
				}
				infof(cmd.OutOrStdout(), "%s.%s -> %s%s", item.Profile, item.Name, envName, required)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "show required secrets for one profile")
	return cmd
}

func newSecretsCheckCommand(app *appContext) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check whether required secrets are available",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(profile) == "" {
				return fmt.Errorf("--profile is required")
			}
			spec, path, err := loadAppSecretSpec(app)
			if err != nil {
				return err
			}
			infof(cmd.OutOrStdout(), "Spec: %s", path)
			missing := 0
			for _, item := range spec.Required(profile) {
				name := firstNonEmpty(item.SQLKitName, item.Name)
				if source, err := resolveAppSecretSource(app, name); err != nil {
					return err
				} else if strings.TrimSpace(source.Value) == "" {
					warnf(cmd.OutOrStdout(), "Missing: %s (%s)", item.Name, name)
					missing++
				} else {
					successf(cmd.OutOrStdout(), "OK: %s <- %s", item.Name, source.Label)
				}
			}
			if missing > 0 {
				return fmt.Errorf("%d required secret(s) missing", missing)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile from secretspec.toml")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newSecretsTemplateCommand(app *appContext) *cobra.Command {
	var profile string
	var output string
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Generate a local dotenv secrets template",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, _, err := loadAppSecretSpec(app)
			if err != nil {
				return err
			}
			items := spec.All()
			if strings.TrimSpace(profile) != "" {
				items = spec.Required(profile)
			}
			var builder strings.Builder
			for _, item := range items {
				envName := firstNonEmpty(item.Env, item.Name)
				if strings.TrimSpace(item.Description) != "" {
					builder.WriteString("# ")
					builder.WriteString(item.Description)
					builder.WriteString("\n")
				}
				builder.WriteString(envName)
				builder.WriteString("=\n\n")
			}
			if strings.TrimSpace(output) == "" {
				fmt.Fprint(cmd.OutOrStdout(), builder.String())
				return nil
			}
			outputPath := resolveRepoPath(app, output)
			if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(outputPath, []byte(builder.String()), 0600); err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Template: %s", outputPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile from secretspec.toml")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output file")
	return cmd
}

func resolveAppSecretValue(app *appContext, secretName string) (string, error) {
	value, source, err := resolveAppSecretValueOptional(app, secretName)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("secret %q is not available; provide it via env, --secrets-file or legacy keyring", secretName)
	}
	_ = source
	return value, nil
}

func resolveAppSecretValueOptional(app *appContext, secretName string) (string, string, error) {
	source, err := resolveAppSecretSource(app, secretName)
	if err != nil {
		return "", "", err
	}
	return source.Value, source.Label, nil
}

func resolveAppSecretSource(app *appContext, secretName string) (appSecretSource, error) {
	name := strings.TrimSpace(secretName)
	if name == "" {
		return appSecretSource{}, fmt.Errorf("secret name is required")
	}
	candidates := secretEnvCandidates(app, name)
	for _, envName := range candidates {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			return appSecretSource{Value: value, Label: "env:" + envName}, nil
		}
	}
	fileValues, filePath, err := loadAppSecretsFile(app)
	if err != nil {
		return appSecretSource{}, err
	}
	for _, envName := range candidates {
		if value := strings.TrimSpace(fileValues[envName]); value != "" {
			return appSecretSource{Value: value, Label: "file:" + filePath + ":" + envName}, nil
		}
	}
	if key := strings.TrimSpace(app.cfg.Secrets[name]); key != "" {
		value, err := config.Secret(key)
		if err != nil {
			return appSecretSource{}, fmt.Errorf("load legacy keyring secret %q: %w", name, err)
		}
		return appSecretSource{Value: value, Label: "keyring:" + key}, nil
	}
	return appSecretSource{}, nil
}

func secretEnvCandidates(app *appContext, secretName string) []string {
	seen := make(map[string]bool)
	var candidates []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		candidates = append(candidates, value)
	}
	if spec, _, err := loadAppSecretSpec(app); err == nil {
		for _, envName := range spec.EnvNames(secretName) {
			add(envName)
		}
	}
	add(sqlPasswordSecretEnv(secretName))
	add("SQLKIT_SECRET_" + envToken(secretName))
	add(envToken(secretName))
	sort.Strings(candidates)
	return candidates
}

func sqlPasswordSecretName(envName string) string {
	normalized, err := config.NormalizeEnvName(envName)
	if err != nil {
		normalized = strings.ToLower(strings.TrimSpace(envName))
	}
	return "env/" + normalized + "/password"
}

func sqlPasswordSecretEnv(secretName string) string {
	switch strings.ToLower(strings.TrimSpace(secretName)) {
	case "env/local/password":
		return "SQLKIT_ENV_LOCAL_PASSWORD"
	case "env/prod/password":
		return "SQLKIT_ENV_PROD_PASSWORD"
	case "env/prod-legacy/password":
		return "SQLKIT_ENV_PROD_LEGACY_PASSWORD"
	case "env/infra/password":
		return "SQLKIT_ENV_INFRA_PASSWORD"
	default:
		return ""
	}
}

func envToken(value string) string {
	var builder strings.Builder
	for _, char := range strings.ToUpper(strings.TrimSpace(value)) {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			continue
		}
		builder.WriteRune('_')
	}
	return strings.Trim(builder.String(), "_")
}

func loadAppSecretSpec(app *appContext) (*secrets.Spec, string, error) {
	path := firstNonEmpty(app.secretsSpec, os.Getenv("SQLKIT_SECRETSPEC"), app.cfg.Paths["secretspec"], "secretspec.toml")
	path = resolveRepoPath(app, path)
	spec, err := secrets.LoadSpec(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("secretspec not found: %s", path)
		}
		return nil, "", err
	}
	return spec, path, nil
}

func loadAppSecretsFile(app *appContext) (map[string]string, string, error) {
	path := firstNonEmpty(app.secretsFile, os.Getenv("SQLKIT_SECRETS_FILE"), app.cfg.Paths["secrets_file"])
	if strings.TrimSpace(path) == "" {
		path = ".local/sqlkit/secrets.env"
	}
	path = resolveRepoPath(app, path)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, path, nil
		}
		return nil, path, err
	}
	values, err := secrets.LoadDotEnv(path)
	return values, path, err
}
