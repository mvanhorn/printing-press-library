package jma

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type WarningKind struct {
	Code       string   `json:"code"`
	Status     string   `json:"status"`
	Additions  []string `json:"additions"`
	Properties any      `json:"properties"`
}
type WarningItem struct {
	Code  string        `json:"areaCode"`
	Kinds []WarningKind `json:"kinds"`
}
type WarningProduct struct {
	Code      string `json:"dataTypeCode"`
	Issued    string `json:"reportDatetime"`
	Control   string `json:"controlDatetime"`
	Info      string `json:"infoType"`
	Publisher string `json:"publishingOffice"`
	Headline  string `json:"headlineText"`
	Warning   struct {
		Municipalities []WarningItem `json:"class20Items"`
		Districts      []WarningItem `json:"class10Items"`
		Coastal        any           `json:"tidalItems"`
	} `json:"warning"`
}
type Hazard struct {
	JA         string `json:"name_ja"`
	EN         string `json:"name_en"`
	Severity   string `json:"severity"`
	AlertLevel any    `json:"alert_level"`
}

// Codes and labels come from Warning.Common h in JMA's r8 frontend, captured 2026-10-01.
func hazard(code string) (Hazard, bool) {
	table := map[string]struct {
		name, en string
		level    int
	}{
		"33": {"大雨", "Heavy Rain", 5}, "43": {"大雨", "Heavy Rain", 4}, "03": {"大雨", "Heavy Rain", 3}, "10": {"大雨", "Heavy Rain", 2},
		"39": {"土砂災害", "Landslide", 5}, "49": {"土砂災害", "Landslide", 4}, "09": {"土砂災害", "Landslide", 3}, "29": {"土砂災害", "Landslide", 2},
		"38": {"高潮", "Storm Surge", 5}, "48": {"高潮", "Storm Surge", 4}, "08": {"高潮", "Storm Surge", 3}, "19": {"高潮", "Storm Surge", 2},
		"35": {"暴風", "Storm", 5}, "05": {"暴風", "Storm", 3}, "15": {"強風", "Gale", 2}, "32": {"暴風雪", "Snowstorm", 5}, "02": {"暴風雪", "Snowstorm", 3}, "13": {"風雪", "Gale and Snow", 2},
		"36": {"大雪", "Heavy Snow", 5}, "06": {"大雪", "Heavy Snow", 3}, "12": {"大雪", "Heavy Snow", 2}, "37": {"波浪", "High Wave", 5}, "07": {"波浪", "High Wave", 3}, "16": {"波浪", "High Wave", 2},
		"14": {"雷", "Thunder Storm", 2}, "17": {"融雪", "Snow Melting", 2}, "20": {"濃霧", "Dense Fog", 2}, "21": {"乾燥", "Dry Air", 2}, "22": {"なだれ", "Avalanche", 2}, "23": {"低温", "Low Temperature", 2}, "24": {"霜", "Frost", 2}, "25": {"着氷", "Ice Accretion", 2}, "26": {"着雪", "Snow Accretion", 2}}
	x, ok := table[code]
	if !ok {
		return Hazard{JA: "unknown", EN: "unknown", Severity: "unknown"}, false
	}
	sev, suffix, english := "advisory", "注意報", "Advisory"
	switch x.level {
	case 3:
		sev, suffix, english = "warning", "警報", "Warning"
	case 4:
		sev, suffix, english = "urgent_warning", "危険警報", "Urgent Warning"
	case 5:
		sev, suffix, english = "emergency_warning", "特別警報", "Emergency Warning"
	}
	name := x.name + suffix
	var alert any
	if x.name == "大雨" || x.name == "土砂災害" || x.name == "高潮" {
		name = fmt.Sprintf("レベル%s%s", map[int]string{2: "２", 3: "３", 4: "４", 5: "５"}[x.level], name)
		alert = x.level
	}
	return Hazard{name, x.en + " " + english, sev, alert}, true
}
func lifecycle(status string) (life string, active any, known bool) {
	switch status {
	case "発表":
		return "issued", true, true
	case "継続":
		return "continued", true, true
	case "解除":
		return "lifted", false, true
	case "発表警報・注意報はなし":
		return "none", false, true
	}
	if map[string]bool{"特別警報から警報": true, "特別警報から注意報": true, "警報から注意報": true, "危険警報から警報": true, "危険警報から注意報": true, "特別警報から危険警報": true}[status] {
		return "updated", true, true
	}
	return "unknown", nil, false
}
func (c *Client) Warnings(ctx context.Context, query string, detail bool, offset, limit int) (Envelope, error) {
	inv, e := c.Inventory(ctx, false)
	if e != nil {
		return Envelope{}, e
	}
	p, e := inv.Resolve(query)
	if e != nil {
		return Envelope{}, e
	}
	ids := []string{}
	for id := range inv.Areas["class20s"] {
		if inv.descendant("class20s", id, p.ID) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return Envelope{}, failure(5, "incomplete", "no municipality mapping for %s", p.ID)
	}
	var products []WarningProduct
	if e = c.Get(ctx, "/warning/data/r8/"+p.OfficeID+".json", time.Minute, &products); e != nil {
		return Envelope{}, e
	}
	expected := map[string]bool{"VPWW55": false, "VPWW56": false, "VPWW58": false, "VPWW59": false, "VPWW61": false}
	allNoWaveTide := true
	for _, id := range ids {
		allNoWaveTide = allNoWaveTide && inv.noWaveTide(p.OfficeID, id)
	}
	if allNoWaveTide {
		expected["VPWW59"] = true
	}
	uncertain := false
	byProduct := map[string]WarningProduct{}
	for _, v := range products {
		if _, ok := expected[v.Code]; !ok {
			uncertain = true
		}
		if v.Code == "" || len(v.Warning.Municipalities) == 0 || v.Publisher == "" {
			return Envelope{}, failure(5, "incomplete", "warning product lacks identity, publisher or municipality items")
		}
		if _, err := parseTime(v.Issued); err != nil {
			return Envelope{}, err
		}
		if v.Info != "発表" {
			uncertain = true
		}
		if old, ok := byProduct[v.Code]; ok {
			if old.Issued >= v.Issued {
				continue
			}
		}
		byProduct[v.Code] = v
		expected[v.Code] = true
	}
	missingProducts := []string{}
	for id, ok := range expected {
		if !ok {
			missingProducts = append(missingProducts, id)
		}
	}
	sort.Strings(missingProducts)
	productCodes := []string{}
	for id := range byProduct {
		productCodes = append(productCodes, id)
	}
	sort.Strings(productCodes)
	summaries := []map[string]any{}
	indexes := map[string]map[string]WarningItem{}
	for _, id := range productCodes {
		v := byProduct[id]
		index := map[string]WarningItem{}
		for _, a := range v.Warning.Municipalities {
			if _, ok := index[a.Code]; ok {
				return Envelope{}, failure(5, "format", "duplicate warning municipality %s", a.Code)
			}
			index[a.Code] = a
		}
		indexes[id] = index
		issue, _ := timeJST(v.Issued)
		summaries = append(summaries, map[string]any{"id": v.Code, "issued_at": issue, "issue_age_hours": ageHours(c.now(), v.Issued), "publisher": v.Publisher, "headline_ja": v.Headline, "headline_scope": "issuing office area", "info_type_ja": v.Info})
	}
	rows := []map[string]any{}
	globalActive := false
	incompleteCount := 0
	for _, id := range ids {
		events := []map[string]any{}
		noWarnings := []string{}
		missing := []string{}
		active := false
		complete := !uncertain
		notApplicable := []string{}
		for _, pid := range []string{"VPWW55", "VPWW56", "VPWW58", "VPWW59", "VPWW61"} {
			v := byProduct[pid]
			a, ok := indexes[pid][id]
			if pid == "VPWW59" && inv.noWaveTide(p.OfficeID, id) {
				clear := true
				for _, k := range a.Kinds {
					_, active, known := lifecycle(k.Status)
					_, validCode := hazard(k.Code)
					if !known || active == true || k.Status == "発表警報・注意報はなし" && k.Code != "" || k.Status != "発表警報・注意報はなし" && !validCode {
						clear = false
					}
				}
				if clear {
					notApplicable = append(notApplicable, pid)
					continue
				}
				complete = false
			}
			if !ok || len(a.Kinds) == 0 {
				complete = false
				missing = append(missing, pid)
				continue
			}
			for _, k := range a.Kinds {
				life, act, known := lifecycle(k.Status)
				if life == "none" {
					if k.Code != "" {
						complete = false
					}
					noWarnings = append(noWarnings, pid)
					continue
				}
				h, codeKnown := hazard(k.Code)
				if !known || !codeKnown {
					complete = false
				}
				if act == true {
					active = true
				}
				issued, _ := timeJST(v.Issued)
				event := map[string]any{"hazard_id": nullable(k.Code), "hazard": h, "status_ja": k.Status, "lifecycle": life, "in_effect": act, "product_id": pid, "issued_at": issued, "valid_until": nil}
				if detail {
					event["additions_ja"] = k.Additions
					event["source_properties"] = k.Properties
				}
				events = append(events, event)
			}
		}
		state := "none_reported"
		if active {
			state = "active"
			globalActive = true
		}
		if !complete {
			state = "incomplete"
			incompleteCount++
		}
		rows = append(rows, map[string]any{"area_id": id, "name_ja": inv.Areas["class20s"][id].Name, "name_en": nullable(inv.Areas["class20s"][id].EnName), "state": state, "has_active_hazards": active, "complete_municipality_products": complete, "no_warning_product_ids": noWarnings, "missing_product_ids": missing, "not_applicable_product_ids": notApplicable, "events": events, "url": canonical("warning", "class20s", id)})
	}
	paged, page, e := pageRows(rows, offset, limit)
	if e != nil {
		return Envelope{}, e
	}
	state := "none_reported"
	if globalActive {
		state = "active"
	}
	if incompleteCount > 0 {
		state = "incomplete"
	}
	coverage := "partial: municipality weather warnings only; river flood bulletins and coastal-zone supplements excluded"
	notes := []string{"State summarizes all resolved municipalities before pagination; events retain lifted and updated source statuses.", "none_reported applies only to the retrieved municipality products; it is not personal safety clearance. Warnings have no source expiry time; issue age is not expiration."}
	if incompleteCount > 0 {
		notes = append(notes, "Missing or unfamiliar source records prevent a no-warning conclusion.")
	}
	result := map[string]any{"area": p, "state": state, "has_active_hazards": globalActive, "municipality_count": len(ids), "incomplete_municipality_count": incompleteCount, "missing_product_ids": missingProducts, "products": summaries, "municipalities": paged, "url": canonical("warning", "offices", p.OfficeID)}
	v := c.Envelope(result, coverage, notes...)
	v.Page = page
	if incompleteCount > 0 {
		return v, failure(5, "incomplete", "%d municipality records have missing/unknown source coverage", incompleteCount)
	}
	return v, nil
}
