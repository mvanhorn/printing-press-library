package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Bulk-edit plan format (shared contract with `listings audit --plan-out`):
//
//	{"version":1,"generated_by":"<cmd>","items":[{"listing_id":1000000001,
//	  "field":"tags|title|price|materials|alt_text","op":"set|add|remove",
//	  "value":<string|number|[]string>,"image_id":<int, alt_text only>,
//	  "reason":"<optional>"}]}
//
// CSV carries the same columns. CSV list values are a JSON array, or tokens
// separated by "|" (or "," when no "|" is present); Etsy tags cannot contain
// either character.

// Etsy listing limits enforced client-side before any write.
const (
	bulkMaxTitleChars    = 140
	bulkMaxTags          = 13
	bulkMaxTagChars      = 20
	bulkMaxMaterials     = 13
	bulkMaxAltTextChars  = 500
	bulkPlanVersion      = 1
	bulkFieldTags        = "tags"
	bulkFieldTitle       = "title"
	bulkFieldPrice       = "price"
	bulkFieldMaterials   = "materials"
	bulkFieldAltText     = "alt_text"
	bulkOpSet            = "set"
	bulkOpAdd            = "add"
	bulkOpRemove         = "remove"
	bulkStatusOK         = "ok"
	bulkStatusUnresolved = "unresolved"
)

// bulkFlexInt decodes a JSON number or numeric string.
type bulkFlexInt int64

func (f *bulkFlexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("not an integer: %q", s)
	}
	*f = bulkFlexInt(n)
	return nil
}

