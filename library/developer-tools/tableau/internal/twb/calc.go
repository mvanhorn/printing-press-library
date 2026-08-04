package twb

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// Calc describes a calculated field.
type Calc struct {
	Caption  string `json:"caption"`
	Name     string `json:"name"`
	Formula  string `json:"formula"`
	Datatype string `json:"datatype"`
	Role     string `json:"role"`
	Source   string `json:"source,omitempty"`
}

// ListCalcs returns calculated fields found under datasources.
// A calculated field is a <column> with a child <calculation class="tableau">.
// Parameter default-value calculations are included only when caption/name looks like a user calc;
// Parameters datasource columns with param-domain-type are skipped.
func (w *Workbook) ListCalcs() []Calc {
	var out []Calc
	dsParent := w.datasourcesParent()
	if dsParent == nil {
		return out
	}
	for _, ds := range dsParent.SelectElements("datasource") {
		dsName := ds.SelectAttrValue("caption", "")
		if dsName == "" {
			dsName = ds.SelectAttrValue("name", "")
		}
		// Skip pure Parameters datasource columns that are parameters.
		isParams := strings.EqualFold(ds.SelectAttrValue("name", ""), "Parameters") ||
			strings.EqualFold(ds.SelectAttrValue("caption", ""), "Parameters")

		for _, col := range ds.SelectElements("column") {
			if isParams && col.SelectAttr("param-domain-type") != nil {
				continue
			}
			calc := col.SelectElement("calculation")
			if calc == nil {
				continue
			}
			// Skip auto-columns like Number of Records that use user:auto-column.
			if attr := col.SelectAttr("auto-column"); attr != nil {
				continue
			}
			// etree stores namespaced attrs with space prefix sometimes.
			skipAuto := false
			for _, a := range col.Attr {
				if strings.HasSuffix(a.Key, "auto-column") || a.Key == "auto-column" {
					skipAuto = true
					break
				}
			}
			if skipAuto {
				continue
			}
			class := calc.SelectAttrValue("class", "")
			if class != "" && class != "tableau" {
				continue
			}
			out = append(out, Calc{
				Caption:  col.SelectAttrValue("caption", ""),
				Name:     col.SelectAttrValue("name", ""),
				Formula:  calc.SelectAttrValue("formula", ""),
				Datatype: col.SelectAttrValue("datatype", ""),
				Role:     col.SelectAttrValue("role", ""),
				Source:   dsName,
			})
		}
	}
	return out
}

// AddCalc appends a calculated field to the first suitable datasource.
// datatype defaults should be provided by the caller (e.g. "real"); role e.g. "measure".
func (w *Workbook) AddCalc(caption, formula, datatype, role string) error {
	if strings.TrimSpace(caption) == "" {
		return fmt.Errorf("caption is required")
	}
	if strings.TrimSpace(formula) == "" {
		return fmt.Errorf("formula is required")
	}
	if datatype == "" {
		datatype = "real"
	}
	if role == "" {
		role = "measure"
	}

	ds := w.pickDatasourceForCalc()
	if ds == nil {
		return fmt.Errorf("no datasource available to attach calculated field")
	}

	// Avoid duplicate captions in the same datasource.
	for _, col := range ds.SelectElements("column") {
		if col.SelectAttrValue("caption", "") == caption {
			return fmt.Errorf("calculated field with caption %q already exists", caption)
		}
	}

	name := fmt.Sprintf("[Calculation_%d]", calcID())
	typ := roleToType(role)

	col := ds.CreateElement("column")
	col.CreateAttr("caption", caption)
	col.CreateAttr("datatype", datatype)
	col.CreateAttr("name", name)
	col.CreateAttr("role", role)
	col.CreateAttr("type", typ)

	calc := col.CreateElement("calculation")
	calc.CreateAttr("class", "tableau")
	calc.CreateAttr("formula", formula)
	return nil
}

// CalcSpec is an input row for bulk calculated-field creation (Ann-style CY/PY packs).
type CalcSpec struct {
	Caption  string `json:"caption"`
	Formula  string `json:"formula"`
	Datatype string `json:"datatype,omitempty"`
	Role     string `json:"role,omitempty"`
}

// AddCalcs adds many calculated fields. Stops on first error (no partial commit of later rows).
func (w *Workbook) AddCalcs(specs []CalcSpec) error {
	if len(specs) == 0 {
		return fmt.Errorf("no calc specs provided")
	}
	for i, s := range specs {
		if err := w.AddCalc(s.Caption, s.Formula, s.Datatype, s.Role); err != nil {
			return fmt.Errorf("spec[%d] %q: %w", i, s.Caption, err)
		}
	}
	return nil
}

// BuildYoYPack builds Ann-style CY / PY / Delta / YoY% calcs for each measure field.
// dateField should be a Tableau field ref without outer brackets if plain, e.g. "Order Date".
// measures are field names like "Sales", "Profit".
func BuildYoYPack(measures []string, dateField string, cyYear, pyYear int) []CalcSpec {
	dateField = strings.TrimSpace(dateField)
	if dateField == "" {
		dateField = "Order Date"
	}
	dateRef := bracketField(dateField)
	var out []CalcSpec
	for _, m := range measures {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		mRef := bracketField(m)
		out = append(out,
			CalcSpec{
				Caption:  m + " CY",
				Formula:  fmt.Sprintf("IF YEAR(%s)=%d THEN %s END", dateRef, cyYear, mRef),
				Datatype: "real",
				Role:     "measure",
			},
			CalcSpec{
				Caption:  m + " PY",
				Formula:  fmt.Sprintf("IF YEAR(%s)=%d THEN %s END", dateRef, pyYear, mRef),
				Datatype: "real",
				Role:     "measure",
			},
			CalcSpec{
				Caption:  m + " Delta",
				Formula:  fmt.Sprintf("SUM([%s CY])-SUM([%s PY])", m, m),
				Datatype: "real",
				Role:     "measure",
			},
			CalcSpec{
				Caption:  m + " YoY %",
				Formula:  fmt.Sprintf("(SUM([%s CY])-SUM([%s PY]))/SUM([%s PY])", m, m, m),
				Datatype: "real",
				Role:     "measure",
			},
		)
	}
	return out
}

func bracketField(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "[") {
		return name
	}
	return "[" + name + "]"
}

func (w *Workbook) pickDatasourceForCalc() *etree.Element {
	dsParent := w.datasourcesParent()
	if dsParent == nil {
		return nil
	}
	var first *etree.Element
	var withCalcs *etree.Element
	for _, ds := range dsParent.SelectElements("datasource") {
		name := ds.SelectAttrValue("name", "")
		caption := ds.SelectAttrValue("caption", "")
		if strings.EqualFold(name, "Parameters") || strings.EqualFold(caption, "Parameters") {
			continue
		}
		if first == nil {
			first = ds
		}
		for _, col := range ds.SelectElements("column") {
			if col.SelectElement("calculation") != nil {
				withCalcs = ds
				break
			}
		}
		if withCalcs != nil {
			return withCalcs
		}
	}
	return first
}

func roleToType(role string) string {
	switch strings.ToLower(role) {
	case "dimension":
		return "nominal"
	default:
		return "quantitative"
	}
}

func calcID() int64 {
	// Tableau-like large id; stable enough uniqueness for CLI use.
	return time.Now().UnixNano()%1_000_000_000_000 + int64(rand.Intn(1_000_000))
}
