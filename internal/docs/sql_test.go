package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeSQLProcedureDocumentsITypeBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_INSUP_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_INSUP_Test]
    @iType TINYINT
    , @iId INT = NULL
    , @iIdSucursal INT = NULL
    , @iNombre VARCHAR(50) = NULL -- nombre visible
    , @oId_utilizado INT = NULL OUTPUT
    , @oError_numero INT = NULL OUTPUT
AS
SET NOCOUNT ON;

IF (@iType = 1) -- 1 = Insert
BEGIN
    INSERT INTO vn.Test(id_sucursal, nombre)
    VALUES (@iIdSucursal, @iNombre)

    SET @oId_utilizado = SCOPE_IDENTITY()
    SET @oError_numero = 0
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 1)
	if err != nil {
		t.Fatal(err)
	}

	if doc.Name != "[vn].[SP_INSUP_Test]" {
		t.Fatalf("got %q", doc.Name)
	}
	if len(doc.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(doc.Blocks))
	}
	block := doc.Blocks[0]
	if block.Type != 1 {
		t.Fatalf("got type %d, want 1", block.Type)
	}
	if len(block.Parameters) != 3 {
		t.Fatalf("got %d params, want 3: %#v", len(block.Parameters), block.Parameters)
	}
	if len(block.Outputs) != 2 {
		t.Fatalf("got %d outputs, want 2: %#v", len(block.Outputs), block.Outputs)
	}

	markdown := RenderSQLMarkdown(doc)
	for _, want := range []string{
		"## `@iType = 1`",
		"`@iNombre`",
		"nombre visible",
		"`@oId_utilizado`",
		"> No se detecto un `SELECT` de salida.",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown does not contain %q:\n%s", want, markdown)
		}
	}
	for _, notWant := range []string{
		"## Origen",
		"## Firma del procedimiento",
		"## Descripcion funcional",
		"`@iId` |",
	} {
		if strings.Contains(markdown, notWant) {
			t.Fatalf("markdown contains %q:\n%s", notWant, markdown)
		}
	}
}

func TestAnalyzeSQLProcedureDocumentsSearchResults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
    , @iId INT = NULL
    , @iSucursales VARCHAR(1000) = NULL
    , @iApellido VARCHAR(45) = NULL
    , @iPagina INT = 1
    , @oTotal_filas INT = NULL OUTPUT
AS
SET NOCOUNT ON;

DECLARE @OmitirReg INT = (@iPagina - 1) * 50

IF (@iType = 10) -- 10 = Buscar registros
BEGIN
    ;WITH CTE_Sucursales AS (
        SELECT VALUE AS id_sucursal
        FROM STRING_SPLIT(@iSucursales, ',')
    )
    SELECT t.id AS Id
        , t.nombre AS Nombre
        , CONCAT(t.apellido, ', ', t.nombre) AS NombreCompleto
        , 1 AS Activo
        , CAST(t.importe AS DECIMAL(17, 2)) AS Importe
    INTO #Tmp
    FROM dbo.Test t
        INNER JOIN CTE_Sucursales su ON su.id_sucursal = t.id_sucursal
    WHERE (@iApellido IS NULL OR t.apellido = @iApellido)

    SELECT @oTotal_filas = @@ROWCOUNT;

    SELECT *
    FROM #Tmp
    ORDER BY Id OFFSET @OmitirReg ROWS
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(doc.Blocks))
	}

	got := doc.Blocks[0].ResultSets[0].Columns
	if len(got) != 5 {
		t.Fatalf("got %d result columns, want 5: %#v", len(got), got)
	}
	if got[0].Name != "Id" || got[1].Name != "Nombre" || got[2].Name != "NombreCompleto" || got[3].Name != "Activo" || got[4].Name != "Importe" {
		t.Fatalf("unexpected columns: %#v", got)
	}
	if got[2].Type != "VARCHAR/NVARCHAR" || got[3].Type != "INT" || got[4].Type != "DECIMAL(17, 2)" {
		t.Fatalf("unexpected inferred types: %#v", got)
	}

	params := doc.Blocks[0].Parameters
	if !findParam(params, "@iSucursales").Required {
		t.Fatalf("@iSucursales should be required: %#v", params)
	}
	if findParam(params, "@iApellido").Required {
		t.Fatalf("@iApellido should be optional: %#v", params)
	}
	if findParam(params, "@iPagina").Name == "" {
		t.Fatalf("@iPagina should be detected through @OmitirReg dependency: %#v", params)
	}
	if findParam(params, "@iPagina").Required {
		t.Fatalf("@iPagina should be optional because it has a non-null default: %#v", params)
	}

	markdown := RenderSQLMarkdown(doc)
	if strings.Contains(markdown, "Descripcion |") {
		t.Fatalf("result table should not include description column:\n%s", markdown)
	}
}

