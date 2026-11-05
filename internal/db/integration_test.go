package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sqlterm/internal/config"
	"sqlterm/internal/result"
	"strings"
	"testing"
	"time"
)

// Integration tests only run against explicitly provided disposable databases.
func TestIntegration(t *testing.T) {
	for _, engine := range []string{"sqlserver", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			prefix := "SQLTERM_TEST_" + strings.ToUpper(engine)
			raw := os.Getenv(prefix + "_PROFILE")
			if raw == "" {
				t.Skip("set " + prefix + "_PROFILE and " + prefix + "_PASSWORD for a disposable test database")
			}
			var p config.Profile
			if e := json.Unmarshal([]byte(raw), &p); e != nil {
				t.Fatal(e)
			}
			p.Engine = engine
			if p.Name == "" {
				p.Name = "integration"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			s, e := Connect(ctx, p, os.Getenv(prefix+"_PASSWORD"))
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			name := fmt.Sprintf("sqlterm_test_%d", time.Now().UnixNano())
			schema := "public"
			if engine == "sqlserver" {
				schema = "dbo"
			}
			if engine == "mysql" {
				schema = p.Database
			}
			o := Object{Schema: schema, Name: name, Kind: "procedure"}
			a := s.Adapter
			target := a.Quote(schema) + "." + a.Quote(name)
			var create string
			// FIRST_VALUE is reserved in MySQL 8. Quote result aliases using the
			// engine's identifier rules, just like schema and procedure names.
			firstAlias, secondAlias := a.Quote("first_value"), a.Quote("second_value")
			switch engine {
			case "sqlserver":
				create = "CREATE PROCEDURE " + target + " @input int, @output int OUTPUT, @optional int=7 AS BEGIN SET NOCOUNT ON; SET @output=@input+1; SELECT @input AS " + firstAlias + "; SELECT @optional AS " + secondAlias + "; RETURN 9; END;"
			case "mysql":
				create = "CREATE PROCEDURE " + target + "(IN p_input INT, INOUT p_output INT) BEGIN SET p_output=p_input+1; SELECT p_input AS " + firstAlias + "; SELECT p_output AS " + secondAlias + "; END"
			default:
				create = "CREATE PROCEDURE " + target + "(IN p_input integer, INOUT p_output integer) LANGUAGE plpgsql AS $body$ BEGIN p_output:=p_input+1; END; $body$;"
			}
			if _, e = s.Conn.ExecContext(ctx, create); e != nil {
				t.Fatal(e)
			}
			defer func() {
				cleanup, c := context.WithTimeout(context.Background(), 10*time.Second)
				defer c()
				drop := "DROP PROCEDURE " + target
				if engine == "postgres" {
					drop += "(integer,integer)"
				}
				cleanupSession, err := Connect(cleanup, p, os.Getenv(prefix+"_PASSWORD"))
				if err != nil {
					t.Error("cleanup connect:", err)
					return
				}
				defer cleanupSession.Close()
				if _, e := cleanupSession.Conn.ExecContext(cleanup, drop); e != nil {
					t.Error("cleanup:", e)
				}
			}()
			objects, e := a.Catalog(ctx, s.Conn)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, item := range objects {
				if item.Name == name {
					o = item
					found = true
					break
				}
			}
			if !found {
				t.Fatal("new procedure missing from catalog")
			}
			definition, e := a.Definition(ctx, s.Conn, o)
			if e != nil || !strings.Contains(definition, name) {
				t.Fatal("definition:", definition, e)
			}
			parameters, e := a.Parameters(ctx, s.Conn, o)
			if e != nil {
				t.Fatal(e)
			}
			values := make([]Value, len(parameters))
			for i, p := range parameters {
				switch {
				case p.Default:
					values[i] = Value{Omit: true}
				case p.Direction == "IN":
					values[i] = Value{Text: "41"}
				default:
					values[i] = Value{Null: true}
				}
			}
			inv, e := a.Invoke(o, parameters, values)
			if e != nil {
				t.Fatal(e)
			}
			var sets []*result.Store
			defer func() {
				for _, set := range sets {
					set.Close()
				}
			}()
			emit := func(ev Event) {
				if ev.Store != nil {
					sets = append(sets, ev.Store)
				}
			}
			if e = s.RunInvocation(ctx, inv, emit); e != nil {
				t.Fatal(e)
			}
			if len(sets) == 0 {
				t.Fatal("no invocation results")
			}
			last := sets[len(sets)-1]
			row, e := last.Row(0)
			if e != nil {
				t.Fatal(e)
			}
			outputOK := false
			for _, v := range row {
				if v.Text == "42" {
					outputOK = true
				}
			}
			if !outputOK {
				t.Fatalf("missing output 42: %#v", row)
			}
			if engine == "sqlserver" {
				if len(sets) != 3 || row[len(row)-1].Text != "9" {
					t.Fatalf("result/return status mismatch: %d %#v", len(sets), row)
				}
			}
			if engine == "mysql" && len(sets) != 3 {
				t.Fatalf("missing mysql result sets: %d", len(sets))
			}
			if engine == "mysql" || engine == "sqlserver" {
				secondValue := "42"
				if engine == "sqlserver" {
					secondValue = "7"
				}
				for i, expected := range []struct{ column, value string }{{"first_value", "41"}, {"second_value", secondValue}} {
					set := sets[i]
					if len(set.Columns) != 1 || set.Columns[0] != expected.column {
						t.Fatalf("result set %d aliases: got %v, want %s", i+1, set.Columns, expected.column)
					}
					values, err := set.Row(0)
					if err != nil || len(values) != 1 || values[0].Null || values[0].Text != expected.value {
						t.Fatalf("result set %d values: got %#v (%v), want %s", i+1, values, err, expected.value)
					}
				}
			}
			// Read and reapply the existing routine using its engine's replacement strategy.
			plan, e := PrepareDefinition(a, &o, definition)
			if e != nil {
				t.Fatal(e)
			}
			for _, st := range plan {
				if _, e = s.Conn.ExecContext(ctx, st.SQL); e != nil {
					t.Fatal("reapply:", e)
				}
			}
			// Ordinary scripts and affected-row reporting must remain usable afterward.
			setsBefore := len(sets)
			if e = s.Run(ctx, []Bound{{SQL: "SELECT 1 AS value"}, {SQL: "SELECT NULL AS empty_value"}}, emit); e != nil {
				t.Fatal(e)
			}
			if len(sets) != setsBefore+2 {
				t.Fatal("script result sets missing")
			}
			slow := "SELECT pg_sleep(10)"
			if engine == "mysql" {
				slow = "SELECT SLEEP(10)"
			}
			if engine == "sqlserver" {
				slow = "WAITFOR DELAY '00:00:10'; SELECT 1"
			}
			short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
			defer stop()
			if e = s.Run(short, []Bound{{SQL: slow}}, emit); e == nil {
				t.Fatal("server query cancellation ignored")
			}
		})
	}
}
