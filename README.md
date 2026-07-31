# sqlkit

CLI en Go para automatizar tareas repetibles de proyectos SQL Server.

`sqlkit` no reemplaza `sqlpackage`, `sqlcmd`, `dotnet`, Pandoc, Mermaid ni Chrome. Los detecta, les pasa parametros consistentes y agrega guardas para evitar errores operativos.

## Alcance

- Build y publish de proyectos SSDT.
- Scripts SQL ordenados y logueados.
- Backups, restores y tareas administrativas frecuentes.
- Docs desde Markdown con Mermaid.
- Chequeo de dependencias externas.

La logica de negocio no vive en `sqlkit`. Las migraciones y scripts especificos van en el repo SQL.

## Arquitectura

Arquitectura CLI modular, orientada a comandos:

- `cmd/sqlkit`: entrypoint, contexto cancelable por `Ctrl+C`/`SIGTERM`.
- `internal/cli`: comandos Cobra y orquestacion.
- `internal/cli/services.go`: servicios chicos compartidos:
  - `dbService`: conexion, `Exec`, `Query`, timeouts SQL.
  - `processService`: ejecucion de procesos externos, timeout y redaccion.
- `internal/sqlserver`: driver `database/sql`, SQL interno parametrizado y runner de `sqlcmd`.
- `internal/ssdt`: argumentos para `sqlpackage`.
- `internal/config`: config de usuario, entornos y keyring.
- `internal/docs`, `lint`, `sqlscripts`, `deps`: funcionalidades aisladas.

La CLI es la capa de orquestacion. Las reglas reutilizables viven fuera de Cobra.

## Build

```bash
go build ./cmd/sqlkit
go test ./...
go vet ./...
```

Instalar desde el repo local:

```bash
go install ./cmd/sqlkit
```

## Configuracion

Crear config de usuario:

```bash
sqlkit config init
sqlkit config set-repo /home/user/workspace/sql
sqlkit config env set local
```

Linux: `~/.config/sqlkit/config.toml`.

Windows: carpeta de config del usuario.

Ejemplo:

```toml
[defaults]
repo = "/home/user/workspace/sql"

[paths]
project = "BD_SISTEMA/BD_SISTEMA.sqlproj"
artifacts = "artifacts"
logs = "logs"
backups = "data/backups"
bacpacs = "data/bacpacs"
sqlserver_backup_dir = "backups"
sqlserver_container = "mssql-db"
sqlserver_data = "/var/opt/mssql/data"

[tools.sqlpackage]
path = "/opt/sqlpackage/sqlpackage"

[tools.sqlcmd]
path = "/opt/mssql-tools/bin/sqlcmd"

[env.local]
server = "localhost"
user = "sa"
password_key = "env/local/password"
encrypt = "disable"
trust_server_certificate = true

[env.infra]
server = "sql.example.com"
user = "infra_user"
password_key = "env/infra/password"
encrypt = "true"
trust_server_certificate = true
```

Los passwords se guardan en el keyring del sistema, no en TOML ni en el repo.

## Conexion SQL

Precedencia:

1. Flags globales: `--server`, `--user`, `--password`.
2. Archivo de password: `--password-file` o `SQLPASSWORD_FILE`.
3. Secreto genérico del keyring: `--password-secret`.
4. Variables de ambiente: `SQLSERVER`, `SQLUSER`, `SQLPASSWORD`.
5. Config de usuario: `[env.<name>]`.
6. Keyring del password configurado para el entorno.

Ejemplo:

```bash
sqlkit --server localhost --user sa --password "$SQLPASSWORD" \
  db exists --env local --database P_BD_SISTEMA
```

Timeouts globales:

```bash
sqlkit --connect-timeout 15s --sql-timeout 2h --process-timeout 2h \
  db exists --env local --database P_BD_SISTEMA
```

Usar `0` desactiva un timeout puntual.

### TLS/encryption

Para Docker local:

```bash
sqlkit config env set local \
  --encrypt disable \
  --trust-server-certificate true
```

Para SQL Server en una VM AWS:

```bash
sqlkit config env set prod \
  --server "<host-or-ip>" \
  --user "<user>" \
  --encrypt true \
  --trust-server-certificate true
```

