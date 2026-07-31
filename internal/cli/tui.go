package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/config"
	"github.com/luem2/sqlkit/internal/dataseed"
)

const (
	tuiMenuBDSistema   = "BD_SISTEMA"
	tuiMenuDB          = "Bases"
	tuiMenuScripts     = "Scripts y datos"
	tuiMenuBackup      = "Backups y cargas"
	tuiMenuCredentials = "Credenciales"
	tuiMenuDiagnostics = "Diagnostico"
	tuiMenuExit        = "Salir"

	tuiActionPublishBDSistema   = "BD_SISTEMA: publish diario"
	tuiActionBootstrapBDSistema = "BD_SISTEMA: bootstrap BD nueva"
	tuiActionGenerateSeed       = "Seeds: generar script desde BD viva"
	tuiActionApplyDomicilios    = "Seeds: aplicar domicilios generados"
	tuiActionValidateBootstrap  = "BD_SISTEMA: validar bootstrap"
	tuiActionMigrateList        = "Legacy a nuevo: listar pasos"
	tuiActionMigrateRun         = "Legacy a nuevo: ejecutar pasos"
	tuiActionBack               = "Volver"
)

func newTUICommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tui",
		Aliases: []string{"ui"},
		Short:   "Interactive SQL workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.nonInteractive {
				return fmt.Errorf("tui cannot run with --non-interactive")
			}
			return runTUI(cmd, app)
		},
	}
	return cmd
}

func runTUI(cmd *cobra.Command, app *appContext) error {
	tuiTitle("sqlkit ui", "Repo: "+app.cfg.Root)
	for {
		menu, err := tuiSelect("Menu", []string{
			tuiMenuBDSistema,
			tuiMenuDB,
			tuiMenuScripts,
			tuiMenuBackup,
			tuiMenuCredentials,
			tuiMenuDiagnostics,
			tuiMenuExit,
		}, tuiMenuBDSistema)
		if err != nil {
			return err
		}

		switch menu {
		case tuiMenuBDSistema:
			if err := tuiBDSistemaMenu(cmd, app); err != nil {
				return err
			}
		case tuiMenuDB:
			if err := tuiDBMenu(cmd, app); err != nil {
				return err
			}
		case tuiMenuScripts:
			if err := tuiScriptsMenu(cmd, app); err != nil {
				return err
			}
		case tuiMenuBackup:
			if err := tuiBackupMenu(cmd, app); err != nil {
				return err
			}
		case tuiMenuCredentials:
			if err := tuiCredentialsMenu(cmd, app); err != nil {
				return err
			}
		case tuiMenuDiagnostics:
			if err := tuiDiagnosticsMenu(cmd, app); err != nil {
				return err
			}
		case tuiMenuExit:
			return nil
		}
	}
}

