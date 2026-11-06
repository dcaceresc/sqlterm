package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
	"strconv"
	"strings"
)

func (u *UI) popup(p tview.Primitive, width, height int) {
	if u.dialogContent == nil {
		u.dialogReturn = u.app.GetFocus()
	}
	u.copyPopup = nil
	u.hideSuggestions()
	if u.dialogCancel != nil {
		cancel := u.dialogCancel
		u.dialogCancel = nil
		cancel()
	}
	inner := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(nil, 0, 1, false).AddItem(p, height, 0, true).AddItem(nil, 0, 1, false)
	frame := &centered{Flex: tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(inner, width, 0, true).AddItem(nil, 0, 1, false), inner: inner, child: p, width: width, height: height}
	frame.SetInputCapture(func(k *tcell.EventKey) *tcell.EventKey {
		if k.Key() == tcell.KeyEscape {
			u.dismiss()
			return nil
		}
		return k
	})
	u.dialogContent = p
	u.pages.AddPage("dialog", frame, true, true)
	u.style(frame)
	u.app.SetFocus(p)
}

type centered struct {
	*tview.Flex
	inner         *tview.Flex
	child         tview.Primitive
	width, height int
}

func (u *UI) dialogContains(x, y int) bool {
	if u.dialogContent == nil {
		return false
	}
	left, top, width, height := u.dialogContent.GetRect()
	return x >= left && x < left+width && y >= top && y < top+height
}