func TestAnalyzeSQLProcedureTreatsOptionalCTEFilterParameterAsOptional(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
    , @iSucursales VARCHAR(1000) = NULL
AS
SET NOCOUNT ON;

IF (@iType = 5)
BEGIN
    WITH CTE_Sucursales AS (
        SELECT VALUE AS id_sucursal
        FROM STRING_SPLIT(@iSucursales, ',')
    )
    SELECT s.id
    FROM vn.Solicitud s
    WHERE (@iSucursales IS NULL OR s.id_sucursal IN (SELECT id_sucursal FROM CTE_Sucursales))
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 5)
	if err != nil {
		t.Fatal(err)
	}

	param := findParam(doc.Blocks[0].Parameters, "@iSucursales")
	if param.Name == "" {
		t.Fatalf("@iSucursales not detected: %#v", doc.Blocks[0].Parameters)
	}
	if param.Required {
		t.Fatalf("@iSucursales should be optional when guarded by IS NULL OR: %#v", param)
	}
}

func TestAnalyzeSQLProcedurePropagatesChainedTempTableResultTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
AS
SET NOCOUNT ON;

IF (@iType = 5)
BEGIN
    SELECT id_solicitud,
        MAX(CAST(CASE WHEN paso = 1 THEN activo END AS INT)) AS W1,
        MAX(CAST(CASE WHEN paso = 2 THEN activo END AS INT)) AS W2
    INTO #SolicitudWorkflow
    FROM vn.Workflow_Solicitud
    GROUP BY id_solicitud;

    SELECT s.id,
        swf.W1,
        swf.W2
    INTO #PAGINAR
    FROM vn.Solicitud s
        INNER JOIN #SolicitudWorkflow swf ON swf.id_solicitud = s.id;

    SELECT id,
        W1,
        W2
    FROM #PAGINAR
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 5)
	if err != nil {
		t.Fatal(err)
	}

	got := doc.Blocks[0].ResultSets[0].Columns
	if len(got) != 3 {
		t.Fatalf("got %d result columns, want 3: %#v", len(got), got)
	}
	if got[1].Name != "W1" || got[1].Type != "INT" || got[2].Name != "W2" || got[2].Type != "INT" {
		t.Fatalf("workflow temp columns did not propagate as INT: %#v", got)
	}

	EnrichSQLDocMetadata(doc, []SQLColumnMetadata{
		{Schema: "vn", Table: "Solicitud", Column: "id", Type: "INT"},
	})
	got = doc.Blocks[0].ResultSets[0].Columns
	if got[0].Name != "id" || got[0].Type != "INT" {
		t.Fatalf("metadata did not propagate through #PAGINAR id: %#v", got)
	}
}

func TestEnrichSQLDocMetadataResolvesBaseColumnTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
AS
SET NOCOUNT ON;

IF (@iType = 1)
BEGIN
    SELECT s.id AS Id
        , s.fecha_solicitud AS FechaSolicitud
        , CONCAT(c.apellido, ', ', c.nombre) AS Nombre
    FROM vn.Solicitud s
        INNER JOIN vn.Cliente c ON c.id = s.id_cliente
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	tables := ReferencedSQLTables(doc)
	if len(tables) != 2 {
		t.Fatalf("got %d tables, want 2: %#v", len(tables), tables)
	}

	EnrichSQLDocMetadata(doc, []SQLColumnMetadata{
		{Schema: "vn", Table: "Solicitud", Column: "id", Type: "INT"},
		{Schema: "vn", Table: "Solicitud", Column: "fecha_solicitud", Type: "DATETIME"},
		{Schema: "vn", Table: "Cliente", Column: "apellido", Type: "VARCHAR(45)"},
		{Schema: "vn", Table: "Cliente", Column: "nombre", Type: "VARCHAR(45)"},
	})

	got := doc.Blocks[0].ResultSets[0].Columns
	if got[0].Type != "INT" || got[1].Type != "DATETIME" || got[2].Type != "VARCHAR/NVARCHAR" {
		t.Fatalf("unexpected enriched types: %#v", got)
	}
}

func TestEnrichSQLDocMetadataResolvesWrapperFunctionTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
AS
SET NOCOUNT ON;

IF (@iType = 5)
BEGIN
    SELECT UPPER(es.descripcion) AS estado_solicitud,
        ISNULL(s.id_calificacion, 0) AS id_calif
    INTO #PAGINAR
    FROM vn.Solicitud s
        INNER JOIN ba.Estado_Solicitud es ON es.id = s.id_estado_solicitud;

    SELECT estado_solicitud,
        id_calif
    FROM #PAGINAR
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	EnrichSQLDocMetadata(doc, []SQLColumnMetadata{
		{Schema: "ba", Table: "Estado_Solicitud", Column: "descripcion", Type: "VARCHAR(60)"},
		{Schema: "vn", Table: "Solicitud", Column: "id_calificacion", Type: "INT"},
	})

	got := doc.Blocks[0].ResultSets[0].Columns
	if len(got) != 2 {
		t.Fatalf("got %d result columns, want 2: %#v", len(got), got)
	}
	if got[0].Name != "estado_solicitud" || got[0].Type != "VARCHAR(60)" {
		t.Fatalf("estado_solicitud did not inherit UPPER metadata type: %#v", got)
	}
	if got[1].Name != "id_calif" || got[1].Type != "INT" {
		t.Fatalf("id_calif did not inherit ISNULL metadata type: %#v", got)
	}
}

