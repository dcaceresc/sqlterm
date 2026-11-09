package db

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"sqlterm/internal/result"
)

func guidWireBytes() []byte {
	return []byte{0x67, 0x45, 0x23, 0x01, 0xab, 0x89, 0xef, 0xcd, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
}

func TestRunSQLServerUniqueIdentifier(t *testing.T) {
	wire := guidWireBytes()
	binary := []byte("0123456789abcdef")
	timestamp := time.Date(2026, 1, 2, 3, 4, 5, 123, time.UTC)
	p := &pagingPlan{
		columns: [][]string{{"id", "binary", "text", "null_id", "created", "count"}},
		types:   [][]string{{"UNIQUEIDENTIFIER", "VARBINARY", "NVARCHAR", "UNIQUEIDENTIFIER", "DATETIME2", "INT"}},
		sets:    [][][]driver.Value{{{wire, binary, []byte("café"), nil, timestamp, int64(42)}, {make([]byte, 16), []byte{}, []byte(""), nil, timestamp, int64(0)}}},
	}
	session, _ := newPagingSession(t, "sqlserver", map[string][]*pagingPlan{"SELECT identifiers": {p}})
	var store *result.Store
	done := false
	err := session.Run(context.Background(), []Bound{{SQL: "SELECT identifiers"}}, func(ev Event) {
		if ev.Store != nil {
			store = ev.Store
		}
		if ev.Kind == "done" {
			done = true
		}
	})
	if store != nil {
		defer store.Close()
	}
	if err != nil || store == nil || !done {
		t.Fatalf("Run: %v, store=%v done=%v", err, store, done)
	}
	row, err := store.Row(0)
	if err != nil {
		t.Fatal(err)
	}
	if row[0].Text != "01234567-89AB-CDEF-0123-456789ABCDEF" || row[0].Null {
		t.Fatalf("GUID rendered as %q", row[0].Text)
	}
	if row[1].Text != string(binary) || row[2].Text != "café" || !row[3].Null || row[4].Text != timestamp.Format(time.RFC3339Nano) || row[5].Text != "42" {
		t.Fatalf("non-GUID values changed: %#v", row)
	}
	row, err = store.Row(1)
	if err != nil || row[0].Text != "00000000-0000-0000-0000-000000000000" || row[1].Text != "" || row[1].Null || row[2].Text != "" || row[2].Null || !row[3].Null {
		t.Fatalf("zero/null/empty values: %#v %v", row, err)
	}
	if store.Count() != 2 || p.closeCalls.Load() != 1 {
		t.Fatal("row count or cursor cleanup changed")
	}
}

func TestRunLeavesOtherBinaryColumnsAndEnginesUnchanged(t *testing.T) {
	for _, tt := range []struct{ engine, typ string }{{"sqlserver", "VARBINARY"}, {"sqlserver", ""}, {"postgres", "UNIQUEIDENTIFIER"}, {"mysql", "UNIQUEIDENTIFIER"}} {
		t.Run(tt.engine+"/"+tt.typ, func(t *testing.T) {
			wire := []byte("0123456789abcdef")
			p := &pagingPlan{columns: [][]string{{"value"}}, types: [][]string{{tt.typ}}, sets: [][][]driver.Value{{{wire}, {nil}}}}
			session, _ := newPagingSession(t, tt.engine, map[string][]*pagingPlan{"SELECT binary": {p}})
			var store *result.Store
			err := session.Run(context.Background(), []Bound{{SQL: "SELECT binary"}}, func(ev Event) {
				if ev.Store != nil {
					store = ev.Store
				}
			})
			if store != nil {
				defer store.Close()
			}
			if err != nil || store == nil {
				t.Fatalf("Run: %v", err)
			}
			row, err := store.Row(0)
			if err != nil || row[0].Text != string(wire) || row[0].Null {
				t.Fatalf("binary reinterpreted: %#v %v", row, err)
			}
			row, err = store.Row(1)
			if err != nil || !row[0].Null {
				t.Fatalf("NULL lost: %#v %v", row, err)
			}
		})
	}
}

func TestRunInvocationPagedUniqueIdentifiersAcrossResultSets(t *testing.T) {
	binary := []byte("0123456789abcdef")
	p := &pagingPlan{
		columns: [][]string{{"id", "binary"}, {"binary", "id"}},
		types:   [][]string{{"UNIQUEIDENTIFIER", "VARBINARY"}, {"VARBINARY", "uniqueidentifier"}},
		sets:    [][][]driver.Value{{{guidWireBytes(), binary}}, {{binary, make([]byte, 16)}}},
	}
	session, state := newPagingSession(t, "sqlserver", map[string][]*pagingPlan{"EXEC identifiers": {p}})
	pager, err := NewPager(1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var stores []*result.Store
	defer func() {
		for _, store := range stores {
			store.Close()
		}
	}()
	pauses := 0
	err = session.RunInvocation(WithPager(ctx, pager), Invocation{SQL: "EXEC identifiers", Before: []Bound{{SQL: "SET fixture"}}}, func(ev Event) {
		if ev.Store != nil {
			stores = append(stores, ev.Store)
		}
		if ev.Kind == "paused" {
			pauses++
			if ev.Set != pauses-1 || ev.Rows != 1 {
				t.Errorf("unexpected pause: set=%d rows=%d", ev.Set, ev.Rows)
			}
			if !pager.Request(ev.Token) {
				t.Error("could not resume GUID result")
			}
		}
	})
	if err != nil || len(stores) != 2 || pauses != 2 {
		t.Fatalf("invocation err=%v sets=%d pauses=%d", err, len(stores), pauses)
	}
	first, err := stores[0].Row(0)
	if err != nil || first[0].Text != "01234567-89AB-CDEF-0123-456789ABCDEF" || first[1].Text != string(binary) {
		t.Fatalf("first result: %#v %v", first, err)
	}
	second, err := stores[1].Row(0)
	if err != nil || second[0].Text != string(binary) || second[1].Text != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("second result metadata: %#v %v", second, err)
	}
	if p.nextCalls.Load() != 4 || p.closeCalls.Load() != 1 {
		t.Fatal("paging or cursor cleanup changed")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 1 || state.execs[0] != "SET fixture" || len(state.queries) != 1 || state.queries[0] != "EXEC identifiers" {
		t.Fatal("invocation ordering changed")
	}
}

func TestRunSQLServerRejectsMalformedUniqueIdentifier(t *testing.T) {
	p := &pagingPlan{columns: [][]string{{"id"}}, types: [][]string{{"UNIQUEIDENTIFIER"}}, sets: [][][]driver.Value{{{[]byte("invalid")}}}}
	session, _ := newPagingSession(t, "sqlserver", map[string][]*pagingPlan{"SELECT malformed": {p}})
	var store *result.Store
	done := false
	err := session.Run(context.Background(), []Bound{{SQL: "SELECT malformed"}}, func(ev Event) {
		if ev.Store != nil {
			store = ev.Store
		}
		if ev.Kind == "done" {
			done = true
		}
	})
	if store != nil {
		defer store.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "invalid UniqueIdentifier length") {
		t.Fatalf("malformed GUID accepted: %v", err)
	}
	if store == nil || store.Count() != 0 || !done || p.closeCalls.Load() != 1 {
		t.Fatal("conversion error appended a row or left cursor open")
	}
}
