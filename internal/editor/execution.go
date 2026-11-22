package editor

// ExecutionText distingue una selección vacía de la ausencia de selección.
func (e *Editor) ExecutionText() (string, error) {
	if err := e.Refresh(); err != nil {
		return "", err
	}
	if e.native != nil {
		text, selected, err := e.native.snapshot.SelectedText()
		if err != nil {
			return "", err
		}
		if selected {
			return text, nil
		}
	}
	text, _, _ := e.Area.GetSelection()
	if text == "" {
		text = e.Area.GetText()
	}
	return text, nil
}
