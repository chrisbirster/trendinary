package history

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/chrisbirster/trendinary/internal/sqlscript"
)

// schemaManagedConnector is the production runtime boundary. Atlas owns schema
// changes, so application connections never send CREATE/ALTER/DROP statements
// to Turso. Legacy package constructors still contain idempotent DDL; returning
// success for schema-only scripts keeps them compatible while making the actual
// production connection schema-read-only.
type schemaManagedConnector struct {
	inner driver.Connector
}

func (c schemaManagedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &schemaManagedConn{Conn: conn}, nil
}

func (c schemaManagedConnector) Driver() driver.Driver { return c.inner.Driver() }

type schemaManagedConn struct {
	driver.Conn
}

func (c *schemaManagedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	statements := sqlscript.Split(query)
	if len(statements) > 0 {
		allDDL := true
		anyDDL := false
		for _, statement := range statements {
			ddl := schemaDDL(statement)
			allDDL = allDDL && ddl
			anyDDL = anyDDL || ddl
		}
		if allDDL {
			if len(args) != 0 {
				return nil, fmt.Errorf("schema mutation is managed by Atlas and cannot bind runtime arguments")
			}
			return driver.RowsAffected(0), nil
		}
		if anyDDL {
			return nil, fmt.Errorf("runtime SQL mixes schema mutation with application statements; run Atlas instead")
		}
	}
	return execDriverConn(ctx, c.Conn, query, args)
}

func schemaDDL(statement string) bool {
	fields := strings.Fields(strings.TrimSpace(statement))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToUpper(fields[0]) {
	case "CREATE", "ALTER", "DROP", "REINDEX":
		return true
	default:
		return false
	}
}

func execDriverConn(ctx context.Context, conn driver.Conn, query string, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := conn.(driver.ExecerContext); ok {
		result, err := execer.ExecContext(ctx, query, args)
		if err != driver.ErrSkip {
			return result, err
		}
	}
	var (
		stmt driver.Stmt
		err  error
	)
	if preparer, ok := conn.(driver.ConnPrepareContext); ok {
		stmt, err = preparer.PrepareContext(ctx, query)
	} else {
		stmt, err = conn.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	return stmt.Exec(values)
}

func (c *schemaManagedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.Conn.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *schemaManagedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if schemaDDL(query) {
		return nil, fmt.Errorf("runtime schema mutation is disabled; run Atlas instead")
	}
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, query)
	}
	return c.Conn.Prepare(query)
}

func (c *schemaManagedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		return beginner.BeginTx(ctx, opts)
	}
	return nil, driver.ErrSkip
}

func (c *schemaManagedConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *schemaManagedConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *schemaManagedConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *schemaManagedConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}
