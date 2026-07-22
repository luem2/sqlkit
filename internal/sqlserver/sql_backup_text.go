package sqlserver

const sqlBackupDatabase = `
USE [master];

SET NOCOUNT ON;

DECLARE @sql nvarchar(max);

IF DB_ID(@database) IS NULL
BEGIN
    THROW 50000, 'Source database does not exist.', 1;
END;

SET @sql = N'BACKUP DATABASE ' + QUOTENAME(@database)
    + N' TO DISK = N''' + REPLACE(@backup_file, '''', '''''') + N''''
    + N' WITH INIT, COPY_ONLY, CHECKSUM, STATS = 10;';

PRINT @sql;
EXEC sys.sp_executesql @sql;

SELECT @backup_file AS backup_file;
`

const sqlOperationalBackup = `
USE [master];

SET NOCOUNT ON;

DECLARE @sql nvarchar(max);
DECLARE @recovery_model nvarchar(60);

IF DB_ID(@database) IS NULL
BEGIN
    THROW 50000, 'Source database does not exist.', 1;
END;

SELECT @recovery_model = recovery_model_desc
FROM sys.databases
WHERE name = @database;

IF @backup_type = N'log' AND @recovery_model NOT IN (N'FULL', N'BULK_LOGGED')
BEGIN
    THROW 50000, 'Transaction log backups require FULL or BULK_LOGGED recovery model.', 1;
END;

IF @backup_type = N'full'
BEGIN
    SET @sql = N'BACKUP DATABASE ' + QUOTENAME(@database)
        + N' TO DISK = N''' + REPLACE(@backup_file, '''', '''''') + N''''
        + N' WITH INIT, COMPRESSION, CHECKSUM, STATS = 10;';
END
ELSE IF @backup_type = N'diff'
BEGIN
    SET @sql = N'BACKUP DATABASE ' + QUOTENAME(@database)
        + N' TO DISK = N''' + REPLACE(@backup_file, '''', '''''') + N''''
        + N' WITH DIFFERENTIAL, INIT, COMPRESSION, CHECKSUM, STATS = 10;';
END
ELSE IF @backup_type = N'log'
BEGIN
    SET @sql = N'BACKUP LOG ' + QUOTENAME(@database)
        + N' TO DISK = N''' + REPLACE(@backup_file, '''', '''''') + N''''
        + N' WITH INIT, COMPRESSION, CHECKSUM, STATS = 10;';
END
ELSE
BEGIN
    THROW 50000, 'Backup type must be full, diff or log.', 1;
END;

PRINT @sql;
EXEC sys.sp_executesql @sql;

SELECT @backup_file AS backup_file;
`

const sqlBackupMetadata = `
USE [master];

SET NOCOUNT ON;

SELECT TOP (1)
    CONVERT(nvarchar(60), bs.first_lsn) AS first_lsn,
    CONVERT(nvarchar(60), bs.last_lsn) AS last_lsn,
    CONVERT(nvarchar(60), bs.checkpoint_lsn) AS checkpoint_lsn,
    CONVERT(nvarchar(60), bs.database_backup_lsn) AS database_backup_lsn,
    CONVERT(nvarchar(30), bs.backup_start_date, 126) AS backup_start_date,
    CONVERT(nvarchar(30), bs.backup_finish_date, 126) AS backup_finish_date
FROM msdb.dbo.backupset bs
INNER JOIN msdb.dbo.backupmediafamily bmf
    ON bs.media_set_id = bmf.media_set_id
WHERE bmf.physical_device_name = @backup_file
ORDER BY bs.backup_finish_date DESC;
`

const sqlVerifyBackup = `
USE [master];

SET NOCOUNT ON;

DECLARE @sql nvarchar(max) = N'RESTORE VERIFYONLY FROM DISK = N''' + REPLACE(@backup_file, '''', '''''') + N''' WITH CHECKSUM;';

PRINT @sql;
EXEC sys.sp_executesql @sql;
`

