package completion

import (
	"sqlterm/internal/db"
	"strings"
	"testing"
)

func fixture() ([]db.Object, map[string][]string) {
	objects := []db.Object{{Schema: "dbo", Name: "Customers", Kind: "table", ID: "1"}, {Schema: "dbo", Name: "Orders", Kind: "table", ID: "2"}, {Schema: "dbo", Name: "Report", Kind: "procedure", Signature: "@id int"}, {Schema: "sales", Name: "Reports", Kind: "view", ID: "3"}}
	return objects, map[string][]string{"dbo.Customers": {"ID", "Name", "City"}, "dbo.Orders": {"ID", "Total"}}
}
func analyzeMarker(sql, engine string) Context {
	pos := strings.IndexByte(sql, '|')
	return Analyze(strings.Replace(sql, "|", "", 1), pos, engine)
}
func TestAliasColumnSuggestionsAndReplacementRange(t *testing.T) {
	objects, columns := fixture()
	sql := "SELECT c.Na| FROM [dbo].[Customers] AS c JOIN dbo.Orders o ON c.ID=o.ID"
	ctx := analyzeMarker(sql, "sqlserver")
	items := Suggestions(ctx, db.NewAdapter("sqlserver"), objects, columns)
	if len(items) != 1 || items[0].Text != "[Name]" || strings.Join(ctx.Qualifier, ".") != "c" {
		t.Fatalf("wrong alias suggestions: %#v %#v", ctx, items)
	}
	original := strings.Replace(sql, "|", "", 1)
	got := original[:ctx.Start] + items[0].Text + original[ctx.End:]
	if !strings.Contains(got, "c.[Name] FROM") {
		t.Fatal(got)
	}
	ctx = analyzeMarker("SELECT o.| FROM dbo.Customers c JOIN dbo.Orders o ON c.ID=o.ID", "sqlserver")
	items = Suggestions(ctx, db.NewAdapter("sqlserver"), objects, columns)
	if len(items) != 2 || items[1].Name != "Total" {
		t.Fatal(items)
	}
}
func TestSchemaRoutineAndDialectSuggestions(t *testing.T) {
	objects, columns := fixture()
	for _, c := range []struct{ sql, engine, want string }{{"SELECT * FROM dbo.Cu|", "sqlserver", "[Customers]"}, {"EXEC dbo.Re|", "sqlserver", "[Report]"}, {"SELECT * FROM sales.Re|", "postgres", `"Reports"`}, {"SEL|", "mysql", "SELECT"}} {
		ctx := analyzeMarker(c.sql, c.engine)
		items := Suggestions(ctx, db.NewAdapter(c.engine), objects, columns)
		found := false
		for _, item := range items {
			if item.Text == c.want {
				found = true
			}
		}
		if !found {
			t.Fatal(c.sql, items)
		}
	}
	for _, kw := range Keywords("sqlserver") {
		if kw == "LIMIT" || kw == "RETURNING" {
			t.Fatal("wrong dialect keyword")
		}
	}
}
func TestDoNotCompleteStringsAndComments(t *testing.T) {
	for _, c := range []struct{ sql, engine string }{{"SELECT 'SEL|ECT'", "sqlserver"}, {"-- SEL|ECT", "sqlserver"}, {"/* SELECT /* nested */ NA| */", "postgres"}, {"SELECT $$ SEL|ECT $$", "postgres"}, {"# SEL|ECT", "mysql"}, {"SELECT E'it\\'s NA|me'", "postgres"}} {
		if ctx := analyzeMarker(c.sql, c.engine); ctx.InCode {
			t.Fatal("suggestions in literal/comment", c.sql)
		}
	}
	if !analyzeMarker("SELECT * FROM #temp WHERE SE|", "sqlserver").InCode {
		t.Fatal("SQL Server temporary table treated as a comment")
	}
}
func TestQuotedIdentifiersAndUnqualifiedColumns(t *testing.T) {
	objects, columns := fixture()
	ctx := analyzeMarker("SELECT c.[Na|me] FROM [dbo].[Customers] c", "sqlserver")
	items := Suggestions(ctx, db.NewAdapter("sqlserver"), objects, columns)
	if len(items) != 1 {
		t.Fatal(items)
	}
	sql := "SELECT c.[Name] FROM [dbo].[Customers] c"
	if got := sql[:ctx.Start] + items[0].Text + sql[ctx.End:]; got != sql {
		t.Fatal("left duplicate closing quote", got)
	}
	ctx = analyzeMarker("SELECT To| FROM dbo.Orders", "sqlserver")
	items = Suggestions(ctx, db.NewAdapter("sqlserver"), objects, columns)
	if len(items) == 0 || items[0].Name != "Total" {
		t.Fatal("referenced columns not prioritized", items)
	}
}
func TestSQLServerProcedureParameters(t *testing.T) {
	objects, columns := fixture()
	ctx := analyzeMarker("EXEC dbo.Report @i|", "sqlserver")
	items := Suggestions(ctx, db.NewAdapter("sqlserver"), objects, columns)
	if len(items) != 1 || items[0].Text != "@id = " {
		t.Fatal("procedure parameter completion missing", ctx, items)
	}
	ctx = analyzeMarker("EXEC dbo.Report; SELECT @i|", "sqlserver")
	if len(ctx.Routine) != 0 {
		t.Fatal("routine context leaked across statements")
	}
}
func TestStatementKeywordRanksBeforeSimilarTable(t *testing.T) {
	objects, columns := fixture()
	objects = append(objects, db.Object{Schema: "dbo", Name: "SelectAudit", Kind: "table"})
	ctx := analyzeMarker("SE|", "sqlserver")
	items := Suggestions(ctx, db.NewAdapter("sqlserver"), objects, columns)
	if len(items) == 0 || items[0].Text != "SELECT" {
		t.Fatal(items)
	}
}
func TestIncompleteQuoteDoesNotReplaceFollowingSQL(t *testing.T) {
	objects, columns := fixture()
	source := "SELECT * FROM dbo.[Cu| WHERE ID=1"
	ctx := analyzeMarker(source, "sqlserver")
	items := Suggestions(ctx, db.NewAdapter("sqlserver"), objects, columns)
	if len(items) != 1 {
		t.Fatal(items)
	}
	sql := strings.Replace(source, "|", "", 1)
	got := sql[:ctx.Start] + items[0].Text + sql[ctx.End:]
	if got != "SELECT * FROM dbo.[Customers] WHERE ID=1" {
		t.Fatal("completion removed following SQL", got)
	}
}