func tuiBDSistemaMenu(cmd *cobra.Command, app *appContext) error {
	tuiSection("BD_SISTEMA")
	action, err := tuiSelect("BD_SISTEMA", []string{
		tuiActionPublishBDSistema,
		tuiActionBootstrapBDSistema,
		tuiActionGenerateSeed,
		tuiActionApplyDomicilios,
		tuiActionValidateBootstrap,
		tuiActionMigrateList,
		tuiActionMigrateRun,
		tuiActionBack,
	}, tuiActionPublishBDSistema)
	if err != nil {
		return err
	}
	switch action {
	case tuiActionPublishBDSistema:
		return tuiPublishBDSistema(cmd, app)
	case tuiActionBootstrapBDSistema:
		return tuiBootstrapBDSistema(cmd, app)
	case tuiActionGenerateSeed:
		return tuiGenerateSeed(cmd, app)
	case tuiActionApplyDomicilios:
		return tuiApplyDomicilios(cmd, app)
	case tuiActionValidateBootstrap:
		return tuiValidateBootstrap(cmd, app)
	case tuiActionMigrateList:
		return tuiMigrateList(cmd, app)
	case tuiActionMigrateRun:
		return tuiMigrateRun(cmd, app)
	case tuiActionBack:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func tuiDBMenu(cmd *cobra.Command, app *appContext) error {
	tuiSection("Bases")
	action, err := tuiSelect("Bases", []string{
		"publish grupo-central",
		"publish facturacion",
		"db exists",
		"db sessions",
		"db kill-sessions",
		"db drop",
		"db rename",
		"db size",
		"db recovery",
		"db build",
		"db script",
		tuiActionBack,
	}, "db sessions")
	if err != nil {
		return err
	}
	switch action {
	case "publish grupo-central":
		return tuiPublishGrupoCentral(cmd, app)
	case "publish facturacion":
		return tuiPublishFacturacion(cmd, app)
	case "db exists":
		return tuiDBExists(cmd, app)
	case "db sessions":
		return tuiDBSimpleDatabaseCommand(cmd, app, "sessions")
	case "db kill-sessions":
		return tuiDBKillSessions(cmd, app)
	case "db drop":
		return tuiDBDrop(cmd, app)
	case "db rename":
		return tuiDBRename(cmd, app)
	case "db size":
		return tuiDBEnvCommand(cmd, app, "size")
	case "db recovery":
		return tuiDBEnvCommand(cmd, app, "recovery")
	case "db build":
		return tuiDBBuild(cmd, app)
	case "db script":
		return tuiDBScript(cmd, app)
	case tuiActionBack:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func tuiScriptsMenu(cmd *cobra.Command, app *appContext) error {
	tuiSection("Scripts y datos")
	action, err := tuiSelect("Scripts y datos", []string{
		"sql run",
		"sql run-dir",
		"locks diagnose",
		"db fk-references",
		"db nulls",
		"db char-scan",
		"db char-clean",
		tuiActionBack,
	}, "sql run")
	if err != nil {
		return err
	}
	switch action {
	case "sql run":
		return tuiSQLRun(cmd, app)
	case "sql run-dir":
		return tuiSQLRunDir(cmd, app)
	case "locks diagnose":
		return tuiLocksDiagnose(cmd, app)
	case "db fk-references":
		return tuiDBTableCommand(cmd, app, "fk-references")
	case "db nulls":
		return tuiDBTableCommand(cmd, app, "nulls")
	case "db char-scan":
		return tuiDBTableCommand(cmd, app, "char-scan")
	case "db char-clean":
		return tuiDBCharClean(cmd, app)
	case tuiActionBack:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func tuiBackupMenu(cmd *cobra.Command, app *appContext) error {
	tuiSection("Backups y cargas")
	action, err := tuiSelect("Backups y cargas", []string{
		"db backup manual",
		"db load",
		"bacpac export",
		"backup status",
		"backup health",
		"backup run",
		"backup verify",
		tuiActionBack,
	}, "db backup manual")
	if err != nil {
		return err
	}
	switch action {
	case "db backup manual":
		return tuiDBBackup(cmd, app)
	case "db load":
		return tuiDBLoad(cmd, app)
	case "bacpac export":
		return tuiBacpacExport(cmd, app)
	case "backup status":
		return tuiBackupPolicyCommand(cmd, app, "status")
	case "backup health":
		return tuiBackupPolicyCommand(cmd, app, "health")
	case "backup run":
		return tuiBackupRun(cmd, app)
	case "backup verify":
		return tuiBackupVerify(cmd, app)
	case tuiActionBack:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func tuiCredentialsMenu(cmd *cobra.Command, app *appContext) error {
	tuiSection("Credenciales")
	action, err := tuiSelect("Credenciales", []string{
		"config resumen seguro",
		"config init",
		"config set-repo",
		"config env set",
		"config secret set",
		"config secret get",
		"env check",
		tuiActionBack,
	}, "config resumen seguro")
	if err != nil {
		return err
	}
	switch action {
	case "config resumen seguro":
		return tuiConfigSummary(cmd, app)
	case "config init":
		return tuiConfigInit(cmd, app)
	case "config set-repo":
		return tuiConfigSetRepo(cmd, app)
	case "config env set":
		return tuiConfigEnvSet(cmd, app)
	case "config secret set":
		return tuiConfigSecretSet(cmd, app)
	case "config secret get":
		return tuiConfigSecretGet(cmd, app)
	case "env check":
		return tuiEnvCheck(cmd, app)
	case tuiActionBack:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func tuiDiagnosticsMenu(cmd *cobra.Command, app *appContext) error {
	tuiSection("Diagnostico")
	action, err := tuiSelect("Diagnostico", []string{
		"doctor",
		"deps check",
		"env list",
		"env check",
		"config path",
		tuiActionBack,
	}, "doctor")
	if err != nil {
		return err
	}
	switch action {
	case "doctor":
		return tuiConfirmAndRun(cmd, app, [][]string{{"doctor"}})
	case "deps check":
		return tuiConfirmAndRun(cmd, app, [][]string{{"deps", "check"}})
	case "env list":
		return tuiConfirmAndRun(cmd, app, [][]string{{"env", "list"}})
	case "env check":
		return tuiEnvCheck(cmd, app)
	case "config path":
		return tuiConfirmAndRun(cmd, app, [][]string{{"config", "path"}})
	case tuiActionBack:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func tuiConfigSummary(cmd *cobra.Command, app *appContext) error {
	path, err := config.UserConfigPath()
	if err != nil {
		return err
	}

	var lines []string
	lines = append(lines, "Config: "+path)
	lines = append(lines, "Repo actual: "+app.cfg.Root)
	if repo := strings.TrimSpace(app.cfg.Defaults["repo"]); repo != "" {
		lines = append(lines, "Repo por defecto: "+repo)
	}
	lines = append(lines, "")
	lines = append(lines, "Entornos SQL")
	envNames := append([]string{}, config.ValidEnvNames...)
	for envName := range app.cfg.Envs {
		if !containsString(envNames, envName) {
			envNames = append(envNames, envName)
		}
	}
	sort.Strings(envNames)
	for _, envName := range envNames {
		envCfg, ok := app.cfg.Envs[envName]
		if !ok {
			lines = append(lines, fmt.Sprintf("- %s: sin configurar", envName))
			continue
		}
		trustServerCertificate := "default(true)"
		if envCfg.TrustServerCertificate != nil {
			trustServerCertificate = strconv.FormatBool(*envCfg.TrustServerCertificate)
		}
		password := "sin password_key"
		if strings.TrimSpace(envCfg.PasswordKey) != "" {
			password = "keyring:" + envCfg.PasswordKey + " (" + tuiKeyringStatus(envCfg.PasswordKey) + ")"
		}
		lines = append(lines, fmt.Sprintf("- %s: server=%s user=%s encrypt=%s trust_server_certificate=%s password=%s",
			envName,
			tuiConfigValue(envCfg.Server),
			tuiConfigValue(envCfg.User),
			tuiConfigValue(firstNonEmpty(envCfg.Encrypt, "disable")),
			trustServerCertificate,
			password,
		))
	}

	lines = append(lines, "")
	lines = append(lines, "Secretos genericos")
	secretNames := sortedKeys(app.cfg.Secrets)
	if len(secretNames) == 0 {
		lines = append(lines, "- sin configurar")
	} else {
		for _, name := range secretNames {
			key := app.cfg.Secrets[name]
			lines = append(lines, fmt.Sprintf("- %s: keyring:%s (%s)", name, key, tuiKeyringStatus(key)))
		}
	}

	pterm.DefaultBox.
		WithTitle("Resumen seguro").
		WithRightPadding(2).
		WithLeftPadding(2).
		Println(strings.Join(lines, "\n"))
	return nil
}

func tuiConfigInit(cmd *cobra.Command, app *appContext) error {
	path, err := config.UserConfigPath()
	if err != nil {
		return err
	}
	force := false
	if _, err := os.Stat(path); err == nil {
		force, err = tuiConfirm("Sobrescribir config existente", false)
		if err != nil {
			return err
		}
		if !force {
			return nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	cfg := config.DefaultUserConfig()
	if strings.TrimSpace(app.cfg.Root) != "" {
		cfg.Defaults["repo"] = app.cfg.Root
	}
	path, err = config.WriteUserConfig(cfg)
	if err != nil {
		return err
	}
	app.cfg = cfg
	app.cfg.Root = firstNonEmpty(app.cfg.Defaults["repo"], app.repoRoot)
	successf(cmd.OutOrStdout(), "Config creada: %s", path)
	return nil
}

func tuiConfigSetRepo(cmd *cobra.Command, app *appContext) error {
	repo, err := tuiInput("Repo SQL por defecto", app.cfg.Root)
	if err != nil {
		return err
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	repo, err = filepath.Abs(repo)
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
	app.cfg.Defaults["repo"] = repo
	if useNow, err := tuiConfirm("Usar este repo en esta sesion", true); err != nil {
		return err
	} else if useNow {
		app.cfg.Root = repo
	}
	successf(cmd.OutOrStdout(), "Repo por defecto: %s", repo)
	infof(cmd.OutOrStdout(), "Config: %s", path)
	return nil
}

func tuiConfigEnvSet(cmd *cobra.Command, app *appContext) error {
	envName, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	envName, err = config.NormalizeEnvName(envName)
	if err != nil {
		return err
	}
	envCfg := app.cfg.Envs[envName]

	server, err := tuiInput("SQL Server", envCfg.Server)
	if err != nil {
		return err
	}
	if strings.TrimSpace(server) == "" {
		return fmt.Errorf("SQL Server is required")
	}
	user, err := tuiInput("SQL User", envCfg.User)
	if err != nil {
		return err
	}
	if strings.TrimSpace(user) == "" {
		return fmt.Errorf("SQL User is required")
	}
	encrypt, err := tuiSelect("Encrypt", []string{"disable", "false", "true", "strict"}, firstNonEmpty(envCfg.Encrypt, "disable"))
	if err != nil {
		return err
	}
	if _, err := config.NormalizeEncrypt(encrypt); err != nil {
		return err
	}
	trustDefault := true
	if envCfg.TrustServerCertificate != nil {
		trustDefault = *envCfg.TrustServerCertificate
	}
	trustServerCertificate, err := tuiConfirm("Trust server certificate", trustDefault)
	if err != nil {
		return err
	}

	passwordKey := strings.TrimSpace(envCfg.PasswordKey)
	updatePassword, err := tuiConfirm("Actualizar password en keyring", passwordKey == "")
	if err != nil {
		return err
	}
	if updatePassword {
		password, err := tuiPasswordInput("SQL Password")
		if err != nil {
			return err
		}
		if strings.TrimSpace(password) == "" {
			return fmt.Errorf("SQL Password is required")
		}
		passwordKey = config.KeyringKey(envName)
		if err := config.SetSecret(passwordKey, password); err != nil {
			return fmt.Errorf("store password in keyring: %w", err)
		}
	} else if passwordKey == "" {
		return fmt.Errorf("password_key is required when password is not updated")
	}

	app.cfg.Envs[envName] = config.EnvConfig{
		Server:                 strings.TrimSpace(server),
		User:                   strings.TrimSpace(user),
		PasswordKey:            passwordKey,
		Encrypt:                encrypt,
		TrustServerCertificate: &trustServerCertificate,
	}
	path, err := config.WriteUserConfig(app.cfg)
	if err != nil {
		return err
	}
	successf(cmd.OutOrStdout(), "Entorno configurado: %s", envName)
	infof(cmd.OutOrStdout(), "Config: %s", path)
	infof(cmd.OutOrStdout(), "Password: keyring:%s", passwordKey)
	return nil
}

func tuiConfigSecretSet(cmd *cobra.Command, app *appContext) error {
	name, err := tuiInput("Nombre secreto", "")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("secret name is required")
	}
	value, err := tuiPasswordInput("Valor secreto")
	if err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("secret value is required")
	}
	key := config.SecretKey(name)
	if err := config.SetSecret(key, value); err != nil {
		return fmt.Errorf("store secret in keyring: %w", err)
	}
	app.cfg.Secrets[name] = key
	path, err := config.WriteUserConfig(app.cfg)
	if err != nil {
		return err
	}
	successf(cmd.OutOrStdout(), "Secret configurado: %s", name)
	infof(cmd.OutOrStdout(), "Config: %s", path)
	infof(cmd.OutOrStdout(), "Secret: keyring:%s", key)
	return nil
}

func tuiConfigSecretGet(cmd *cobra.Command, app *appContext) error {
	name, err := tuiSecretName(app)
	if err != nil {
		return err
	}
	key := strings.TrimSpace(app.cfg.Secrets[name])
	if key == "" {
		return fmt.Errorf("keyring secret %q is not configured", name)
	}

	pterm.DefaultBox.
		WithTitle("Secret").
		WithRightPadding(2).
		WithLeftPadding(2).
		Println(fmt.Sprintf("Nombre: %s\nKeyring: %s\nEstado: %s", name, key, tuiKeyringStatus(key)))

	showValue, err := tuiConfirm("Mostrar valor secreto en pantalla", false)
	if err != nil {
		return err
	}
	if !showValue {
		return nil
	}
	value, err := config.Secret(key)
	if err != nil {
		return fmt.Errorf("load secret from keyring (%s): %w", key, err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), value)
	return err
}

func tuiEnvCheck(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	return tuiConfirmAndRun(cmd, app, [][]string{{"env", "check", env}})
}

func tuiPublishBDSistema(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	company, err := tuiCompany("P")
	if err != nil {
		return err
	}
	defaultDB, err := companyTargetDatabase(company, "_BD_SISTEMA")
	if err != nil {
		return err
	}
	database, err := tuiInput("Base destino", defaultDB)
	if err != nil {
		return err
	}
	buildFirst, err := tuiConfirm("Compilar DACPAC antes de publicar", true)
	if err != nil {
		return err
	}
	skipSecurity, err := tuiConfirm("Omitir seguridad SQL", false)
	if err != nil {
		return err
	}
	allowProd, err := tuiAllowProd(env)
	if err != nil {
		return err
	}

	var commands [][]string
	if buildFirst {
		commands = append(commands, []string{"db", "build", "--project", "BD_SISTEMA/BD_SISTEMA.sqlproj"})
	}
	publish := []string{"publish", "bd-sistema", "--env", env, "--company", company, "--database", database}
	if skipSecurity {
		publish = append(publish, "--skip-security")
	}
	if allowProd {
		publish = append(publish, "--allow-prod")
	}
	commands = append(commands, publish)
	return tuiConfirmAndRun(cmd, app, commands)
}

func tuiPublishGrupoCentral(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	buildFirst, err := tuiConfirm("Compilar DACPAC antes de publicar", true)
	if err != nil {
		return err
	}
	skipSecurity, err := tuiConfirm("Omitir seguridad SQL", false)
	if err != nil {
		return err
	}
	allowProd, err := tuiAllowProd(env)
	if err != nil {
		return err
	}

	var commands [][]string
	if buildFirst {
		commands = append(commands, []string{"db", "build", "--project", "GRUPO_CENTRAL/GRUPO_CENTRAL.sqlproj"})
	}
	publish := []string{"publish", "grupo-central", "--env", env}
	if skipSecurity {
		publish = append(publish, "--skip-security")
	}
	if allowProd {
		publish = append(publish, "--allow-prod")
	}
	commands = append(commands, publish)
	return tuiConfirmAndRun(cmd, app, commands)
}

func tuiPublishFacturacion(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	company, err := tuiCompany("P")
	if err != nil {
		return err
	}
	defaultDB, err := companyTargetDatabase(company, "_FACTURACION")
	if err != nil {
		return err
	}
	database, err := tuiInput("Base destino", defaultDB)
	if err != nil {
		return err
	}
	buildFirst, err := tuiConfirm("Compilar DACPAC antes de publicar", true)
	if err != nil {
		return err
	}
	allowProd, err := tuiAllowProd(env)
	if err != nil {
		return err
	}

	var commands [][]string
	if buildFirst {
		commands = append(commands, []string{"db", "build", "--project", "FACTURACION/FACTURACION.sqlproj"})
	}
	publish := []string{"publish", "facturacion", "--env", env, "--company", company, "--database", database}
	if allowProd {
		publish = append(publish, "--allow-prod")
	}
	commands = append(commands, publish)
	return tuiConfirmAndRun(cmd, app, commands)
}

func tuiBootstrapBDSistema(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	company, err := tuiCompany("P")
	if err != nil {
		return err
	}
	defaultDB, err := companyTargetDatabase(company, "_BD_SISTEMA")
	if err != nil {
		return err
	}
	database, err := tuiInput("Base destino", defaultDB)
	if err != nil {
		return err
	}
	copyUsers, err := tuiConfirm("Copiar usuarios/relaciones desde BD viva", true)
	if err != nil {
		return err
	}

	args := []string{"bootstrap", "bd-sistema", "--env", env, "--company", company, "--database", database}
	allowProd := false
	if copyUsers {
		sourceEnv, err := tuiEnv(app, env)
		if err != nil {
			return err
		}
		sourceDBDefault, err := companyTargetDatabase(company, "_BD_SISTEMA")
		if err != nil {
			return err
		}
		sourceDB, err := tuiInput("Base fuente usuarios", sourceDBDefault)
		if err != nil {
			return err
		}
		args = append(args, "--sensitive-source-env", sourceEnv, "--sensitive-source-database", sourceDB)
		if prod, err := tuiAllowProd(sourceEnv); err != nil {
			return err
		} else {
			allowProd = prod
		}
	} else {
		args = append(args, "--skip-sensitive")
	}
	if targetProd, err := tuiAllowProd(env); err != nil {
		return err
	} else {
		allowProd = allowProd || targetProd
	}
	if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiGenerateSeed(cmd *cobra.Command, app *appContext) error {
	manifestPath := "BD_SISTEMA/postdeploy/data-seeds.manifest.toml"
	manifest, err := dataseed.LoadManifest(app.cfg.Root, manifestPath)
	if err != nil {
		return err
	}
	groupNames := make([]string, 0, len(manifest.Groups))
	groupByName := make(map[string]dataseed.GroupConfig, len(manifest.Groups))
	for _, group := range manifest.Groups {
		groupNames = append(groupNames, group.Name)
		groupByName[group.Name] = group
	}
	sort.Strings(groupNames)

	groupName, err := tuiSelect("Grupo seed", groupNames, firstString(groupNames))
	if err != nil {
		return err
	}
	group := groupByName[groupName]
	env, err := tuiEnv(app, firstNonEmpty(manifest.Defaults.SourceEnv, config.EnvLocal))
	if err != nil {
		return err
	}

	args := []string{"db", "data-script", "--manifest", manifestPath, "--env", env, "--group", groupName}
	sourceDB, err := tuiOptionalInput("Base fuente override", group.SourceDatabase)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sourceDB) != "" && !strings.EqualFold(strings.TrimSpace(sourceDB), strings.TrimSpace(group.SourceDatabase)) {
		args = append(args, "--source-database", strings.TrimSpace(sourceDB))
	}
	tableMode, err := tuiConfirm("Generar una tabla puntual", false)
	if err != nil {
		return err
	}
	if tableMode {
		tableNames := make([]string, 0, len(group.Tables))
		for _, table := range group.Tables {
			tableNames = append(tableNames, table.Name)
		}
		tableName, err := tuiSelect("Tabla", tableNames, firstString(tableNames))
		if err != nil {
			return err
		}
		outputDefault := filepath.ToSlash(filepath.Join("artifacts", "seeds", tuiSafeToken(groupName)+"-"+tuiSafeToken(tableName)+".sql"))
		output, err := tuiInput("Output", outputDefault)
		if err != nil {
			return err
		}
		args = append(args, "--table", tableName, "--output", output)
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiApplyDomicilios(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	company, err := tuiCompany("P")
	if err != nil {
		return err
	}
	defaultDB, err := companyTargetDatabase(company, "_BD_SISTEMA")
	if err != nil {
		return err
	}
	database, err := tuiInput("Base destino", defaultDB)
	if err != nil {
		return err
	}
	generateFirst, err := tuiConfirm("Regenerar domicilios antes de aplicar", false)
	if err != nil {
		return err
	}
	allowProd, err := tuiAllowProd(env)
	if err != nil {
		return err
	}

	var commands [][]string
	if generateFirst {
		sourceEnv, err := tuiEnv(app, env)
		if err != nil {
			return err
		}
		generate := []string{"db", "data-script", "--env", sourceEnv, "--group", "domicilios-base"}
		sourceDB, err := tuiOptionalInput("Base fuente domicilios override", "")
		if err != nil {
			return err
		}
		if sourceDB != "" {
			generate = append(generate, "--source-database", sourceDB)
		}
		if sourceProd, err := tuiAllowProd(sourceEnv); err != nil {
			return err
		} else if sourceProd {
			generate = append(generate, "--allow-prod")
		}
		commands = append(commands, generate)
	}
	run := []string{"sql", "run", "BD_SISTEMA/bootstrap/bootstrap-domicilios.sql", "--env", env, "--database", database}
	if allowProd {
		run = append(run, "--allow-prod")
	}
	commands = append(commands, run)
	return tuiConfirmAndRun(cmd, app, commands)
}

func tuiValidateBootstrap(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	company, err := tuiCompany("P")
	if err != nil {
		return err
	}
	defaultDB, err := companyTargetDatabase(company, "_BD_SISTEMA")
	if err != nil {
		return err
	}
	database, err := tuiInput("Base destino", defaultDB)
	if err != nil {
		return err
	}
	args := []string{"sql", "run", "scripts/validate-bd-sistema-bootstrap.sql", "--env", env, "--database", database}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiMigrateList(cmd *cobra.Command, app *appContext) error {
	return tuiConfirmAndRun(cmd, app, [][]string{{"migrate", "bd-sistema", "list"}})
}

func tuiMigrateRun(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	company, err := tuiCompany("P")
	if err != nil {
		return err
	}
	defaultDB, err := companyTargetDatabase(company, "_BD_SISTEMA")
	if err != nil {
		return err
	}
	database, err := tuiInput("Base destino", defaultDB)
	if err != nil {
		return err
	}

	manifest, err := loadBDSistemaMigrationManifest(app, "")
	if err != nil {
		return err
	}
	stepNames := make([]string, 0, len(manifest.Steps))
	for _, step := range manifest.Steps {
		stepNames = append(stepNames, step.Name)
	}
	mode, err := tuiSelect("Modo", []string{"step", "range", "all"}, "step")
	if err != nil {
		return err
	}
	args := []string{"migrate", "bd-sistema", "run", "--env", env, "--company", company, "--database", database}
	switch mode {
	case "step":
		step, err := tuiSelect("Paso", stepNames, firstString(stepNames))
		if err != nil {
			return err
		}
		args = append(args, "--step", step)
	case "range":
		from, err := tuiSelect("Desde", stepNames, firstString(stepNames))
		if err != nil {
			return err
		}
		to, err := tuiSelect("Hasta", stepNames, stepNames[len(stepNames)-1])
		if err != nil {
			return err
		}
		args = append(args, "--from", from, "--to", to)
	case "all":
		args = append(args, "--all")
	}
	if companyID, ok := manifest.CompanyID(company); !ok || companyID <= 0 {
		value, err := tuiInput("CompanyId legacy", "")
		if err != nil {
			return err
		}
		if _, err := strconv.Atoi(strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("CompanyId legacy must be numeric")
		}
		args = append(args, "--company-id", strings.TrimSpace(value))
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBExists(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	args := []string{"db", "exists", "--env", env, "--database", database}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBSimpleDatabaseCommand(cmd *cobra.Command, app *appContext, action string) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	args := []string{"db", action, "--env", env, "--database", database}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBKillSessions(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	args := []string{"db", "kill-sessions", "--env", env, "--database", database}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBDrop(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	if ok, err := tuiConfirm("Confirmar DROP de "+database, false); err != nil {
		return err
	} else if !ok {
		return nil
	}
	args := []string{"db", "drop", "--env", env, "--database", database, "--yes"}
	if deleteHistory, err := tuiConfirm("Borrar historial de backups en msdb", false); err != nil {
		return err
	} else if deleteHistory {
		args = append(args, "--delete-backup-history")
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBRename(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	newName, err := tuiInput("Nuevo nombre", database+"_OLD")
	if err != nil {
		return err
	}
	args := []string{"db", "rename", "--env", env, "--database", database, "--new-name", newName, "--yes"}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBEnvCommand(cmd *cobra.Command, app *appContext, action string) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	args := []string{"db", action, "--env", env}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBBuild(cmd *cobra.Command, app *appContext) error {
	project, err := tuiProject("BD_SISTEMA")
	if err != nil {
		return err
	}
	configValue, err := tuiInput("Configuracion build", "Debug")
	if err != nil {
		return err
	}
	return tuiConfirmAndRun(cmd, app, [][]string{{"db", "build", "--project", project, "--configuration", configValue}})
}

func tuiDBScript(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	project, err := tuiProject("BD_SISTEMA")
	if err != nil {
		return err
	}
	outputDefault := filepath.ToSlash(filepath.Join("artifacts", "publish-"+tuiSafeToken(database)+".sql"))
	output, err := tuiInput("Output", outputDefault)
	if err != nil {
		return err
	}
	args := []string{"db", "script", "--env", env, "--database", database, "--project", project, "--output", output}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiSQLRun(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "master")
	if err != nil {
		return err
	}
	script, err := tuiInput("Script SQL", "")
	if err != nil {
		return err
	}
	args := []string{"sql", "run", script, "--env", env, "--database", database}
	args, err = tuiSQLVars(args)
	if err != nil {
		return err
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiSQLRunDir(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "master")
	if err != nil {
		return err
	}
	dir, err := tuiInput("Directorio SQL", "")
	if err != nil {
		return err
	}
	args := []string{"sql", "run-dir", dir, "--env", env, "--database", database}
	args, err = tuiSQLVars(args)
	if err != nil {
		return err
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiLocksDiagnose(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	args := []string{"locks", "diagnose", "--env", env}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBTableCommand(cmd *cobra.Command, app *appContext, action string) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	table, err := tuiInput("Tabla schema.tabla", "")
	if err != nil {
		return err
	}
	args := []string{"db", action, "--env", env, "--database", database, "--table", table}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBCharClean(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	table, err := tuiInput("Tabla schema.tabla", "")
	if err != nil {
		return err
	}
	if ok, err := tuiConfirm("Confirmar limpieza de caracteres en "+table, false); err != nil {
		return err
	} else if !ok {
		return nil
	}
	args := []string{"db", "char-clean", "--env", env, "--database", database, "--table", table, "--yes"}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBBackup(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	args := []string{"db", "backup", "--env", env, "--database", database}
	if output, err := tuiOptionalInput("Output host opcional", ""); err != nil {
		return err
	} else if output != "" {
		args = append(args, "--output", output)
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiDBLoad(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	database, err := tuiInput("Base destino", "")
	if err != nil {
		return err
	}
	source, err := tuiInput("Archivo fuente .bak/.bacpac/.dacpac", "")
	if err != nil {
		return err
	}
	args := []string{"db", "load", "--env", env, "--database", database, "--source", source}
	keepStaged, err := tuiConfirm("Conservar staging temporal", false)
	if err != nil {
		return err
	}
	if keepStaged {
		args = append(args, "--keep-staged")
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiBacpacExport(cmd *cobra.Command, app *appContext) error {
	env, database, err := tuiEnvDatabase(app, config.EnvLocal, "")
	if err != nil {
		return err
	}
	args := []string{"bacpac", "export", "--env", env, "--database", database}
	if output, err := tuiOptionalInput("Output opcional", ""); err != nil {
		return err
	} else if output != "" {
		args = append(args, "--output", output)
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiBackupPolicyCommand(cmd *cobra.Command, app *appContext, action string) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	policy, err := tuiInput("Policy TOML", "_infra/backups/policies/prod.toml")
	if err != nil {
		return err
	}
	args := []string{"backup", action, "--env", env, "--policy", policy}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiBackupRun(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	backupType, err := tuiSelect("Tipo backup", []string{"full", "diff", "log"}, "full")
	if err != nil {
		return err
	}
	policy, err := tuiInput("Policy TOML", "_infra/backups/policies/prod.toml")
	if err != nil {
		return err
	}
	args := []string{"backup", "run", "--env", env, "--type", backupType, "--policy", policy}
	if database, err := tuiOptionalInput("Base opcional", ""); err != nil {
		return err
	} else if database != "" {
		args = append(args, "--database", database)
	}
	if allowProd, err := tuiAllowProd(env); err != nil {
		return err
	} else if allowProd {
		args = append(args, "--allow-prod")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiBackupVerify(cmd *cobra.Command, app *appContext) error {
	env, err := tuiEnv(app, config.EnvLocal)
	if err != nil {
		return err
	}
	policy, err := tuiInput("Policy TOML", "_infra/backups/policies/prod.toml")
	if err != nil {
		return err
	}
	args := []string{"backup", "verify", "--env", env, "--policy", policy}
	if database, err := tuiOptionalInput("Base opcional", ""); err != nil {
		return err
	} else if database != "" {
		args = append(args, "--database", database)
	}
	verifyAll, err := tuiConfirm("Verificar todos los manifests", false)
	if err != nil {
		return err
	}
	if verifyAll {
		args = append(args, "--all")
	}
	return tuiConfirmAndRun(cmd, app, [][]string{args})
}

func tuiEnv(app *appContext, fallback string) (string, error) {
	options := make([]string, 0, len(config.ValidEnvNames))
	for _, env := range config.ValidEnvNames {
		options = append(options, env)
	}
	defaultOption := fallback
	if _, ok := app.cfg.Envs[defaultOption]; !ok && defaultOption != config.EnvLocal {
		defaultOption = config.EnvLocal
	}
	return tuiSelect("Entorno", options, defaultOption)
}

func tuiCompany(fallback string) (string, error) {
	return tuiSelect("Empresa", []string{"P", "A", "C"}, strings.ToUpper(strings.TrimSpace(fallback)))
}

func tuiEnvDatabase(app *appContext, envFallback string, databaseFallback string) (string, string, error) {
	env, err := tuiEnv(app, envFallback)
	if err != nil {
		return "", "", err
	}
	database, err := tuiInput("Base", databaseFallback)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(database) == "" {
		return "", "", fmt.Errorf("database is required")
	}
	return env, strings.TrimSpace(database), nil
}

func tuiProject(fallback string) (string, error) {
	options := []string{
		"BD_SISTEMA/BD_SISTEMA.sqlproj",
		"GRUPO_CENTRAL/GRUPO_CENTRAL.sqlproj",
		"INFRA/INFRA.sqlproj",
		"custom",
	}
	defaultOption := options[0]
	switch strings.ToUpper(strings.TrimSpace(fallback)) {
	case "GRUPO_CENTRAL":
		defaultOption = options[1]
	case "INFRA":
		defaultOption = options[2]
	}
	selected, err := tuiSelect("Proyecto", options, defaultOption)
	if err != nil {
		return "", err
	}
	if selected != "custom" {
		return selected, nil
	}
	return tuiInput("Proyecto", "")
}

func tuiTitle(title string, subtitle string) {
	pterm.DefaultHeader.
		WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println(title)
	if strings.TrimSpace(subtitle) != "" {
		pterm.Info.Println(subtitle)
	}
}

func tuiSection(title string) {
	pterm.Println()
	pterm.DefaultSection.Println(title)
}

func tuiSelect(label string, options []string, defaultOption string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("%s has no options", label)
	}
	if strings.TrimSpace(defaultOption) == "" {
		defaultOption = options[0]
	}
	return pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultOption(defaultOption).
		WithFilter(true).
		Show(label)
}

func tuiInput(label string, defaultValue string) (string, error) {
	return pterm.DefaultInteractiveTextInput.
		WithDefaultValue(defaultValue).
		Show(label)
}

func tuiPasswordInput(label string) (string, error) {
	return pterm.DefaultInteractiveTextInput.
		WithMask("*").
		Show(label)
}

func tuiOptionalInput(label string, defaultValue string) (string, error) {
	value, err := tuiInput(label, defaultValue)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func tuiSQLVars(args []string) ([]string, error) {
	for {
		addVar, err := tuiConfirm("Agregar SQLCMD variable", false)
		if err != nil {
			return nil, err
		}
		if !addVar {
			break
		}
		value, err := tuiInput("Variable NAME=VALUE", "")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(value) != "" {
			args = append(args, "--var", strings.TrimSpace(value))
		}
	}
	for {
		addSecretVar, err := tuiConfirm("Agregar SQLCMD secret-var", false)
		if err != nil {
			return nil, err
		}
		if !addSecretVar {
			break
		}
		value, err := tuiInput("Secret var NAME=KEYRING_SECRET", "")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(value) != "" {
			args = append(args, "--secret-var", strings.TrimSpace(value))
		}
	}
	return args, nil
}

func tuiConfirm(label string, fallback bool) (bool, error) {
	return pterm.DefaultInteractiveConfirm.
		WithDefaultValue(fallback).
		Show(label)
}

func tuiAllowProd(env string) (bool, error) {
	if !isProdLike(env) {
		return false, nil
	}
	return tuiConfirm("Confirmar ejecucion contra "+env, false)
}

func tuiConfirmAndRun(cmd *cobra.Command, app *appContext, commands [][]string) error {
	fmt.Fprintln(cmd.OutOrStdout())
	for _, args := range commands {
		infof(cmd.OutOrStdout(), "Comando: %s", shellCommandLine(args))
	}
	ok, err := tuiConfirm("Ejecutar", true)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	for _, args := range commands {
		fullArgs := append([]string{"--repo", app.cfg.Root}, args...)
		result := newProcessService(cmd, app).RunStreaming(executable, fullArgs...)
		if result.ExitCode != 0 {
			return fmt.Errorf("command failed with exit code %d: %s", result.ExitCode, shellCommandLine(args))
		}
	}
	return nil
}

func tuiSecretName(app *appContext) (string, error) {
	options := sortedKeys(app.cfg.Secrets)
	options = append(options, "custom")
	selected, err := tuiSelect("Secreto", options, firstString(options))
	if err != nil {
		return "", err
	}
	if selected != "custom" {
		return selected, nil
	}
	value, err := tuiInput("Nombre secreto", "")
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("secret name is required")
	}
	return value, nil
}

func tuiConfigValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<vacio>"
	}
	return value
}

func tuiKeyringStatus(key string) string {
	if strings.TrimSpace(key) == "" {
		return "sin key"
	}
	if _, err := config.Secret(key); err != nil {
		return "faltante"
	}
	return "ok"
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func shellCommandLine(args []string) string {
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, "sqlkit")
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\"'") {
			quoted = append(quoted, strconv.Quote(arg))
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func tuiSafeToken(value string) string {
	var builder strings.Builder
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			continue
		}
		builder.WriteRune('-')
	}
	token := strings.Trim(builder.String(), "-")
	if token == "" {
		return "seed"
	}
	return token
}
