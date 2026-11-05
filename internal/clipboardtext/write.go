// Package clipboardtext makes database text safe for native text clipboards.
package clipboardtext

import (
	"fmt"
	"strings"
)

// Write escapes embedded NULs, which terminate Windows clipboard text and cause
// the current native backend's UTF-16 conversion to panic. No result data changes.
// Recovery is limited to the third-party clipboard call, not result processing.
func Write(writer func(string) error, text string) (escaped bool, err error) {
	escaped = strings.IndexByte(text, 0) >= 0
	if escaped {
		text = strings.ReplaceAll(text, "\x00", `\0`)
	}
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("clipboard backend rejected the text (%T); the application remains open", failure)
		}
	}()
	err = writer(text)
	return escaped, err
}
