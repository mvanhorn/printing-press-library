package twb

import (
	"fmt"
	"strings"

	"github.com/beevik/etree"
	"github.com/google/uuid"
)

// Dashboard describes a dashboard and its zone count.
type Dashboard struct {
	Name      string `json:"name"`
	ZoneCount int    `json:"zone_count"`
}

// DashboardTemplate is a known-good layout agents may scaffold (never freeform zones).
type DashboardTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MinSheets   int    `json:"min_sheets"`
	MaxSheets   int    `json:"max_sheets"`
}

// ListDashboardTemplates returns built-in templates derived from legal Desktop-saved patterns
// (e.g. official Document API filtering.twb two-pane layout).
func ListDashboardTemplates() []DashboardTemplate {
	return []DashboardTemplate{
		{
			ID:          "single",
			Name:        "Single pane",
			Description: "One worksheet filling the dashboard (layout-basic root + sheet zone)",
			MinSheets:   1,
			MaxSheets:   1,
		},
		{
			ID:          "two-pane",
			Name:        "Two pane horizontal",
			Description: "Side-by-side sheets — structure from Document API filtering.twb setTest",
			MinSheets:   2,
			MaxSheets:   2,
		},
		{
			ID:          "quad",
			Name:        "2x2 grid",
			Description: "Four sheet panes in a 2x2 layout-basic grid",
			MinSheets:   4,
			MaxSheets:   4,
		},
		{
			ID:          "three-row",
			Name:        "Three row vertical",
			Description: "Three stacked full-width sheet panes (KPI strip pattern)",
			MinSheets:   3,
			MaxSheets:   3,
		},
	}
}

// ListDashboards returns dashboards in document order with zone counts.
func (w *Workbook) ListDashboards() []Dashboard {
	parent := w.dashboardsParent()
	if parent == nil {
		return nil
	}
	var out []Dashboard
	for _, el := range parent.SelectElements("dashboard") {
		name := el.SelectAttrValue("name", "")
		zones := 0
		for range el.FindElements(".//zone") {
			zones++
		}
		out = append(out, Dashboard{Name: name, ZoneCount: zones})
	}
	return out
}

// ScaffoldDashboard adds a dashboard using only a known-good template.
// sheetNames must exist on the workbook and match template sheet cardinality.
func (w *Workbook) ScaffoldDashboard(dashName, templateID string, sheetNames []string) error {
	dashName = strings.TrimSpace(dashName)
	if dashName == "" {
		return fmt.Errorf("dashboard name is required")
	}
	tpl, err := lookupTemplate(templateID)
	if err != nil {
		return err
	}
	if len(sheetNames) < tpl.MinSheets || len(sheetNames) > tpl.MaxSheets {
		return fmt.Errorf("template %q requires %d–%d sheet(s), got %d", tpl.ID, tpl.MinSheets, tpl.MaxSheets, len(sheetNames))
	}
	for _, s := range sheetNames {
		if w.findWorksheet(s) == nil {
			return fmt.Errorf("sheet %q not found in workbook", s)
		}
	}
	parent := w.ensureDashboardsParent()
	for _, d := range parent.SelectElements("dashboard") {
		if d.SelectAttrValue("name", "") == dashName {
			return fmt.Errorf("dashboard %q already exists", dashName)
		}
	}

	var dash *etree.Element
	switch tpl.ID {
	case "single":
		dash = buildSinglePaneDashboard(dashName, sheetNames[0])
	case "two-pane":
		dash = buildTwoPaneDashboard(dashName, sheetNames[0], sheetNames[1])
	case "quad":
		dash = buildQuadDashboard(dashName, sheetNames)
	case "three-row":
		dash = buildThreeRowDashboard(dashName, sheetNames)
	default:
		return fmt.Errorf("template %q not implemented", tpl.ID)
	}
	parent.AddChild(dash)
	return nil
}

