package db

import (
	"sqlterm/internal/config"
	"testing"
)

func TestServerProfileDoesNotPersistBootstrapDatabase(t *testing.T) {
	for _, c := range []struct{ engine, want string }{{"sqlserver", "master"}, {"postgres", "postgres"}, {"mysql", ""}} {
		original := config.Profile{Engine: c.engine}
		wire := serverProfile(original)
		if wire.Database != c.want || original.Database != "" {
			t.Fatal(c, wire)
		}
		original.Database = "app"
		if serverProfile(original).Database != "app" {
			t.Fatal("explicit database replaced")
		}
	}
}
