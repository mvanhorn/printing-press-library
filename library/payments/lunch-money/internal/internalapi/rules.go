package internalapi

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
)

// Rule mirrors the response shape from GET/POST/PUT /rules.
//
// The wire format has TWO field-name conventions:
//   - Conditions are prefixed `criteria_*` (criteria_payee_name, criteria_amount, etc.)
//   - Actions sit at top level (category_id, mark_as_unreviewed, set_uncategorized, etc.)
//
// `criteria_category_id` is a legacy/duplicate field that's always null on rules
// created via /rules — the real action category is the top-level `category_id`.
//
// Numeric fields that ship as strings (criteria_amount) stay as json.RawMessage
// to preserve precision and handle null/number/string gracefully.
type Rule struct {
	// Identity
	RuleID         int64 `json:"rule_id"`
	RuleCriteriaID int64 `json:"rule_criteria_id"`

	// Conditions (criteria_*)
	CriteriaPayeeName      *string         `json:"criteria_payee_name,omitempty"`
	CriteriaPayeeNameMatch *string         `json:"criteria_payee_name_match,omitempty"`
	CriteriaNotes          *string         `json:"criteria_notes,omitempty"`
	CriteriaNotesMatch     *string         `json:"criteria_notes_match,omitempty"`
	CriteriaAmount         json.RawMessage `json:"criteria_amount,omitempty"`
	CriteriaAmount2        json.RawMessage `json:"criteria_amount_2,omitempty"`
	CriteriaAmountCurrency *string         `json:"criteria_amount_currency,omitempty"`
	CriteriaAmountMatch    *string         `json:"criteria_amount_match,omitempty"`
	CriteriaDay            *int            `json:"criteria_day,omitempty"`
	CriteriaDay2           *int            `json:"criteria_day_2,omitempty"`
	CriteriaDayMatch       *string         `json:"criteria_day_match,omitempty"`
	CriteriaAssetID        *int64          `json:"criteria_asset_id,omitempty"`
	CriteriaPlaidAccountID *int64          `json:"criteria_plaid_account_id,omitempty"`
	CriteriaPriority       int             `json:"criteria_priority"`
	CriteriaOnPlaid        bool            `json:"criteria_on_plaid"`
	CriteriaOnCSV          bool            `json:"criteria_on_csv"`
	CriteriaOnManual       bool            `json:"criteria_on_manual"`
	CriteriaOnAPI          bool            `json:"criteria_on_api"`
	CriteriaEnabled        bool            `json:"criteria_enabled"`
	RunOnUpdate            bool            `json:"run_on_update"`
	CriteriaSource         *string         `json:"criteria_source,omitempty"`
	CriteriaSuggested      bool            `json:"criteria_suggested"`

	// Actions (top-level, NOT criteria_*)
	CategoryID           *int64          `json:"category_id,omitempty"`
	PayeeName            *string         `json:"payee_name,omitempty"`
	Notes                *string         `json:"notes,omitempty"`
	AddTagIDs            []int64         `json:"add_tag_ids,omitempty"`
	MarkAsReviewed       *bool           `json:"mark_as_reviewed,omitempty"`
	MarkAsUnreviewed     *bool           `json:"mark_as_unreviewed,omitempty"`
	SetUncategorized     *bool           `json:"set_uncategorized,omitempty"`
	SendEmail            json.RawMessage `json:"send_email,omitempty"`
	ShouldSendEmail      bool            `json:"should_send_email"`
	ShouldDelete         bool            `json:"should_delete"`
	HasSplit             bool            `json:"has_split"`
	StopProcessingOthers bool            `json:"stop_processing_others"`
	OneTimeRule          bool            `json:"one_time_rule"`
	SkipRecurring        bool            `json:"skip_recurring"`
	SkipRule             bool            `json:"skip_rule"`
	Description          *string         `json:"description,omitempty"`

	// Delete-transaction action (when set)
	DeleteTransactionID       *int64   `json:"delete_transaction_id,omitempty"`
	DeleteTransactionPayee    *string  `json:"delete_transaction_payee,omitempty"`
	DeleteTransactionAmount   *float64 `json:"delete_transaction_amount,omitempty"`
	DeleteTransactionCurrency *string  `json:"delete_transaction_currency,omitempty"`
	DeleteTransactionDate     *string  `json:"delete_transaction_date,omitempty"`

	// Recurring-link action (when set)
	RecurringID          *int64  `json:"recurring_id,omitempty"`
	RecurringPayee       *string `json:"recurring_payee,omitempty"`
	RecurringAmount      *string `json:"recurring_amount,omitempty"`
	RecurringCurrency    *string `json:"recurring_currency,omitempty"`
	RecurringType        *string `json:"recurring_type,omitempty"`
	RecurringDescription *string `json:"recurring_description,omitempty"`
	RecurringCategoryID  *int64  `json:"recurring_category_id,omitempty"`

	// Timestamps
	CriteriaCreatedAt       *string `json:"criteria_created_at,omitempty"`
	CriteriaUpdatedAt       *string `json:"criteria_updated_at,omitempty"`
	CriteriaLastTriggeredAt *string `json:"criteria_last_triggered_at,omitempty"`
	RulesUpdatedAt          *string `json:"rules_updated_at,omitempty"`
	UpdatedAt               *string `json:"updated_at,omitempty"`
}

