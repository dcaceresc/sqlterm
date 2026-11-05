package db

import (
	"strings"
	"testing"
)

func TestInvokeSQLServerTypedOutputs(t *testing.T) {
	a := NewAdapter("sqlserver")
	params := []Parameter{{Name: "@text", Type: "[nvarchar](max)", Direction: "IN"}, {Name: "@total", Type: "[decimal](18,4)", Direction: "INOUT"}, {Name: "@optional", Type: "[int]", Direction: "IN", Default: true}}
	payload := "'; DROP TABLE x; --"
	inv, e := a.Invoke(Object{Schema: "dbo", Name: "report", Kind: "procedure"}, params, []Value{{Text: payload}, {Null: true}, {Omit: true}})
	if e != nil {
		t.Fatal(e)
	}
	for _, part := range []string{"[nvarchar](max)", "[decimal](18,4)", "OUTPUT", "RETURN_STATUS", "EXEC @sqlterm_return = [dbo].[report]"} {
		if !strings.Contains(inv.SQL, part) {
			t.Errorf("missing %s in %s", part, inv.SQL)
		}
	}
	if strings.Contains(inv.SQL, payload) || len(inv.Args) != 2 || inv.Args[0] != payload || inv.Args[1] != nil {
		t.Fatalf("unsafe or incorrect binding: %#v", inv)
	}
	if strings.Contains(inv.SQL, "@optional") {
		t.Fatal("omitted parameter included")
	}
}
func TestInvokePostgresOverloadAndOut(t *testing.T) {
	a := NewAdapter("postgres")
	params := []Parameter{{Name: "a", Type: "integer", Direction: "IN"}, {Name: "result", Type: "text", Direction: "OUT"}, {Name: "b", Type: "text", Direction: "IN", Default: true}}
	inv, e := a.Invoke(Object{Schema: "public", Name: "f", Kind: "function"}, params, []Value{{Text: "7"}, {Null: true}, {Omit: true}})
	if e != nil {
		t.Fatal(e)
	}
	if inv.SQL != `SELECT * FROM "public"."f"($1::integer)` || len(inv.Args) != 1 {
		t.Fatalf("bad call: %#v", inv)
	}
	params[0].Default = true
	if _, e = a.Invoke(Object{Kind: "function"}, params, []Value{{Omit: true}, {}, {Text: "x"}}); e == nil {
		t.Fatal("accepted non-trailing omission")
	}
}
func TestInvokeMySQLSessionOutputs(t *testing.T) {
	inv, e := NewAdapter("mysql").Invoke(Object{Schema: "app", Name: "p", Kind: "procedure"}, []Parameter{{Name: "a", Type: "int", Direction: "IN"}, {Name: "b", Type: "int", Direction: "INOUT"}}, []Value{{Text: "1"}, {Text: "2"}})
	if e != nil {
		t.Fatal(e)
	}
	if inv.SQL != "CALL `app`.`p`(?, @sqlterm_out_1)" || len(inv.Before) != 1 || inv.Before[0].Args[0] != "2" || inv.After != "SELECT @sqlterm_out_1 AS `b`" {
		t.Fatalf("bad call: %#v", inv)
	}
}
func TestPrepareDefinition(t *testing.T) {
	o := Object{Schema: "dbo", Name: "p", Kind: "procedure"}
	plan, e := PrepareDefinition(NewAdapter("sqlserver"), &o, "-- comment\nCREATE PROCEDURE [dbo].[p] AS SELECT 1;")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(plan[0].SQL, "ALTER PROCEDURE [dbo].[p]") {
		t.Fatal(plan)
	}
	if _, e = PrepareDefinition(NewAdapter("sqlserver"), &o, "CREATE PROC dbo.other AS SELECT 1"); e == nil {
		t.Fatal("accepted changed target")
	}
	o.Schema = "app"
	plan, e = PrepareDefinition(NewAdapter("mysql"), &o, "CREATE DEFINER=`root`@`%` PROCEDURE `p`() BEGIN SELECT 1; END")
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 2 || plan[0].SQL != "DROP PROCEDURE `app`.`p`" || !strings.Contains(plan[1].SQL, "PROCEDURE `app`.`p`()") {
		t.Fatalf("bad replacement: %#v", plan)
	}
	if _, e = PrepareDefinition(NewAdapter("postgres"), nil, "SELECT 1"); e == nil {
		t.Fatal("accepted non-definition")
	}
}
func TestMySQLViewRoundTripPreservesOptions(t *testing.T) {
	o := Object{Schema: "app", Name: "v", Kind: "view"}
	plan, e := PrepareDefinition(NewAdapter("mysql"), &o, "CREATE ALGORITHM=UNDEFINED DEFINER=`root`@`%` SQL SECURITY DEFINER VIEW `v` AS SELECT 1 AS a")
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 1 || !strings.Contains(plan[0].SQL, "CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=`root`@`%` SQL SECURITY DEFINER VIEW `app`.`v`") {
		t.Fatal(plan)
	}
	o.Kind = "procedure"
	o.Name = "p"
	if _, e = PrepareDefinition(NewAdapter("mysql"), &o, "ALTER PROCEDURE app.p() BEGIN SELECT 1; END"); e == nil {
		t.Fatal("accepted ALTER after a destructive drop")
	}
}
