package sqlserver

import "fmt"

func ExistsSQL(database string) Statement {
	return sqlTextStatement("exists", sqlExists, StringParam("database", database))
}

func SessionsSQL(database string) Statement {
	return sqlTextStatement("sessions", sqlSessions, StringParam("database", database))
}

func KillSessionsSQL(database string) Statement {
	return sqlTextStatement("kill_sessions", sqlKillSessions, StringParam("database", database))
}

func RenameDatabaseSQL(database string, newName string) (Statement, error) {
	if err := ValidateUserDatabaseName(database); err != nil {
		return Statement{}, err
	}
	if err := ValidateUserDatabaseName(newName); err != nil {
		return Statement{}, err
	}

	return sqlTextStatement(
		"rename_database",
		sqlRenameDatabase,
		StringParam("database", database),
		StringParam("new_name", newName),
	), nil
}

func DropDatabaseSQL(database string, deleteBackupHistory bool) (Statement, error) {
	if err := ValidateUserDatabaseName(database); err != nil {
		return Statement{}, err
	}

	return sqlTextStatement(
		"drop_database",
		sqlDropDatabase,
		StringParam("database", database),
		BoolParam("delete_backup_history", deleteBackupHistory),
	), nil
}

func BackupDatabaseSQL(database string, backupFile string) (Statement, error) {
	if err := ValidateUserDatabaseName(database); err != nil {
		return Statement{}, err
	}
	if backupFile == "" {
		return Statement{}, errBackupFileRequired()
	}

	return sqlTextStatement(
		"backup_database",
		sqlBackupDatabase,
		StringParam("database", database),
		StringParam("backup_file", backupFile),
	), nil
}

func OperationalBackupSQL(database string, backupFile string, backupType string) (Statement, error) {
	if err := ValidateUserDatabaseName(database); err != nil {
		return Statement{}, err
	}
	if backupFile == "" {
		return Statement{}, errBackupFileRequired()
	}
	switch backupType {
	case "full", "diff", "log":
	default:
		return Statement{}, fmt.Errorf("backup type must be full, diff or log")
	}

	return sqlTextStatement(
		"operational_backup",
		sqlOperationalBackup,
		StringParam("database", database),
		StringParam("backup_file", backupFile),
		StringParam("backup_type", backupType),
	), nil
}

func BackupMetadataSQL(backupFile string) (Statement, error) {
	if backupFile == "" {
		return Statement{}, errBackupFileRequired()
	}
	return sqlTextStatement("backup_metadata", sqlBackupMetadata, StringParam("backup_file", backupFile)), nil
}

func VerifyBackupSQL(backupFile string) (Statement, error) {
	if backupFile == "" {
		return Statement{}, errBackupFileRequired()
	}
	return sqlTextStatement("verify_backup", sqlVerifyBackup, StringParam("backup_file", backupFile)), nil
}

func RestoreDrillSQL(targetDatabase string, fullFile string, diffFile string, logFilesCSV string, checkDB bool) (Statement, error) {
	if err := ValidateUserDatabaseName(targetDatabase); err != nil {
		return Statement{}, err
	}
	if fullFile == "" {
		return Statement{}, errBackupFileRequired()
	}
	return sqlTextStatement(
		"restore_drill",
		sqlRestoreDrill,
		StringParam("target_database", targetDatabase),
		StringParam("full_file", fullFile),
		StringParam("diff_file", diffFile),
		StringParam("log_files_csv", logFilesCSV),
		BoolParam("checkdb", checkDB),
	), nil
}

func RestoreDatabaseSQL(database string, backupFile string) (Statement, error) {
	if err := ValidateUserDatabaseName(database); err != nil {
		return Statement{}, err
	}
	if backupFile == "" {
		return Statement{}, errBackupFileRequired()
	}

	return sqlTextStatement(
		"restore_database",
		sqlRestoreDatabase,
		StringParam("target_database", database),
		StringParam("backup_file", backupFile),
	), nil
}

func FKReferencesSQL(schema string, table string) Statement {
	return sqlTextStatement("fk_references", sqlFkReferences, StringParam("schema", schema), StringParam("table", table))
}

func NullRowsSQL(schema string, table string) Statement {
	return sqlTextStatement("null_rows", sqlNullRows, StringParam("schema", schema), StringParam("table", table))
}

func CharScanSQL(schema string, table string) Statement {
	return sqlTextStatement("char_scan", sqlCharScan, StringParam("schema", schema), StringParam("table", table))
}

func CharCleanSQL(schema string, table string) Statement {
	return sqlTextStatement("char_clean", sqlCharClean, StringParam("schema", schema), StringParam("table", table))
}

func DatabaseSizeSQL(database string) Statement {
	return sqlTextStatement("database_size", sqlDatabaseSize, StringParam("database", database))
}

func RecoveryModelSQL(database string) Statement {
	return sqlTextStatement("recovery_model", sqlRecoveryModel, StringParam("database", database))
}

func LocksDiagnoseSQL() Statement {
	return sqlTextStatement("locks_diagnose", sqlLocksDiagnose)
}
