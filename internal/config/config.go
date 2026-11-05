package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type Profile struct {
	Name                   string `json:"name"`
	Folder                 string `json:"folder,omitempty"`
	Engine                 string `json:"engine"`
	Host                   string `json:"host"`
	Port                   int    `json:"port,omitempty"`
	Database               string `json:"database"`
	User                   string `json:"user"`
	PasswordEnv            string `json:"password_env,omitempty"`
	CredentialID           string `json:"credential_id,omitempty"`
	Encrypt                *bool  `json:"encrypt,omitempty"`
	TrustServerCertificate bool   `json:"trust_server_certificate,omitempty"`
	TLS                    string `json:"tls,omitempty"`
	CAFile                 string `json:"ca_file,omitempty"`
}

func (p Profile) DisplayName() string {
	if p.Folder != "" {
		return p.Folder + "/" + p.Name
	}
	return p.Name
}
func NormalizeFolder(raw string) (string, error) {
	for _, r := range raw {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("folder cannot contain control characters")
		}
	}
	raw = strings.Trim(strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/")), "/")
	if raw == "" {
		return "", nil
	}
	parts := strings.Split(raw, "/")
	for i, s := range parts {
		s = strings.TrimSpace(s)
		if s == "" || s == "." || s == ".." {
			return "", fmt.Errorf("folder components cannot be empty, . or ..")
		}
		parts[i] = s
	}
	return strings.Join(parts, "/"), nil
}
func FolderPaths(c Config) []string {
	paths := map[string]bool{}
	add := func(s string) {
		for s != "" {
			paths[s] = true
			i := strings.LastIndex(s, "/")
			if i < 0 {
				break
			}
			s = s[:i]
		}
	}
	for _, s := range c.Folders {
		add(s)
	}
	for _, p := range c.Profiles {
		add(p.Folder)
	}
	out := make([]string, 0, len(paths))
	for s := range paths {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func DefaultPort(engine string) int {
	switch engine {
	case "sqlserver":
		return 1433
	case "mysql":
		return 3306
	case "postgres":
		return 5432
	}
	return 0
}
func (p Profile) EffectivePort() int {
	if p.Port != 0 {
		return p.Port
	}
	return DefaultPort(p.Engine)
}

// TLSMode translates connection options and preserves older profile files.
func (p Profile) TLSMode() string {
	if p.Encrypt != nil {
		if !*p.Encrypt {
			return "disable"
		}
		if p.TrustServerCertificate {
			return "insecure"
		}
		return "verify"
	}
	if p.TLS == "disable" {
		return "disable"
	}
	if p.TrustServerCertificate || p.TLS == "insecure" {
		return "insecure"
	}
	return "verify"
}

type Config struct {
	Profiles []Profile         `json:"profiles"`
	History  bool              `json:"history"`
	Keys     map[string]string `json:"keys,omitempty"`
	Theme    string            `json:"theme,omitempty"`
	Folders  []string          `json:"folders"`
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "sqlterm", "config.json")
}
func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	names := map[string]bool{}
	for i := range c.Profiles {
		p := &c.Profiles[i]
		p.Folder, err = NormalizeFolder(p.Folder)
		if err != nil {
			return c, err
		}
		if err := p.Validate(); err != nil {
			return c, err
		}
		key := p.Folder + "\x00" + p.Name
		if names[key] {
			return c, fmt.Errorf("duplicate profile %q", p.DisplayName())
		}
		names[key] = true
	}
	if c.Folders != nil {
		folders := []string{}
		seen := map[string]bool{}
		for _, s := range c.Folders {
			path, e := NormalizeFolder(s)
			if e != nil {
				return c, e
			}
			if path != "" && !seen[path] {
				folders = append(folders, path)
				seen[path] = true
			}
		}
		c.Folders = folders
	}
	return c, nil
}
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(append(b, '\n'))
	}
	ce := tmp.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	return os.Rename(name, path)
}
func (p Profile) Validate() error {
	if p.Name == "" || p.Host == "" || p.User == "" {
		return fmt.Errorf("profile requires name, host and user")
	}
	if p.Engine != "sqlserver" && p.Engine != "mysql" && p.Engine != "postgres" {
		return fmt.Errorf("unknown engine %q", p.Engine)
	}
	if p.Port < 0 || p.Port > 65535 {
		return fmt.Errorf("invalid port")
	}
	if p.TLS != "" && p.TLS != "verify" && p.TLS != "insecure" && p.TLS != "disable" {
		return fmt.Errorf("TLS must be verify, insecure or disable")
	}
	return nil
}
func (p Profile) Password() (string, bool) {
	if p.PasswordEnv == "" {
		return "", false
	}
	return os.LookupEnv(p.PasswordEnv)
}
