package sqlserver

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/luem2/sqlkit/internal/config"
	"github.com/luem2/sqlkit/internal/process"
)

type Runner struct {
	SQLCmd     string
	Conn       *config.SQLConnection
	Redactions []string
}

func (r Runner) Run(ctx context.Context, database string, sql string) process.Result {
	args := []string{
		"-S", r.Conn.Server,
		"-U", r.Conn.User,
		"-d", database,
		"-b",
		"-Q", sql,
	}
	return r.run(ctx, args...)
}

func (r Runner) Query(ctx context.Context, database string, sql string) process.Result {
	args := []string{
		"-S", r.Conn.Server,
		"-U", r.Conn.User,
		"-d", database,
		"-b",
		"-W",
		"-s", "\t",
		"-Q", sql,
	}
	return r.run(ctx, args...)
}

func (r Runner) RunFile(ctx context.Context, database string, path string) process.Result {
	args := []string{
		"-S", r.Conn.Server,
		"-U", r.Conn.User,
		"-d", database,
		"-b",
		"-i", path,
	}
	return r.runWithEnvironment(ctx, nil, nil, filepath.Dir(path), args...)
}

func (r Runner) RunFileWithVariables(ctx context.Context, database string, path string, variables map[string]string) process.Result {
	return r.RunFileWithVariablesAndEnvironment(ctx, database, path, variables, nil, nil)
}

func (r Runner) RunFileWithVariablesAndEnvironment(ctx context.Context, database string, path string, variables map[string]string, environment map[string]string, redactions []string) process.Result {
	args := []string{
		"-S", r.Conn.Server,
		"-U", r.Conn.User,
		"-d", database,
		"-b",
		"-i", path,
	}
	if len(variables) > 0 {
		args = append(args, "-v")
		names := make([]string, 0, len(variables))
		for name := range variables {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value := variables[name]
			args = append(args, name+"="+value)
		}
	}
	return r.runWithEnvironment(ctx, environment, redactions, filepath.Dir(path), args...)
}

func (r Runner) RequireSuccess(ctx context.Context, database string, sql string) error {
	result := r.Run(ctx, database, sql)
	if result.ExitCode != 0 {
		return fmt.Errorf("sqlcmd failed with exit code %d\n%s", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (r Runner) run(ctx context.Context, args ...string) process.Result {
	return r.runWithEnvironment(ctx, nil, nil, "", args...)
}

func (r Runner) runWithEnvironment(ctx context.Context, environment map[string]string, extraRedactions []string, workingDirectory string, args ...string) process.Result {
	env := []string{"SQLCMDPASSWORD=" + r.Conn.Password}
	redactions := append([]string{}, r.Redactions...)
	redactions = append(redactions, extraRedactions...)
	if len(environment) > 0 {
		names := make([]string, 0, len(environment))
		for name := range environment {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value := environment[name]
			env = append(env, name+"="+value)
			redactions = append(redactions, value)
		}
	}
	return process.RunOptions(ctx, process.Options{
		Redactions:       redactions,
		Env:              env,
		WorkingDirectory: workingDirectory,
	}, r.SQLCmd, args...)
}
