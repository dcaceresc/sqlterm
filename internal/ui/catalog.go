package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rivo/tview"
	"sqlterm/internal/config"
	"sqlterm/internal/db"
)

func (u *UI) rebuildCatalog() {
	if len(u.tabs) == 0 || u.restoringCatalog {
		return
	}
	t := u.active()
	selected := t.CatalogSelection
	u.objects.Clear()
	u.catalogRows = map[int]db.Object{}
	needle := strings.ToLower(u.search.GetText())
	t.CatalogFilter = u.search.GetText()
	t.CatalogKind, _ = u.kind.GetCurrentOption()
	u.kind.SetDisabled(t.SelectingDatabase)
	if t.SelectingDatabase {
		u.objects.SetTitle(u.tr(" Databases · Enter select · F4 "))
		for _, name := range t.Databases {
			n := name
			if fuzzy(needle, strings.ToLower(n)) {
				u.objects.AddItem(tview.Escape(n), "", 0, func() { p := t.Profile; p.Database = n; u.connect(t, p, t.Password) })
			}
		}
		u.objectDetail.SetText(fmt.Sprintf("%d databases · F4 choose", u.objects.GetItemCount()))
		return
	}
	u.objects.SetTitle(u.tr(" Objects · Enter definition "))
	index, _ := u.kind.GetCurrentOption()
	kind := "all"
	if index >= 0 && index < len(objectKinds) {
		kind = objectKinds[index]
	}
	filtered := []db.Object{}
	for _, o := range t.Catalog {
		if kind != "" && kind != "all" && o.Kind != kind {
			continue
		}
		if fuzzy(needle, strings.ToLower(o.Label()+" "+o.Kind)) {
			filtered = append(filtered, o)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Schema+"/"+filtered[i].Kind+"/"+filtered[i].Name < filtered[j].Schema+"/"+filtered[j].Kind+"/"+filtered[j].Name
	})
	addObject := func(o db.Object, text string) {
		i := u.objects.GetItemCount()
		u.catalogRows[i] = o
		u.objects.AddItem(tview.Escape(text), "", 0, func() { u.openObject(o) })
		if o.Kind+" "+o.Label() == selected {
			u.objects.SetCurrentItem(i)
		}
	}
	if t.Collapsed == nil {
		t.Collapsed = map[string]bool{}
	}
	lastSchema := "\x00"
	for i := 0; i < len(filtered); {
		o := filtered[i]
		if u.cfg.FlatCatalog {
			addObject(o, u.tr(o.Kind)+" "+o.Label())
			i++
			continue
		}
		schemaKey := "schema:" + o.Schema
		if o.Schema != lastSchema {
			lastSchema = o.Schema
			schema := o.Schema
			marker := "−"
			if t.Collapsed[schemaKey] && needle == "" {
				marker = "+"
			}
			u.objects.AddItem(marker+" "+tview.Escape(schema), "", 0, func() { t.Collapsed["schema:"+schema] = !t.Collapsed["schema:"+schema]; u.rebuildCatalog() })
		}
		end := i + 1
		for end < len(filtered) && filtered[end].Schema == o.Schema && filtered[end].Kind == o.Kind {
			end++
		}
		key := o.Schema + "/" + o.Kind
		if t.Collapsed[schemaKey] && needle == "" {
			i = end
			continue
		}
		marker := "−"
		if t.Collapsed[key] && needle == "" {
			marker = "+"
		}
		u.objects.AddItem(fmt.Sprintf("  %s %s (%d)", marker, u.tr(o.Kind), end-i), "", 0, func() { t.Collapsed[key] = !t.Collapsed[key]; u.rebuildCatalog() })
		if !t.Collapsed[key] || needle != "" {
			for _, child := range filtered[i:end] {
				addObject(child, "    "+child.Name)
			}
		}
		i = end
	}
	if len(filtered) == 0 {
		text := "No matching objects"
		if len(t.Catalog) == 0 {
			text = "No objects · Ctrl+P Refresh catalog"
		}
		if t.CatalogStatus != "" {
			text = t.CatalogStatus
		}
		u.objectDetail.SetText(u.tr(text))
	} else {
		u.objectDetail.SetText(fmt.Sprintf(u.tr("%d matches / %d objects"), len(filtered), len(t.Catalog)))
	}
}

var objectKinds = []string{"all", "table", "view", "procedure", "function"}

func (u *UI) kindOptions() []string {
	labels := make([]string, len(objectKinds))
	for i, kind := range objectKinds {
		labels[i] = u.tr(kind)
	}
	return labels
}

func (u *UI) toggleCatalog() {
	next := u.cfg
	next.FlatCatalog = !next.FlatCatalog
	if err := config.Save(u.configPath, next); err != nil {
		u.message(err.Error())
		return
	}
	u.cfg = next
	u.filterObjects()
}

func (u *UI) insertObjectName(selectSQL bool) {
	o, ok := u.selectedObject()
	if !ok || u.active().Session == nil {
		u.message(u.tr("Select an object first"))
		return
	}
	t := u.active()
	a := t.Session.Adapter
	name := a.Quote(o.Schema) + "." + a.Quote(o.Name)
	text := name
	if selectSQL {
		text = "SELECT * FROM " + name + ";"
	}
	start, end := t.Editor.Cursor(), t.Editor.Cursor()
	_, from, to := t.Editor.Area.GetSelection()
	if from != to {
		start, end = from, to
	}
	sql := t.Editor.Area.GetText()
	t.Editor.SetText(sql[:start] + text + sql[end:])
	t.Editor.Area.Select(start+len(text), start+len(text))
	u.app.SetFocus(t.Editor.Area)
}
