// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
)

// PATCH: Add local read-only transcendence query helpers over the synced HubSpot SQLite mirror.

type staleLeadRow struct {
	ContactID          string `json:"contactId"`
	Email              string `json:"email,omitempty"`
	FirstName          string `json:"firstName,omitempty"`
	LastName           string `json:"lastName,omitempty"`
	LifecycleStage     string `json:"lifecycleStage"`
	DaysSinceTouch     int    `json:"days_since_touch"`
	LastEngagementType string `json:"last_engagement_type,omitempty"`
	LastEngagementAt   string `json:"last_engagement_at,omitempty"`
}

type staleLeadsResult struct {
	Rows  []staleLeadRow `json:"rows"`
	Total int            `json:"total"`
	AsOf  string         `json:"as_of"`
}

type pipelineHealthRow struct {
	DealID         string  `json:"dealId"`
	DealName       string  `json:"dealname"`
	Stage          string  `json:"stage"`
	StageLabel     string  `json:"stage_label,omitempty"`
	Amount         float64 `json:"amount"`
	Probability    float64 `json:"probability"`
	WeightedValue  float64 `json:"weighted_value"`
	OwnerID        string  `json:"ownerId,omitempty"`
	AgeInStageDays int     `json:"age_in_stage_days"`
}

type pipelineStageBreakdown struct {
	Count int     `json:"count"`
	Total float64 `json:"total"`
}

type pipelineHealthResult struct {
	Rows               []pipelineHealthRow               `json:"rows"`
	Total              int                               `json:"total"`
	TotalWeightedValue float64                           `json:"total_weighted_value"`
	StageBreakdown     map[string]pipelineStageBreakdown `json:"stage_breakdown"`
}

type recentIntakeRow struct {
	ContactID    string `json:"contactId"`
	Email        string `json:"email,omitempty"`
	FirstName    string `json:"firstName,omitempty"`
	LastName     string `json:"lastName,omitempty"`
	CreatedAt    string `json:"createdAt"`
	Source       string `json:"hs_analytics_source,omitempty"`
	SourceData1  string `json:"hs_analytics_source_data_1,omitempty"`
	SourceData2  string `json:"hs_analytics_source_data_2,omitempty"`
	LatestSource string `json:"hs_latest_source,omitempty"`
	UtmSource    string `json:"utm_source,omitempty"`
	UtmMedium    string `json:"utm_medium,omitempty"`
	UtmCampaign  string `json:"utm_campaign,omitempty"`
}

type recentIntakeResult struct {
	Rows  []recentIntakeRow `json:"rows"`
	Total int               `json:"total"`
	AsOf  string            `json:"as_of"`
}

type dedupCluster struct {
	Key      string         `json:"key"`
	Count    int            `json:"count"`
	Contacts []dedupContact `json:"contacts"`
}

type dedupContact struct {
	ContactID string `json:"contactId"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Company   string `json:"company,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type dedupResult struct {
	Rows  []dedupCluster `json:"rows"`
	Total int            `json:"total"`
}

type engagementDecayRow struct {
	ContactID   string `json:"contactId"`
	Email       string `json:"email,omitempty"`
	PriorCount  int    `json:"prev_count"`
	RecentCount int    `json:"curr_count"`
	Delta       int    `json:"delta"`
}

type engagementDecayResult struct {
	Rows  []engagementDecayRow `json:"rows"`
	Total int                  `json:"total"`
	AsOf  string               `json:"as_of"`
}

type leadTrace struct {
	ContactID         string                `json:"contactId"`
	Contact           map[string]any        `json:"contact"`
	Deals             []leadTraceDeal       `json:"deals"`
	Companies         []leadTraceCompany    `json:"companies"`
	Engagements       []leadTraceEngagement `json:"engagements"`
	SourceAttribution map[string]string     `json:"source_attribution"`
}

type leadTraceDeal struct {
	DealID    string  `json:"dealId"`
	DealName  string  `json:"dealname,omitempty"`
	Stage     string  `json:"stage,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	Pipeline  string  `json:"pipeline,omitempty"`
	CloseDate string  `json:"closedate,omitempty"`
}

type leadTraceCompany struct {
	CompanyID string `json:"companyId"`
	Name      string `json:"name,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Industry  string `json:"industry,omitempty"`
}

type leadTraceEngagement struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Subject   string `json:"subject,omitempty"`
}