// PayeeDisplay returns the rule's payee match string with HTML entities decoded
// (the server stores `A&#x2F;C` for `A/C`, etc.). Returns "" if the rule has
// no payee condition.
func (r *Rule) PayeeDisplay() string {
	if r.CriteriaPayeeName == nil {
		return ""
	}
	return html.UnescapeString(*r.CriteriaPayeeName)
}

// DescriptionDisplay returns the rule's description with HTML entities decoded.
func (r *Rule) DescriptionDisplay() string {
	if r.Description == nil {
		return ""
	}
	return html.UnescapeString(*r.Description)
}

// ActionSummary returns a one-line human-readable summary of what the rule does.
// Useful for the `rules list` table output.
func (r *Rule) ActionSummary() string {
	var parts []string
	if r.CategoryID != nil {
		parts = append(parts, fmt.Sprintf("category→%d", *r.CategoryID))
	}
	if r.SetUncategorized != nil && *r.SetUncategorized {
		parts = append(parts, "clear-category")
	}
	if r.MarkAsReviewed != nil && *r.MarkAsReviewed {
		parts = append(parts, "mark-reviewed")
	}
	if r.MarkAsUnreviewed != nil && *r.MarkAsUnreviewed {
		parts = append(parts, "mark-unreviewed")
	}
	if r.PayeeName != nil {
		parts = append(parts, fmt.Sprintf("payee=%q", *r.PayeeName))
	}
	if r.Notes != nil {
		parts = append(parts, fmt.Sprintf("notes=%q", *r.Notes))
	}
	if r.HasSplit {
		parts = append(parts, "split")
	}
	if r.RecurringID != nil {
		parts = append(parts, fmt.Sprintf("recurring→%d", *r.RecurringID))
	}
	if r.ShouldSendEmail {
		parts = append(parts, "email")
	}
	if r.StopProcessingOthers {
		parts = append(parts, "stop")
	}
	if r.SkipRecurring {
		parts = append(parts, "skip-recurring")
	}
	if len(parts) == 0 {
		return "(no actions)"
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += "," + p
	}
	return result
}

// rulesListResponse is the wrapper shape returned by GET /rules.
type rulesListResponse struct {
	TotalReturned int    `json:"total_returned"`
	Rules         []Rule `json:"rules"`
}

// RulePayee is the payee-match clause in a rule's `conditions`.
type RulePayee struct {
	Name  string `json:"name"`
	Match string `json:"match"` // "contain" | "exact" | "start" | "end" | "regex"
}

// RuleConditions is the `conditions` half of CREATE/UPDATE request bodies.
type RuleConditions struct {
	Payee    *RulePayee `json:"payee,omitempty"`
	OnPlaid  bool       `json:"on_plaid,omitempty"`
	OnCSV    bool       `json:"on_csv,omitempty"`
	OnManual bool       `json:"on_manual,omitempty"`
	OnAPI    bool       `json:"on_api,omitempty"`
	Priority string     `json:"priority,omitempty"` // wire sends as string
}

