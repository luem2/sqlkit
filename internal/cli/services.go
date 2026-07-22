package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/config"
	"github.com/luem2/sqlkit/internal/process"
	"github.com/luem2/sqlkit/internal/sqlserver"
)

const (
	defaultConnectTimeout = 15 * time.Second
	defaultSQLTimeout     = 2 * time.Hour
	defaultProcessTimeout = 2 * time.Hour
)

type dbService struct {
	app  *appContext
	cmd  *cobra.Command
	conn *config.SQLConnection
}

func newDBService(cmd *cobra.Command, app *appContext, envName string) (*dbService, error) {
	conn, err := loadConnection(app, envName)
	if err != nil {
		return nil, err
	}
	return &dbService{app: app, cmd: cmd, conn: conn}, nil
}

func (s *dbService) open(database string) (*sqlserver.Client, error) {
	ctx, cancel := timeoutContext(commandContext(s.cmd), s.app.connectTimeout)
	defer cancel()
	return sqlserver.OpenClient(ctx, s.conn, database)
}

func (s *dbService) exec(client *sqlserver.Client, statement sqlserver.Statement) error {
	ctx, cancel := timeoutContext(commandContext(s.cmd), s.app.sqlTimeout)
	defer cancel()
	return client.Exec(ctx, statement)
}

func (s *dbService) query(client *sqlserver.Client, statement sqlserver.Statement) ([]sqlserver.ResultSet, error) {
	ctx, cancel := timeoutContext(commandContext(s.cmd), s.app.sqlTimeout)
	defer cancel()
	return client.Query(ctx, statement)
}

func (s *dbService) execIn(database string, statement sqlserver.Statement) error {
	client, err := s.open(database)
	if err != nil {
		return err
	}
	defer client.Close()
	return s.exec(client, statement)
}

func (s *dbService) queryIn(database string, statement sqlserver.Statement) ([]sqlserver.ResultSet, error) {
	client, err := s.open(database)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return s.query(client, statement)
}

func (s *dbService) runner(sqlcmd string) sqlserver.Runner {
	return sqlserver.Runner{
		SQLCmd:     sqlcmd,
		Conn:       s.conn,
		Redactions: []string{s.conn.Password},
	}
}

type processService struct {
	app *appContext
	cmd *cobra.Command
}

func newProcessService(cmd *cobra.Command, app *appContext) processService {
	return processService{app: app, cmd: cmd}
}

func (s processService) Run(name string, args ...string) process.Result {
	return s.RunRedacted(nil, name, args...)
}

func (s processService) RunStreaming(name string, args ...string) process.Result {
	return s.RunStreamingRedacted(nil, name, args...)
}

func (s processService) RunRedacted(redactions []string, name string, args ...string) process.Result {
	ctx, cancel := timeoutContext(commandContext(s.cmd), s.app.processTimeout)
	defer cancel()
	return process.RunOptions(ctx, process.Options{Redactions: redactions}, name, args...)
}

func (s processService) RunStreamingRedacted(redactions []string, name string, args ...string) process.Result {
	ctx, cancel := timeoutContext(commandContext(s.cmd), s.app.processTimeout)
	defer cancel()
	return process.RunStreamingOptions(ctx, s.cmd.OutOrStdout(), s.cmd.ErrOrStderr(), process.Options{Redactions: redactions}, name, args...)
}

func timeoutContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}