`encrypt=true` cifra la conexion. `trust_server_certificate=true` evita validar la cadena del certificado; sirve cuando la VM usa certificado self-signed o no tenes una CA configurada. Si mas adelante instalas un certificado valido en SQL Server, cambia a `trust_server_certificate=false`.

## Seguridad

- Los comandos internos usan `database/sql` con parametros.
- Los identificadores dinamicos en SQL Server se construyen con `QUOTENAME` o listas permitidas.
- Bases de sistema (`master`, `model`, `msdb`, `tempdb`) se bloquean para operaciones destructivas.
- `prod` y `prod-legacy` requieren `--allow-prod` en comandos riesgosos.
- `--non-interactive` falla si el comando necesita prompt y faltan flags explicitos.
- La salida y los logs capturados redactan passwords.
- `sqlcmd` recibe la password por `SQLCMDPASSWORD`, no por `-P`.
- `sqlpackage` sigue usando SQL Auth por parametros. Es una decision aceptada para este proyecto; la salida y logs se redactan.

## Referencia CLI

Usar `sqlkit <comando> --help` como referencia exacta de flags. Esta sección
documenta los flujos soportados y los flags más usados.

### Diagnóstico y dependencias

```bash
sqlkit doctor
sqlkit deps check
sqlkit deps install sqlpackage
sqlkit deps install mmdc
sqlkit deps install pandoc
```

- `doctor`: chequeo rápido de herramientas externas.
- `deps check`: valida herramientas requeridas.
- `deps install <tool>`: instala o imprime pasos de instalación.
- `deps install <tool> --yes`: ejecuta instalación sin confirmación cuando el
  instalador lo soporta.

### Configuración, secretos y entornos

```bash
sqlkit config path
sqlkit config init
sqlkit config init --force
sqlkit config set-repo /ruta/al/repo/sql

sqlkit config secret set sqlkit-backup-password
sqlkit config secret set sqlkit-backup-password --password "<valor>"
sqlkit config secret get sqlkit-backup-password

sqlkit config env set local \
  --server localhost \
  --user sa \
  --password "<password>" \
  --encrypt disable \
  --trust-server-certificate true

sqlkit config env set infra \
  --server "<SQLSERVER_INFRA>" \
  --user infra_user \
  --password "<password>" \
  --encrypt true \
  --trust-server-certificate true

sqlkit env list
sqlkit env check local
sqlkit env check infra
```

- `config secret set`: guarda secretos genéricos en keyring.
- `config secret get`: imprime un secreto genérico desde keyring; usar sólo en
  automatizaciones controladas porque escribe el valor en stdout.
- `config env set`: guarda servidor/usuario/TLS y password del entorno.
- `env list`: muestra entornos conocidos (`local`, `prod`, `prod-legacy`,
  `infra`).
- `env check <env>`: valida conexión.

### Administración de bases

```bash
sqlkit db exists --env local --database P_BD_SISTEMA
sqlkit db sessions --env local --database P_BD_SISTEMA
sqlkit db kill-sessions --env local --database P_BD_SISTEMA
sqlkit db drop --env local --database P_BD_SISTEMA --yes
sqlkit db rename --env local --database OLD_DB --new-name NEW_DB --yes
sqlkit db size --env local
sqlkit db recovery --env local
```

Flags comunes:

- `--env`: entorno SQL.
- `--database`: base objetivo.
- `--yes`: confirma acciones destructivas.
- `--allow-prod`: requerido para acciones riesgosas contra `prod` o
  `prod-legacy`.
- `--delete-backup-history`: en `db drop`, borra historial de backups en `msdb`.

### Backups manuales COPY_ONLY y restores simples

```bash
sqlkit db load --env local --database P_BD_SISTEMA_RESTORE --source ./P_BD_SISTEMA.bak
sqlkit db load --env local --database P_BD_SISTEMA_RESTORE --source ./P_BD_SISTEMA.bacpac
sqlkit db load --env local --database P_BD_SISTEMA --source ./BD_SISTEMA.dacpac

sqlkit db backup --env local --database P_BD_SISTEMA
sqlkit db backup --env local --database P_BD_SISTEMA --output ./data/backups
sqlkit db backup --env local --database P_BD_SISTEMA --server-output /var/opt/mssql/backup/P_BD_SISTEMA.bak
```