// RuleActions is the `actions` half of CREATE/UPDATE request bodies.
// Full action enum, captured live + bundle-confirmed.
type RuleActions struct {
	CategoryID           *int64          `json:"category_id,omitempty"`
	SetUncategorized     *bool           `json:"set_uncategorized,omitempty"`
	Payee                *string         `json:"payee,omitempty"`
	Notes                *string         `json:"notes,omitempty"`
	Description          *string         `json:"description,omitempty"`
	AddTagIDs            []int64         `json:"add_tag_ids,omitempty"`
	MarkAsReviewed       *bool           `json:"mark_as_reviewed,omitempty"`
	MarkAsUnreviewed     *bool           `json:"mark_as_unreviewed,omitempty"`
	ShouldSendEmail      *bool           `json:"should_send_email,omitempty"`
	ShouldDelete         *bool           `json:"should_delete,omitempty"`
	ShouldSplit          json.RawMessage `json:"should_split,omitempty"` // object — see captures
	SkipRecurring        *bool           `json:"skip_recurring,omitempty"`
	DontRunRules         *bool           `json:"dont_run_rules,omitempty"`
	StopProcessingOthers *bool           `json:"stop_processing_others,omitempty"`
}

// RuleSpec is the full CREATE/UPDATE request body shape.
type RuleSpec struct {
	Conditions RuleConditions `json:"conditions"`
	Actions    RuleActions    `json:"actions"`
}

// ToSpec converts the full response shape back into the PUT/POST request
// shape. It intentionally covers the action fields the CLI knows how to edit.
func (r Rule) ToSpec() RuleSpec {
	spec := RuleSpec{
		Conditions: RuleConditions{
			OnPlaid:  r.CriteriaOnPlaid,
			OnCSV:    r.CriteriaOnCSV,
			OnManual: r.CriteriaOnManual,
			OnAPI:    r.CriteriaOnAPI,
		},
	}
	if r.CriteriaPriority != 0 {
		spec.Conditions.Priority = strconv.Itoa(r.CriteriaPriority)
	}
	if r.CriteriaPayeeName != nil {
		match := "contain"
		if r.CriteriaPayeeNameMatch != nil && *r.CriteriaPayeeNameMatch != "" {
			match = *r.CriteriaPayeeNameMatch
		}
		spec.Conditions.Payee = &RulePayee{
			Name:  html.UnescapeString(*r.CriteriaPayeeName),
			Match: match,
		}
	}

	a := &spec.Actions
	if r.CategoryID != nil {
		v := *r.CategoryID
		a.CategoryID = &v
	}
	if r.SetUncategorized != nil && *r.SetUncategorized {
		t := true
		a.SetUncategorized = &t
	}
	if r.PayeeName != nil {
		v := html.UnescapeString(*r.PayeeName)
		a.Payee = &v
	}
	if r.Notes != nil {
		v := html.UnescapeString(*r.Notes)
		a.Notes = &v
	}
	if r.Description != nil {
		v := html.UnescapeString(*r.Description)
		a.Description = &v
	}
	if len(r.AddTagIDs) > 0 {
		a.AddTagIDs = append([]int64(nil), r.AddTagIDs...)
	}
	if r.MarkAsReviewed != nil && *r.MarkAsReviewed {
		t := true
		a.MarkAsReviewed = &t
	}
	if r.MarkAsUnreviewed != nil && *r.MarkAsUnreviewed {
		t := true
		a.MarkAsUnreviewed = &t
	}
	if r.ShouldSendEmail {
		t := true
		a.ShouldSendEmail = &t
	}
	if r.ShouldDelete || r.DeleteTransactionID != nil {
		t := true
		a.ShouldDelete = &t
	}
	if r.SkipRecurring {
		t := true
		a.SkipRecurring = &t
	}
	if r.SkipRule {
		t := true
		a.DontRunRules = &t
	}
	if r.StopProcessingOthers {
		t := true
		a.StopProcessingOthers = &t
	}
	return spec
}

// Negative-lookahead regex is accepted by /rules create but silently matches
// everything in the engine. Reject early to save the user a debugging session.
var negLookaheadRE = regexp.MustCompile(`\(\?!`)

// ValidateMatchString returns an error when a regex match string contains
// PCRE features (negative lookahead, lookbehind) that the engine accepts at
// create time but mishandles at apply time.
func ValidateMatchString(match, pattern string) error {
	if match != "regex" {
		return nil
	}
	if negLookaheadRE.MatchString(pattern) {
		return fmt.Errorf("regex contains negative lookahead `(?!...)` — the engine accepts this but matches everything at apply time; use rule ordering with stop_processing_others instead")
	}
	return nil
}

