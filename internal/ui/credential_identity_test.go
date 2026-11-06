package ui

import (
	"path/filepath"
	"testing"

	"github.com/rivo/tview"
	"sqlterm/internal/config"
)

func TestEditedEndpointRequiresExplicitPassword(t *testing.T) {
	for _, field := range []string{"host", "port", "engine", "user"} {
		t.Run(field, func(t *testing.T) {
			original := config.Profile{Name: "saved", Engine: "sqlserver", Host: "old", User: "u", CredentialID: "saved-secret"}
			u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Profiles: []config.Profile{original}})
			defer u.cancel()
			vault := &memoryVault{values: map[string]string{"saved-secret": "old-password"}}
			u.vault = vault
			u.profileForm(0)
			f := currentForm(u)
			switch field {
			case "host":
				f.GetFormItem(2).(*tview.InputField).SetText("new-host")
			case "port":
				f.GetFormItem(3).(*tview.InputField).SetText("1444")
			case "user":
				f.GetFormItem(5).(*tview.InputField).SetText("another-user")
			case "engine":
				f.GetFormItem(1).(*tview.DropDown).SetCurrentOption(1)
			}
			if _, _, err := testProfileFromForm(f, original); err == nil {
				t.Error("test accepted a changed endpoint without a new password")
			}
			// Repeated saves must not mutate the captured original and bypass the guard.
			for i := 0; i < 2; i++ {
				saveForm(u, f)
				if u.cfg.Profiles[0] != original || len(vault.values) != 1 || vault.values["saved-secret"] != "old-password" {
					t.Fatal("rejected save changed profile or credentials")
				}
			}
			f.GetFormItem(6).(*tview.InputField).SetText("new-password")
			if _, secret, err := testProfileFromForm(f, original); err != nil || secret != "new-password" {
				t.Fatalf("explicit password rejected: %v", err)
			}
			saveForm(u, f)
			saved := u.cfg.Profiles[0]
			if saved.CredentialID == "saved-secret" || vault.values[saved.CredentialID] != "new-password" {
				t.Fatal("new credential was not bound to edited endpoint")
			}
			loaded, err := config.Load(u.configPath)
			if err != nil || loaded.Profiles[0].Host != saved.Host || loaded.Profiles[0].CredentialID != saved.CredentialID {
				t.Fatalf("saved profile mismatch: %v", err)
			}
		})
	}
}

func TestPasswordEnvEndpointChangeRequiresExplicitPassword(t *testing.T) {
	original := config.Profile{Name: "saved", Engine: "postgres", Host: "old", User: "u", PasswordEnv: "SQLTERM_PASSWORD"}
	edited := original
	edited.Host = "new"
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Profiles: []config.Profile{original}})
	defer u.cancel()
	u.vault = &memoryVault{values: map[string]string{}}
	u.profileForm(0)
	f := currentForm(u)
	f.GetFormItem(2).(*tview.InputField).SetText("new")
	if _, _, err := testProfileFromForm(f, original); err == nil {
		t.Fatal("test reused password_env credential for a changed endpoint")
	}

	if err := u.persistProfile(0, edited, "", false); err == nil {
		t.Fatal("password_env credential was reused for a changed endpoint")
	}
	if u.cfg.Profiles[0] != original || len(u.vault.(*memoryVault).values) != 0 {
		t.Fatal("rejected password_env save changed profile or credentials")
	}
	if err := u.persistProfile(0, edited, "new-password", true); err != nil {
		t.Fatalf("explicit replacement password rejected: %v", err)
	}
	if u.cfg.Profiles[0].PasswordEnv != "" || u.cfg.Profiles[0].CredentialID == "" {
		t.Fatal("replacement password did not replace password_env binding")
	}
}

func TestCredentialIdentityRetainsUnchangedEndpoint(t *testing.T) {
	original := config.Profile{Name: "saved", Engine: "sqlserver", Host: "old", User: "u", CredentialID: "id", PasswordEnv: "ENV"}
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{Profiles: []config.Profile{original}})
	defer u.cancel()
	u.profileForm(0)
	f := currentForm(u)
	f.GetFormItem(0).(*tview.InputField).SetText("renamed")
	f.GetFormItem(4).(*tview.InputField).SetText("other-db")
	f.GetFormItem(9).(*tview.InputField).SetText("DEV")
	p, secret, err := testProfileFromForm(f, original)
	if err != nil || secret != "" || p.CredentialID != "id" || p.PasswordEnv != "ENV" {
		t.Fatalf("unchanged endpoint lost credentials: %+v %v", p, err)
	}
	saveForm(u, f)
	saved := u.cfg.Profiles[0]
	if saved.Name != "renamed" || saved.Folder != "DEV" || saved.Database != "other-db" || saved.CredentialID != "id" || saved.PasswordEnv != "ENV" {
		t.Fatalf("non-endpoint edit did not retain credentials: %+v", saved)
	}
}