`db load` es el flujo recomendado para cargar una base desde un archivo. Detecta
el tipo por extension:

- `.bak`: ejecuta restore. En `env local`, si el archivo existe en el host y hay
  contenedor SQL Server configurado, lo copia al contenedor, restaura y lo borra
  al finalizar.
- `.bacpac`: importa con SqlPackage.
- `.dacpac`: publica con SqlPackage.

Para cargas nuevas, usar `db load`: centraliza restore de `.bak`, import de
`.bacpac` y publish de `.dacpac` en un solo comando.

`sqlkit db backup` crea backups manuales `COPY_ONLY`; no modifica la cadena
operativa `FULL` + `DIFF` + `LOG`.

Flags útiles:

- `--source`: archivo de entrada para `db load`.
- `--source-type`: fuerza `bak`, `bacpac` o `dacpac` cuando no se puede inferir
  por extension.
- `--keep-staged`: conserva el `.bak` copiado al contenedor durante `db load`.
- `--output`, `-o`: ruta/carpeta final en host.
- `--server-output`: ruta exacta vista por SQL Server.
- `--bak-dir`: carpeta de backups vista por SQL Server.
- `--move-to-host`: fuerza mover backup desde contenedor Docker al host.
- `--container`: contenedor SQL Server. En `db backup`, copia el `.bak` del
  container al host. En `db load --env local`, si `--source` es un `.bak` del
  host, lo copia temporalmente al container, restaura y borra el temporal.

### Backups operativos por policy

```bash
sqlkit backup run --env prod --type full --policy _infra/backups/policies/prod.toml --allow-prod
sqlkit backup run --env prod --type diff --policy _infra/backups/policies/prod.toml --allow-prod
sqlkit backup run --env prod --type log --policy _infra/backups/policies/prod.toml --allow-prod
sqlkit backup run --env prod --type log --database P_BD_SISTEMA --policy _infra/backups/policies/prod.toml --allow-prod

sqlkit backup status --env prod --policy _infra/backups/policies/prod.toml
sqlkit backup health --env prod --policy _infra/backups/policies/prod.toml --fail
sqlkit backup verify --env prod --policy _infra/backups/policies/prod.toml
sqlkit backup verify --env prod --policy _infra/backups/policies/prod.toml --database P_BD_SISTEMA --all
sqlkit backup restore-drill --env prod --policy _infra/backups/policies/prod.toml --allow-prod
sqlkit backup prune --env prod --policy _infra/backups/policies/prod.toml --allow-prod --dry-run
sqlkit backup prune --env prod --policy _infra/backups/policies/prod.toml --allow-prod --yes
```

Comandos:

- `backup run`: ejecuta `FULL`, `DIFF` o `LOG`.
- `backup status`: muestra último backup por base/tipo y si fue subido a S3.
- `backup health`: emite JSON para monitoreo; `--fail` devuelve exit code no
  cero si hay problemas.
- `backup verify`: corre `RESTORE VERIFYONLY`.
- `backup restore-drill`: restaura cadenas válidas en bases temporales si la
  policy lo habilita.
- `backup prune`: borra backups/manifests vencidos según retención y limpia
  directorios locales vacíos hasta `local_root`.

Flags principales:

- `--policy`: TOML de policy operativa.
- `--type`: `full`, `diff` o `log`.
- `--database`: limita a una base incluida en la policy.
- `--allow-prod`: requerido para `prod` / `prod-legacy` en operaciones
  productivas o destructivas.
- `--skip-s3`: evita subir o borrar objetos S3 aunque exista `s3_prefix`.
- `--dry-run`: muestra candidatos de `prune` sin borrar.
- `--yes`: confirma borrado en `prune`.
- `--all`: en `verify`, verifica todos los manifests exitosos.
- `--at`: en `restore-drill`, punto máximo RFC3339 para elegir cadena.
- `--checkdb`: fuerza `DBCC CHECKDB` en restore drill.