const sqlRestoreDatabase = `
USE [master];

SET NOCOUNT ON;

DECLARE @data_dir nvarchar(4000) = CONVERT(nvarchar(4000), SERVERPROPERTY('InstanceDefaultDataPath'));
DECLARE @log_dir nvarchar(4000) = CONVERT(nvarchar(4000), SERVERPROPERTY('InstanceDefaultLogPath'));
DECLARE @file_prefix nvarchar(260) = @target_database;
DECLARE @moves nvarchar(max);
DECLARE @sql nvarchar(max);

IF DB_ID(@target_database) IS NOT NULL
BEGIN
    THROW 50000, 'Target database already exists. Restore to a new database name.', 1;
END;

IF @data_dir IS NULL
BEGIN
    SELECT TOP (1)
        @data_dir = LEFT(physical_name, LEN(physical_name) - CHARINDEX(CASE WHEN CHARINDEX('\', REVERSE(physical_name)) > 0 THEN '\' ELSE '/' END, REVERSE(physical_name)) + 1)
    FROM sys.master_files
    WHERE database_id = DB_ID(N'master') AND type = 0;
END;

IF @log_dir IS NULL
BEGIN
    SELECT TOP (1)
        @log_dir = LEFT(physical_name, LEN(physical_name) - CHARINDEX(CASE WHEN CHARINDEX('\', REVERSE(physical_name)) > 0 THEN '\' ELSE '/' END, REVERSE(physical_name)) + 1)
    FROM sys.master_files
    WHERE database_id = DB_ID(N'master') AND type = 1;
END;

IF RIGHT(@data_dir, 1) NOT IN (N'/', N'\')
    SET @data_dir += CASE WHEN CHARINDEX(N'\', @data_dir) > 0 THEN N'\' ELSE N'/' END;

IF RIGHT(@log_dir, 1) NOT IN (N'/', N'\')
    SET @log_dir += CASE WHEN CHARINDEX(N'\', @log_dir) > 0 THEN N'\' ELSE N'/' END;

SET @file_prefix = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(@file_prefix, N'\', N'_'), N'/', N'_'), N':', N'_'), N'*', N'_'), N'?', N'_'), N'"', N'_'), N'<', N'_'), N'>', N'_'), N'|', N'_');

CREATE TABLE #file_list (
    LogicalName nvarchar(128),
    PhysicalName nvarchar(260),
    Type char(1),
    FileGroupName nvarchar(128) NULL,
    Size numeric(20, 0),
    MaxSize numeric(20, 0),
    FileId bigint,
    CreateLSN numeric(25, 0) NULL,
    DropLSN numeric(25, 0) NULL,
    UniqueId uniqueidentifier,
    ReadOnlyLSN numeric(25, 0) NULL,
    ReadWriteLSN numeric(25, 0) NULL,
    BackupSizeInBytes bigint,
    SourceBlockSize int,
    FileGroupId int,
    LogGroupGUID uniqueidentifier NULL,
    DifferentialBaseLSN numeric(25, 0) NULL,
    DifferentialBaseGUID uniqueidentifier NULL,
    IsReadOnly bit,
    IsPresent bit,
    TDEThumbprint varbinary(32) NULL,
    SnapshotUrl nvarchar(360) NULL
);

SET @sql = N'RESTORE FILELISTONLY FROM DISK = N''' + REPLACE(@backup_file, '''', '''''') + N'''';
INSERT INTO #file_list
EXEC sys.sp_executesql @sql;

;WITH files AS (
    SELECT
        LogicalName,
        Type,
        FileId,
        ROW_NUMBER() OVER (PARTITION BY Type ORDER BY FileId) AS type_ordinal
    FROM #file_list
    WHERE IsPresent = 1
)
SELECT @moves = STUFF((
    SELECT
        N', MOVE N''' + REPLACE(LogicalName, '''', '''''') + N''' TO N'''
        + REPLACE(
            CASE
                WHEN Type = 'L' THEN @log_dir + @file_prefix + CASE WHEN type_ordinal = 1 THEN N'_log.ldf' ELSE N'_log_' + CONVERT(nvarchar(10), type_ordinal) + N'.ldf' END
                ELSE @data_dir + @file_prefix + CASE WHEN type_ordinal = 1 THEN N'.mdf' ELSE N'_' + CONVERT(nvarchar(10), type_ordinal) + N'.ndf' END
            END,
            '''',
            ''''''
        )
        + N''''
    FROM files
    ORDER BY FileId
    FOR XML PATH(''), TYPE
).value('.', 'nvarchar(max)'), 1, 2, N'');

SET @sql = N'RESTORE DATABASE ' + QUOTENAME(@target_database)
    + N' FROM DISK = N''' + REPLACE(@backup_file, '''', '''''') + N''''
    + N' WITH ' + @moves + N', STATS = 10;';

PRINT @sql;
EXEC sys.sp_executesql @sql;
`

