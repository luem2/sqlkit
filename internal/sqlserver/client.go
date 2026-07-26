package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/luem2/sqlkit/internal/config"
)

type Client struct {
	db *sql.DB
}

type ResultSet struct {
	Columns []string
	Rows    [][]string
}

func OpenClient(ctx context.Context, conn *config.SQLConnection, database string) (*Client, error) {
	db, err := sql.Open("sqlserver", connectionString(conn, database))
	if err != nil {
		return nil, err
	}

	client := &Client{db: db}
	if err := db.PingContext(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

func (c *Client) Exec(ctx context.Context, statement Statement) error {
	_, err := c.db.ExecContext(ctx, statement.Text, statement.sqlArgs()...)
	return err
}

func (c *Client) Query(ctx context.Context, statement Statement) ([]ResultSet, error) {
	rows, err := c.db.QueryContext(ctx, statement.Text, statement.sqlArgs()...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var resultSets []ResultSet
	for {
		resultSet, err := scanCurrentResultSet(rows)
		if err != nil {
			return nil, err
		}
		if len(resultSet.Columns) > 0 {
			resultSets = append(resultSets, resultSet)
		}
		if !rows.NextResultSet() {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return resultSets, nil
}

func scanCurrentResultSet(rows *sql.Rows) (ResultSet, error) {
	columns, err := rows.Columns()
	if err != nil {
		return ResultSet{}, err
	}

	resultSet := ResultSet{Columns: columns}
	for rows.Next() {
		rawValues := make([]interface{}, len(columns))
		scanValues := make([]interface{}, len(columns))
		for index := range rawValues {
			scanValues[index] = &rawValues[index]
		}
		if err := rows.Scan(scanValues...); err != nil {
			return ResultSet{}, err
		}

		row := make([]string, len(columns))
		for index, value := range rawValues {
			row[index] = formatSQLValue(value)
		}
		resultSet.Rows = append(resultSet.Rows, row)
	}

	return resultSet, nil
}

func formatSQLValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format("2006-01-02 15:04:05")
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func connectionString(conn *config.SQLConnection, database string) string {
	dsn := url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(conn.User, conn.Password),
		Host:   sqlServerURLHost(conn.Server),
	}

	if host, instance, ok := strings.Cut(dsn.Host, `\`); ok {
		dsn.Host = host
		dsn.Path = "/" + instance
	}

	query := dsn.Query()
	query.Set("database", database)
	encrypt := conn.Encrypt
	trustServerCertificate := conn.TrustServerCertificate
	if strings.TrimSpace(encrypt) == "" {
		encrypt = "disable"
		trustServerCertificate = true
	}
	query.Set("encrypt", encrypt)
	query.Set("TrustServerCertificate", strconv.FormatBool(trustServerCertificate))
	dsn.RawQuery = query.Encode()

	return dsn.String()
}

func sqlServerURLHost(server string) string {
	server = strings.TrimSpace(server)
	server = strings.TrimPrefix(server, "tcp:")
	if strings.Contains(server, ",") && !strings.Contains(server, ":") {
		host, port, ok := strings.Cut(server, ",")
		if ok {
			return strings.TrimSpace(host) + ":" + strings.TrimSpace(port)
		}
	}
	return server
}

func (s Statement) sqlArgs() []interface{} {
	args := make([]interface{}, 0, len(s.Parameters))
	for _, parameter := range s.Parameters {
		args = append(args, sql.Named(parameter.Name, parameter.Value))
	}
	return args
}

func (s Statement) String() string {
	if s.Name == "" {
		return "<sql statement>"
	}
	return fmt.Sprintf("<sql statement %s>", s.Name)
}