Las salidas automáticas se organizan así:

```text
data/backups/<env>/<database>/manual/YYYY/MM/DD/<archivo>.bak
<local_root>/<env>/<database>/<full|diff|log>/YYYY/MM/DD/<archivo>
<local_root>/<env>/<database>/manifest/YYYY/MM/DD/<archivo>.json
data/bacpacs/<env>/<database>/export/YYYY/MM/DD/<archivo>.bacpac
```

Policy mínima:

```toml
environment = "prod"
enabled = true
local_root = "E:\\BACKUPS"
sqlserver_root = "/opt/backup"
s3_prefix = "s3://bucket/sql-backups"
databases = ["P_BD_SISTEMA"]

[remote_copy]
enabled = true
host = "10.0.0.10"
user = "sqlbackup"
keep_remote = false

[retention]
full_days = 30
diff_days = 15
log_days = 15
```

Overrides para reutilizar la misma policy en otra topología:

```bash
sqlkit backup run \
  --env prod \
  --type full \
  --policy _infra/backups/policies/prod.toml \
  --local-root /opt/backup \
  --sqlserver-root /opt/backup \
  --disable-remote-copy \
  --allow-prod \
  --skip-s3
```

- `--local-root`: reemplaza `local_root`.
- `--sqlserver-root`: reemplaza `sqlserver_root`.
- `--disable-remote-copy`: ignora `[remote_copy]` y usa filesystem directo.

### Inspección y limpieza de datos

```bash
sqlkit db fk-references --env local --database P_BD_SISTEMA --table dbo.Solicitud
sqlkit db nulls --env local --database P_BD_SISTEMA --table dbo.Solicitud
sqlkit db char-scan --env local --database P_BD_SISTEMA --table dbo.Solicitud
sqlkit db char-clean --env local --database P_BD_SISTEMA --table dbo.Solicitud --yes
```

- `fk-references`: lista FKs que referencian una tabla.
- `nulls`: devuelve filas con columnas nullable en `NULL`.
- `char-scan`: busca saltos de línea, tabs y espacios extremos en texto.
- `char-clean`: limpia esos caracteres; requiere `--yes`.

### Locks

```bash
sqlkit locks diagnose --env local
```

Muestra sesiones bloqueantes y objetos bloqueados.

### SSDT / SQL Server Database Project

```bash
sqlkit db build
sqlkit db build --project BD_SISTEMA/BD_SISTEMA.sqlproj --configuration Release

sqlkit db script --env local --database P_BD_SISTEMA
sqlkit db script --env local --database P_BD_SISTEMA --dacpac artifacts/BD_SISTEMA.dacpac -o artifacts/publish.sql
```

Flags:

- `--project`: `.sqlproj`; por defecto `paths.project`.
- `--configuration`, `-c`: configuración de build.
- `--dacpac`: dacpac explícito.
- `--output`, `-o`: script de publish.

### Seed data versionable

```bash
sqlkit db data-script \
  --repo C:\Users\luche\workspace\sql \
  --manifest BD_SISTEMA/postdeploy/data-seeds.manifest.toml \
  --group clf-parametros-vigentes

sqlkit db data-script \
  --repo C:\Users\luche\workspace\sql \
  --manifest BD_SISTEMA/postdeploy/data-seeds.manifest.toml \
  --group catalogos-ba \
  --table ba.Autorizacion_Tipo \
  --output artifacts/seeds/ba-autorizacion-tipo.sql
```

Genera un script SQL idempotente desde un grupo declarado en el manifiesto del
repo SQL. La salida por defecto es la configurada en el grupo (`output`) y puede
sobrescribirse con `--output`.

Con `--table` se genera un subconjunto del grupo. En ese caso `--output` es
obligatorio para no sobrescribir accidentalmente el script completo del grupo.
Si la tabla depende de una tabla padre filtrada por el manifiesto, hay que
incluir tambien la padre o generar el grupo completo.

Flags:

- `--manifest`: manifiesto de seeds; por defecto
  `BD_SISTEMA/postdeploy/data-seeds.manifest.toml`.
