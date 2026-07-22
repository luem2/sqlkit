package sqlserver

import (
	"database/sql/driver"
	"fmt"
)

type Parameter struct {
	Name  string
	Value driver.Value
}

type Statement struct {
	Name       string
	Text       string
	Parameters []Parameter
}

func sqlTextStatement(name string, text string, parameters ...Parameter) Statement {
	return Statement{
		Name:       name,
		Text:       text,
		Parameters: parameters,
	}
}

func StringParam(name string, value string) Parameter {
	return Parameter{Name: name, Value: value}
}

func BoolParam(name string, value bool) Parameter {
	return Parameter{Name: name, Value: value}
}

func errBackupFileRequired() error {
	return fmt.Errorf("backup file is required")
}
