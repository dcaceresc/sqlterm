package ui

import (
	"context"
	"fmt"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/diagnostic"
	"strconv"
	"strings"
	"time"
)

func testProfileFromForm(f *tview.Form, original config.Profile) (config.Profile, string, error) {
	get := func(i int) string { return strings.TrimSpace(f.GetFormItem(i).(*tview.InputField).GetText()) }
	p := original
	p.Name = get(0)
	if p.Name == "" {
		p.Name = "Connection test"
	}
	_, p.Engine = f.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
	p.Host = get(2)
	p.Database = get(4)
	p.User = get(5)
	p.Port = config.DefaultPort(p.Engine)
	if raw := get(3); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return p, "", fmt.Errorf("invalid port")
		}
		if port != 0 {
			p.Port = port
		}
	}
	encrypt := f.GetFormItem(7).(*tview.Checkbox).IsChecked()
	p.Encrypt = &encrypt
	p.TrustServerCertificate = f.GetFormItem(8).(*tview.Checkbox).IsChecked()
	p.TLS = ""
	password := f.GetFormItem(6).(*tview.InputField).GetText()
	if endpointChangeRequiresPassword(original, p, password) {
		return p, "", fmt.Errorf("%s", endpointChangedPasswordRequired)
	}
	return p, password, p.Validate()
}
func (u *UI) testConnection(f *tview.Form, result *tview.TextView, state *Tab, p config.Profile, password string) {
	if state.Busy {
		if state.Cancel != nil {
			state.Cancel()
		}
		return
	}
	if password == "" && (p.CredentialID != "" || p.PasswordEnv != "") {
		saved, ok, err := u.resolvePassword(p)
		if err != nil {
			result.SetText(err.Error())
			return
		}
		if !ok {
			result.SetText(u.tr("Enter the password to test this connection"))
			return
		}
		password = saved
	}
	result.SetText(u.tr("Testing connection…"))
	f.GetButton(1).SetLabel(u.tr("Cancel test"))
	successMessage, failurePrefix := u.tr("Connection successful"), u.tr("Connection failed: ")
	u.task(state, "Test connection", func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		session, err := u.openSession(ctx, p, password)
		if session != nil {
			session.Close()
		}
		message := successMessage
		if err != nil {
			message = diagnostic.Redact(err, password).Error()
			message = failurePrefix + message
		}
		return func() { f.GetButton(1).SetLabel(u.tr("Test connection")); result.SetText(message) }, nil
	})
}