- `--group`: grupo a generar.
- `--table`: tabla del grupo a generar; repetible y requiere `--output`.
- `--env`: entorno fuente; por defecto `defaults.source_env` del manifiesto.
- `--output`, `-o`: destino alternativo para el script generado.
- `--allow-prod`: requerido si `--env` es `prod` o `prod-legacy`.

El manifiesto puede declarar `column_lookups` para no copiar IDs fragiles desde
la base fuente. Ejemplo: resolver una FK a una empresa por codigo en la base
destino:

```toml
[[groups.tables.column_lookups]]
column = "id_empresa"
lookup_table = "dbo.Empresa"
lookup_column = "id"
match_column = "codigo"
match_value = "$(Company)"
```

### BACPAC

```bash
sqlkit bacpac export --env local --database P_BD_SISTEMA
sqlkit bacpac export --env local --database P_BD_SISTEMA -o ./data/bacpacs/P_BD_SISTEMA.bacpac
```

Para cargar un `.bacpac` en una base, usar `sqlkit db load --source archivo.bacpac`.

Flags:

- `--output`, `-o`: destino del export.
- `--allow-prod`: requerido contra producción.

### Publish específico del repo SQL

```bash
sqlkit publish bd-sistema --env local --company A
sqlkit publish bd-sistema --env local --company P --database P_BD_SISTEMA_PUBLISH_TEST --skip-security
sqlkit publish grupo-central --env local
sqlkit publish facturacion --env local --company P
```

En `bd-sistema`, `--company` resuelve la base por defecto y tambien pasa
`Company=<codigo>` como variable SQLCMD al postdeploy. Si usas `--database`
para publicar en una base alternativa, el seed de empresa sigue saliendo de
`--company`.

`bd-sistema` y `grupo-central` aplican seguridad SQL despues del publish:
primero `_infra/logins/apply.sql` en `master` y luego el script de seguridad de
base correspondiente. Usar `--skip-security` solamente para casos especiales.
Las rutas pueden configurarse con `paths.security_logins_script`,
`paths.bd_sistema_security_script` y `paths.grupo_central_security_script`.

Flags:

- `--company`: código de empresa cuando aplica.
- `--database`: base destino alternativa para `bd-sistema`.
- `--dacpac`: dacpac explícito.
- `--profile`: publish profile explícito.
- `--skip-security`: omite los scripts SQL de seguridad posteriores al publish.
- `--allow-prod`: requerido contra producción.

### Bootstrap BD_SISTEMA

```bash
sqlkit bootstrap bd-sistema \
  --env local \
  --company P \
  --database P_BD_SISTEMA_NUEVA \
  --sensitive-source-database P_BD_SISTEMA

sqlkit bootstrap bd-sistema \
  --env local \
  --company P \
  --database P_BD_SISTEMA_NUEVA \
  --skip-sensitive
```

Este flujo es para una BD nueva funcional: sucursales, centros de costo,
usuarios, parametros vigentes, productos/tasas y datos por empresa. No se
ejecuta en el publish diario.

Por defecto `sqlkit` usa el runner dividido:

1. `BD_SISTEMA/bootstrap/bootstrap-core.sql`
2. genera y ejecuta `BD_SISTEMA/bootstrap-sensitive/generated/company/<empresa>.sql`
3. `BD_SISTEMA/bootstrap/bootstrap-after-users.sql`

Los scripts sensibles se generan desde una BD viva, no se versionan y pueden
contener `auth.Usuario`, hashes, salts, emails y relaciones de usuario.

Usar `--skip-sensitive` solo para casos especiales donde los usuarios ya fueron
provisionados previamente en la base destino. En ese modo se ejecuta
`BD_SISTEMA/bootstrap/bootstrap.sql`.

Flags:

- `--company`: empresa que selecciona seeds A/C/P.
- `--database`: base destino.
- `--script`: runner bootstrap alternativo; requiere `--skip-sensitive`.
- `--skip-sensitive`: no genera ni ejecuta bootstrap sensible.
- `--sensitive-source-env`: entorno fuente para datos sensibles; por defecto
  usa `--env`.
