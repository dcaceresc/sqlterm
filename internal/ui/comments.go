package ui

func (u *UI) commentSQL(all bool) {
	editor := u.active().Editor
	if !editor.CommentLines(all) {
		if !all {
			u.message(u.tr("Select SQL text before commenting"))
		}
		return
	}
	u.hideSuggestions()
	u.app.SetFocus(editor.Area)
	u.refreshBars()
}