type utmCohortResult struct {
	Campaign         string              `json:"campaign"`
	CohortSize       int                 `json:"cohort_size"`
	TotalDeals       int                 `json:"total_deals"`
	PercentClosedWon float64             `json:"percent_closed_won"`
	AvgDealAmount    float64             `json:"avg_deal_amount"`
	StageBreakdown   []utmCohortStageRow `json:"stage_breakdown"`
}

type utmCohortStageRow struct {
	Stage       string  `json:"stage"`
	StageLabel  string  `json:"stage_label,omitempty"`
	Count       int     `json:"count"`
	TotalAmount float64 `json:"total_amount"`
}

type digestResult struct {
	Since          string              `json:"since"`
	AsOf           string              `json:"as_of"`
	NewContacts    []recentIntakeRow   `json:"new_contacts"`
	DealsAdvanced  []pipelineHealthRow `json:"deals_advanced"`
	DealsClosedWon []pipelineHealthRow `json:"deals_closed_won"`
	StalestLeads   []staleLeadRow      `json:"stalest_leads"`
	RecentIntake   []recentIntakeRow   `json:"recent_intake"`
}

type stageInfo struct {
	Label       string
	Probability float64
}

type engagementTouch struct {
	When time.Time
	Type string
}

var engagementTables = []struct {
	table       string
	kind        string
	subjectProp string
}{
	{"calls_crm", "calls", "hs_call_title"},
	{"emails_crm", "emails", "hs_email_subject"},
	{"meetings_crm", "meetings", "hs_meeting_title"},
	{"notes_crm", "notes", "hs_note_body"},
	{"tasks_crm", "tasks", "hs_task_subject"},
}

func openTranscendenceStore(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("hubspot-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return db, nil
}

func marshalAndPrint(cmdOut interface{ Write([]byte) (int, error) }, payload any, flags *rootFlags) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmdOut, data, flags)
}

func shouldStructured(flags *rootFlags, out interface{}) bool {
	w, ok := out.(interface{ Write([]byte) (int, error) })
	_ = w
	return (flags.asJSON || (ok && !isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain))
}

func parseLookbackDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func loadJSONRows(ctx context.Context, db *store.Store, table string) ([]map[string]any, error) {
	rows, err := db.DB().QueryContext(ctx, fmt.Sprintf(`SELECT data FROM %s`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

func loadJSONByID(ctx context.Context, db *store.Store, table, id string) (map[string]any, error) {
	var raw []byte
	if err := db.DB().QueryRowContext(ctx, fmt.Sprintf(`SELECT data FROM %s WHERE id = ?`, table), id).Scan(&raw); err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func countTable(ctx context.Context, db *store.Store, table string) (int, error) {
	var count int
	if err := db.DB().QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func objID(obj map[string]any) string {
	return asString(obj["id"])
}

func properties(obj map[string]any) map[string]any {
	if props, ok := obj["properties"].(map[string]any); ok {
		return props
	}
	return map[string]any{}
}

func propString(obj map[string]any, key string) string {
	return asString(properties(obj)[key])
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		return strconv.FormatBool(t)
	default:
		return ""
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}

func parseHubspotTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		if ms > 100000000000 {
			return time.UnixMilli(ms), true
		}
		return time.Unix(ms, 0), true
	}
	return time.Time{}, false
}

func associatedIDs(obj map[string]any, assocName string) []string {
	assocs, ok := obj["associations"].(map[string]any)
	if !ok {
		return nil
	}
	block, ok := assocs[assocName].(map[string]any)
	if !ok {
		return nil
	}
	results, ok := block["results"].([]any)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(results))
	for _, item := range results {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := asString(m["id"])
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func loadLastEngagements(ctx context.Context, db *store.Store) (map[string]engagementTouch, error) {
	touches := map[string]engagementTouch{}
	for _, spec := range engagementTables {
		items, err := loadJSONRows(ctx, db, spec.table)
		if err != nil {
			return nil, err
		}
		for _, obj := range items {
			ts, ok := parseHubspotTime(propString(obj, "hs_timestamp"))
			if !ok {
				continue
			}
			for _, contactID := range associatedIDs(obj, "contacts") {
				if cur, exists := touches[contactID]; !exists || ts.After(cur.When) {
					touches[contactID] = engagementTouch{When: ts, Type: spec.kind}
				}
			}
		}
	}
	return touches, nil
}

func computeStaleLeads(ctx context.Context, db *store.Store, stage string, days, limit int) ([]staleLeadRow, error) {
	count, err := countTable(ctx, db, "crm")
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, notFoundErr(fmt.Errorf("no contacts in local store -- run 'hubspot-pp-cli sync' first"))
	}
	contacts, err := loadJSONRows(ctx, db, "crm")
	if err != nil {
		return nil, err
	}
	touches, err := loadLastEngagements(ctx, db)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rows := make([]staleLeadRow, 0)
	for _, c := range contacts {
		if propString(c, "lifecyclestage") != stage {
			continue
		}
		id := objID(c)
		daysSince := int(1e9)
		var touch engagementTouch
		if t, ok := touches[id]; ok {
			touch = t
			daysSince = int(now.Sub(t.When).Hours() / 24)
		}
		if daysSince < days {
			continue
		}
		row := staleLeadRow{
			ContactID:      id,
			Email:          propString(c, "email"),
			FirstName:      propString(c, "firstname"),
			LastName:       propString(c, "lastname"),
			LifecycleStage: propString(c, "lifecyclestage"),
			DaysSinceTouch: daysSince,
		}
		if !touch.When.IsZero() {
			row.LastEngagementType = touch.Type
			row.LastEngagementAt = touch.When.Format(time.RFC3339)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].DaysSinceTouch > rows[j].DaysSinceTouch })
	return limitRows(rows, limit), nil
}

func loadPipelineStages(ctx context.Context, db *store.Store) (map[string]stageInfo, error) {
	pipes, err := loadJSONRows(ctx, db, "pipelines_crm")
	if err != nil {
		return nil, err
	}
	out := map[string]stageInfo{}
	for _, p := range pipes {
		pid := objID(p)
		stages, _ := p["stages"].([]any)
		for _, item := range stages {
			stage, ok := item.(map[string]any)
			if !ok {
				continue
			}
			sid := asString(stage["id"])
			meta, _ := stage["metadata"].(map[string]any)
			prob := asFloat(meta["probability"])
			out[pid+"\x00"+sid] = stageInfo{Label: asString(stage["label"]), Probability: prob}
		}
	}
	return out, nil
}

func computePipelineHealth(ctx context.Context, db *store.Store, owner string, limit int) (pipelineHealthResult, error) {
	stages, err := loadPipelineStages(ctx, db)
	if err != nil {
		return pipelineHealthResult{}, err
	}
	deals, err := loadJSONRows(ctx, db, "deals_crm")
	if err != nil {
		return pipelineHealthResult{}, err
	}
	now := time.Now()
	result := pipelineHealthResult{StageBreakdown: map[string]pipelineStageBreakdown{}}
	for _, d := range deals {
		ownerID := propString(d, "hubspot_owner_id")
		if owner != "" && ownerID != owner {
			continue
		}
		pipeline := propString(d, "pipeline")
		stage := propString(d, "dealstage")
		info, ok := stages[pipeline+"\x00"+stage]
		if !ok || info.Probability == 0 {
			info.Probability = 0.5
		}
		amount := asFloat(properties(d)["amount"])
		age := 0
		if t, ok := parseHubspotTime(propString(d, "hs_lastmodifieddate")); ok {
			age = int(now.Sub(t).Hours() / 24)
		}
		row := pipelineHealthRow{
			DealID:         objID(d),
			DealName:       propString(d, "dealname"),
			Stage:          stage,
			StageLabel:     info.Label,
			Amount:         amount,
			Probability:    info.Probability,
			WeightedValue:  amount * info.Probability,
			OwnerID:        ownerID,
			AgeInStageDays: age,
		}
		result.Rows = append(result.Rows, row)
		result.TotalWeightedValue += row.WeightedValue
		b := result.StageBreakdown[stage]
		b.Count++
		b.Total += amount
		result.StageBreakdown[stage] = b
	}
	sort.Slice(result.Rows, func(i, j int) bool { return result.Rows[i].WeightedValue > result.Rows[j].WeightedValue })
	result.Rows = limitRows(result.Rows, limit)
	result.Total = len(result.Rows)
	return result, nil
}

func computeRecentIntake(ctx context.Context, db *store.Store, hours int, source string, limit int) ([]recentIntakeRow, error) {
	count, err := countTable(ctx, db, "crm")
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, notFoundErr(fmt.Errorf("no contacts in local store -- run 'hubspot-pp-cli sync' first"))
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	contacts, err := loadJSONRows(ctx, db, "crm")
	if err != nil {
		return nil, err
	}
	rows := make([]recentIntakeRow, 0)
	for _, c := range contacts {
		created := asString(c["createdAt"])
		t, ok := parseHubspotTime(created)
		if !ok || t.Before(cutoff) {
			continue
		}
		if source != "" && propString(c, "hs_analytics_source") != source {
			continue
		}
		rows = append(rows, recentIntakeRow{
			ContactID:    objID(c),
			Email:        propString(c, "email"),
			FirstName:    propString(c, "firstname"),
			LastName:     propString(c, "lastname"),
			CreatedAt:    created,
			Source:       propString(c, "hs_analytics_source"),
			SourceData1:  propString(c, "hs_analytics_source_data_1"),
			SourceData2:  propString(c, "hs_analytics_source_data_2"),
			LatestSource: propString(c, "hs_latest_source"),
			UtmSource:    propString(c, "utm_source"),
			UtmMedium:    propString(c, "utm_medium"),
			UtmCampaign:  propString(c, "utm_campaign"),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	return limitRows(rows, limit), nil
}

func computeDedup(ctx context.Context, db *store.Store, key, threshold string, limit int) ([]dedupCluster, error) {
	contacts, err := loadJSONRows(ctx, db, "crm")
	if err != nil {
		return nil, err
	}
	loose := threshold == "loose"
	groups := map[string][]dedupContact{}
	for _, c := range contacts {
		email := propString(c, "email")
		phone := propString(c, "phone")
		company := propString(c, "company")
		var norm string
		switch key {
		case "email":
			norm = normalizeEmail(email, loose)
		case "phone":
			norm = normalizePhone(phone)
		case "domain":
			norm = normalizeDomain(email, company)
		}
		if norm == "" {
			continue
		}
		groups[norm] = append(groups[norm], dedupContact{
			ContactID: objID(c),
			Email:     email,
			Phone:     phone,
			Company:   company,
			CreatedAt: asString(c["createdAt"]),
		})
	}
	clusters := make([]dedupCluster, 0)
	for k, contacts := range groups {
		if len(contacts) > 1 {
			clusters = append(clusters, dedupCluster{Key: k, Count: len(contacts), Contacts: contacts})
		}
	}
	sort.Slice(clusters, func(i, j int) bool { return clusters[i].Count > clusters[j].Count })
	return limitRows(clusters, limit), nil
}

func computeEngagementDecay(ctx context.Context, db *store.Store, days, window, limit int) ([]engagementDecayRow, error) {
	now := time.Now()
	recentStart := now.Add(-time.Duration(window) * 24 * time.Hour)
	priorStart := recentStart.Add(-time.Duration(window) * 24 * time.Hour)
	lookback := now.Add(-time.Duration(days) * 24 * time.Hour)
	recentCount := map[string]int{}
	priorCount := map[string]int{}
	lastSeen := map[string]time.Time{}
	for _, spec := range engagementTables {
		items, err := loadJSONRows(ctx, db, spec.table)
		if err != nil {
			return nil, err
		}
		for _, obj := range items {
			ts, ok := parseHubspotTime(propString(obj, "hs_timestamp"))
			if !ok {
				continue
			}
			for _, contactID := range associatedIDs(obj, "contacts") {
				if cur, ok := lastSeen[contactID]; !ok || ts.After(cur) {
					lastSeen[contactID] = ts
				}
				if !ts.Before(recentStart) && !ts.After(now) {
					recentCount[contactID]++
				} else if !ts.Before(priorStart) && ts.Before(recentStart) {
					priorCount[contactID]++
				}
			}
		}
	}
	rows := make([]engagementDecayRow, 0)
	for contactID, prev := range priorCount {
		if prev == 0 || lastSeen[contactID].Before(lookback) {
			continue
		}
		curr := recentCount[contactID]
		delta := curr - prev
		if delta >= 0 {
			continue
		}
		email := ""
		if c, err := loadJSONByID(ctx, db, "crm", contactID); err == nil {
			email = propString(c, "email")
		}
		rows = append(rows, engagementDecayRow{ContactID: contactID, Email: email, PriorCount: prev, RecentCount: curr, Delta: delta})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Delta < rows[j].Delta })
	return limitRows(rows, limit), nil
}

func computeClosedWonHandoff(ctx context.Context, db *store.Store, cutoff time.Time, format string, limit int) ([]any, error) {
	deals, err := loadJSONRows(ctx, db, "deals_crm")
	if err != nil {
		return nil, err
	}
	rows := make([]any, 0)
	for _, d := range deals {
		if propString(d, "dealstage") != "closedwon" && propString(d, "hs_is_closed_won") != "true" {
			continue
		}
		when, ok := parseHubspotTime(propString(d, "closedate"))
		if !ok {
			when, ok = parseHubspotTime(propString(d, "hs_lastmodifieddate"))
		}
		if !ok || when.Before(cutoff) {
			continue
		}
		deal := map[string]any{
			"dealId":    objID(d),
			"dealname":  propString(d, "dealname"),
			"amount":    propString(d, "amount"),
			"closedate": propString(d, "closedate"),
		}
		for _, contactID := range associatedIDs(d, "contacts") {
			c, err := loadJSONByID(ctx, db, "crm", contactID)
			if err != nil {
				continue
			}
			props := map[string]any{}
			for k, v := range properties(c) {
				props[k] = v
			}
			if format == "clickup" {
				name := strings.TrimSpace(propString(c, "firstname") + " " + propString(c, "lastname"))
				if name == "" {
					name = propString(c, "email")
				}
				rows = append(rows, map[string]any{
					"name":          name,
					"description":   fmt.Sprintf("Closed Won from HubSpot deal: %s (%s)", propString(d, "dealname"), propString(d, "amount")),
					"custom_fields": props,
					"tags":          []string{"hubspot-handoff", "closed-won"},
				})
			} else {
				props["contactId"] = contactID
				props["email"] = propString(c, "email")
				props["_deal"] = deal
				rows = append(rows, props)
			}
			if limit > 0 && len(rows) >= limit {
				return rows, nil
			}
		}
	}
	return rows, nil
}

func computeLeadTrace(ctx context.Context, db *store.Store, contactID string) (leadTrace, error) {
	contact, err := loadJSONByID(ctx, db, "crm", contactID)
	if err != nil {
		if err == sql.ErrNoRows {
			return leadTrace{}, notFoundErr(fmt.Errorf("contact %s not found in local store", contactID))
		}
		return leadTrace{}, err
	}
	trace := leadTrace{
		ContactID: contactID,
		Contact: map[string]any{
			"email":          propString(contact, "email"),
			"firstname":      propString(contact, "firstname"),
			"lastname":       propString(contact, "lastname"),
			"lifecyclestage": propString(contact, "lifecyclestage"),
			"createdAt":      asString(contact["createdAt"]),
		},
		SourceAttribution: sourceAttribution(contact),
	}
	for _, dealID := range associatedIDs(contact, "deals") {
		if d, err := loadJSONByID(ctx, db, "deals_crm", dealID); err == nil {
			trace.Deals = append(trace.Deals, leadTraceDeal{
				DealID:    dealID,
				DealName:  propString(d, "dealname"),
				Stage:     propString(d, "dealstage"),
				Amount:    asFloat(properties(d)["amount"]),
				Pipeline:  propString(d, "pipeline"),
				CloseDate: propString(d, "closedate"),
			})
		}
	}
	for _, companyID := range associatedIDs(contact, "companies") {
		if c, err := loadJSONByID(ctx, db, "companies_crm", companyID); err == nil {
			trace.Companies = append(trace.Companies, leadTraceCompany{
				CompanyID: companyID,
				Name:      propString(c, "name"),
				Domain:    propString(c, "domain"),
				Industry:  propString(c, "industry"),
			})
		}
	}
	for _, spec := range engagementTables {
		items, err := loadJSONRows(ctx, db, spec.table)
		if err != nil {
			return leadTrace{}, err
		}
		for _, e := range items {
			if !containsString(associatedIDs(e, "contacts"), contactID) {
				continue
			}
			trace.Engagements = append(trace.Engagements, leadTraceEngagement{
				Type:      spec.kind,
				ID:        objID(e),
				Timestamp: propString(e, "hs_timestamp"),
				Subject:   truncate(propString(e, spec.subjectProp), 80),
			})
		}
	}
	sort.Slice(trace.Engagements, func(i, j int) bool { return trace.Engagements[i].Timestamp > trace.Engagements[j].Timestamp })
	return trace, nil
}

func computeUtmCohort(ctx context.Context, db *store.Store, campaign string, limit int) (utmCohortResult, error) {
	contacts, err := loadJSONRows(ctx, db, "crm")
	if err != nil {
		return utmCohortResult{}, err
	}
	stages, err := loadPipelineStages(ctx, db)
	if err != nil {
		return utmCohortResult{}, err
	}
	result := utmCohortResult{Campaign: campaign}
	stageRows := map[string]*utmCohortStageRow{}
	closedWonContacts := map[string]bool{}
	totalAmount := 0.0
	for _, c := range contacts {
		if propString(c, "utm_campaign") != campaign {
			continue
		}
		result.CohortSize++
		for _, dealID := range associatedIDs(c, "deals") {
			d, err := loadJSONByID(ctx, db, "deals_crm", dealID)
			if err != nil {
				continue
			}
			stage := propString(d, "dealstage")
			pipeline := propString(d, "pipeline")
			amount := asFloat(properties(d)["amount"])
			totalAmount += amount
			result.TotalDeals++
			if stage == "closedwon" || propString(d, "hs_is_closed_won") == "true" {
				closedWonContacts[objID(c)] = true
			}
			row := stageRows[stage]
			if row == nil {
				info := stages[pipeline+"\x00"+stage]
				row = &utmCohortStageRow{Stage: stage, StageLabel: info.Label}
				stageRows[stage] = row
			}
			row.Count++
			row.TotalAmount += amount
		}
	}
	if result.CohortSize == 0 {
		return utmCohortResult{}, notFoundErr(fmt.Errorf("no contacts found for UTM campaign %q", campaign))
	}
	if result.CohortSize > 0 {
		result.PercentClosedWon = float64(len(closedWonContacts)) * 100 / float64(result.CohortSize)
	}
	if result.TotalDeals > 0 {
		result.AvgDealAmount = totalAmount / float64(result.TotalDeals)
	}
	for _, row := range stageRows {
		result.StageBreakdown = append(result.StageBreakdown, *row)
	}
	sort.Slice(result.StageBreakdown, func(i, j int) bool {
		return result.StageBreakdown[i].TotalAmount > result.StageBreakdown[j].TotalAmount
	})
	result.StageBreakdown = limitRows(result.StageBreakdown, limit)
	return result, nil
}

func filterDealsSince(ctx context.Context, db *store.Store, rows []pipelineHealthRow, cutoff time.Time, closedWonOnly bool, limit int) []pipelineHealthRow {
	out := make([]pipelineHealthRow, 0)
	for _, row := range rows {
		deal, err := loadJSONByID(ctx, db, "deals_crm", row.DealID)
		if err != nil {
			continue
		}
		if closedWonOnly && propString(deal, "dealstage") != "closedwon" && propString(deal, "hs_is_closed_won") != "true" {
			continue
		}
		tsKey := "hs_lastmodifieddate"
		if closedWonOnly {
			tsKey = "closedate"
		}
		ts, ok := parseHubspotTime(propString(deal, tsKey))
		if !ok && closedWonOnly {
			ts, ok = parseHubspotTime(propString(deal, "hs_lastmodifieddate"))
		}
		if !ok || ts.Before(cutoff) {
			continue
		}
		out = append(out, row)
		if limit > 0 && len(out) >= limit {
			return out
		}
	}
	return out
}

func sourceAttribution(contact map[string]any) map[string]string {
	keys := []string{
		"hs_analytics_source", "hs_analytics_source_data_1", "hs_analytics_source_data_2",
		"hs_latest_source", "hs_latest_source_data_1", "hs_latest_source_data_2",
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
	}
	out := map[string]string{}
	for _, key := range keys {
		if v := propString(contact, key); v != "" {
			out[key] = v
		}
	}
	return out
}

func normalizeEmail(s string, loose bool) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !loose || !strings.HasSuffix(s, "@gmail.com") {
		return s
	}
	local, domain, ok := strings.Cut(s, "@")
	if !ok {
		return s
	}
	return strings.ReplaceAll(local, ".", "") + "@" + domain
}

func normalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return ""
	}
	if len(digits) == 10 {
		return "+1" + digits
	}
	if len(digits) == 11 && strings.HasPrefix(digits, "1") {
		return "+" + digits
	}
	return "+" + digits
}

func normalizeDomain(email, company string) string {
	if _, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@"); ok {
		return strings.TrimPrefix(domain, "www.")
	}
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(company)), "www.")
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func limitRows[T any](rows []T, limit int) []T {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
