package docs

type SQLParameter struct {
	Name        string
	Type        string
	Default     string
	Output      bool
	Required    bool
	Observation string
}

type SQLResultColumn struct {
	Name       string
	Type       string
	Expression string
}

type SQLResultSet struct {
	Columns []SQLResultColumn
}

type SQLTableRef struct {
	Schema string
	Table  string
}

type SQLColumnMetadata struct {
	Schema string
	Table  string
	Column string
	Type   string
}

type SQLTypeBlock struct {
	Type        int
	Description string
	Body        string
	Parameters  []SQLParameter
	Outputs     []SQLParameter
	ResultSets  []SQLResultSet
}

type SQLProcedureDoc struct {
	SourcePath string
	Name       string
	Parameters []SQLParameter
	Blocks     []SQLTypeBlock
}
