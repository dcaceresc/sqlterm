package result

import "unicode"

// SpreadsheetSafeText prevents spreadsheet applications from interpreting
// untrusted text as a formula. It does not change the stored result value.
func SpreadsheetSafeText(text string) string {
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		switch r {
		case '=', '+', '-', '@':
			return "'" + text
		default:
			return text
		}
	}
	return text
}

func spreadsheetSafeRecord(record []string) []string {
	safe := make([]string, len(record))
	for i, value := range record {
		safe[i] = SpreadsheetSafeText(value)
	}
	return safe
}