func (c *centered) Draw(screen tcell.Screen) {
	x, y, w, h := c.GetRect()
	// Cover the workspace behind dialogs while keeping the destination line and
	// bottom shortcuts visible. Otherwise underlying borders look like nested
	// modal borders, particularly on 80-column terminals.
	style := tcell.StyleDefault.Background(c.GetBackgroundColor())
	for row := y + 1; row < y+h-1; row++ {
		for col := x; col < x+w; col++ {
			screen.SetContent(col, row, ' ', nil, style)
		}
	}
	c.Flex.ResizeItem(c.inner, min(c.width, max(1, w-2)), 0)
	c.inner.ResizeItem(c.child, min(c.height, max(1, h-2)), 0)
	c.Flex.Draw(screen)
}
func (u *UI) dismiss() {
	previous := u.dialogReturn
	u.dialogReturn = nil
	u.dialogContent = nil
	u.copyPopup = nil
	if u.dialogCancel != nil {
		cancel := u.dialogCancel
		u.dialogCancel = nil
		cancel()
	}
	u.pages.RemovePage("dialog")
	if previous == nil {
		previous = u.active().Editor.Area
	}
	u.app.SetFocus(previous)
	u.refreshBars()
}
func (u *UI) notice(title, text string) {
	v := tview.NewTextView().SetText(text).SetWrap(true).SetScrollable(true)
	v.SetBorder(true).SetTitle(u.tr(" ") + title + " · Esc close ")
	u.popup(v, 100, 24)
}
func (u *UI) confirm(text string, yes func()) {
	m := tview.NewModal().SetText(text).AddButtons([]string{"Cancel", "Confirm"}).SetDoneFunc(func(_ int, label string) {
		u.dismiss()
		if label == "Confirm" {
			yes()
		}
	})
	u.popup(m, 80, 12)
}
func (u *UI) prompt(title, initial string, secret bool, done func(string)) {
	f := tview.NewForm()
	f.SetBorder(true).SetTitle(u.tr(" ") + title + " ")
	mask := rune(0)
	if secret {
		mask = '*'
	}
	if secret {
		f.AddPasswordField(u.tr("Value"), initial, 60, mask, nil)
	} else {
		f.AddInputField(u.tr("Value"), initial, 60, nil, nil)
	}
	f.AddButton(u.tr("OK"), func() { s := f.GetFormItem(0).(*tview.InputField).GetText(); u.dismiss(); done(s) }).AddButton(u.tr("Cancel"), u.dismiss)
	u.popup(f, 90, 9)
}
func (u *UI) chooseConnection() {
	u.connectionBrowser()
}
func (u *UI) pickProfileEdit() {
	l := tview.NewList()
	l.SetBorder(true).SetTitle(u.tr(" Edit profile "))
	for i, p := range u.cfg.Profiles {
		idx := i
		l.AddItem(p.DisplayName(), "", 0, func() { u.dismiss(); u.profileForm(idx) })
	}
	u.popup(l, 70, 16)
}
func (u *UI) password(p config.Profile, done func(string)) {
	password, ok, err := u.resolvePassword(p)
	if err != nil {
		u.notice(u.tr("Saved password"), err.Error())
		return
	}
	if ok {
		if p.CredentialID == "" {
			if err = u.rememberPassword(p, password); err != nil {
				u.notice(u.tr("Save password"), err.Error())
				return
			}
		}
		done(password)
		return
	}
	title := "Password for " + p.DisplayName()
	if p.PasswordEnv != "" {
		title += " (environment variable not set)"
	}
	u.prompt(title, "", true, func(password string) {
		if err := u.rememberPassword(p, password); err != nil {
			u.notice(u.tr("Save password"), err.Error())
			return
		}
		done(password)
	})
}
func (u *UI) profileForm(index int) {
	p := config.Profile{Engine: "sqlserver", TLS: "verify"}
	p.Folder = u.connectionFolder
	if index >= 0 {
		p = u.cfg.Profiles[index]
	}
	engines := []string{"sqlserver", "mysql", "postgres"}
	ei := 0
	for i, x := range engines {
		if x == p.Engine {
			ei = i
		}
	}
	f := tview.NewForm().SetItemPadding(0)
	f.SetBorder(true).SetTitle(u.tr(" Connection profile · Tab next "))
	f.AddInputField(u.tr("Name"), p.Name, 45, nil, nil).AddDropDown("Engine", engines, ei, nil).AddInputField(u.tr("Host"), p.Host, 45, nil, nil).AddInputField(u.tr("Port"), strconv.Itoa(p.EffectivePort()), 7, tview.InputFieldInteger, nil).AddInputField(u.tr("Database"), p.Database, 45, nil, nil).AddInputField(u.tr("User"), p.User, 45, nil, nil).AddPasswordField(u.tr("Password"), "", 45, '*', nil).AddCheckbox(u.tr("Encrypt connection"), p.TLSMode() != "disable", nil).AddCheckbox(u.tr("Trust server certificate"), p.TLSMode() == "insecure", nil)
	f.AddInputField(u.tr("Folder"), p.Folder, 45, nil, nil)
	f.GetFormItem(9).(*tview.InputField).SetPlaceholder("DEV, QA, PROD or DEV/Project")
	f.GetFormItem(4).(*tview.InputField).SetPlaceholder("Optional - choose a database after connecting")
	testState := &Tab{}
	testResult := tview.NewTextView().SetWrap(true).SetScrollable(true)
	testResult.SetLabel(u.tr("Test")).SetSize(2, 45)
	testResult.SetText(u.tr("Not tested"))
	f.AddFormItem(testResult)
	previousEngine := p.Engine
	f.GetFormItem(1).(*tview.DropDown).SetSelectedFunc(func(engine string, _ int) {
		portField := f.GetFormItem(3).(*tview.InputField)
		text := strings.TrimSpace(portField.GetText())
		if text == "" || text == "0" || text == strconv.Itoa(config.DefaultPort(previousEngine)) {
			portField.SetText(strconv.Itoa(config.DefaultPort(engine)))
		}
		previousEngine = engine
	})
	if p.CredentialID != "" {
		f.GetFormItem(6).(*tview.InputField).SetPlaceholder("Saved - leave blank to keep")
	}
	f.AddButton(u.tr("Save"), func() {
		get := func(i int) string { return strings.TrimSpace(f.GetFormItem(i).(*tview.InputField).GetText()) }
		_, engine := f.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
		encrypt := f.GetFormItem(7).(*tview.Checkbox).IsChecked()
		trust := f.GetFormItem(8).(*tview.Checkbox).IsChecked()
		secret := f.GetFormItem(6).(*tview.InputField).GetText()
		port := config.DefaultPort(engine)
		var e error
		if raw := get(3); raw != "" {
			port, e = strconv.Atoi(raw)
		}
		if e != nil {
			f.SetTitle(u.tr(" Invalid port "))
			return
		}
		if port == 0 {
			port = config.DefaultPort(engine)
		}
		folder, e := config.NormalizeFolder(get(9))
		if e != nil {
			f.SetTitle(u.tr(" ") + e.Error() + " ")
			return
		}
		p := config.Profile{Name: get(0), Folder: folder, Engine: engine, Host: get(2), Port: port, Database: get(4), User: get(5), CredentialID: p.CredentialID, PasswordEnv: p.PasswordEnv, Encrypt: &encrypt, TrustServerCertificate: trust, CAFile: p.CAFile}
		if e = p.Validate(); e != nil {
			f.SetTitle(u.tr(" ") + e.Error() + " ")
			return
		}
		for i, x := range u.cfg.Profiles {
			if i != index && x.Name == p.Name && x.Folder == p.Folder {
				f.SetTitle(u.tr(" Duplicate profile name in folder "))
				return
			}
		}
		if e = u.persistProfile(index, p, secret, secret != "" || (p.CredentialID == "" && p.PasswordEnv == "")); e != nil {
			f.SetTitle(u.tr(" ") + e.Error() + " ")
			return
		}
		u.dismiss()
		u.chooseConnection()
	})
	f.AddButton(u.tr("Test connection"), func() {
		if testState.Busy {
			if testState.Cancel != nil {
				testState.Cancel()
			}
			return
		}
		profile, secret, err := testProfileFromForm(f, p)
		if err != nil {
			testResult.SetText(u.tr(err.Error()))
			return
		}
		u.testConnection(f, testResult, testState, profile, secret)
	})
	if index >= 0 {
		f.AddButton(u.tr("Delete"), func() {
			u.confirm(u.tr("Delete connection profile ")+p.Name+"?", func() {
				next := u.cfg
				next.Profiles = append([]config.Profile(nil), u.cfg.Profiles...)
				next.Profiles = append(next.Profiles[:index], next.Profiles[index+1:]...)
				if e := config.Save(u.configPath, next); e != nil {
					u.message(e.Error())
					return
				}
				u.cfg = next
				if p.CredentialID != "" {
					if err := u.vault.Delete(p.CredentialID); err != nil {
						u.notice(u.tr("Credential cleanup"), "Profile deleted, but the saved credential could not be removed: "+err.Error())
						return
					}
				}
				u.chooseConnection()
			})
		})
	}
	f.AddButton(u.tr("Cancel"), u.dismiss)
	u.popup(f, 88, 27)
	u.dialogCancel = func() {
		if testState.Cancel != nil {
			testState.Cancel()
		}
	}
}
func (u *UI) connectionFailure(p config.Profile, err error) {
	title := "Connection failed"
	detail := err.Error()
	if db.IsCertificateError(err) {
		title = "TLS certificate verification failed"
		detail = "The server certificate could not be verified.\n\nIf this is your expected server:\n1. Choose Edit profile.\n2. Enable Trust server certificate.\n3. Keep Encrypt connection enabled.\n\nSave and select the connection again.\n\n" + detail
	}
	f := tview.NewForm().AddTextView("", detail, 74, 14, false, true)
	f.SetBorder(true).SetTitle(u.tr(" ") + title + " ")
	for i, profile := range u.cfg.Profiles {
		if profile.Name == p.Name && profile.Folder == p.Folder {
			index := i
			f.AddButton(u.tr("Edit profile"), func() { u.dismiss(); u.profileForm(index) })
			break
		}
	}
	f.AddButton(u.tr("Close"), u.dismiss)
	u.popup(f, 88, 22)
}
func (u *UI) showHelp() {
	u.searchableHelp()
}

