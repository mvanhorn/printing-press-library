package twb

import (
	"fmt"

	"github.com/beevik/etree"
)

// ListSheets returns worksheet names in document order.
func (w *Workbook) ListSheets() []string {
	parent := w.Root().SelectElement("worksheets")
	if parent == nil {
		return nil
	}
	var names []string
	for _, el := range parent.SelectElements("worksheet") {
		names = append(names, el.SelectAttrValue("name", ""))
	}
	return names
}

// CloneSheet deep-copies worksheet named from into a new worksheet named to.
func (w *Workbook) CloneSheet(from, to string) error {
	if from == "" || to == "" {
		return fmt.Errorf("both --from and --to sheet names are required")
	}
	if from == to {
		return fmt.Errorf("source and destination sheet names must differ")
	}
	src := w.findWorksheet(from)
	if src == nil {
		return fmt.Errorf("worksheet %q not found", from)
	}
	if w.findWorksheet(to) != nil {
		return fmt.Errorf("worksheet %q already exists", to)
	}

	parent := w.worksheetsParent()
	clone := cloneElement(src)
	if attr := clone.SelectAttr("name"); attr != nil {
		attr.Value = to
	} else {
		clone.CreateAttr("name", to)
	}
	// Drop simple-id so Tableau can assign a new one.
	if sid := clone.SelectElement("simple-id"); sid != nil {
		clone.RemoveChild(sid)
	}
	parent.AddChild(clone)

	// Mirror a windows entry when the source has one (keeps Desktop happier).
	w.cloneWindowEntry(from, to)
	return nil
}

func (w *Workbook) cloneWindowEntry(from, to string) {
	windows := w.Root().SelectElement("windows")
	if windows == nil {
		return
	}
	var srcWin *etree.Element
	for _, win := range windows.SelectElements("window") {
		if win.SelectAttrValue("class", "") == "worksheet" && win.SelectAttrValue("name", "") == from {
			srcWin = win
			break
		}
	}
	if srcWin == nil {
		// Minimal worksheet window.
		win := windows.CreateElement("window")
		win.CreateAttr("class", "worksheet")
		win.CreateAttr("name", to)
		return
	}
	// Avoid duplicates.
	for _, win := range windows.SelectElements("window") {
		if win.SelectAttrValue("name", "") == to {
			return
		}
	}
	c := cloneElement(srcWin)
	if attr := c.SelectAttr("name"); attr != nil {
		attr.Value = to
	} else {
		c.CreateAttr("name", to)
	}
	if sid := c.SelectElement("simple-id"); sid != nil {
		c.RemoveChild(sid)
	}
	windows.AddChild(c)
}
