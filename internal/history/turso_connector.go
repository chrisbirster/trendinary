package history

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/chrisbirster/trendinary/internal/sqlscript"
)

// scriptConnector normalizes the one important behavioral difference between
// local SQLite and the remote libSQL driver: SQLite accepts a schema script in
// ExecContext, while libSQL requires one statement per request. Keeping this at
// the connector boundary means every existing Store sharing the Turso *sql.DB
// (history, editorial, following, and lazy schemas) sees consistent behavior.
type scriptConnector struct {
	inner driver.Connector
}

func (c scriptConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &scriptConn{Conn: conn}, nil
}

func (c scriptConnector) Driver() driver.Driver { return c.inner.Driver() }

type scriptConn struct {
	driver.Conn
}

func (c *scriptConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	statements := sqlscript.Split(query)
	if len(statements) <= 1 {
		statement := query
		if len(statements) == 1 {
			statement = statements[0]
		}
		return c.execOne(ctx, statement, args)
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("libSQL multi-statement ExecContext cannot bind arguments across %d statements", len(statements))
	}

	var result driver.Result = driver.RowsAffected(0)
	for index, statement := range statements {
		var err error
		result, err = c.execOne(ctx, statement, nil)
		if err != nil {
			return nil, fmt.Errorf("libSQL script statement %d: %w", index+1, err)
		}
	}
	return result, nil
}

func (c *scriptConn) execOne(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := c.Conn.(driver.ExecerContext); ok {
		result, err := execer.ExecContext(ctx, query, args)
		if err != driver.ErrSkip {
			return result, err
		}
	}

	var (
		stmt driver.Stmt
		err  error
	)
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		stmt, err = preparer.PrepareContext(ctx, query)
	} else {
		stmt, err = c.Conn.Prepare(query)
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

func (c *scriptConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.Conn.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *scriptConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, query)
	}
	return c.Conn.Prepare(query)
}

func (c *scriptConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		return beginner.BeginTx(ctx, opts)
	}
	return nil, driver.ErrSkip
}

func (c *scriptConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *scriptConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *scriptConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *scriptConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}
