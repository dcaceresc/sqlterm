package db

import (
	"strings"
	"testing"
)

func TestSplitScript(t *testing.T) {
	cases := []struct {
		name, engine, sql string
		count             int
	}{
		{"server batches", "sqlserver", "SELECT ';GO';\nGO 2 -- repeat\nSELECT 2; SELECT 3;", 3},
		{"server quoted go", "sqlserver", "SELECT 'hello\nGO\nworld';\n/*\nGO\n*/\nGO\nSELECT [semi;colon];", 2},
		{"mysql routine", "mysql", "DELIMITER $$\nCREATE PROCEDURE p() BEGIN SELECT ';'; SELECT 2; END$$\nDELIMITER ;\nCALL p();", 2},
		{"mysql escape", "mysql", `SELECT 'it\'s;fine'; SELECT 2;`, 2},
		{"mysql subtraction", "mysql", "SELECT 1--2; SELECT 3;", 2},
		{"pg dollar body", "postgres", `CREATE FUNCTION f() RETURNS void AS $fn$ BEGIN PERFORM ';'; END; $fn$ LANGUAGE plpgsql; SELECT 1;`, 2},
		{"pg escapes", "postgres", `SELECT E'it\'s;fine'; SELECT 'a'';b';`, 2},
		{"pg identifier dollar", "postgres", `SELECT foo$bar$ FROM x; SELECT 2;`, 2},
		{"nested comments", "postgres", "/* outer /* inner */ ; */ SELECT 1;-- trailing", 1},
		{"comment only", "mysql", "-- comment\n# x\n/* x */", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, e := SplitScript(c.engine, c.sql)
			if e != nil {
				t.Fatal(e)
			}
			if len(got) != c.count {
				t.Fatalf("got %d statements, want %d: %#v", len(got), c.count, got)
			}
			for _, s := range got {
				if strings.HasPrefix(strings.ToUpper(s), "DELIMITER ") {
					t.Fatal("client directive leaked")
				}
			}
		})
	}
	for _, sql := range []string{"SELECT 'unterminated", "SELECT $$unterminated", "/* unclosed"} {
		if _, e := SplitScript("postgres", sql); e == nil {
			t.Errorf("accepted %q", sql)
		}
	}
	if _, e := SplitScript("sqlserver", "SELECT 1\nGO 1001"); e == nil {
		t.Fatal("unbounded GO count")
	}
}
func TestParameterDefaults(t *testing.T) {
	d := parameterDefaults("CREATE PROC [dbo].[p] @a decimal(10,2)=1.2, @b nvarchar(80) = 'AS, @c=4', @c int /* = 9 */ AS SELECT @c=5;")
	if !d["@a"] || !d["@b"] || d["@c"] {
		t.Fatalf("wrong defaults: %#v", d)
	}
}
