package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/golang-sql/sqlexp"
	mssql "github.com/microsoft/go-mssqldb"
	"sqlterm/internal/result"
	"strings"
	"time"
)

type Event struct {
	Kind      string
	Set, Rows int
	Token     PageToken
	Message   string
	Duration  time.Duration
	Store     *result.Store
}

// Run streams events synchronously. The receiver owns each emitted Store.
func (s *Session) Run(ctx context.Context, statements []Bound, emit func(Event)) error {
	start := time.Now()
	set := 0
	paging, err := beginPaging(ctx)
	if err != nil {
		return err
	}
	defer paging.finish()
	defer func() { s.drainMessages(emit); emit(Event{Kind: "done", Duration: time.Since(start)}) }()
	for _, st := range statements {
		if e := ctx.Err(); e != nil {
			return e
		}
		if s.Adapter.Engine() == "sqlserver" {
			if e := s.runSQLServer(ctx, st, emit, &set, paging); e != nil {
				return e
			}
			continue
		}
		if isExec(st.SQL) {
			r, e := s.Conn.ExecContext(ctx, st.SQL, st.Args...)
			if e != nil {
				return e
			}
			if n, e := r.RowsAffected(); e == nil {
				emit(Event{Kind: "message", Message: fmt.Sprintf("%d affected rows", n)})
			}
			continue
		}
		rows, e := s.Conn.QueryContext(ctx, st.SQL, st.Args...)
		if e != nil {
			return e
		}
		e = func() error {
			defer rows.Close()
			for {
				if e := s.scanSet(ctx, rows, emit, &set, paging); e != nil {
					return e
				}
				if !rows.NextResultSet() {
					return rows.Err()
				}
			}
		}()
		if e != nil {
			return e
		}
	}
	return nil
}
func (s *Session) drainMessages(emit func(Event)) {
	if s.Messages == nil {
		return
	}
	for {
		select {
		case m := <-s.Messages:
			emit(Event{Kind: "message", Message: m})
		default:
			return
		}
	}
}
func (s *Session) scanSet(ctx context.Context, rows *sql.Rows, emit func(Event), set *int, paging *pageRun) error {
	cols, e := rows.Columns()
	if e != nil {
		return e
	}
	if len(cols) == 0 {
		return nil
	}
	guidColumns := make([]bool, len(cols))
	if s.Adapter.Engine() == "sqlserver" {
		types, err := rows.ColumnTypes()
		if err != nil {
			return err
		}
		for i, column := range types {
			guidColumns[i] = strings.EqualFold(column.DatabaseTypeName(), "UNIQUEIDENTIFIER")
		}
	}
	store, e := result.New(cols)
	if e != nil {
		return e
	}
	idx := *set
	*set++
	emit(Event{Kind: "columns", Set: idx, Store: store})
	vals := make([]any, len(cols))
	dest := make([]any, len(cols))
	for i := range vals {
		dest[i] = &vals[i]
	}
	count := 0
	blockRows := 0
	for {
		if paging != nil && blockRows == paging.blockSize() {
			if e = store.Flush(); e != nil {
				return e
			}
			emit(Event{Kind: "rows", Set: idx, Rows: count})
			if e = paging.wait(ctx, idx, count, emit); e != nil {
				return e
			}
			blockRows = 0
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		if !rows.Next() {
			break
		}
		if e = rows.Scan(dest...); e != nil {
			return e
		}
		r := make([]result.Cell, len(cols))
		for i, v := range vals {
			if guidColumns[i] {
				if raw, ok := v.([]byte); ok {
					// SQL Server encodes the first three UUID groups little-endian.
					// Metadata, not byte length, distinguishes GUIDs from binary data.
					var id mssql.UniqueIdentifier
					if err := id.Scan(raw); err != nil {
						return fmt.Errorf("column %q: %w", cols[i], err)
					}
					v = id.String()
				}
			}
			r[i] = cell(v)
		}
		if e = store.Append(r); e != nil {
			return e
		}
		count++
		blockRows++
		if paging == nil && count%256 == 0 {
			if e = store.Flush(); e != nil {
				return e
			}
			emit(Event{Kind: "rows", Set: idx, Rows: count})
		}
	}
	if e = store.Flush(); e != nil {
		return e
	}
	emit(Event{Kind: "rows", Set: idx, Rows: count})
	return rows.Err()
}
func (s *Session) runSQLServer(parent context.Context, st Bound, emit func(Event), set *int, paging *pageRun) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	messages := &sqlexp.ReturnMessage{}
	args := append(append([]any{}, st.Args...), messages)
	rows, e := s.Conn.QueryContext(ctx, st.SQL, args...)
	if e != nil {
		return e
	}
	defer func() { cancel(); rows.Close() }()
	for {
		if e = ctx.Err(); e != nil {
			return e
		}
		switch m := messages.Message(ctx).(type) {
		case sqlexp.MsgNext:
			if e = s.scanSet(ctx, rows, emit, set, paging); e != nil {
				return e
			}
		case sqlexp.MsgNextResultSet:
			if !rows.NextResultSet() {
				return rows.Err()
			}
		case sqlexp.MsgRowsAffected:
			emit(Event{Kind: "message", Message: fmt.Sprintf("%d affected rows", m.Count)})
		case sqlexp.MsgNotice:
			emit(Event{Kind: "message", Message: m.Message.String()})
		case sqlexp.MsgError:
			return m.Error
		}
	}
}
func cell(v any) result.Cell {
	if v == nil {
		return result.Cell{Null: true}
	}
	switch x := v.(type) {
	case []byte:
		return result.Cell{Text: string(x)}
	case time.Time:
		return result.Cell{Text: x.Format(time.RFC3339Nano)}
	default:
		return result.Cell{Text: fmt.Sprint(v)}
	}
}
func (s *Session) RunInvocation(ctx context.Context, inv Invocation, emit func(Event)) error {
	for _, b := range inv.Before {
		if _, e := s.Conn.ExecContext(ctx, b.SQL, b.Args...); e != nil {
			return e
		}
	}
	statements := []Bound{{inv.SQL, inv.Args}}
	if inv.After != "" {
		statements = append(statements, Bound{SQL: inv.After})
	}
	return s.Run(ctx, statements, emit)
}
