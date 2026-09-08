package sqlscript

import (
	"context"
	"fmt"
	"strings"
)

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

// Execute runs every statement using the provided operation. The statement
// index is included in failures so remote migration errors are actionable.
func Execute(ctx context.Context, script string, exec func(context.Context, string) error) error {
	for index, statement := range Split(script) {
		if err := exec(ctx, statement); err != nil {
			return fmt.Errorf("statement %d: %w", index+1, err)
		}
	}
	return nil
}
