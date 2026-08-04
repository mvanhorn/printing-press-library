package twb

import (
	"fmt"

	"github.com/beevik/etree"
)

// ApplyStyle copies style-ish subtrees from worksheet "from" onto worksheet "to".
// It replaces destination <style> nodes under the worksheet (and table/panes)
// with deep copies of the corresponding source styles when present.
func (w *Workbook) ApplyStyle(from, to string) error {
	if from == "" || to == "" {
		return fmt.Errorf("both --from and --to sheet names are required")
	}
	src := w.findWorksheet(from)
	if src == nil {
		return fmt.Errorf("source worksheet %q not found", from)
	}
	dst := w.findWorksheet(to)
	if dst == nil {
		return fmt.Errorf("destination worksheet %q not found", to)
	}

	// Prefer table/style (primary formatting) then worksheet-level style.
	copyStyleAt(src, dst, []string{"table", "style"})
	copyStyleAt(src, dst, []string{"style"})

	// Pane-level styles: align by index under table/panes/pane.
	srcPanes := pathElement(src, "table", "panes")
	dstPanes := pathElement(dst, "table", "panes")
	if srcPanes != nil && dstPanes != nil {
		srcList := srcPanes.SelectElements("pane")
		dstList := dstPanes.SelectElements("pane")
		n := len(srcList)
		if len(dstList) < n {
			n = len(dstList)
		}
		for i := 0; i < n; i++ {
			replaceChildStyle(srcList[i], dstList[i])
		}
	}
	return nil
}

func pathElement(el *etree.Element, tags ...string) *etree.Element {
	cur := el
	for _, t := range tags {
		if cur == nil {
			return nil
		}
		cur = cur.SelectElement(t)
	}
	return cur
}

// copyStyleAt finds path under src and replaces the matching style under dst.
// tags is a path ending in "style" (or any last tag that is the style node name).
func copyStyleAt(src, dst *etree.Element, tags []string) {
	if len(tags) == 0 {
		return
	}
	srcStyle := pathElement(src, tags...)
	if srcStyle == nil {
		return
	}
	// Navigate to parent path on destination.
	parentTags := tags[:len(tags)-1]
	dstParent := dst
	if len(parentTags) > 0 {
		dstParent = pathElement(dst, parentTags...)
		if dstParent == nil {
			// Build missing intermediate path.
			dstParent = dst
			for _, t := range parentTags {
				next := dstParent.SelectElement(t)
				if next == nil {
					next = dstParent.CreateElement(t)
				}
				dstParent = next
			}
		}
	}
	replaceNamedChild(dstParent, tags[len(tags)-1], srcStyle)
}

func replaceChildStyle(srcPane, dstPane *etree.Element) {
	srcStyle := srcPane.SelectElement("style")
	if srcStyle == nil {
		return
	}
	replaceNamedChild(dstPane, "style", srcStyle)
}

func replaceNamedChild(parent *etree.Element, tag string, src *etree.Element) {
	if parent == nil || src == nil {
		return
	}
	if existing := parent.SelectElement(tag); existing != nil {
		parent.RemoveChild(existing)
	}
	parent.AddChild(cloneElement(src))
}