func TestAnalyzeSQLProcedureSeparatesMultipleResultSets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
AS
SET NOCOUNT ON;

IF (@iType = 50)
BEGIN
    SELECT 1 AS Primero

    SELECT 'x' AS Segundo
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 50)
	if err != nil {
		t.Fatal(err)
	}

	block := doc.Blocks[0]
	if len(block.ResultSets) != 2 {
		t.Fatalf("got %d result sets, want 2: %#v", len(block.ResultSets), block.ResultSets)
	}
	if block.ResultSets[0].Columns[0].Name != "Primero" || block.ResultSets[1].Columns[0].Name != "Segundo" {
		t.Fatalf("unexpected result set columns: %#v", block.ResultSets)
	}
	markdown := RenderSQLMarkdown(doc)
	for _, want := range []string{"#### Result set 1", "#### Result set 2", "`Primero`", "`Segundo`"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown does not contain %q:\n%s", want, markdown)
		}
	}
}

func TestEnrichSQLDocMetadataResolvesApplyAggregatesArithmeticCaseAndNestedCast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
    , @iIdCliente INT = NULL
    , @iIdTipoParticipanteSolicitud INT = NULL
AS
SET NOCOUNT ON;

IF (@iType = 50)
BEGIN
    SELECT c.id AS IdCliente
        , cc.Debe
        , cc.Haber
        , cc.Saldo
        , cc.PrimeraEmision
        , cc.UltimoVto
    FROM vn.Cliente c
        OUTER APPLY (
            SELECT SUM(cuota.importe_debe) AS Debe
                , SUM(cuota.importe_haber) AS Haber
                , SUM(cuota.importe_debe - cuota.importe_haber) AS Saldo
                , MIN(cuota.fecha_emision) AS PrimeraEmision
                , MAX(cuota.vencimiento) AS UltimoVto
            FROM vn.Cuota cuota
                INNER JOIN vn.Solicitud_Titular s ON s.id = cuota.id_solicitud
            WHERE cuota.estado_registro = 1
                AND s.estado_registro = 1
        ) cc
    WHERE c.id = @iIdCliente

    SELECT cuota.importe_debe AS Debe
        , cuota.importe_haber AS Haber
        , cuota.importe_debe - cuota.importe_haber AS Saldo
    FROM vn.Cuota cuota

    SELECT CONCAT(CAST(s.monto_capital_total AS INT), 'x', s.cantidad_cuotas) AS Producto
        , CASE
            WHEN s.id_tipo_producto = 1 THEN 'COMÚN'
            WHEN s.id_tipo_producto = 2 THEN 'PROMO'
        END AS TipoProducto
    FROM vn.Solicitud_Titular s
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 50)
	if err != nil {
		t.Fatal(err)
	}
	EnrichSQLDocMetadata(doc, []SQLColumnMetadata{
		{Schema: "vn", Table: "Cliente", Column: "id", Type: "INT"},
		{Schema: "vn", Table: "Cuota", Column: "importe_debe", Type: "DECIMAL(17, 2)"},
		{Schema: "vn", Table: "Cuota", Column: "importe_haber", Type: "DECIMAL(17, 2)"},
		{Schema: "vn", Table: "Cuota", Column: "fecha_emision", Type: "DATE"},
		{Schema: "vn", Table: "Cuota", Column: "vencimiento", Type: "DATE"},
		{Schema: "vn", Table: "Solicitud_Titular", Column: "monto_capital_total", Type: "DECIMAL(17, 2)"},
		{Schema: "vn", Table: "Solicitud_Titular", Column: "cantidad_cuotas", Type: "INT"},
		{Schema: "vn", Table: "Solicitud_Titular", Column: "id_tipo_producto", Type: "INT"},
	})

	resultSets := doc.Blocks[0].ResultSets
	if len(resultSets) != 3 {
		t.Fatalf("got %d result sets, want 3: %#v", len(resultSets), resultSets)
	}
	first := resultSets[0].Columns
	if first[1].Name != "Debe" || first[1].Type != "DECIMAL(38, 2)" {
		t.Fatalf("OUTER APPLY SUM Debe type was not inferred: %#v", first)
	}
	if first[3].Name != "Saldo" || first[3].Type != "DECIMAL(38, 2)" {
		t.Fatalf("OUTER APPLY SUM arithmetic Saldo type was not inferred: %#v", first)
	}
	if first[4].Name != "PrimeraEmision" || first[4].Type != "DATE" || first[5].Name != "UltimoVto" || first[5].Type != "DATE" {
		t.Fatalf("OUTER APPLY MIN/MAX date types were not inferred: %#v", first)
	}

	second := resultSets[1].Columns
	if second[2].Name != "Saldo" || second[2].Type != "DECIMAL(18, 2)" {
		t.Fatalf("arithmetic Saldo type was not inferred: %#v", second)
	}

	third := resultSets[2].Columns
	if third[0].Name != "Producto" || third[0].Type != "VARCHAR/NVARCHAR" {
		t.Fatalf("CONCAT(CAST(...)) should infer string type: %#v", third)
	}
	if third[1].Name != "TipoProducto" || third[1].Type != "VARCHAR/NVARCHAR" {
		t.Fatalf("CASE string type was not inferred: %#v", third)
	}
}

