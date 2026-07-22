package sqlserver

const sqlFkReferences = `
SET NOCOUNT ON;

DECLARE @object_id int = OBJECT_ID(QUOTENAME(@schema) + N'.' + QUOTENAME(@table), N'U');

IF @object_id IS NULL
BEGIN
    THROW 50000, 'Table does not exist.', 1;
END;

SELECT
    fk.name AS foreign_key_name,
    SCHEMA_NAME(parent_table.schema_id) AS referencing_schema_name,
    parent_table.name AS referencing_table_name,
    parent_column.name AS referencing_column_name,
    SCHEMA_NAME(referenced_table.schema_id) AS referenced_schema_name,
    referenced_table.name AS referenced_table_name,
    referenced_column.name AS referenced_column_name
FROM sys.foreign_keys AS fk
INNER JOIN sys.foreign_key_columns AS fkc
    ON fk.object_id = fkc.constraint_object_id
INNER JOIN sys.tables AS parent_table
    ON fkc.parent_object_id = parent_table.object_id
INNER JOIN sys.columns AS parent_column
    ON fkc.parent_object_id = parent_column.object_id
    AND fkc.parent_column_id = parent_column.column_id
INNER JOIN sys.tables AS referenced_table
    ON fkc.referenced_object_id = referenced_table.object_id
INNER JOIN sys.columns AS referenced_column
    ON fkc.referenced_object_id = referenced_column.object_id
    AND fkc.referenced_column_id = referenced_column.column_id
WHERE fkc.referenced_object_id = @object_id
ORDER BY
    foreign_key_name,
    referencing_schema_name,
    referencing_table_name,
    referencing_column_name;
`

const sqlNullRows = `
SET NOCOUNT ON;

DECLARE @object_id int = OBJECT_ID(QUOTENAME(@schema) + N'.' + QUOTENAME(@table), N'U');
DECLARE @target nvarchar(517) = QUOTENAME(@schema) + N'.' + QUOTENAME(@table);
DECLARE @sql nvarchar(max);

IF @object_id IS NULL
BEGIN
    THROW 50000, 'Table does not exist.', 1;
END;

SELECT @sql = STRING_AGG(
    CONVERT(nvarchar(max),
        N'SELECT N''' + REPLACE(c.name, '''', '''''') + N''' AS null_column_name, * FROM '
        + @target
        + N' WHERE ' + QUOTENAME(c.name) + N' IS NULL'
    ),
    N' UNION ALL '
)
FROM sys.columns AS c
WHERE c.object_id = @object_id
    AND c.is_nullable = 1;

IF @sql IS NULL
BEGIN
    PRINT 'No nullable columns found.';
    RETURN;
END;

EXEC sys.sp_executesql @sql;
`

const sqlCharScan = `
SET NOCOUNT ON;

DECLARE @object_id int = OBJECT_ID(QUOTENAME(@schema) + N'.' + QUOTENAME(@table), N'U');
DECLARE @target nvarchar(517) = QUOTENAME(@schema) + N'.' + QUOTENAME(@table);
DECLARE @sql nvarchar(max);

IF @object_id IS NULL
BEGIN
    THROW 50000, 'Table does not exist.', 1;
END;

SELECT @sql = STRING_AGG(
    CONVERT(nvarchar(max),
        N'SELECT N''' + REPLACE(c.name, '''', '''''') + N''' AS column_name,'
        + N' SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(13) + N''%'' THEN 1 ELSE 0 END) AS char_13_count,'
        + N' SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(10) + N''%'' THEN 1 ELSE 0 END) AS char_10_count,'
        + N' SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(9) + N''%'' THEN 1 ELSE 0 END) AS tab_count,'
        + N' SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N'' %'' THEN 1 ELSE 0 END) AS leading_space_count,'
        + N' SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''% '' THEN 1 ELSE 0 END) AS trailing_space_count'
        + N' FROM ' + @target
        + N' HAVING'
        + N' SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(13) + N''%'' THEN 1 ELSE 0 END)'
        + N' + SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(10) + N''%'' THEN 1 ELSE 0 END)'
        + N' + SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(9) + N''%'' THEN 1 ELSE 0 END)'
        + N' + SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N'' %'' THEN 1 ELSE 0 END)'
        + N' + SUM(CASE WHEN ' + QUOTENAME(c.name) + N' LIKE N''% '' THEN 1 ELSE 0 END) > 0'
    ),
    N' UNION ALL '
)
FROM sys.columns AS c
INNER JOIN sys.types AS t
    ON c.user_type_id = t.user_type_id
WHERE c.object_id = @object_id
    AND t.name IN (N'char', N'varchar', N'nchar', N'nvarchar', N'text', N'ntext');

IF @sql IS NULL
BEGIN
    PRINT 'No text columns found.';
    RETURN;
END;

EXEC sys.sp_executesql @sql;
`

