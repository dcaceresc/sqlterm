package db

import (
	"context"
	"database/sql"
	"fmt"
	"sqlterm/internal/config"
	"strings"
	"time"
)

type Object struct {
	Schema, Name, Kind, ID, Signature string
	Scalar                            bool
}

func (o Object) Label() string {
	s := o.Schema + "." + o.Name
	if o.Signature != "" {
		s += "(" + o.Signature + ")"
	}
	return s
}

type Parameter struct {
	Name, Type, Direction string
	Default               bool
}
type Value struct {
	Text       string
	Null, Omit bool
}
type Invocation struct {
	SQL    string
	Args   []any
	Before []Bound
	After  string
}

type Capabilities struct{ ReplaceRoutine, OutputParameters, ReturnStatus bool }
type Adapter interface {
	Engine() string
	Capabilities() Capabilities
	Catalog(context.Context, *sql.Conn) ([]Object, error)
	Definition(context.Context, *sql.Conn, Object) (string, error)
	Parameters(context.Context, *sql.Conn, Object) ([]Parameter, error)
	Columns(context.Context, *sql.Conn, Object) ([]string, error)
	Databases(context.Context, *sql.Conn) ([]string, error)
	Invoke(Object, []Parameter, []Value) (Invocation, error)
	Quote(string) string
}
type Session struct {
	DB       *sql.DB
	Conn     *sql.Conn
	Adapter  Adapter
	Profile  config.Profile
	Messages chan string
}

func (s *Session) Close() {
	if s.Conn != nil {
		_ = s.Conn.Close()
	}
	if s.DB != nil {
		_ = s.DB.Close()
	}
}
func NewAdapter(engine string) Adapter { return &adapter{engine: engine} }

type adapter struct{ engine string }

func (a *adapter) Engine() string { return a.engine }
func (a *adapter) Capabilities() Capabilities {
	return Capabilities{a.engine != "mysql", true, a.engine == "sqlserver"}
}
func (a *adapter) Quote(s string) string {
	switch a.engine {
	case "sqlserver":
		return "[" + strings.ReplaceAll(s, "]", "]]") + "]"
	case "mysql":
		return "`" + strings.ReplaceAll(s, "`", "``") + "`"
	default:
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
}
func (a *adapter) qualified(o Object) string { return a.Quote(o.Schema) + "." + a.Quote(o.Name) }
func queryStrings(ctx context.Context, c *sql.Conn, q string, args ...any) ([]string, error) {
	r, e := c.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	var out []string
	for r.Next() {
		var s string
		if e = r.Scan(&s); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, r.Err()
}
func deadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 20*time.Second)
}
func unsupported(s string) error { return fmt.Errorf("%s is not supported", s) }