func TestAnalyzeSQLProcedureInfersConcatWSTempTableColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
AS
SET NOCOUNT ON;

IF (@iType = 51)
BEGIN
    SELECT CONCAT_WS(' ', d.calle, d.domicilio_numero) AS domicilio
    INTO #PAGINAR_SOL
    FROM vn.Solicitud_Titular s
        CROSS APPLY vn.fn_ObtenerDatosCliente(NULL) d

    SELECT domicilio
    FROM #PAGINAR_SOL
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 51)
	if err != nil {
		t.Fatal(err)
	}

	got := doc.Blocks[0].ResultSets[0].Columns
	if len(got) != 1 {
		t.Fatalf("got %d result columns, want 1: %#v", len(got), got)
	}
	if got[0].Name != "domicilio" || got[0].Type != "VARCHAR/NVARCHAR" {
		t.Fatalf("CONCAT_WS temp column type was not inferred: %#v", got)
	}
}

func TestEnrichSQLDocMetadataPropagatesTempTableColumnsThroughCTE(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SP_SEARCH_Test.sql")
	content := `
CREATE PROCEDURE [vn].[SP_SEARCH_Test]
    @iType TINYINT
AS
SET NOCOUNT ON;

IF (@iType = 51)
BEGIN
    SELECT s.id,
        s.id_sucursal,
        IIF(ws.activo = 1, NULL, ws.fecha) AS fecha_entrega_capital
    INTO #PAGINAR_SOL
    FROM vn.Solicitud_Titular s
        INNER JOIN vn.Workflow_Solicitud ws ON ws.id_solicitud = s.id

    ;WITH Pagina AS (
        SELECT id,
            id_sucursal,
            fecha_entrega_capital
        FROM #PAGINAR_SOL
    )
    SELECT p.id,
        p.id_sucursal,
        p.fecha_entrega_capital,
        CONCAT_WS(' ', d.calle, d.domicilio_numero) AS domicilio
    FROM Pagina p
        LEFT JOIN vn.VW_Domicilio_Cliente d ON d.id_cliente = p.id
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	doc, err := AnalyzeSQLProcedure(path, 51)
	if err != nil {
		t.Fatal(err)
	}
	EnrichSQLDocMetadata(doc, []SQLColumnMetadata{
		{Schema: "vn", Table: "Solicitud_Titular", Column: "id", Type: "INT"},
		{Schema: "vn", Table: "Solicitud_Titular", Column: "id_sucursal", Type: "INT"},
		{Schema: "vn", Table: "Workflow_Solicitud", Column: "activo", Type: "BIT"},
		{Schema: "vn", Table: "Workflow_Solicitud", Column: "fecha", Type: "DATETIME"},
	})

	got := doc.Blocks[0].ResultSets[0].Columns
	if len(got) != 4 {
		t.Fatalf("got %d result columns, want 4: %#v", len(got), got)
	}
	if got[0].Name != "id" || got[0].Type != "INT" {
		t.Fatalf("CTE p.id type was not propagated: %#v", got)
	}
	if got[1].Name != "id_sucursal" || got[1].Type != "INT" {
		t.Fatalf("CTE p.id_sucursal type was not propagated: %#v", got)
	}
	if got[2].Name != "fecha_entrega_capital" || got[2].Type != "DATETIME" {
		t.Fatalf("IIF/CTE date type was not propagated: %#v", got)
	}
	if got[3].Name != "domicilio" || got[3].Type != "VARCHAR/NVARCHAR" {
		t.Fatalf("CONCAT_WS type was not inferred: %#v", got)
	}
}

func findParam(params []SQLParameter, name string) SQLParameter {
	for _, param := range params {
		if param.Name == name {
			return param
		}
	}
	return SQLParameter{}
}