func (u *UI) legacyHelp() string {
	return fmt.Sprintf(`F2 connections/folders     F3 themes     F4 database     F5 execute SQL
F6 object search     F7 editor     F8 results     F10 cancel
Ctrl+N new session     Alt+1..9 switch tabs     Ctrl+O open     Ctrl+S save
IntelliSense: automatic while typing; Ctrl+Space opens suggestions.
Up/Down or Ctrl+N/P select; Tab/Enter accept; Esc closes and returns NORMAL.
Ctrl+Q quit; Ctrl+C opens result copy options from any panel, or closes a dialog.
Esc closes dialogs / returns to NORMAL.

Editor: i/a/I/A/o insert; Esc normal; v visual (half-open selection).
h/j/k/l, w/b, 0/$, gg/G move. yy/dd copy/cut line; y/d visual selection;
x delete; p paste; u undo; Ctrl+R redo; / search forward.
Space: c connect, f find, x run, p invoke, a apply definition,
r refresh catalog, n new tab, w save, o open, q close tab,
h history, e export, b change database, d disconnect, t routine template.
Space T themes. Themes preview with arrows; Enter saves, Esc restores.
Connections: Enter connect/expand, + add profile, n new folder,
e edit profile, r rename folder, d delete empty folder, / search.

Results: [ / ] previous/next result set; Enter inspect cell; m messages;
/ search rows; s sort selected column ascending; S descending; e export.
Ctrl+C copy menu; y/Y copy all without/with headers; r/R current row;
c current cell. Copy shortcuts and CSV default to spreadsheet-safe text;
menus expose literal copy/CSV with an untrusted-data warning. Clipboard rows
are TSV in displayed order. JSON remains literal and unchanged.
Sorting is lexical, NULL first ascending. Export includes the complete set.

SQL Server: GO separates batches. MySQL: DELIMITER for routine scripts.
PostgreSQL: dollar-quoted function bodies are supported.
Blank Database: choose from the explorer or press F4 before executing SQL.
Connection form: Test connection uses unsaved fields and does not save them.
Changing engine, host, port or user requires entering the password again before testing or saving.
Each tab owns a connection. Queries run with the server's transaction rules;
use BEGIN / COMMIT / ROLLBACK explicitly when needed.
History is opt-in (Space h). Passwords are never saved in profiles.

Config: %s`, u.configPath)
}
