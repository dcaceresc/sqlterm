package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sqlterm/internal/nvim"
)

type cachedQuery struct {
	Text  string `json:"text"`
	Path  string `json:"path,omitempty"`
	Saved string `json:"saved,omitempty"`
	CRLF  bool   `json:"crlf,omitempty"`
}

type cachedWorkspace struct {
	Tabs    []cachedQuery `json:"tabs"`
	Current int           `json:"current"`
}

func (u *UI) restoreWorkspace() {
	var workspace cachedWorkspace
	if readCache(u.configPath+".workspace.json", &workspace) != nil || len(workspace.Tabs) == 0 {
		u.newTab(nil)
		return
	}
	for _, query := range workspace.Tabs {
		u.newTab(nil)
		tab := u.active()
		if query.Path != "" && strings.Contains(query.Saved, "\r\n") {
			// Los cachés anteriores podían contener los terminadores originales.
			query.CRLF = usesCRLF(query.Saved)
			query.Saved = strings.ReplaceAll(query.Saved, "\r\n", "\n")
			query.Text = strings.ReplaceAll(query.Text, "\r\n", "\n")
		}
		tab.Path, tab.Saved, tab.fileCRLF = query.Path, query.Saved, query.CRLF
		_ = tab.Editor.SetText(query.Text)
	}
	u.current = max(0, min(workspace.Current, len(u.tabs)-1))
	u.showTab()
}

func (u *UI) cacheWorkspace() error {
	workspace := cachedWorkspace{Current: u.current, Tabs: make([]cachedQuery, 0, len(u.tabs))}
	for i, tab := range u.tabs {
		err := tab.Editor.Refresh()
		// Estos errores solo afectan la selección, no el texto del snapshot.
		if err != nil && !errors.Is(err, nvim.ErrBlockSelection) && !errors.Is(err, nvim.ErrUnsupportedSelection) {
			return fmt.Errorf("%s: %w", fmt.Sprintf(u.tr("Query %d"), i+1), err)
		}
		workspace.Tabs = append(workspace.Tabs, cachedQuery{Text: tab.Editor.Area.GetText(), Path: tab.Path, Saved: tab.Saved, CRLF: tab.Path != "" && tab.fileCRLF})
	}
	return writeCache(u.configPath+".workspace.json", workspace)
}

func readCache(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func writeCache(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".sqlterm-cache-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	writeErr := json.NewEncoder(file).Encode(value)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