- `--sensitive-source-database`: base fuente alternativa para datos sensibles.
- `--seed-manifest`: manifiesto de seeds; por defecto
  `BD_SISTEMA/postdeploy/data-seeds.manifest.toml`.
- `--allow-prod`: requerido contra producción.

### Migracion BD_SISTEMA

```bash
sqlkit migrate bd-sistema list

sqlkit migrate bd-sistema run \
  --env local \
  --company P \
  --database P_BD_SISTEMA_NUEVA \
  --step proveedores

sqlkit migrate bd-sistema run \
  --env local \
  --company P \
  --database P_BD_SISTEMA_NUEVA \
  --from clientes \
  --to cuotas

sqlkit migrate bd-sistema run \
  --env local \
  --company P \
  --database P_BD_SISTEMA_NUEVA \
  --all
```

Lee `BD_SISTEMA/migration/bd-sistema.toml` y ejecuta los pasos declarados en
orden. `--company-id` permite pasar `id_empresa` legacy si todavia no esta
declarado en el manifiesto.

### Scripts SQL con sqlcmd

```bash
sqlkit sql run scripts/migration/01-preflight.sql --env local
sqlkit sql run scripts/migration/01-preflight.sql --env local --database P_BD_SISTEMA
sqlkit sql run-dir scripts/migration --env local
```

Variables SQLCMD:

```bash
sqlkit sql run script.sql --env local --var Company=A --var DryRun=1
```

Secretos desde keyring:

```bash
sqlkit config secret set sqlkit-backup-password
sqlkit sql run _infra/logins/sqlkit-backup.sql --env prod --database master --allow-prod \
  --secret-var BackupPassword=sqlkit-backup-password
```

`--secret-var SQLCMD_NAME=KEYRING_NAME` carga el valor desde keyring, escapa
comillas para literales T-SQL y lo entrega solo al entorno de `sqlcmd`.
No usar `--var` para passwords.

Flags:

- `--database`: catálogo inicial; por defecto `master`.
- `--log-dir`: carpeta de logs por script.
- `--var`: variable SQLCMD no sensible, repetible.
- `--secret-var`: variable SQLCMD desde keyring, repetible.
- `--allow-prod`: requerido contra producción.

### Documentación

```bash
sqlkit docs mermaid docs/refactor.md
sqlkit docs html docs/refactor.md
sqlkit docs pdf docs/refactor.md
sqlkit docs html docs/refactor.md --output-dir docs/generated --css docs/style.css

sqlkit docs sql BD_SISTEMA/vn/StoredProcedures/SP_INSUP_Solicitud.sql --itype 1
sqlkit docs sql SP_SEARCH_Solicitud --itype 10
sqlkit docs sql SP_SEARCH_Solicitud --itype 10 --env local --database P_BD_SISTEMA
sqlkit docs sql BD_SISTEMA/vn/StoredProcedures/SP_SEARCH_Solicitud.sql -o docs/SP_SEARCH_Solicitud.md
```

- `docs mermaid`: extrae bloques Mermaid y renderiza SVG.
- `docs html`: genera HTML desde Markdown con diagramas Mermaid.
- `docs pdf`: genera PDF desde Markdown con diagramas Mermaid.
- `docs sql`: genera Markdown para stored procedures.

Flags útiles:

- `--output`, `-o`: archivo de salida.
- `--output-dir`: carpeta de intermedios/salida.
- `--css`: CSS para HTML/PDF.
- `--itype`: documenta un bloque `@iType` puntual.
- `--metadata`: consulta SQL Server para resolver tipos de columnas.
- `--env` y `--database`: requeridos cuando se usa metadata SQL.

### Lint SQL

```bash
sqlkit lint unused-vars path/to/procedure.sql
sqlkit lint unused-vars BD_SISTEMA --recursive
sqlkit lint unused-vars BD_SISTEMA --recursive --json
```

- `--recursive`: recorre directorios.
- `--json`: salida JSON.

## Estado del roadmap

Roadmap base completo.

Pendientes opcionales:

- Ampliar instaladores automaticos para herramientas que dependen del SO (`sqlcmd`, Chrome, Pandoc).
- Mejorar CSS/saltos de pagina para PDFs si aparecen documentos reales que lo necesiten.