const sqlCharClean = `
SET NOCOUNT ON;

DECLARE @object_id int = OBJECT_ID(QUOTENAME(@schema) + N'.' + QUOTENAME(@table), N'U');
DECLARE @target nvarchar(517) = QUOTENAME(@schema) + N'.' + QUOTENAME(@table);
DECLARE @sql nvarchar(max);

IF @object_id IS NULL
BEGIN
    THROW 50000, 'Table does not exist.', 1;
END;

SELECT @sql = STRING_AGG(
    CONVERT(nvarchar(max),
        N'UPDATE ' + @target
        + N' SET ' + QUOTENAME(c.name) + N' = TRIM(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE('
        + QUOTENAME(c.name)
        + N', CHAR(2), ''''), CHAR(3), ''''), CHAR(9), ''''), CHAR(10), ''''), CHAR(13), ''''), CHAR(39), ''''))'
        + N' WHERE ' + QUOTENAME(c.name) + N' IS NOT NULL AND ('
        + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(2) + N''%'' OR '
        + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(3) + N''%'' OR '
        + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(9) + N''%'' OR '
        + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(10) + N''%'' OR '
        + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(13) + N''%'' OR '
        + QUOTENAME(c.name) + N' LIKE N''%'' + CHAR(39) + N''%'' OR '
        + QUOTENAME(c.name) + N' LIKE N'' %'' OR '
        + QUOTENAME(c.name) + N' LIKE N''% '')'
    ),
    N';' + CHAR(13) + CHAR(10)
)
FROM sys.columns AS c
INNER JOIN sys.types AS t
    ON c.user_type_id = t.user_type_id
WHERE c.object_id = @object_id
    AND t.name IN (N'char', N'varchar', N'nchar', N'nvarchar');

IF @sql IS NULL
BEGIN
    PRINT 'No text columns found.';
    RETURN;
END;

SET @sql += N';';
PRINT @sql;
EXEC sys.sp_executesql @sql;
`

const sqlDatabaseSize = `
SET NOCOUNT ON;

SELECT
    DB_NAME(database_id) AS database_name,
    CONVERT(decimal(18, 2), SUM(size) * 8.0 / 1024.0) AS total_size_mb,
    CONVERT(decimal(18, 2), SUM(CASE WHEN type = 0 THEN size ELSE 0 END) * 8.0 / 1024.0) AS data_size_mb,
    CONVERT(decimal(18, 2), SUM(CASE WHEN type = 1 THEN size ELSE 0 END) * 8.0 / 1024.0) AS log_size_mb
FROM sys.master_files
WHERE NULLIF(@database, N'') IS NULL
    OR DB_NAME(database_id) = @database
GROUP BY database_id
ORDER BY database_name;
`

const sqlRecoveryModel = `
SET NOCOUNT ON;

SELECT
    name AS database_name,
    recovery_model_desc AS recovery_model
FROM sys.databases
WHERE NULLIF(@database, N'') IS NULL
    OR name = @database
ORDER BY database_name;
`

const sqlLocksDiagnose = `
SET NOCOUNT ON;

SELECT
    r.session_id,
    r.blocking_session_id,
    s.status,
    s.login_name,
    s.host_name,
    s.program_name,
    DB_NAME(r.database_id) AS database_name,
    r.command,
    r.wait_type,
    r.wait_time AS wait_time_ms,
    t.text AS running_sql
FROM sys.dm_exec_requests AS r
INNER JOIN sys.dm_exec_sessions AS s
    ON r.session_id = s.session_id
OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) AS t
WHERE r.blocking_session_id <> 0
ORDER BY r.session_id;

SELECT
    l.request_session_id AS session_id,
    DB_NAME(l.resource_database_id) AS database_name,
    OBJECT_SCHEMA_NAME(p.object_id, l.resource_database_id) AS schema_name,
    OBJECT_NAME(p.object_id, l.resource_database_id) AS object_name,
    l.resource_type,
    l.request_mode,
    l.request_status
FROM sys.dm_tran_locks AS l
LEFT JOIN sys.partitions AS p
    ON l.resource_associated_entity_id = p.hobt_id
WHERE l.request_mode IN (N'Sch-S', N'Sch-M', N'X', N'IX')
ORDER BY
    session_id,
    database_name,
    schema_name,
    object_name;

SELECT TOP (20)
    r.blocking_session_id,
    COUNT(*) AS blocked_session_count
FROM sys.dm_exec_requests AS r
WHERE r.blocking_session_id <> 0
GROUP BY r.blocking_session_id
ORDER BY blocked_session_count DESC;
`