type bulkPlanItem struct {
	ListingID bulkFlexInt `json:"listing_id"`
	Field     string      `json:"field"`
	Op        string      `json:"op,omitempty"`
	Value     any         `json:"value,omitempty"`
	ImageID   bulkFlexInt `json:"image_id,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

type bulkPlan struct {
	Version     int            `json:"version"`
	GeneratedBy string         `json:"generated_by,omitempty"`
	Items       []bulkPlanItem `json:"items"`
}

// bulkProblem names the offending listing and field.
type bulkProblem struct {
	ListingID int64  `json:"listing_id"`
	Field     string `json:"field"`
	Message   string `json:"message"`
}

func (p bulkProblem) String() string {
	return fmt.Sprintf("listing %d field %s: %s", p.ListingID, p.Field, p.Message)
}

// bulkChange is one resolved old->new change shown in the preview diff.
type bulkChange struct {
	Offerings []bulkOfferingChange `json:"offerings,omitempty"`
	ListingID int64                `json:"listing_id"`
	Field     string               `json:"field"`
	ImageID   int64                `json:"image_id,omitempty"`
	Op        string               `json:"op"`
	Old       any                  `json:"old"`
	New       any                  `json:"new"`
	Status    string               `json:"status"`
	Note      string               `json:"note,omitempty"`
	Reason    string               `json:"reason,omitempty"`
}

// bulkCurrent is the editable state of a listing (from the local store or a
// live GET) that changes are resolved against.
type bulkCurrent struct {
	Offerings       []bulkOffering
	VariationPrices bool
	Found           bool
	Title           string
	Tags            []string
	Materials       []string
	Price           *float64
	Alt             map[int64]string
}

func bulkLoadPlanFile(path string) (*bulkPlan, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-supplied --plan file is this command's documented input.
	if err != nil {
		return nil, fmt.Errorf("reading plan %s: %w", path, err)
	}
	return bulkParsePlan(raw, strings.EqualFold(filepath.Ext(path), ".csv"))
}

func bulkParsePlan(raw []byte, isCSV bool) (*bulkPlan, error) {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, fmt.Errorf("plan is empty")
	}
	if isCSV || (trim[0] != '{' && trim[0] != '[') {
		return bulkParseCSV(trim)
	}
	plan := &bulkPlan{}
	if trim[0] == '[' {
		plan.Version = bulkPlanVersion
		if err := json.Unmarshal(trim, &plan.Items); err != nil {
			return nil, fmt.Errorf("parsing plan JSON: %w", err)
		}
	} else if err := json.Unmarshal(trim, plan); err != nil {
		return nil, fmt.Errorf("parsing plan JSON: %w", err)
	}
	if plan.Version == 0 {
		plan.Version = bulkPlanVersion
	}
	if plan.Version != bulkPlanVersion {
		return nil, fmt.Errorf("unsupported plan version %d (this CLI reads version %d)", plan.Version, bulkPlanVersion)
	}
	return plan, nil
}

func bulkParseCSV(raw []byte) (*bulkPlan, error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing plan CSV: %w", err)
	}
	if len(recs) < 1 {
		return nil, fmt.Errorf("plan CSV has no header row")
	}
	col := map[string]int{}
	for i, h := range recs[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"listing_id", "field"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("plan CSV header must include %q (columns: listing_id,field,op,value,image_id,reason)", need)
		}
	}
	get := func(rec []string, name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	plan := &bulkPlan{Version: bulkPlanVersion, Items: make([]bulkPlanItem, 0, len(recs)-1)}
	for ln, rec := range recs[1:] {
		if len(rec) == 0 || (len(rec) == 1 && strings.TrimSpace(rec[0]) == "") {
			continue
		}
		var it bulkPlanItem
		id, err := strconv.ParseInt(get(rec, "listing_id"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("plan CSV row %d: listing_id %q is not an integer", ln+2, get(rec, "listing_id"))
		}
		it.ListingID = bulkFlexInt(id)
		it.Field = strings.ToLower(get(rec, "field"))
		it.Op = strings.ToLower(get(rec, "op"))
		it.Reason = get(rec, "reason")
		if v := get(rec, "image_id"); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("plan CSV row %d: image_id %q is not an integer", ln+2, v)
			}
			it.ImageID = bulkFlexInt(n)
		}
		val := get(rec, "value")
		switch {
		case val == "":
		case strings.HasPrefix(val, "["):
			var arr []any
			if err := json.Unmarshal([]byte(val), &arr); err != nil {
				return nil, fmt.Errorf("plan CSV row %d: value is not a JSON array: %w", ln+2, err)
			}
			it.Value = arr
		case it.Field == bulkFieldTags || it.Field == bulkFieldMaterials:
			sep := ","
			if strings.Contains(val, "|") {
				sep = "|"
			}
			arr := make([]any, 0)
			for _, p := range strings.Split(val, sep) {
				if p = strings.TrimSpace(p); p != "" {
					arr = append(arr, p)
				}
			}
			it.Value = arr
		case it.Field == bulkFieldPrice:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				it.Value = f
			} else {
				it.Value = val
			}
		default:
			it.Value = val
		}
		plan.Items = append(plan.Items, it)
	}
	return plan, nil
}

func bulkNormOp(it bulkPlanItem) string {
	op := strings.ToLower(strings.TrimSpace(it.Op))
	if op == "" {
		return bulkOpSet
	}
	return op
}

func bulkValueStrings(v any) ([]string, bool) {
	switch t := v.(type) {
	case nil:
		return nil, false
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, false
		}
		return []string{strings.TrimSpace(t)}, true
	case []string:
		return t, true
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			s, ok := e.(string)
			if !ok {
				return nil, false
			}
			out = append(out, strings.TrimSpace(s))
		}
		return out, true
	}
	return nil, false
}

func bulkValueNumber(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, !math.IsNaN(t) && !math.IsInf(t, 0)
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
}

func bulkRuneLen(s string) int { return utf8.RuneCountInString(s) }

// bulkValidateStatic checks each item's shape and explicit values against the
// Etsy limits. It needs no listing state.
func bulkValidateStatic(items []bulkPlanItem) []bulkProblem {
	probs := make([]bulkProblem, 0)
	if len(items) == 0 {
		return append(probs, bulkProblem{Field: "plan", Message: "plan has no items"})
	}
	for _, it := range items {
		id := int64(it.ListingID)
		add := func(msg string, a ...any) {
			probs = append(probs, bulkProblem{ListingID: id, Field: it.Field, Message: fmt.Sprintf(msg, a...)})
		}
		if id <= 0 {
			add("listing_id must be a positive integer")
			continue
		}
		op := bulkNormOp(it)
		switch it.Field {
		case bulkFieldTitle, bulkFieldPrice, bulkFieldAltText:
			if op != bulkOpSet {
				add("op %q is not supported for %s (use set)", op, it.Field)
				continue
			}
		case bulkFieldTags, bulkFieldMaterials:
			if op != bulkOpSet && op != bulkOpAdd && op != bulkOpRemove {
				add("op %q must be set, add or remove", op)
				continue
			}
		default:
			add("unknown field %q (use tags, title, price, materials, alt_text)", it.Field)
			continue
		}
		switch it.Field {
		case bulkFieldTitle:
			s, ok := bulkValueStrings(it.Value)
			if !ok || len(s) != 1 {
				add("title needs one non-empty string value")
			} else if n := bulkRuneLen(s[0]); n > bulkMaxTitleChars {
				add("title is %d characters; Etsy allows at most %d", n, bulkMaxTitleChars)
			}
		case bulkFieldPrice:
			if f, ok := bulkValueNumber(it.Value); !ok || f <= 0 {
				add("price needs a positive number")
			}
		case bulkFieldAltText:
			s, ok := bulkValueStrings(it.Value)
			switch {
			case it.ImageID <= 0:
				add("alt_text needs image_id (the existing listing image to describe)")
			case !ok || len(s) != 1:
				add("alt_text needs one non-empty string value")
			case bulkRuneLen(s[0]) > bulkMaxAltTextChars:
				add("alt_text is %d characters; at most %d allowed", bulkRuneLen(s[0]), bulkMaxAltTextChars)
			}
		case bulkFieldTags, bulkFieldMaterials:
			vals, ok := bulkValueStrings(it.Value)
			if !ok || len(vals) == 0 {
				add("%s needs a non-empty string or list of strings", it.Field)
				continue
			}
			limit := bulkMaxTags
			if it.Field == bulkFieldMaterials {
				limit = bulkMaxMaterials
			}
			for _, v := range vals {
				if v == "" {
					add("%s contains an empty entry", it.Field)
				} else if it.Field == bulkFieldTags && bulkRuneLen(v) > bulkMaxTagChars {
					add("tag %q is %d characters; Etsy allows at most %d", v, bulkRuneLen(v), bulkMaxTagChars)
				}
			}
			if op == bulkOpSet && len(vals) > limit {
				add("%d %s; Etsy allows at most %d", len(vals), it.Field, limit)
			}
		}
	}
	return probs
}

func bulkContainsFold(list []string, s string) int {
	for i, e := range list {
		if strings.EqualFold(e, s) {
			return i
		}
	}
	return -1
}

func bulkCloneStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

type bulkFieldKey struct {
	listing int64
	field   string
	image   int64
}

// bulkResolve applies items in plan order against the supplied current state
// and returns the net per-(listing,field) change plus any limit violations
// the final values would cause. Listings missing from cur resolve set ops
// without an old value and mark add/remove changes unresolved.
func bulkResolve(items []bulkPlanItem, cur map[int64]*bulkCurrent) ([]bulkChange, []bulkProblem) {
	type acc struct {
		change  bulkChange
		tags    []string
		known   bool
		order   int
		hasNew  bool
		reasons []string
	}
	accs := map[bulkFieldKey]*acc{}
	keys := make([]bulkFieldKey, 0)
	probs := make([]bulkProblem, 0)
	for _, it := range items {
		id := int64(it.ListingID)
		op := bulkNormOp(it)
		k := bulkFieldKey{id, it.Field, 0}
		if it.Field == bulkFieldAltText {
			k.image = int64(it.ImageID)
		}
		c := cur[id]
		if c == nil {
			c = &bulkCurrent{}
		}
		a := accs[k]
		if a == nil {
			a = &acc{change: bulkChange{ListingID: id, Field: it.Field, ImageID: k.image, Op: op, Status: bulkStatusOK, Reason: it.Reason}}
			switch it.Field {
			case bulkFieldTitle:
				if c.Found {
					a.change.Old = c.Title
				}
			case bulkFieldPrice:
				if c.Found && c.Price != nil {
					a.change.Old = *c.Price
				}
			case bulkFieldAltText:
				if c.Found {
					if v, ok := c.Alt[k.image]; ok {
						a.change.Old = v
					}
				}
			case bulkFieldTags:
				if c.Found {
					a.change.Old = bulkCloneStrings(c.Tags)
					a.tags, a.known = bulkCloneStrings(c.Tags), true
				}
			case bulkFieldMaterials:
				if c.Found {
					a.change.Old = bulkCloneStrings(c.Materials)
					a.tags, a.known = bulkCloneStrings(c.Materials), true
				}
			}
			accs[k] = a
			keys = append(keys, k)
		} else if a.change.Op != op {
			a.change.Op = "mixed"
		}
		if it.Reason != "" && a.change.Reason == "" {
			a.change.Reason = it.Reason
		}
		switch it.Field {
		case bulkFieldTitle, bulkFieldAltText:
			s, _ := bulkValueStrings(it.Value)
			if len(s) == 1 {
				a.change.New, a.hasNew = s[0], true
			}
		case bulkFieldPrice:
			if f, ok := bulkValueNumber(it.Value); ok {
				a.change.New, a.hasNew = f, true
			}
		case bulkFieldTags, bulkFieldMaterials:
			vals, _ := bulkValueStrings(it.Value)
			switch op {
			case bulkOpSet:
				a.tags, a.known, a.hasNew = nil, true, true
				for _, v := range vals {
					if bulkContainsFold(a.tags, v) < 0 {
						a.tags = append(a.tags, v)
					}
				}
			case bulkOpAdd, bulkOpRemove:
				if !a.known {
					a.change.Status = bulkStatusUnresolved
					a.change.Note = "listing is not in the local store; run `etsy-pp-cli sync` to preview add/remove (it is resolved against the live listing on --apply)"
					continue
				}
				for _, v := range vals {
					i := bulkContainsFold(a.tags, v)
					if op == bulkOpAdd && i < 0 {
						a.tags = append(a.tags, v)
					} else if op == bulkOpRemove && i >= 0 {
						a.tags = append(a.tags[:i], a.tags[i+1:]...)
					}
				}
				a.hasNew = true
			}
		}
	}
	out := make([]bulkChange, 0, len(keys))
	for _, k := range keys {
		a := accs[k]
		if k.field == bulkFieldTags || k.field == bulkFieldMaterials {
			if a.known && a.hasNew {
				if a.tags == nil {
					a.tags = []string{}
				}
				a.change.New = a.tags
				limit := bulkMaxTags
				if k.field == bulkFieldMaterials {
					limit = bulkMaxMaterials
				}
				if len(a.tags) > limit {
					probs = append(probs, bulkProblem{k.listing, k.field, fmt.Sprintf("result would have %d %s; Etsy allows at most %d", len(a.tags), k.field, limit)})
				}
				if k.field == bulkFieldTags {
					for _, t := range a.tags {
						if bulkRuneLen(t) > bulkMaxTagChars {
							probs = append(probs, bulkProblem{k.listing, k.field, fmt.Sprintf("tag %q is %d characters; Etsy allows at most %d", t, bulkRuneLen(t), bulkMaxTagChars)})
						}
					}
				}
			}
		}
		if a.change.Note == "" && !cur[k.listing].bulkFound() {
			a.change.Note = "listing is not in the local store; old value unknown"
		}
		out = append(out, a.change)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ListingID < out[j].ListingID })
	return out, probs
}

func (c *bulkCurrent) bulkFound() bool { return c != nil && c.Found }

// bulkDedupeProblems removes repeated identical problems, preserving order.
func bulkDedupeProblems(in []bulkProblem) []bulkProblem {
	seen := map[string]bool{}
	out := make([]bulkProblem, 0, len(in))
	for _, p := range in {
		if k := p.String(); !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	return out
}

// bulkPlanHash identifies a plan's content so the journal can resume it.
func bulkPlanItemsCanonical(items []bulkPlanItem) []byte {
	b, _ := json.Marshal(items)
	return b
}

type bulkOffering struct {
	ProductID  string  `json:"product_id"`
	SKU        string  `json:"sku"`
	OfferingID string  `json:"offering_id"`
	Price      float64 `json:"price"`
}
type bulkOfferingChange struct {
	ProductID  string  `json:"product_id"`
	SKU        string  `json:"sku"`
	OfferingID string  `json:"offering_id"`
	Old        float64 `json:"old"`
	New        float64 `json:"new"`
}

func bulkInventoryCurrent(c *bulkCurrent, raw []byte) {
	m, err := s2DecodeObject(raw)
	if err != nil {
		return
	}
	props, _ := m["price_on_property"].([]any)
	c.VariationPrices = len(props) > 0
	products, _ := m["products"].([]any)
	for _, p := range products {
		prod, ok := p.(map[string]any)
		if !ok || s2Bool(prod["is_deleted"]) {
			continue
		}
		offs, _ := prod["offerings"].([]any)
		for _, o := range offs {
			off, ok := o.(map[string]any)
			if !ok || s2Bool(off["is_deleted"]) {
				continue
			}
			price, _ := s2MoneyFloat(off["price"])
			c.Offerings = append(c.Offerings, bulkOffering{ProductID: s2Str(prod["product_id"]), SKU: s2Str(prod["sku"]), OfferingID: s2Str(off["offering_id"]), Price: price})
		}
	}
	for _, off := range c.Offerings {
		if off.Price != c.Offerings[0].Price {
			c.VariationPrices = true
		}
	}
}

func bulkApplyVariationRules(changes []bulkChange, cur map[int64]*bulkCurrent, flatten bool) ([]bulkChange, []bulkProblem) {
	var problems []bulkProblem
	for i := range changes {
		ch := &changes[i]
		c := cur[ch.ListingID]
		if ch.Field != bulkFieldPrice || c == nil {
			continue
		}
		price, ok := ch.New.(float64)
		if !ok {
			continue
		}
		for _, o := range c.Offerings {
			ch.Offerings = append(ch.Offerings, bulkOfferingChange{ProductID: o.ProductID, SKU: o.SKU, OfferingID: o.OfferingID, Old: o.Price, New: price})
		}
		if c.VariationPrices && !flatten {
			problems = append(problems, bulkProblem{ListingID: ch.ListingID, Field: "price", Message: fmt.Sprintf("per-variation prices (stored inventory): a price change flattens %d offerings to one price; pass --flatten-variation-prices to allow", len(c.Offerings))})
		}
	}
	return changes, problems
}
