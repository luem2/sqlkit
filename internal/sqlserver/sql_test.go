package sqlserver

import (
	"database/sql/driver"
	"strings"
	"testing"
)

func TestDropDatabaseSQL(t *testing.T) {
	sql, err := DropDatabaseSQL("P_BD_SISTEMA", false)
	if err != nil {
		t.Fatal(err)
	}

	mustContain(t, sql.Text, "SET SINGLE_USER WITH ROLLBACK IMMEDIATE")
	mustContain(t, sql.Text, "DROP DATABASE")
	mustContain(t, sql.Text, "@database")
	mustContain(t, sql.Text, "sp_delete_database_backuphistory")
	mustHaveParam(t, sql, "database", "P_BD_SISTEMA")
	mustHaveParam(t, sql, "delete_backup_history", false)
}

func TestDropDatabaseSQLWithBackupHistory(t *testing.T) {
	sql, err := DropDatabaseSQL("P_BD_SISTEMA", true)
	if err != nil {
		t.Fatal(err)
	}
	mustHaveParam(t, sql, "delete_backup_history", true)
}

func TestDropDatabaseSQLBlocksSystemDatabases(t *testing.T) {
	for _, database := range []string{"master", "model", "msdb", "tempdb"} {
		if _, err := DropDatabaseSQL(database, false); err == nil {
			t.Fatalf("expected error for %s", database)
		}
	}
}

func TestRenameDatabaseSQL(t *testing.T) {
	sql, err := RenameDatabaseSQL("OLD_DB", "NEW_DB")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, sql.Text, "MODIFY NAME")
	mustContain(t, sql.Text, "SET SINGLE_USER WITH ROLLBACK IMMEDIATE")
	mustContain(t, sql.Text, "SET MULTI_USER")
	mustHaveParam(t, sql, "database", "OLD_DB")
	mustHaveParam(t, sql, "new_name", "NEW_DB")
}

func TestBackupDatabaseSQL(t *testing.T) {
	sql, err := BackupDatabaseSQL("P_BD_SISTEMA", "backups/P_BD_SISTEMA.bak")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, sql.Text, "BACKUP DATABASE")
	mustContain(t, sql.Text, "COPY_ONLY")
	mustContain(t, sql.Text, "CHECKSUM")
	mustHaveParam(t, sql, "backup_file", "backups/P_BD_SISTEMA.bak")
}

func TestOperationalBackupSQL(t *testing.T) {
	fullSQL, err := OperationalBackupSQL("P_BD_SISTEMA", "backups/P_BD_SISTEMA.bak", "full")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, fullSQL.Text, "BACKUP DATABASE")
	mustContain(t, fullSQL.Text, "COMPRESSION")
	mustContain(t, fullSQL.Text, "CHECKSUM")
	mustNotContain(t, fullSQL.Text, "COPY_ONLY")
	mustHaveParam(t, fullSQL, "backup_type", "full")

	diffSQL, err := OperationalBackupSQL("P_BD_SISTEMA", "backups/P_BD_SISTEMA.diff", "diff")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, diffSQL.Text, "WITH DIFFERENTIAL")

	logSQL, err := OperationalBackupSQL("P_BD_SISTEMA", "backups/P_BD_SISTEMA.trn", "log")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, logSQL.Text, "BACKUP LOG")
}

func TestOperationalBackupSQLRejectsInvalidType(t *testing.T) {
	if _, err := OperationalBackupSQL("P_BD_SISTEMA", "backup.bak", "copy"); err == nil {
		t.Fatal("expected invalid backup type error")
	}
}

func TestBackupMetadataAndVerifySQL(t *testing.T) {
	metadataSQL, err := BackupMetadataSQL("backups/P_BD_SISTEMA.bak")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, metadataSQL.Text, "msdb.dbo.backupset")
	mustContain(t, metadataSQL.Text, "first_lsn")

	verifySQL, err := VerifyBackupSQL("backups/P_BD_SISTEMA.bak")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, verifySQL.Text, "RESTORE VERIFYONLY")
	mustContain(t, verifySQL.Text, "WITH CHECKSUM")
}

