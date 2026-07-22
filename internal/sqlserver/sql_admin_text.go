package sqlserver

const sqlExists = `
SET NOCOUNT ON;

SELECT
    CASE WHEN DB_ID(@database) IS NULL THEN 0 ELSE 1 END AS database_exists;
`

const sqlSessions = `
SET NOCOUNT ON;

SELECT
    s.session_id,
    s.login_name,
    s.host_name,
    s.program_name,
    s.status
FROM sys.dm_exec_sessions s
WHERE s.database_id = DB_ID(@database)
    AND s.session_id <> @@SPID
ORDER BY s.session_id;
`

const sqlKillSessions = `
SET NOCOUNT ON;

DECLARE @sql nvarchar(max) = N'';

SELECT @sql = STRING_AGG(N'KILL ' + CONVERT(nvarchar(20), s.session_id), N';' + CHAR(13) + CHAR(10))
FROM sys.dm_exec_sessions s
WHERE s.database_id = DB_ID(@database)
    AND s.session_id <> @@SPID;

IF @sql IS NULL OR @sql = N''
BEGIN
    PRINT 'No sessions to kill.';
    RETURN;
END;

PRINT @sql;
EXEC sys.sp_executesql @sql;
`

const sqlRenameDatabase = `
USE [master];

SET NOCOUNT ON;

DECLARE @sql nvarchar(max);

IF DB_ID(@database) IS NULL
BEGIN
    THROW 50000, 'Source database does not exist.', 1;
END;

IF DB_ID(@new_name) IS NOT NULL
BEGIN
    THROW 50001, 'Target database already exists.', 1;
END;

SET @sql = N'ALTER DATABASE ' + QUOTENAME(@database) + N' SET SINGLE_USER WITH ROLLBACK IMMEDIATE;';
PRINT @sql;
EXEC sys.sp_executesql @sql;

SET @sql = N'ALTER DATABASE ' + QUOTENAME(@database) + N' MODIFY NAME = ' + QUOTENAME(@new_name) + N';';
PRINT @sql;
EXEC sys.sp_executesql @sql;

SET @sql = N'ALTER DATABASE ' + QUOTENAME(@new_name) + N' SET MULTI_USER;';
PRINT @sql;
EXEC sys.sp_executesql @sql;
`

const sqlDropDatabase = `
USE [master];

SET NOCOUNT ON;

DECLARE @sql nvarchar(max);

IF DB_ID(@database) IS NULL
BEGIN
    THROW 50000, 'Database does not exist.', 1;
END;

IF LOWER(@database) IN (N'master', N'model', N'msdb', N'tempdb')
BEGIN
    THROW 50001, 'Refusing to drop a system database.', 1;
END;

SET @sql = N'ALTER DATABASE ' + QUOTENAME(@database) + N' SET SINGLE_USER WITH ROLLBACK IMMEDIATE;';
PRINT @sql;
EXEC sys.sp_executesql @sql;

SET @sql = N'DROP DATABASE ' + QUOTENAME(@database) + N';';
PRINT @sql;
EXEC sys.sp_executesql @sql;
IF @delete_backup_history = 1
BEGIN
    EXEC msdb.dbo.sp_delete_database_backuphistory
        @database_name = @database;
END;
`
