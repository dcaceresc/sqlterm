package ui

import (
	"testing"

	"sqlterm/internal/db"
)

func TestCatalogSearchRequiresContiguousTerms(t *testing.T) {
	u, _ := modeTestUI(t)
	const target = "sprec_alumnoscausaleli"
	u.active().Catalog = []db.Object{
		{Schema: "dbo", Name: target, Kind: "procedure"},
		{Schema: "dbo", Name: "sprec_alumnospromedios", Kind: "procedure"},
		{Schema: "dbo", Name: "sprec_busquedaalumnos", Kind: "procedure"},
		{Schema: "dbo", Name: "sprec_alumnos_causal_eli", Kind: "procedure"},
	}
	for _, query := range []string{target, "  SPREC_ALUMNOSCAUSALE  ", "dbo.sprec_alumnoscausaleli procedure"} {
		u.search.SetText(query)
		if len(u.catalogRows) != 1 {
			t.Errorf("search %q: got %v, want only %s", query, u.catalogRows, target)
			continue
		}
		for _, o := range u.catalogRows {
			if o.Name != target {
				t.Errorf("search %q: got %s, want %s", query, o.Name, target)
			}
		}
	}
	u.search.SetText("sprec_alumnoscausalelix")
	if len(u.catalogRows) != 0 || u.objectDetail.GetText(false) != "No matching objects" {
		t.Errorf("missing name returned %v, detail %q", u.catalogRows, u.objectDetail.GetText(false))
	}
}
