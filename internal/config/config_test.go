package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilePersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "config.json")
	c := Config{Profiles: []Profile{{Name: "local", Engine: "postgres", Host: "localhost", User: "user", PasswordEnv: "SQLTERM_TEST_PASSWORD", TLS: "verify"}}}
	t.Setenv("SQLTERM_TEST_PASSWORD", "never-write-this")
	if e := Save(p, c); e != nil {
		t.Fatal(e)
	}
	got, e := Load(p)
	if e != nil {
		t.Fatal(e)
	}
	if password, ok := got.Profiles[0].Password(); !ok || password != "never-write-this" {
		t.Fatal("environment resolution failed")
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "never-write-this") {
		t.Fatal("persisted password")
	}
	c.History = true
	if e = Save(p, c); e != nil {
		t.Fatalf("cannot replace existing config: %v", e)
	}
	got, e = Load(p)
	if e != nil || !got.History {
		t.Fatal(got, e)
	}
}
func TestRejectDuplicateAndInvalidProfiles(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	profile := Profile{Name: "x", Engine: "mysql", Host: "localhost", User: "x"}
	Save(p, Config{Profiles: []Profile{profile, profile}})
	if _, e := Load(p); e == nil {
		t.Fatal("duplicate accepted")
	}
	profile.TLS = "trust-everything"
	if e := profile.Validate(); e == nil {
		t.Fatal("invalid TLS accepted")
	}
}
func TestConnectionOptionsAndLegacyProfiles(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		p    Profile
		want string
	}{{Profile{}, "verify"}, {Profile{TrustServerCertificate: true}, "insecure"}, {Profile{Encrypt: &yes, TrustServerCertificate: true}, "insecure"}, {Profile{Encrypt: &yes, TLS: "insecure"}, "verify"}, {Profile{Encrypt: &no, TrustServerCertificate: true}, "disable"}, {Profile{TLS: "insecure"}, "insecure"}, {Profile{TLS: "disable"}, "disable"}}
	for _, c := range cases {
		if got := c.p.TLSMode(); got != c.want {
			t.Fatalf("mode %s want %s", got, c.want)
		}
	}
}
func TestDefaultPortsAndOverrides(t *testing.T) {
	for engine, want := range map[string]int{"sqlserver": 1433, "mysql": 3306, "postgres": 5432} {
		p := Profile{Engine: engine}
		if p.EffectivePort() != want || DefaultPort(engine) != want {
			t.Fatal(engine, "wrong default port")
		}
		p.Port = 15000
		if p.EffectivePort() != 15000 {
			t.Fatal(engine, "custom port overwritten")
		}
	}
}
func TestResultBlockSizeLoadValidationAndPersistence(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
		want string
	}{
		{name: "negative", json: `{"result_block_size":-1}`, want: "result_block_size must be between 1 and 100000"},
		{name: "too large", json: `{"result_block_size":100001}`, want: "result_block_size must be between 1 and 100000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tc.json), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load error = %v, want %q", err, tc.want)
			}
		})
	}

	for _, size := range []int{1, 100000} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"result_block_size":%d}`, size)), 0600); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(path)
		if err != nil || loaded.ResultBlockSize != size {
			t.Fatalf("valid boundary %d rejected: %+v, %v", size, loaded, err)
		}
	}

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"history":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"result_block_size":1000`) {
		t.Fatalf("missing/default block size was not normalized: %s", encoded)
	}
}

func TestFolderNormalizationAndDuplicateNames(t *testing.T) {
	normalized, err := NormalizeFolder(` DEV\ Project / QA `)
	if err != nil || normalized != "DEV/Project/QA" {
		t.Fatal(normalized, err)
	}
	for _, s := range []string{"DEV/../PROD", "DEV//QA", "DEV/\nQA"} {
		if _, err = NormalizeFolder(s); err == nil {
			t.Fatal("invalid folder accepted", s)
		}
	}
	path := filepath.Join(t.TempDir(), "config.json")
	dev := Profile{Name: "Shared", Folder: "DEV", Engine: "sqlserver", Host: "localhost", User: "u"}
	prod := dev
	prod.Folder = "PROD"
	c := Config{Theme: "nord", Folders: []string{"DEV", "PROD"}, Profiles: []Profile{dev, prod}}
	if err = Save(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.Theme != "nord" || len(loaded.Profiles) != 2 {
		t.Fatal("names in different folders must coexist", err)
	}
	c.Profiles[1].Folder = "DEV"
	Save(path, c)
	if _, err = Load(path); err == nil {
		t.Fatal("duplicate within folder accepted")
	}
}