// ListRules returns all rules. The API supports offset/limit pagination
// (default 100); we follow until the page is short.
func (c *Client) ListRules(limit int) ([]Rule, error) {
	if limit <= 0 {
		limit = 100
	}
	var out []Rule
	offset := 0
	for {
		q := url.Values{}
		q.Set("offset", strconv.Itoa(offset))
		q.Set("limit", strconv.Itoa(limit))
		var page rulesListResponse
		_, err := c.Do("GET", "/rules?"+q.Encode(), nil, &page)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Rules...)
		if len(page.Rules) < limit {
			break
		}
		offset += limit
	}
	return out, nil
}

// GetRule finds one rule by criteria_id using the captured list endpoint.
func (c *Client) GetRule(criteriaID int64) (*Rule, error) {
	rules, err := c.ListRules(100)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		if rules[i].RuleCriteriaID == criteriaID {
			return &rules[i], nil
		}
	}
	return nil, fmt.Errorf("rule criteria_id %d not found", criteriaID)
}

// CreateRule POST /rules.
func (c *Client) CreateRule(spec RuleSpec) (*Rule, error) {
	if spec.Conditions.Payee != nil {
		if err := ValidateMatchString(spec.Conditions.Payee.Match, spec.Conditions.Payee.Name); err != nil {
			return nil, err
		}
	}
	var out Rule
	_, err := c.Do("POST", "/rules", spec, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRule PUT /rules/{criteria_id}. Works on the internal API; PATCH 404s.
func (c *Client) UpdateRule(criteriaID int64, spec RuleSpec) (*Rule, error) {
	if spec.Conditions.Payee != nil {
		if err := ValidateMatchString(spec.Conditions.Payee.Match, spec.Conditions.Payee.Name); err != nil {
			return nil, err
		}
	}
	var out Rule
	_, err := c.Do("PUT", fmt.Sprintf("/rules/%d", criteriaID), spec, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRules bulk-delete by criteria_id.
func (c *Client) DeleteRules(criteriaIDs []int64) error {
	body := map[string]any{"criteria_ids": criteriaIDs}
	_, err := c.Do("POST", "/rules/bulk_delete", body, nil)
	return err
}

// ApplyRulesDryRun returns the transactions that WOULD be affected.
// Pass empty includeTransactionIDs to apply to all matching.
//
// IMPORTANT: dry-run evaluates a single rule in isolation. It does NOT simulate
// the priority chain or stop_processing_others. To see the real auto-apply
// behavior, use the web UI or call this on each rule in priority order while
// tracking which transactions get "claimed" by earlier rules.
func (c *Client) ApplyRulesDryRun(criteriaIDs, includeTxIDs []int64) ([]map[string]any, error) {
	return c.applyRules(criteriaIDs, includeTxIDs, true)
}

// ApplyRules actually applies the rules. Use ApplyRulesDryRun first.
func (c *Client) ApplyRules(criteriaIDs, includeTxIDs []int64) ([]map[string]any, error) {
	return c.applyRules(criteriaIDs, includeTxIDs, false)
}

func (c *Client) applyRules(criteriaIDs, includeTxIDs []int64, dryRun bool) ([]map[string]any, error) {
	if includeTxIDs == nil {
		includeTxIDs = []int64{}
	}
	if len(criteriaIDs) == 0 {
		return c.applyRulesRequest(criteriaIDs, includeTxIDs, dryRun)
	}
	all := make([]map[string]any, 0)
	for _, criteriaID := range criteriaIDs {
		// PATCH: /rules/apply misapplies batched commit requests by using the
		// last rule's action for the combined match set. Serialize both commit
		// and dry-run calls so multi-id CLI invocations are never sent as one
		// criteria_ids array.
		results, err := c.applyRulesRequest([]int64{criteriaID}, includeTxIDs, dryRun)
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
	}
	return all, nil
}

func (c *Client) applyRulesRequest(criteriaIDs, includeTxIDs []int64, dryRun bool) ([]map[string]any, error) {
	body := map[string]any{
		"criteria_ids":            criteriaIDs,
		"dry_run":                 dryRun,
		"include_transaction_ids": includeTxIDs,
	}
	var out []map[string]any
	_, err := c.Do("POST", "/rules/apply", body, &out)
	return out, err
}