// CloneDashboard deep-copies an existing dashboard under a new name (template-safe: copy only).
func (w *Workbook) CloneDashboard(fromName, toName string) error {
	fromName = strings.TrimSpace(fromName)
	toName = strings.TrimSpace(toName)
	if fromName == "" || toName == "" {
		return fmt.Errorf("from and to dashboard names are required")
	}
	parent := w.dashboardsParent()
	if parent == nil {
		return fmt.Errorf("workbook has no dashboards")
	}
	var src *etree.Element
	for _, d := range parent.SelectElements("dashboard") {
		if d.SelectAttrValue("name", "") == fromName {
			src = d
			break
		}
	}
	if src == nil {
		return fmt.Errorf("dashboard %q not found", fromName)
	}
	for _, d := range parent.SelectElements("dashboard") {
		if d.SelectAttrValue("name", "") == toName {
			return fmt.Errorf("dashboard %q already exists", toName)
		}
	}
	cp := cloneElement(src)
	if a := cp.SelectAttr("name"); a != nil {
		a.Value = toName
	} else {
		cp.CreateAttr("name", toName)
	}
	// Fresh simple-id so Desktop doesn't see a UUID collision.
	if sid := cp.SelectElement("simple-id"); sid != nil {
		if a := sid.SelectAttr("uuid"); a != nil {
			a.Value = "{" + strings.ToUpper(uuid.NewString()) + "}"
		}
	} else {
		sid := cp.CreateElement("simple-id")
		sid.CreateAttr("uuid", "{"+strings.ToUpper(uuid.NewString())+"}")
	}
	parent.AddChild(cp)
	return nil
}

func lookupTemplate(id string) (DashboardTemplate, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	for _, t := range ListDashboardTemplates() {
		if t.ID == id {
			return t, nil
		}
	}
	var ids []string
	for _, t := range ListDashboardTemplates() {
		ids = append(ids, t.ID)
	}
	return DashboardTemplate{}, fmt.Errorf("unknown template %q (want one of: %s)", id, strings.Join(ids, ", "))
}

func (w *Workbook) ensureDashboardsParent() *etree.Element {
	root := w.Root()
	d := root.SelectElement("dashboards")
	if d != nil {
		return d
	}
	// Insert after worksheets when present for a conventional document order.
	d = etree.NewElement("dashboards")
	if ws := root.SelectElement("worksheets"); ws != nil {
		// etree InsertChild: find index of worksheets and insert after.
		// Fallback: append.
		root.InsertChildAt(ws.Index()+1, d)
		return d
	}
	root.AddChild(d)
	return d
}

func newDashboardShell(name string, maxW, maxH int) *etree.Element {
	dash := etree.NewElement("dashboard")
	dash.CreateAttr("name", name)
	dash.CreateElement("style")
	size := dash.CreateElement("size")
	size.CreateAttr("maxheight", fmt.Sprintf("%d", maxH))
	size.CreateAttr("maxwidth", fmt.Sprintf("%d", maxW))
	size.CreateAttr("minheight", fmt.Sprintf("%d", maxH))
	size.CreateAttr("minwidth", fmt.Sprintf("%d", maxW))
	return dash
}

func addSimpleID(dash *etree.Element) {
	sid := dash.CreateElement("simple-id")
	sid.CreateAttr("uuid", "{"+strings.ToUpper(uuid.NewString())+"}")
}

func zoneStyle(el *etree.Element, margin string) {
	zs := el.CreateElement("zone-style")
	addFmt := func(attr, value string) {
		f := zs.CreateElement("format")
		f.CreateAttr("attr", attr)
		f.CreateAttr("value", value)
	}
	addFmt("border-color", "#000000")
	addFmt("border-style", "none")
	addFmt("border-width", "0")
	addFmt("margin", margin)
}

func sheetZone(id, name, h, w, x, y string) *etree.Element {
	z := etree.NewElement("zone")
	z.CreateAttr("h", h)
	z.CreateAttr("id", id)
	z.CreateAttr("name", name)
	z.CreateAttr("w", w)
	z.CreateAttr("x", x)
	z.CreateAttr("y", y)
	zoneStyle(z, "4")
	return z
}

