package twb

import (
	"fmt"
	"strings"

	"github.com/beevik/etree"
)

// Severity levels for lint issues.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Issue is a single lint finding.
type Issue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

// Lint runs structural and known-illegal-pattern checks.
// Checks:
//   - well-formed root <workbook>
//   - duplicate worksheet names
//   - empty calculated-field formulas
//   - attribute value exactly "bold" (illegal enum agents invent; Ann Jackson case)
//   - basic structure (worksheet missing name; dashboard missing name)
func (w *Workbook) Lint() []Issue {
	var issues []Issue
	root := w.Root()
	if root == nil || root.Tag != "workbook" {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Code:     "not-workbook",
			Message:  "root element is not <workbook>",
		})
		return issues
	}

	// Duplicate sheet names.
	seen := map[string]int{}
	if parent := root.SelectElement("worksheets"); parent != nil {
		for i, el := range parent.SelectElements("worksheet") {
			name := el.SelectAttrValue("name", "")
			path := fmt.Sprintf("worksheets/worksheet[%d]", i)
			if name == "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Code:     "sheet-missing-name",
					Message:  "worksheet missing name attribute",
					Path:     path,
				})
				continue
			}
			seen[name]++
		}
		for name, n := range seen {
			if n > 1 {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Code:     "duplicate-sheet-name",
					Message:  fmt.Sprintf("duplicate worksheet name %q (%d occurrences)", name, n),
					Path:     "worksheets",
				})
			}
		}
	}

	// Empty calc formulas + structure under datasources.
	if dsParent := root.SelectElement("datasources"); dsParent != nil {
		for di, ds := range dsParent.SelectElements("datasource") {
			for ci, col := range ds.SelectElements("column") {
				calc := col.SelectElement("calculation")
				if calc == nil {
					continue
				}
				// Skip parameters.
				if col.SelectAttr("param-domain-type") != nil {
					continue
				}
				formula := calc.SelectAttrValue("formula", "")
				if strings.TrimSpace(formula) == "" {
					caption := col.SelectAttrValue("caption", col.SelectAttrValue("name", ""))
					issues = append(issues, Issue{
						Severity: SeverityError,
						Code:     "empty-calc-formula",
						Message:  fmt.Sprintf("calculated field %q has empty formula", caption),
						Path:     fmt.Sprintf("datasources/datasource[%d]/column[%d]", di, ci),
					})
				}
			}
		}
	}

	// Dashboards basic structure + Ann failure modes (missing zones / simple-id).
	if dashParent := root.SelectElement("dashboards"); dashParent != nil {
		for i, d := range dashParent.SelectElements("dashboard") {
			path := fmt.Sprintf("dashboards/dashboard[%d]", i)
			name := d.SelectAttrValue("name", "")
			if name == "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Code:     "dashboard-missing-name",
					Message:  "dashboard missing name attribute",
					Path:     path,
				})
			}
			zonesEl := d.SelectElement("zones")
			zoneCount := 0
			if zonesEl != nil {
				for range zonesEl.FindElements(".//zone") {
					zoneCount++
				}
			}
			if zonesEl == nil || zoneCount == 0 {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Code:     "dashboard-missing-zones",
					Message:  fmt.Sprintf("dashboard %q has no zones (Desktop will fail content model)", name),
					Path:     path,
				})
			}
			if d.SelectElement("simple-id") == nil {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Code:     "dashboard-missing-simple-id",
					Message:  fmt.Sprintf("dashboard %q missing required simple-id (Ann content-model failure)", name),
					Path:     path,
				})
			}
			// Sheet zones must reference existing worksheets.
			sheetSet := map[string]struct{}{}
			if wp := root.SelectElement("worksheets"); wp != nil {
				for _, ws := range wp.SelectElements("worksheet") {
					sheetSet[ws.SelectAttrValue("name", "")] = struct{}{}
				}
			}
			if zonesEl != nil {
				for _, z := range zonesEl.FindElements(".//zone") {
					zn := z.SelectAttrValue("name", "")
					if zn == "" {
						continue
					}
					// layout containers may not have worksheet names
					if z.SelectAttrValue("type-v2", "") != "" && strings.HasPrefix(z.SelectAttrValue("type-v2", ""), "layout-") {
						continue
					}
					if _, ok := sheetSet[zn]; !ok {
						// Only flag if it looks like a sheet zone (has name, no type-v2 layout)
						if z.SelectAttr("type-v2") == nil && z.SelectAttr("type") == nil {
							issues = append(issues, Issue{
								Severity: SeverityError,
								Code:     "dashboard-unknown-sheet",
								Message:  fmt.Sprintf("dashboard %q zone references unknown sheet %q", name, zn),
								Path:     path + "/zones",
							})
						}
					}
				}
			}
		}
	}

	// Flag attribute value exactly "bold" — illegal enum agents invent.
	// Valid Tableau uses e.g. bold="true" on <run>, not value="bold".
	walkAttrs(root, "", func(path, key, value string) {
		if value == "bold" {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Code:     "illegal-bold-enum",
				Message:  fmt.Sprintf("illegal attribute value exactly %q on %s (@%s); Tableau rejects this enum", "bold", path, key),
				Path:     path,
			})
		}
	})

	return issues
}

// HasErrors reports whether any issue is severity error.
func HasErrors(issues []Issue) bool {
	for _, iss := range issues {
		if iss.Severity == SeverityError {
			return true
		}
	}
	return false
}

func walkAttrs(el *etree.Element, path string, fn func(path, key, value string)) {
	if el == nil {
		return
	}
	p := path
	if p == "" {
		p = el.Tag
	} else {
		p = path + "/" + el.Tag
	}
	for _, a := range el.Attr {
		fn(p, a.Key, a.Value)
	}
	for _, child := range el.ChildElements() {
		walkAttrs(child, p, fn)
	}
}
