package ui

func (u *UI) commentSQL(all bool) {
	if !u.editorReady(u.active()) {
		return
	}
	editor := u.active().Editor
	if !editor.CommentLines(all) {
		return
	}
	u.hideSuggestions()
	u.app.SetFocus(editor.Area)
	u.refreshBars()
}
