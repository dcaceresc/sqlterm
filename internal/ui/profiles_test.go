package ui

import (
	"errors"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"os"
	"path/filepath"
	"sqlterm/internal/config"
	"sqlterm/internal/credentials"
	"strings"
	"testing"
)

type memoryVault struct {
	values  map[string]string
	failure error
}

func (v *memoryVault) Get(id string) (string, error) {
	if v.failure != nil {
		return "", v.failure
	}
	s, ok := v.values[id]
	if !ok {
		return "", credentials.ErrNotFound
	}
	return s, nil
}
func (v *memoryVault) Set(id, secret string) error {
	if v.failure != nil {
		return v.failure
	}
	v.values[id] = secret
	return nil
}
func (v *memoryVault) Delete(id string) error { delete(v.values, id); return nil }
func currentForm(u *UI) *tview.Form {
	_, page := u.pages.GetFrontPage()
	return page.(*centered).child.(*tview.Form)
}
func saveForm(u *UI, f *tview.Form) {
	f.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
}
func TestProfileFormTrustAndSavedPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	u := New(path, config.Config{})
	defer u.cancel()
	vault := &memoryVault{values: map[string]string{}}
	u.vault = vault
	u.profileForm(-1)
	f := currentForm(u)
	for i := 0; i < f.GetFormItemCount(); i++ {
		label := f.GetFormItem(i).GetLabel()
		if label == "TLS" || label == "CA file" || label == "Password env" {
			t.Fatalf("old field still present: %s", label)
		}
	}
	for i, text := range map[int]string{0: "local", 2: "localhost", 4: "master", 5: "sa", 6: " secret with spaces "} {
		f.GetFormItem(i).(*tview.InputField).SetText(text)
	}
	if !f.GetFormItem(7).(*tview.Checkbox).IsChecked() {
		t.Fatal("encryption must default on")
	}
	f.GetFormItem(8).(*tview.Checkbox).SetChecked(true)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 35)
	u.pages.SetRect(0, 0, 120, 35)
	u.pages.Draw(screen)
	screen.Show()
	if strings.Contains(screenText(screen), "secret with spaces") {
		t.Fatal("password was visible")
	}
	screen.SetSize(80, 24)
	u.pages.SetRect(0, 0, 80, 24)
	u.pages.Draw(screen)
	screen.Show()
	if text := screenText(screen); !strings.Contains(text, "Trust server certificate") || !strings.Contains(text, "Encrypt connection") || strings.Contains(text, "secret with spaces") {
		t.Fatal("connection options must fit the small terminal without exposing the password")
	}
	saveForm(u, f)
	if len(u.cfg.Profiles) != 1 {
		t.Fatal("profile not saved")
	}
	p := u.cfg.Profiles[0]
	if p.CredentialID == "" || p.TLSMode() != "insecure" {
		t.Fatal("credential reference or trust option missing")
	}
	if vault.values[p.CredentialID] != " secret with spaces " {
		t.Fatal("secret lost or trimmed")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret with spaces") {
		t.Fatal("password written in profile")
	}
	id := p.CredentialID
	u.dismiss()
	u.profileForm(0)
	f = currentForm(u)
	if f.GetFormItem(6).(*tview.InputField).GetText() != "" {
		t.Fatal("saved password exposed in form buffer")
	}
	saveForm(u, f)
	if u.cfg.Profiles[0].CredentialID != id {
		t.Fatal("blank edit replaced saved password")
	}
	restarted := New(path, u.cfg)
	defer restarted.cancel()
	restarted.vault = vault
	secret, ok, err := restarted.resolvePassword(u.cfg.Profiles[0])
	if err != nil || !ok || secret != " secret with spaces " {
		t.Fatal("saved password not recovered after restart")
	}
}
func TestCredentialReplacementAndSaveRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	u := New(path, config.Config{})
	defer u.cancel()
	vault := &memoryVault{values: map[string]string{}}
	u.vault = vault
	p := config.Profile{Name: "test", Engine: "sqlserver", Host: "localhost", User: "sa"}
	if err := u.persistProfile(-1, p, "first", true); err != nil {
		t.Fatal(err)
	}
	oldID := u.cfg.Profiles[0].CredentialID
	p = u.cfg.Profiles[0]
	if err := u.persistProfile(0, p, "second", true); err != nil {
		t.Fatal(err)
	}
	id := u.cfg.Profiles[0].CredentialID
	if id == oldID || len(vault.values) != 1 || vault.values[id] != "second" {
		t.Fatal("credential replacement failed")
	}
	block := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(block, []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	u.configPath = filepath.Join(block, "config.json")
	if err := u.persistProfile(0, u.cfg.Profiles[0], "third", true); err == nil {
		t.Fatal("expected config write failure")
	}
	if len(vault.values) != 1 || vault.values[id] != "second" || u.cfg.Profiles[0].CredentialID != id {
		t.Fatal("rollback destroyed existing password")
	}
	vault.failure = errors.New("credential store locked")
	u.configPath = path
	if err := u.persistProfile(0, u.cfg.Profiles[0], "third", true); err == nil {
		t.Fatal("secret store error ignored")
	}
	loaded, err := config.Load(path)
	if err != nil || loaded.Profiles[0].CredentialID != id {
		t.Fatal("profile changed after failed secret write")
	}
}
func TestProfilePortsFollowEngineAndPreserveCustomValues(t *testing.T) {
	u := New(filepath.Join(t.TempDir(), "config.json"), config.Config{})
	defer u.cancel()
	u.vault = &memoryVault{values: map[string]string{}}
	u.profileForm(-1)
	f := currentForm(u)
	port := f.GetFormItem(3).(*tview.InputField)
	engine := f.GetFormItem(1).(*tview.DropDown)
	if port.GetText() != "1433" {
		t.Fatal("new SQL Server profile must show 1433")
	}
	engine.SetCurrentOption(1)
	if port.GetText() != "3306" {
		t.Fatal("MySQL default not selected")
	}
	engine.SetCurrentOption(2)
	if port.GetText() != "5432" {
		t.Fatal("PostgreSQL default not selected")
	}
	port.SetText("15432")
	engine.SetCurrentOption(0)
	if port.GetText() != "15432" {
		t.Fatal("custom port changed")
	}
	port.SetText("")
	engine.SetCurrentOption(1)
	if port.GetText() != "3306" {
		t.Fatal("blank port not filled on engine change")
	}
	f.GetFormItem(0).(*tview.InputField).SetText("test")
	f.GetFormItem(2).(*tview.InputField).SetText("localhost")
	f.GetFormItem(5).(*tview.InputField).SetText("user")
	port.SetText("")
	saveForm(u, f)
	if len(u.cfg.Profiles) != 1 || u.cfg.Profiles[0].Port != 3306 {
		t.Fatal("blank port not saved with engine default")
	}
	u.cfg.Profiles[0].Port = 0
	u.dismiss()
	u.profileForm(0)
	if currentForm(u).GetFormItem(3).(*tview.InputField).GetText() != "3306" {
		t.Fatal("legacy zero port not displayed as engine default")
	}
}
