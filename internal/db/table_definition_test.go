package db

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// Se comprueba el SQL generado, no el evaluador de intercalaciones de SQL Server.
// La regresión completa aún requiere un servidor real con distintas intercalaciones.
func TestSQLServerTableMetadataUsesDatabaseCollation(t *testing.T) {
	fields := map[string][]string{
		"FROM sys.columns":         {"c.name", "t.name", "cc.definition", "c.collation_name", "dc.name", "dc.definition"},
		"FROM sys.key_constraints": {"k.name", "c.name", "c.definition", "f.name", "f.delete_referential_action_desc", "f.update_referential_action_desc"},
		"FROM sys.indexes":         {"i.name", "c.name", "i.filter_definition"},
	}
	matcher := sqlmock.QueryMatcherFunc(func(want, query string) error {
		if !strings.Contains(query, want) {
			return fmt.Errorf("unexpected metadata query")
		}
		for _, field := range fields[want] {
			if !strings.Contains(query, field+" COLLATE DATABASE_DEFAULT") {
				return fmt.Errorf("metadata text %s is not normalized to the database collation", field)
			}
		}
		return nil
	})
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	conn, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	mock.ExpectQuery("FROM sys.columns").WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow("[id] [int] NOT NULL")).RowsWillBeClosed()
	mock.ExpectQuery("FROM sys.key_constraints").WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"ddl"})).RowsWillBeClosed()
	mock.ExpectQuery("FROM sys.indexes").WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"ddl"})).RowsWillBeClosed()
	got, err := NewAdapter("sqlserver").Definition(context.Background(), conn, Object{Schema: "dbo", Name: "example", Kind: "table", ID: "42"})
	want := "CREATE TABLE [dbo].[example] (\n    [id] [int] NOT NULL\n);"
	if err != nil || got != want {
		t.Fatalf("Definition() = %q, %v; want %q", got, err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDefinitionReadErrorsAndExistingObjects(t *testing.T) {
	for _, engine := range []string{"sqlserver", "postgres", "mysql"} {
		for _, kind := range []string{"table", "view", "procedure", "function"} {
			t.Run(engine+"/"+kind, func(t *testing.T) {
				database, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				conn, err := database.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				o := Object{Schema: "app", Name: "item", ID: "42", Kind: kind}
				a := NewAdapter(engine)
				want := "CREATE " + kind + " example"
				if kind == "table" {
					mock.ExpectQuery("SELECT|SHOW").WillReturnError(fmt.Errorf("metadata denied"))
				} else if engine == "mysql" {
					mock.ExpectQuery("SHOW CREATE").WillReturnRows(sqlmock.NewRows([]string{"Name", "Create " + kind}).AddRow(o.Name, want))
				} else {
					mock.ExpectQuery("SELECT").WithArgs(o.ID).WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow(want))
					if engine == "postgres" && kind == "view" {
						want = "CREATE OR REPLACE VIEW \"app\".\"item\" AS\n" + want
					}
				}
				got, err := a.Definition(context.Background(), conn, o)
				if kind == "table" {
					if err == nil || err.Error() != "metadata denied" || got != "" {
						t.Fatalf("partial or hidden error: %q, %v", got, err)
					}
				} else if err != nil || got != want {
					t.Fatalf("existing definition changed: %q, %v", got, err)
				}
				if err = mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestTableDefinitionIndexesAndIncompleteMetadata(t *testing.T) {
	for _, engine := range []string{"sqlserver", "postgres"} {
		for _, stage := range []string{"indexes", "empty", "constraints-error", "indexes-error"} {
			t.Run(engine+"/"+stage, func(t *testing.T) {
				database, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				conn, err := database.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				rows := sqlmock.NewRows([]string{"ddl"})
				if stage != "empty" {
					rows.AddRow("id int NULL")
				}
				mock.ExpectQuery("SELECT").WithArgs("42").WillReturnRows(rows).RowsWillBeClosed()
				if stage != "empty" {
					expect := mock.ExpectQuery("SELECT").WithArgs("42")
					if stage == "constraints-error" {
						expect.WillReturnError(fmt.Errorf("constraints failed"))
					} else {
						expect.WillReturnRows(sqlmock.NewRows([]string{"ddl"})).RowsWillBeClosed()
						expect = mock.ExpectQuery("SELECT").WithArgs("42")
						if stage == "indexes-error" {
							expect.WillReturnError(fmt.Errorf("indexes failed"))
						} else {
							expect.WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow("CREATE INDEX idx ON t (id)")).RowsWillBeClosed()
						}
					}
				}
				a := NewAdapter(engine)
				got, err := a.Definition(context.Background(), conn, Object{Schema: "app", Name: "t", Kind: "table", ID: "42"})
				want := "CREATE TABLE " + a.Quote("app") + "." + a.Quote("t") + " (\n    id int NULL\n);\nCREATE INDEX idx ON t (id);"
				if stage == "indexes" {
					if err != nil || got != want {
						t.Fatalf("%q, %v", got, err)
					}
				} else if err == nil || got != "" {
					t.Fatalf("incomplete metadata returned: %q, %v", got, err)
				}
				if err = mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