// buildTwoPaneDashboard mirrors Document API filtering.twb dashboard setTest.
func buildTwoPaneDashboard(dashName, sheetA, sheetB string) *etree.Element {
	dash := newDashboardShell(dashName, 1000, 800)
	zones := dash.CreateElement("zones")
	rootZ := zones.CreateElement("zone")
	rootZ.CreateAttr("type-v2", "layout-basic")
	rootZ.CreateAttr("h", "100000")
	rootZ.CreateAttr("id", "4")
	rootZ.CreateAttr("w", "100000")
	rootZ.CreateAttr("x", "0")
	rootZ.CreateAttr("y", "0")
	rootZ.AddChild(sheetZone("3", sheetA, "98000", "49200", "800", "1000"))
	rootZ.AddChild(sheetZone("5", sheetB, "98000", "49200", "50000", "1000"))
	zoneStyle(rootZ, "8")
	addSimpleID(dash)
	return dash
}

func buildSinglePaneDashboard(dashName, sheet string) *etree.Element {
	dash := newDashboardShell(dashName, 1000, 800)
	zones := dash.CreateElement("zones")
	rootZ := zones.CreateElement("zone")
	rootZ.CreateAttr("type-v2", "layout-basic")
	rootZ.CreateAttr("h", "100000")
	rootZ.CreateAttr("id", "4")
	rootZ.CreateAttr("w", "100000")
	rootZ.CreateAttr("x", "0")
	rootZ.CreateAttr("y", "0")
	rootZ.AddChild(sheetZone("3", sheet, "98000", "98400", "800", "1000"))
	zoneStyle(rootZ, "8")
	addSimpleID(dash)
	return dash
}

func buildQuadDashboard(dashName string, sheets []string) *etree.Element {
	// 2x2 using same coordinate system as filtering.twb (0..100000).
	dash := newDashboardShell(dashName, 1200, 900)
	zones := dash.CreateElement("zones")
	rootZ := zones.CreateElement("zone")
	rootZ.CreateAttr("type-v2", "layout-basic")
	rootZ.CreateAttr("h", "100000")
	rootZ.CreateAttr("id", "1")
	rootZ.CreateAttr("w", "100000")
	rootZ.CreateAttr("x", "0")
	rootZ.CreateAttr("y", "0")
	// TL TR BL BR
	rootZ.AddChild(sheetZone("2", sheets[0], "48000", "49200", "800", "1000"))
	rootZ.AddChild(sheetZone("3", sheets[1], "48000", "49200", "50000", "1000"))
	rootZ.AddChild(sheetZone("4", sheets[2], "48000", "49200", "800", "51000"))
	rootZ.AddChild(sheetZone("5", sheets[3], "48000", "49200", "50000", "51000"))
	zoneStyle(rootZ, "8")
	addSimpleID(dash)
	return dash
}

func buildThreeRowDashboard(dashName string, sheets []string) *etree.Element {
	dash := newDashboardShell(dashName, 1000, 900)
	zones := dash.CreateElement("zones")
	rootZ := zones.CreateElement("zone")
	rootZ.CreateAttr("type-v2", "layout-basic")
	rootZ.CreateAttr("h", "100000")
	rootZ.CreateAttr("id", "1")
	rootZ.CreateAttr("w", "100000")
	rootZ.CreateAttr("x", "0")
	rootZ.CreateAttr("y", "0")
	rootZ.AddChild(sheetZone("2", sheets[0], "31600", "98400", "800", "1000"))
	rootZ.AddChild(sheetZone("3", sheets[1], "31600", "98400", "800", "34200"))
	rootZ.AddChild(sheetZone("4", sheets[2], "31600", "98400", "800", "67400"))
	zoneStyle(rootZ, "8")
	addSimpleID(dash)
	return dash
}
