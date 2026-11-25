package result

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	xlsxMaxRows    = 1048576
	xlsxMaxColumns = 16384
	xlsxMaxText    = 32767
	xlsxMaxLF      = 253
	xlsxXML        = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
	xlsxSheetNS    = `http://schemas.openxmlformats.org/spreadsheetml/2006/main`
	xlsxOfficeNS   = `http://schemas.openxmlformats.org/officeDocument/2006/relationships`
	xlsxPackageNS  = `http://schemas.openxmlformats.org/package/2006/relationships`
)

var xlsxParts = [...]struct{ name, body string }{
	{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`</Types>`},
	{"_rels/.rels", `<Relationships xmlns="` + xlsxPackageNS + `">` +
		`<Relationship Id="rId1" Type="` + xlsxOfficeNS + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
	{"xl/workbook.xml", `<workbook xmlns="` + xlsxSheetNS + `" xmlns:r="` + xlsxOfficeNS + `">` +
		`<sheets><sheet name="Results" sheetId="1" r:id="rId1"/></sheets></workbook>`},
	{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="` + xlsxPackageNS + `">` +
		`<Relationship Id="rId1" Type="` + xlsxOfficeNS + `/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
}

func writeXLSX(ctx context.Context, dst io.Writer, columns []string, rows int, readRow func(int) ([]Cell, error)) error {
	if rows >= xlsxMaxRows {
		return fmt.Errorf("export: XLSX exceeds %d worksheet rows including headers", xlsxMaxRows)
	}
	if len(columns) > xlsxMaxColumns {
		return fmt.Errorf("export: XLSX exceeds %d worksheet columns", xlsxMaxColumns)
	}
	archive := zip.NewWriter(dst)
	for _, part := range xlsxParts {
		w, err := archive.Create(part.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, xlsxXML+part.body); err != nil {
			return err
		}
	}
	w, err := archive.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, xlsxXML+`<worksheet xmlns="`+xlsxSheetNS+`"><sheetData>`); err != nil {
		return err
	}
	refs := make([]string, len(columns))
	for i := range columns {
		refs[i] = xlsxColumn(i)
	}
	for i := 0; i <= rows; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var row []Cell
		if i > 0 {
			row, err = readRow(i - 1)
			if err != nil {
				return err
			}
		}
		number := strconv.Itoa(i + 1)
		if _, err := io.WriteString(w, `<row r="`+number+`">`); err != nil {
			return err
		}
		for j, column := range columns {
			if i > 0 {
				if row[j].Null {
					continue
				}
				column = row[j].Text
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := writeXLSXText(w, refs[j]+number, column); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, `</row>`); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, `</sheetData></worksheet>`); err != nil {
		return err
	}
	return archive.Close()
}

func xlsxColumn(index int) string {
	var ref [3]byte
	pos := len(ref)
	for n := index + 1; n > 0; n = (n - 1) / 26 {
		pos--
		ref[pos] = byte('A' + (n-1)%26)
	}
	return string(ref[pos:])
}

var xlsxEscapePattern = regexp.MustCompile(`_x[0-9a-fA-F]{4}_`)

func escapeXLSXText(text string) string {
	var escaped strings.Builder
	for {
		match := xlsxEscapePattern.FindStringIndex(text)
		if match == nil {
			if escaped.Len() == 0 {
				return text
			}
			escaped.WriteString(text)
			return escaped.String()
		}
		// Avanza solo sobre el guion inicial: dos escapes pueden compartir un guion.
		escaped.WriteString(text[:match[0]])
		escaped.WriteString("_x005F_")
		text = text[match[0]+1:]
	}
}

func writeXLSXText(w io.Writer, ref, text string) error {
	if !utf8.ValidString(text) {
		return fmt.Errorf("export: XLSX cell %s contains invalid XML text", ref)
	}
	units := 0
	for _, r := range text {
		switch {
		case r < 0x20 && r != '\t' && r != '\n' && r != '\r', r == 0xfffe, r == 0xffff:
			return fmt.Errorf("export: XLSX cell %s contains invalid XML text", ref)
		case r > 0xffff:
			units += 2
		default:
			units++
		}
		if units > xlsxMaxText {
			return fmt.Errorf("export: XLSX cell %s exceeds %d UTF-16 code units", ref, xlsxMaxText)
		}
	}
	if strings.Count(text, "\n") > xlsxMaxLF {
		return fmt.Errorf("export: XLSX cell %s exceeds %d line feeds", ref, xlsxMaxLF)
	}
	text = escapeXLSXText(text)
	if _, err := io.WriteString(w, `<c r="`+ref+`" t="inlineStr"><is><t xml:space="preserve">`); err != nil {
		return err
	}
	if err := xml.EscapeText(w, []byte(text)); err != nil {
		return err
	}
	_, err := io.WriteString(w, `</t></is></c>`)
	return err
}
