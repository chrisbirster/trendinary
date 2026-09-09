package sqlscript

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Split separates a SQL migration script into individual statements without
// splitting semicolons that occur inside quoted strings/identifiers or SQL
// comments. libSQL's remote protocol accepts one statement per request, while
// local SQLite historically allowed multiple statements in one Exec call.
func Split(script string) []string {
	var statements []string
	start := 0
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(script); i++ {
		c := script[i]
		next := byte(0)
		if i+1 < len(script) {
			next = script[i+1]
		}

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if c == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inSingle {
			if c == '\'' {
				if next == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			if c == '"' {
				if next == '"' {
					i++
					continue
				}
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if c == '`' {
				inBacktick = false
			}
			continue
		}

		switch {
		case c == '-' && next == '-':
			inLineComment = true
			i++
		case c == '/' && next == '*':
			inBlockComment = true
			i++
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '`':
			inBacktick = true
		case c == ';':
			if statement := strings.TrimSpace(script[start:i]); statement != "" {
				statements = append(statements, statement)
			}
			start = i + 1
		}
	}
	if statement := strings.TrimSpace(script[start:]); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

// Execute runs every statement using a database/sql compatible executor. The
// statement index is included in failures so remote migration errors are easy
// to diagnose.
func Execute(ctx context.Context, execer Execer, script string) error {
	for index, statement := range Split(script) {
		if _, err := execer.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("statement %d: %w", index+1, err)
		}
	}
	return nil
}