const sqlRestoreDrill = `
USE [master];

SET NOCOUNT ON;

DECLARE @data_dir nvarchar(4000) = CONVERT(nvarchar(4000), SERVERPROPERTY('InstanceDefaultDataPath'));
DECLARE @log_dir nvarchar(4000) = CONVERT(nvarchar(4000), SERVERPROPERTY('InstanceDefaultLogPath'));
DECLARE @file_prefix nvarchar(260) = @target_database;
DECLARE @moves nvarchar(max);
DECLARE @sql nvarchar(max);
DECLARE @log_file nvarchar(4000);
DECLARE @position int = 1;
DECLARE @next int;

BEGIN TRY

IF DB_ID(@target_database) IS NOT NULL
BEGIN
    SET @sql = N'ALTER DATABASE ' + QUOTENAME(@target_database) + N' SET SINGLE_USER WITH ROLLBACK IMMEDIATE;'
        + N'DROP DATABASE ' + QUOTENAME(@target_database) + N';';
    EXEC sys.sp_executesql @sql;
END;

IF @data_dir IS NULL
BEGIN
    SELECT TOP (1)
        @data_dir = LEFT(physical_name, LEN(physical_name) - CHARINDEX(CASE WHEN CHARINDEX('\', REVERSE(physical_name)) > 0 THEN '\' ELSE '/' END, REVERSE(physical_name)) + 1)
    FROM sys.master_files
    WHERE database_id = DB_ID(N'master') AND type = 0;
END;

IF @log_dir IS NULL
BEGIN
    SELECT TOP (1)
        @log_dir = LEFT(physical_name, LEN(physical_name) - CHARINDEX(CASE WHEN CHARINDEX('\', REVERSE(physical_name)) > 0 THEN '\' ELSE '/' END, REVERSE(physical_name)) + 1)
    FROM sys.master_files
    WHERE database_id = DB_ID(N'master') AND type = 1;
END;

IF RIGHT(@data_dir, 1) NOT IN (N'/', N'\')
    SET @data_dir += CASE WHEN CHARINDEX(N'\', @data_dir) > 0 THEN N'\' ELSE N'/' END;

IF RIGHT(@log_dir, 1) NOT IN (N'/', N'\')
    SET @log_dir += CASE WHEN CHARINDEX(N'\', @log_dir) > 0 THEN N'\' ELSE N'/' END;

SET @file_prefix = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(@file_prefix, N'\', N'_'), N'/', N'_'), N':', N'_'), N'*', N'_'), N'?', N'_'), N'"', N'_'), N'<', N'_'), N'>', N'_'), N'|', N'_');

CREATE TABLE #file_list (
    LogicalName nvarchar(128),
    PhysicalName nvarchar(260),
    Type char(1),
    FileGroupName nvarchar(128) NULL,
    Size numeric(20, 0),
    MaxSize numeric(20, 0),
    FileId bigint,
    CreateLSN numeric(25, 0) NULL,
    DropLSN numeric(25, 0) NULL,
    UniqueId uniqueidentifier,
    ReadOnlyLSN numeric(25, 0) NULL,
    ReadWriteLSN numeric(25, 0) NULL,
    BackupSizeInBytes bigint,
    SourceBlockSize int,
    FileGroupId int,
    LogGroupGUID uniqueidentifier NULL,
    DifferentialBaseLSN numeric(25, 0) NULL,
    DifferentialBaseGUID uniqueidentifier NULL,
    IsReadOnly bit,
    IsPresent bit,
    TDEThumbprint varbinary(32) NULL,
    SnapshotUrl nvarchar(360) NULL
);

SET @sql = N'RESTORE FILELISTONLY FROM DISK = N''' + REPLACE(@full_file, '''', '''''') + N'''';
INSERT INTO #file_list
EXEC sys.sp_executesql @sql;

;WITH files AS (
    SELECT
        LogicalName,
        Type,
        FileId,
        ROW_NUMBER() OVER (PARTITION BY Type ORDER BY FileId) AS type_ordinal
    FROM #file_list
    WHERE IsPresent = 1
)
SELECT @moves = STUFF((
    SELECT
        N', MOVE N''' + REPLACE(LogicalName, '''', '''''') + N''' TO N'''
        + REPLACE(
            CASE
                WHEN Type = 'L' THEN @log_dir + @file_prefix + CASE WHEN type_ordinal = 1 THEN N'_log.ldf' ELSE N'_log_' + CONVERT(nvarchar(10), type_ordinal) + N'.ldf' END
                ELSE @data_dir + @file_prefix + CASE WHEN type_ordinal = 1 THEN N'.mdf' ELSE N'_' + CONVERT(nvarchar(10), type_ordinal) + N'.ndf' END
            END,
            '''',
            ''''''
        )
        + N''''
    FROM files
    ORDER BY FileId
    FOR XML PATH(''), TYPE
).value('.', 'nvarchar(max)'), 1, 2, N'');

SET @sql = N'RESTORE DATABASE ' + QUOTENAME(@target_database)
    + N' FROM DISK = N''' + REPLACE(@full_file, '''', '''''') + N''''
    + N' WITH ' + @moves + N', NORECOVERY, REPLACE, STATS = 10;';
PRINT @sql;
EXEC sys.sp_executesql @sql;

IF NULLIF(LTRIM(RTRIM(@diff_file)), N'') IS NOT NULL
BEGIN
    SET @sql = N'RESTORE DATABASE ' + QUOTENAME(@target_database)
        + N' FROM DISK = N''' + REPLACE(@diff_file, '''', '''''') + N''''
        + N' WITH NORECOVERY, STATS = 10;';
    PRINT @sql;
    EXEC sys.sp_executesql @sql;
END;

WHILE @position <= LEN(ISNULL(@log_files_csv, N''))
BEGIN
    SET @next = CHARINDEX(NCHAR(10), @log_files_csv, @position);
    IF @next = 0 SET @next = LEN(@log_files_csv) + 1;
    SET @log_file = LTRIM(RTRIM(SUBSTRING(@log_files_csv, @position, @next - @position)));

    IF @log_file <> N''
    BEGIN
        SET @sql = N'RESTORE LOG ' + QUOTENAME(@target_database)
            + N' FROM DISK = N''' + REPLACE(@log_file, '''', '''''') + N''''
            + N' WITH NORECOVERY, STATS = 10;';
        PRINT @sql;
        EXEC sys.sp_executesql @sql;
    END;

    SET @position = @next + 1;
END;

SET @sql = N'RESTORE DATABASE ' + QUOTENAME(@target_database) + N' WITH RECOVERY;';
PRINT @sql;
EXEC sys.sp_executesql @sql;

IF @checkdb = 1
BEGIN
    SET @sql = N'DBCC CHECKDB (' + QUOTENAME(@target_database) + N') WITH NO_INFOMSGS;';
    PRINT @sql;
    EXEC sys.sp_executesql @sql;
END;

SET @sql = N'ALTER DATABASE ' + QUOTENAME(@target_database) + N' SET SINGLE_USER WITH ROLLBACK IMMEDIATE;'
    + N'DROP DATABASE ' + QUOTENAME(@target_database) + N';';
PRINT @sql;
EXEC sys.sp_executesql @sql;

END TRY
BEGIN CATCH
    IF DB_ID(@target_database) IS NOT NULL
    BEGIN
        BEGIN TRY
            SET @sql = N'ALTER DATABASE ' + QUOTENAME(@target_database) + N' SET SINGLE_USER WITH ROLLBACK IMMEDIATE;'
                + N'DROP DATABASE ' + QUOTENAME(@target_database) + N';';
            EXEC sys.sp_executesql @sql;
        END TRY
        BEGIN CATCH
        END CATCH;
    END;
    THROW;
END CATCH;
`
