package db

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"sqlterm/internal/result"
	"testing"
	"time"
)

func TestRunStreamsMultipleSetsAndAffectedRows(t *testing.T) {
	database, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer database.Close()
	conn, e := database.Conn(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	s := &Session{Conn: conn, Adapter: NewAdapter("postgres")}
	mock.ExpectExec("UPDATE").WithArgs("new").WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a", "b"}).AddRow(nil, "").AddRow("ñ", "line\nbreak"), sqlmock.NewRows([]string{"c"}).AddRow(42))
	var stores []*result.Store
	defer func() {
		for _, s := range stores {
			s.Close()
		}
	}()
	affected := false
	e = s.Run(context.Background(), []Bound{{SQL: "UPDATE t SET name=$1", Args: []any{"new"}}, {SQL: "SELECT results"}}, func(ev Event) {
		if ev.Store != nil {
			stores = append(stores, ev.Store)
		}
		if ev.Message == "3 affected rows" {
			affected = true
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	if len(stores) != 2 || !affected {
		t.Fatalf("sets=%d affected=%v", len(stores), affected)
	}
	r, e := stores[0].Row(0)
	if e != nil || !r[0].Null || r[1].Null || r[1].Text != "" {
		t.Fatalf("NULL lost: %#v %v", r, e)
	}
	if stores[1].Count() != 1 {
		t.Fatal("missing second set")
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestRunCancellationAndErrors(t *testing.T) {
	database, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer database.Close()
	conn, _ := database.Conn(context.Background())
	defer conn.Close()
	s := &Session{Conn: conn, Adapter: NewAdapter("postgres")}
	mock.ExpectQuery("SELECT").WillDelayFor(time.Second).WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if e = s.Run(ctx, []Bound{{SQL: "SELECT slow"}}, func(Event) {}); e == nil {
		t.Fatal("ignored cancellation")
	}
	mock.ExpectQuery("SELECT").WillReturnError(errors.New("permission denied"))
	if e = s.Run(context.Background(), []Bound{{SQL: "SELECT denied"}}, func(Event) {}); e == nil {
		t.Fatal("ignored permission error")
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