func TestRestoreDrillSQL(t *testing.T) {
	sql, err := RestoreDrillSQL("P_BD_SISTEMA_RESTORE_DRILL", "full.bak", "diff.diff", "one.trn\ntwo.trn", true)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, sql.Text, "RESTORE DATABASE")
	mustContain(t, sql.Text, "NORECOVERY")
	mustContain(t, sql.Text, "RESTORE LOG")
	mustContain(t, sql.Text, "WITH RECOVERY")
	mustContain(t, sql.Text, "DBCC CHECKDB")
}

func TestRestoreDatabaseSQL(t *testing.T) {
	sql, err := RestoreDatabaseSQL("P_BD_SISTEMA_RESTORE", "backups/P_BD_SISTEMA.bak")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, sql.Text, "RESTORE FILELISTONLY")
	mustContain(t, sql.Text, "RESTORE DATABASE")
	mustContain(t, sql.Text, "Target database already exists")
}

func TestBackupRestoreBlocksSystemDatabases(t *testing.T) {
	if _, err := BackupDatabaseSQL("master", "master.bak"); err == nil {
		t.Fatal("expected BackupDatabaseSQL to reject master")
	}
	if _, err := RestoreDatabaseSQL("msdb", "msdb.bak"); err == nil {
		t.Fatal("expected RestoreDatabaseSQL to reject msdb")
	}
}

func TestParseSchemaTable(t *testing.T) {
	schema, table, err := ParseSchemaTable("dbo.Solicitud")
	if err != nil {
		t.Fatal(err)
	}
	if schema != "dbo" || table != "Solicitud" {
		t.Fatalf("got %s.%s", schema, table)
	}
	if _, _, err := ParseSchemaTable("Solicitud"); err == nil {
		t.Fatal("expected schema.table validation error")
	}
}

func TestInspectionSQLUsesSnakeCaseAliases(t *testing.T) {
	fkSQL := FKReferencesSQL("dbo", "Solicitud")
	mustContain(t, fkSQL.Text, "foreign_key_name")
	mustContain(t, fkSQL.Text, "referencing_table_name")

	nullSQL := NullRowsSQL("dbo", "Solicitud")
	mustContain(t, nullSQL.Text, "null_column_name")

	charSQL := CharScanSQL("dbo", "Solicitud")
	mustContain(t, charSQL.Text, "column_name")
	mustContain(t, charSQL.Text, "char_13_count")
	mustContain(t, charSQL.Text, "leading_space_count")

	cleanSQL := CharCleanSQL("dbo", "Solicitud")
	mustContain(t, cleanSQL.Text, "UPDATE")
	mustContain(t, cleanSQL.Text, "CHAR(13)")
	mustContain(t, cleanSQL.Text, "TRIM")
}

func TestMaintenanceSQLUsesSnakeCaseAliases(t *testing.T) {
	sizeSQL := DatabaseSizeSQL("P_BD_SISTEMA")
	mustContain(t, sizeSQL.Text, "database_name")
	mustContain(t, sizeSQL.Text, "total_size_mb")

	recoverySQL := RecoveryModelSQL("")
	mustContain(t, recoverySQL.Text, "recovery_model")

	locksSQL := LocksDiagnoseSQL()
	mustContain(t, locksSQL.Text, "blocking_session_id")
	mustContain(t, locksSQL.Text, "blocked_session_count")
}

func TestStatementsKeepLiteralsAsParameters(t *testing.T) {
	sql := SessionsSQL("O'Brien")
	mustContain(t, sql.Text, "@database")
	mustHaveParam(t, sql, "database", "O'Brien")
}

func TestExistsSQLMarker(t *testing.T) {
	sql := ExistsSQL("P_BD_SISTEMA")
	mustContain(t, sql.Text, "database_exists")
	mustHaveParam(t, sql, "database", "P_BD_SISTEMA")
}

func mustContain(t *testing.T, value, fragment string) {
	t.Helper()
	if !strings.Contains(value, fragment) {
		t.Fatalf("expected %q to contain %q", value, fragment)
	}
}

func mustNotContain(t *testing.T, value, fragment string) {
	t.Helper()
	if strings.Contains(value, fragment) {
		t.Fatalf("expected %q not to contain %q", value, fragment)
	}
}

func mustHaveParam(t *testing.T, statement Statement, name string, value driver.Value) {
	t.Helper()
	for _, parameter := range statement.Parameters {
		if parameter.Name == name && parameter.Value == value {
			return
		}
	}
	t.Fatalf("expected %s to have parameter %s=%v", statement.Name, name, value)
}
