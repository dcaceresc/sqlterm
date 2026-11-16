package db

import (
	"context"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func TestSQLServerCatalogWithoutModernAggregates(t *testing.T) {
	matcher := sqlmock.QueryMatcherFunc(func(want, sql string) error {
		if strings.Contains(strings.ToUpper(sql), "STRING_AGG") || strings.Contains(strings.ToUpper(sql), "WITHIN GROUP") {
			return fmt.Errorf("function not available on legacy SQL Server")
		}
		if !strings.Contains(sql, want) {
			return fmt.Errorf("unexpected metadata query")
		}
		return nil
	})
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := database.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	mock.ExpectQuery("FROM sys.objects").WillReturnRows(sqlmock.NewRows([]string{"schema", "name", "kind", "id", "signature", "scalar"}).AddRow("dbo", "customers", "table", "10", "", false).AddRow("dbo", "report", "procedure", "20", "", false).AddRow("dbo", "calculate", "function", "30", "", true).AddRow("other", "report", "procedure", "40", "", false)).RowsWillBeClosed()
	mock.ExpectQuery("FROM sys.parameters").WillReturnRows(sqlmock.NewRows([]string{"object_id", "name", "type"}).AddRow("20", "@id", "int").AddRow("20", "@año", "nvarchar").AddRow("30", "@value", "special<&type>").AddRow("99", "@removed", "int")).RowsWillBeClosed()
	objects, err := NewAdapter("sqlserver").Catalog(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 4 || objects[0].Signature != "" || objects[1].Signature != "@id int, @año nvarchar" || objects[2].Signature != "@value special<&type>" || !objects[2].Scalar || objects[3].Signature != "" {
		t.Fatalf("incorrect objects or signatures: %#v", objects)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestTableDefinitions(t *testing.T) {
	for _, tc := range []struct {
		engine, schema, name string
	}{
		{"sqlserver", "sales", "Order"},
		{"postgres", "Sales", "Order"},
		{"mysql", "sales", "order"},
	} {
		t.Run(tc.engine, func(t *testing.T) {
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
			a := NewAdapter(tc.engine)
			target := a.Quote(tc.schema) + "." + a.Quote(tc.name)
			want := "CREATE TABLE " + target + " (\n    id int NOT NULL,\n    CONSTRAINT pk PRIMARY KEY (id)\n);"
			if tc.engine == "mysql" {
				mock.ExpectQuery("SHOW CREATE TABLE").WillReturnRows(sqlmock.NewRows([]string{"Table", "Create Table"}).AddRow(tc.name, want)).RowsWillBeClosed()
			} else {
				columns := "FROM sys.columns"
				constraints := "FROM sys.key_constraints"
				indexes := "FROM sys.indexes"
				if tc.engine == "postgres" {
					columns, constraints, indexes = "FROM pg_attribute", "FROM pg_constraint", "FROM pg_index"
				}
				mock.ExpectQuery(columns).WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"definition"}).AddRow("id int NOT NULL")).RowsWillBeClosed()
				mock.ExpectQuery(constraints).WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"definition"}).AddRow("CONSTRAINT pk PRIMARY KEY (id)")).RowsWillBeClosed()
				mock.ExpectQuery(indexes).WithArgs("42").WillReturnRows(sqlmock.NewRows([]string{"definition"})).RowsWillBeClosed()
			}
			got, err := a.Definition(context.Background(), conn, Object{Schema: tc.schema, Name: tc.name, Kind: "table", ID: "42"})
			if err != nil || got != want {
				t.Fatalf("Definition() = %q, %v; want %q", got, err, want)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSQLServerTableOnlyCatalogNeedsNoParameterQuery(t *testing.T) {
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
	mock.ExpectQuery("SELECT s.name").WillReturnRows(sqlmock.NewRows([]string{"schema", "name", "kind", "id", "signature", "scalar"}).AddRow("dbo", "table", "table", "1", "", false)).RowsWillBeClosed()
	objects, err := NewAdapter("sqlserver").Catalog(context.Background(), conn)
	if err != nil || len(objects) != 1 {
		t.Fatal(objects, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
