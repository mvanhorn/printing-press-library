package internalapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// ============================================================================
// /me
// ============================================================================

type Me struct {
	ID                   int64     `json:"id"`
	AccountID            int64     `json:"account_id"`
	AccountDisplayName   string    `json:"account_display_name"`
	Name                 string    `json:"name"`
	Email                string    `json:"email"`
	Phone                *string   `json:"phone,omitempty"`
	PrimaryCurrency      string    `json:"primary_currency"`
	StripePlanCurrency   string    `json:"stripe_plan_currency"`
	Type                 string    `json:"type"`
	VerifiedEmail        bool      `json:"verified_email"`
	IsAdminUser          bool      `json:"is_admin_user"`
	IsBeta               bool      `json:"is_beta"`
	IsAlpha              bool      `json:"is_alpha"`
	IsDemo               bool      `json:"is_demo"`
	HasStripeAccount     bool      `json:"has_stripe_account"`
	BudgetWasAutoCreated bool      `json:"budget_was_auto_created"`
	ScheduledForDeletion bool      `json:"scheduled_for_deletion"`
	JoinDate             time.Time `json:"join_date"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (c *Client) GetMe() (*Me, error) {
	var out Me
	_, err := c.Do("GET", "/me", nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ============================================================================
// /billing
// ============================================================================

func (c *Client) GetBilling() (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", "/billing", nil, &out)
	return out, err
}

// ============================================================================
// /api_tokens
// ============================================================================

type APIToken struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Label        string    `json:"label"`
	Status       string    `json:"status"`
	LastActivity time.Time `json:"last_activity"`
	CreatedAt    time.Time `json:"created_at"`
}

func (c *Client) ListAPITokens() ([]APIToken, error) {
	var out []APIToken
	_, err := c.Do("GET", "/api_tokens", nil, &out)
	return out, err
}

func (c *Client) DeleteAPIToken(id int64) error {
	_, err := c.Do("DELETE", fmt.Sprintf("/api_tokens/%d", id), nil, nil)
	return err
}

// ============================================================================
// /assets — manual accounts via the internal endpoint
// ============================================================================

// Asset is the rich shape returned by GET /assets. Plaid-linked assets include
// access_token + item_id; callers should redact these before logging.
type Asset struct {
	ID                  int64           `json:"id"`
	AccountID           int64           `json:"account_id"`
	Source              string          `json:"source"` // manual | plaid
	Name                string          `json:"name"`
	OfficialName        *string         `json:"official_name,omitempty"`
	DisplayName         *string         `json:"display_name,omitempty"`
	InstitutionName     *string         `json:"institution_name,omitempty"`
	Type                string          `json:"type"` // cash | credit | investment | ...
	Subtype             string          `json:"subtype"`
	TypeID              *int64          `json:"type_id,omitempty"`
	SubtypeID           *int64          `json:"subtype_id,omitempty"`
	Mask                *string         `json:"mask,omitempty"`
	Balance             json.RawMessage `json:"balance,omitempty"`
	Currency            string          `json:"currency"`
	Status              string          `json:"status"`
	ExcludeTransactions bool            `json:"exclude_transactions"`
	ClosedOn            *string         `json:"closed_on,omitempty"`
	DateLinked          time.Time       `json:"date_linked"`
	LastImport          *time.Time      `json:"last_import,omitempty"`
	BalanceLastUpdate   *time.Time      `json:"balance_last_update,omitempty"`
	CreatedAt           *time.Time      `json:"created_at,omitempty"`
	UpdatedAt           *time.Time      `json:"updated_at,omitempty"`
	// Sensitive — populated for Plaid-linked assets only.
	AccessToken   string  `json:"access_token,omitempty"`
	ItemID        string  `json:"item_id,omitempty"`
	InstitutionID *int64  `json:"institution_id,omitempty"`
	ExternalID    *string `json:"external_id,omitempty"`
}

func (c *Client) ListAssets() ([]Asset, error) {
	var out []Asset
	_, err := c.Do("GET", "/assets", nil, &out)
	return out, err
}

func (c *Client) GetAssetStatus(id int64) (map[string]bool, error) {
	var out map[string]bool
	_, err := c.Do("GET", fmt.Sprintf("/assets/%d/status", id), nil, &out)
	return out, err
}

func (c *Client) DeleteAsset(id int64, keepItems bool) error {
	_, err := c.Do("PUT", fmt.Sprintf("/assets/%d/delete", id), map[string]bool{"keep_items": keepItems}, nil)
	return err
}

// AssetSubtypes returns the valid type→subtype tree for manual accounts.
func (c *Client) AssetSubtypes() (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", "/assets/subtypes", nil, &out)
	return out, err
}

// ============================================================================
// /balance_history — net worth over time
// ============================================================================

func (c *Client) BalanceHistory(startDate, endDate string) (map[string]any, error) {
	q := url.Values{}
	q.Set("start_date", startDate)
	q.Set("end_date", endDate)
	var out map[string]any
	_, err := c.Do("GET", "/balance_history?"+q.Encode(), nil, &out)
	return out, err
}

// ============================================================================
// /plaid/categories — autocategorization
// ============================================================================

// PlaidCategory is one entry in the Plaid taxonomy + current LM mapping.
// `tx` is the mapped LM category id (or list of ids); raw because the server
// returns either a single id or null or an array depending on the row.
type PlaidCategory struct {
	Primary             string          `json:"primary"`
	Detailed            string          `json:"detailed"`
	Description         string          `json:"description"`
	PrimaryDisplayName  string          `json:"primary_display_name"`
	DetailedDisplayName string          `json:"detailed_display_name"`
	TX                  json.RawMessage `json:"tx,omitempty"`
	Count               int             `json:"count"`
}

func (c *Client) ListPlaidCategories() ([]PlaidCategory, error) {
	var out []PlaidCategory
	_, err := c.Do("GET", "/plaid/categories", nil, &out)
	return out, err
}

func (c *Client) PopulatePlaidCategories() error {
	_, err := c.Do("POST", "/plaid/categories/populate", map[string]any{}, nil)
	return err
}

// ============================================================================
// /summary — budget + spend per category
// ============================================================================

// SummaryOptions controls the Budget page summary fetch. Set Includes via the
// fluent helpers below.
type SummaryOptions struct {
	StartDate string
	EndDate   string
	Includes  map[string]bool
}

func (o *SummaryOptions) Include(flags ...string) *SummaryOptions {
	if o.Includes == nil {
		o.Includes = map[string]bool{}
	}
	for _, f := range flags {
		o.Includes[f] = true
	}
	return o
}

func (c *Client) Summary(opts SummaryOptions) (map[string]any, error) {
	q := url.Values{}
	q.Set("start_date", opts.StartDate)
	q.Set("end_date", opts.EndDate)
	for k, v := range opts.Includes {
		if v {
			q.Set(k, "true")
		}
	}
	var out map[string]any
	_, err := c.Do("GET", "/summary?"+q.Encode(), nil, &out)
	return out, err
}

// ============================================================================
// /transactions — internal flavor (richer query params + bulk_update)
// ============================================================================

// TransactionUpdate is the writable subset of the per-transaction PUT body.
// Use nil pointers for fields you don't want to change.
type TransactionUpdate struct {
	Status            *string `json:"status,omitempty"` // "cleared" | "uncleared"
	CategoryID        *int64  `json:"category_id,omitempty"`
	Payee             *string `json:"payee,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	Amount            *string `json:"amount,omitempty"` // string form to preserve precision
	Date              *string `json:"date,omitempty"`
	TagIDs            []int64 `json:"tag_ids,omitempty"`
	ExcludeFromTotals *bool   `json:"exclude_from_totals,omitempty"`
	ExcludeFromBudget *bool   `json:"exclude_from_budget,omitempty"`
	RecurringID       *int64  `json:"recurring_id,omitempty"`
}

func (c *Client) UpdateTransaction(id int64, upd TransactionUpdate) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("PUT", fmt.Sprintf("/transactions/%d", id), upd, &out)
	return out, err
}

func (c *Client) BulkUpdateTransactions(ids []string, upd TransactionUpdate) (map[string]any, error) {
	body := map[string]any{
		"transactionIds": ids,
		"updateObj":      upd,
	}
	var out map[string]any
	_, err := c.Do("PUT", "/transactions/bulk_update", body, &out)
	return out, err
}

// TransactionFilter shapes a /transactions list query.
type TransactionFilter struct {
	StartDate      string
	EndDate        string
	IsUnreviewed   bool
	Match          string // "all" | "any"
	Minimal        bool
	ExcludePending bool
	ExcludeParents bool
	Paginate       bool
	Limit          int
	Offset         int
}

func (c *Client) ListTransactions(f TransactionFilter) (map[string]any, error) {
	q := url.Values{}
	if f.StartDate != "" {
		q.Set("start_date", f.StartDate)
	}
	if f.EndDate != "" {
		q.Set("end_date", f.EndDate)
	}
	if f.IsUnreviewed {
		q.Set("is_unreviewed", "true")
	}
	if f.Match != "" {
		q.Set("match", f.Match)
	}
	if f.Minimal {
		q.Set("minimal", "true")
	}
	if f.ExcludePending {
		q.Set("exclude_pending", "true")
	}
	if f.ExcludeParents {
		q.Set("exclude_parents", "true")
	}
	if f.Paginate {
		q.Set("paginate", "true")
	}
	if f.Limit > 0 {
		q.Set("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		q.Set("offset", strconv.Itoa(f.Offset))
	}
	var out map[string]any
	_, err := c.Do("GET", "/transactions?"+q.Encode(), nil, &out)
	return out, err
}

// ============================================================================
// /recurring_items
// ============================================================================

func (c *Client) ListRecurringItems(startDate, endDate string) ([]map[string]any, error) {
	path := "/recurring_items"
	if startDate != "" || endDate != "" {
		q := url.Values{}
		if startDate != "" {
			q.Set("start_date", startDate)
		}
		if endDate != "" {
			q.Set("end_date", endDate)
		}
		path += "?" + q.Encode()
	}
	var out []map[string]any
	_, err := c.Do("GET", path, nil, &out)
	return out, err
}

// ============================================================================
// /snapshot/*
// ============================================================================

func (c *Client) SnapshotTransactionsPage() (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", "/snapshot/transactions_page", nil, &out)
	return out, err
}

func (c *Client) SnapshotTagsPage() (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", "/snapshot/tags_page", nil, &out)
	return out, err
}

// ============================================================================
// Misc
// ============================================================================

func (c *Client) Currencies() ([]map[string]any, error) {
	var out []map[string]any
	_, err := c.Do("GET", "/currencies", nil, &out)
	return out, err
}

func (c *Client) Notifications() ([]map[string]any, error) {
	var out []map[string]any
	_, err := c.Do("GET", "/notifications", nil, &out)
	return out, err
}

// ============================================================================
// /trends — analytics aggregations
// ============================================================================

// TrendsOptions configures the /trends call.
type TrendsOptions struct {
	StartDate                string
	EndDate                  string
	IncludeRecurring         bool
	IncludeExcludeFromTotals bool
	IncludePending           bool
	GroupBy                  string // "category" | "payee" | "tag" | "asset" | "type"
}

func (c *Client) Trends(opts TrendsOptions) (map[string]any, error) {
	q := url.Values{}
	q.Set("start_date", opts.StartDate)
	q.Set("end_date", opts.EndDate)
	q.Set("include_recurring", boolStr(opts.IncludeRecurring))
	q.Set("include_exclude_from_totals", boolStr(opts.IncludeExcludeFromTotals))
	if opts.IncludePending {
		q.Set("include_pending", "true")
	}
	if opts.GroupBy != "" {
		q.Set("group_by", opts.GroupBy)
	}
	var out map[string]any
	_, err := c.Do("GET", "/trends?"+q.Encode(), nil, &out)
	return out, err
}

// ============================================================================
// /stats — top transactions by price within a window
// ============================================================================

func (c *Client) Stats(opts TrendsOptions) (map[string]any, error) {
	q := url.Values{}
	q.Set("start_date", opts.StartDate)
	q.Set("end_date", opts.EndDate)
	q.Set("include_recurring", boolStr(opts.IncludeRecurring))
	q.Set("include_exclude_from_totals", boolStr(opts.IncludeExcludeFromTotals))
	var out map[string]any
	_, err := c.Do("GET", "/stats?"+q.Encode(), nil, &out)
	return out, err
}

// ============================================================================
// /calendar — daily transaction grid
// ============================================================================

func (c *Client) Calendar(startDate, endDate string, includeRecurring bool) (map[string]any, error) {
	q := url.Values{}
	q.Set("start_date", startDate)
	q.Set("end_date", endDate)
	if includeRecurring {
		q.Set("include_recurring", "true")
	}
	var out map[string]any
	_, err := c.Do("GET", "/calendar?"+q.Encode(), nil, &out)
	return out, err
}

// ============================================================================
// /referral
// ============================================================================

type ReferralInfo struct {
	Token       string           `json:"token"`
	CountActive int              `json:"count_active"`
	Users       []map[string]any `json:"users"`
}

func (c *Client) Referral() (*ReferralInfo, error) {
	var out ReferralInfo
	_, err := c.Do("GET", "/referral", nil, &out)
	return &out, err
}

// ============================================================================
// /api_tokens — CREATE
// ============================================================================

// CreateAPIToken issues a new public-API token. Returns the raw secret (the
// caller must save it — it cannot be retrieved later).
func (c *Client) CreateAPIToken(label, reasonText string) (string, error) {
	body := map[string]any{
		"label":            label,
		"apiKeyReason":     nil,
		"apiKeyReasonText": reasonText,
	}
	var out string
	_, err := c.Do("POST", "/api_tokens", body, &out)
	return out, err
}

// ============================================================================
// /v2/plaid_accounts — Plaid account listing + refresh trigger
// ============================================================================

func (c *Client) ListPlaidAccounts() (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", "/v2/plaid_accounts", nil, &out)
	return out, err
}

func (c *Client) GetPlaidAccount(id int64) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", fmt.Sprintf("/v2/plaid_accounts/%d", id), nil, &out)
	return out, err
}

// PlaidFetchOptions controls POST /v2/plaid_accounts/fetch.
type PlaidFetchOptions struct {
	ID        int64
	StartDate string
	EndDate   string
}

// TriggerPlaidFetch queues a background Plaid fetch. The internal host uses the
// same public-v2 shape, but mounted at /v2/plaid_accounts/fetch.
// PATCH: Wire the live-captured internal Plaid refresh endpoint.
func (c *Client) TriggerPlaidFetch(opts PlaidFetchOptions) (int, error) {
	q := url.Values{}
	if opts.ID != 0 {
		q.Set("id", strconv.FormatInt(opts.ID, 10))
	}
	if opts.StartDate != "" {
		q.Set("start_date", opts.StartDate)
	}
	if opts.EndDate != "" {
		q.Set("end_date", opts.EndDate)
	}
	path := "/v2/plaid_accounts/fetch"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	status, _, err := c.DoRaw("POST", path, map[string]any{})
	return status, err
}

// ============================================================================
// /v2/budgets — budget writes mounted on the internal host
// ============================================================================

type BudgetUpsert struct {
	CategoryID int64  `json:"category_id"`
	StartDate  string `json:"start_date"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// UpsertBudget creates or updates one category budget for a period.
// PATCH: Wire internal-host v2 budget writes captured from the budget page.
func (c *Client) UpsertBudget(req BudgetUpsert) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("PUT", "/v2/budgets", req, &out)
	return out, err
}

// DeleteBudget clears one category budget for a period.
// PATCH: Wire the budget clear path paired with UpsertBudget.
func (c *Client) DeleteBudget(categoryID int64, startDate string) error {
	q := url.Values{}
	q.Set("category_id", strconv.FormatInt(categoryID, 10))
	q.Set("start_date", startDate)
	_, err := c.Do("DELETE", "/v2/budgets?"+q.Encode(), nil, nil)
	return err
}

// ============================================================================
// /tags/colors
// ============================================================================

func (c *Client) TagColors() (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", "/tags/colors", nil, &out)
	return out, err
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ============================================================================
// /transactions/group, /transactions/split, /transactions, /transactions/file
// Mutation endpoints captured from the web UI on 2026-05-13.
// See captures/transactions-mutations.md for wire-level details + wire paths
// (these stubs use the path the bundle's hand-rolled client passes; the
// http layer must prepend /v2 for `group` and `split` per the captures).
// ============================================================================

// TransactionGroupCreate is the body for POST /v2/transactions/group.
type TransactionGroupCreate struct {
	IDs        []int64 `json:"ids"`  // child transaction IDs to group
	Date       string  `json:"date"` // YYYY-MM-DD
	Payee      string  `json:"payee"`
	CategoryID *int64  `json:"category_id,omitempty"`
	Notes      *string `json:"notes,omitempty"`
	TagIDs     []int64 `json:"tag_ids,omitempty"`
}

// CreateTransactionGroup → POST /v2/transactions/group.
// Returns {parent, children} per the bundle's typed-client consumers.
func (c *Client) CreateTransactionGroup(req TransactionGroupCreate) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("POST", "/v2/transactions/group", req, &out)
	return out, err
}

// UngroupTransactionGroup → DELETE /v2/transactions/group/{group_id}.
// Returns 204 No Content. Children become unparented.
func (c *Client) UngroupTransactionGroup(groupID int64) error {
	_, err := c.Do("DELETE", fmt.Sprintf("/v2/transactions/group/%d", groupID), nil, nil)
	return err
}

// UpdateTransactionGroup → PUT /transactions/group/{group_id}.
// Updates group metadata (date, payee, category, notes, tags) without splitting.
// Bundle-referenced but not live-verified — capture before relying on it.
func (c *Client) UpdateTransactionGroup(groupID int64, req TransactionGroupCreate) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("PUT", fmt.Sprintf("/transactions/group/%d", groupID), req, &out)
	return out, err
}

// BulkUngroupTransactions → PUT /transactions/group/bulk_ungroup.
// Body: {transaction_ids:[...]}. Returns {orphaned, asset_update?, background_job?}.
func (c *Client) BulkUngroupTransactions(transactionIDs []int64) (map[string]any, error) {
	body := map[string]any{"transaction_ids": transactionIDs}
	var out map[string]any
	_, err := c.Do("PUT", "/transactions/group/bulk_ungroup", body, &out)
	return out, err
}

// SplitChild describes one child of a split. Either Amount or AmountPct must be
// non-nil. The web UI sends both for percentage splits.
type SplitChild struct {
	Date       string  `json:"date,omitempty"`
	Payee      string  `json:"payee,omitempty"`
	Amount     *string `json:"amount,omitempty"`     // string form
	AmountPct  *int    `json:"amount_pct,omitempty"` // 0–100
	Currency   string  `json:"currency,omitempty"`
	CategoryID *int64  `json:"category_id,omitempty"`
	Notes      *string `json:"notes,omitempty"`
	TagIDs     []int64 `json:"tag_ids,omitempty"`
}

// SplitTransaction → POST /v2/transactions/split/{parent_id}.
// Body: {child_transactions:[...]}. Returns {split:[children], parent}.
// Parent gets has_children:true on success.
func (c *Client) SplitTransaction(parentID int64, children []SplitChild) (map[string]any, error) {
	body := map[string]any{"child_transactions": children}
	var out map[string]any
	_, err := c.Do("POST", fmt.Sprintf("/v2/transactions/split/%d", parentID), body, &out)
	return out, err
}

// BulkUnsplitTransactions → PUT /transactions/split/bulk_unsplit.
// removeParents=false keeps the parent (default for single-tx unsplit).
// removeParents=true also deletes the parent.
func (c *Client) BulkUnsplitTransactions(transactionIDs []int64, removeParents bool) (map[string]any, error) {
	body := map[string]any{
		"transaction_ids": transactionIDs,
		"remove_parents":  removeParents,
	}
	var out map[string]any
	_, err := c.Do("PUT", "/transactions/split/bulk_unsplit", body, &out)
	return out, err
}

// TransactionCreate is the writable subset of the create-transaction body.
type TransactionCreate struct {
	Date           string  `json:"date"` // YYYY-MM-DD
	Payee          string  `json:"payee"`
	Notes          *string `json:"notes,omitempty"`
	Amount         float64 `json:"amount"`   // negative = outflow
	Currency       string  `json:"currency"` // "usd", etc.
	CategoryID     *int64  `json:"category_id,omitempty"`
	AssetID        *int64  `json:"asset_id,omitempty"`         // for manual accounts
	PlaidAccountID *int64  `json:"plaid_account_id,omitempty"` // for Plaid accounts
	Status         *string `json:"status,omitempty"`           // "cleared" | "uncleared"
	TagIDs         []int64 `json:"tag_ids,omitempty"`
}

// TransactionCreateOpts wraps server-side processing options.
type TransactionCreateOpts struct {
	ShouldConvert bool `json:"should_convert"` // true: convert from foreign currency
}

// CreateTransaction → POST /transactions.
// Returns {transactions:[{id, ...}], asset_update?:[...]}.
// IMPORTANT: do not mimic the web UI's inline-edit-row pattern — focus drift
// onto pending rows triggers PUT /transactions/{id} on real bank transactions.
// Always pass an explicit asset_id (or plaid_account_id).
// PATCH: Implement the captured internal transaction mutation endpoint.
func (c *Client) CreateTransaction(tx TransactionCreate, opts TransactionCreateOpts) (map[string]any, error) {
	body := map[string]any{
		"transaction": tx,
		"opts":        opts,
	}
	var out map[string]any
	_, err := c.Do("POST", "/transactions", body, &out)
	return out, err
}

// DeleteTransaction → DELETE /transactions/{id}.
func (c *Client) DeleteTransaction(id int64) error {
	_, err := c.Do("DELETE", fmt.Sprintf("/transactions/%d", id), nil, nil)
	return err
}

// BulkDeleteTransactions → POST /transactions/bulk_delete.
// Body: {transaction_ids:[...]}. Returns {asset_update?, background_job?}.
func (c *Client) BulkDeleteTransactions(ids []int64) (map[string]any, error) {
	body := map[string]any{"transaction_ids": ids}
	var out map[string]any
	_, err := c.Do("POST", "/transactions/bulk_delete", body, &out)
	return out, err
}

// UploadTransactionFile → POST /transactions/file/{transaction_id}.
// fileContent is the raw file bytes. mime is the content type (auto-detected
// from filename if "").
// Wire format: multipart/form-data with a single "file" field.
// This is the LEGACY path used by the active web UI; see also AttachFile.
// PATCH: Implement captured legacy transaction attachment upload.
func (c *Client) UploadTransactionFile(txID int64, filename string, fileContent []byte, mime string) (map[string]any, error) {
	var out map[string]any
	_, err := c.DoMultipart("POST", fmt.Sprintf("/transactions/file/%d", txID), nil, "file", filename, fileContent, mime, &out)
	return out, err
}

// UpdateTransactionFile → PUT /transactions/file/{file_id}.
// Body: {transaction_id:<new_tx_id>} re-assigns the file to a different tx.
func (c *Client) UpdateTransactionFile(fileID int64, newTxID int64) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("PUT", fmt.Sprintf("/transactions/file/%d", fileID), map[string]any{"transaction_id": newTxID}, &out)
	return out, err
}

// DeleteTransactionFile → DELETE /transactions/file/{file_id}.
func (c *Client) DeleteTransactionFile(fileID int64) error {
	_, err := c.Do("DELETE", fmt.Sprintf("/transactions/file/%d", fileID), nil, nil)
	return err
}

// GetTransactionFileURL → GET /transactions/file/{file_id}.
// Returns the signed-URL response body. Use Resp.file_url to download.
func (c *Client) GetTransactionFileURL(fileID int64) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", fmt.Sprintf("/transactions/file/%d", fileID), nil, &out)
	return out, err
}

// AttachFile → POST /v2/transactions/{transaction_id}/attachments.
// The newer typed-client path. Same multipart body as UploadTransactionFile.
// PATCH: Implement captured typed attachment upload path.
func (c *Client) AttachFile(txID int64, filename string, fileContent []byte, mime string) (map[string]any, error) {
	var out map[string]any
	_, err := c.DoMultipart("POST", fmt.Sprintf("/v2/transactions/%d/attachments", txID), nil, "file", filename, fileContent, mime, &out)
	return out, err
}

// GetAttachmentURL → GET /v2/transactions/attachments/{file_id}.
func (c *Client) GetAttachmentURL(fileID int64) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("GET", fmt.Sprintf("/v2/transactions/attachments/%d", fileID), nil, &out)
	return out, err
}

// DeleteAttachment → DELETE /v2/transactions/attachments/{file_id}.
func (c *Client) DeleteAttachment(fileID int64) error {
	_, err := c.Do("DELETE", fmt.Sprintf("/v2/transactions/attachments/%d", fileID), nil, nil)
	return err
}

// UploadPDF → POST /transactions/file/pdf for statement parsing.
// assetID and plaidAccountID select the destination account; one must be set.
// Returns {data:{uuids:[...], processing:true}}.
// Poll GetPDFStatus until processing:false.
// PATCH: Implement captured PDF parsing upload/status helpers.
func (c *Client) UploadPDF(filename string, fileContent []byte, assetID, plaidAccountID *int64) (map[string]any, error) {
	fields := map[string]string{}
	if assetID != nil {
		fields["asset_id"] = strconv.FormatInt(*assetID, 10)
	}
	if plaidAccountID != nil {
		fields["plaid_account_id"] = strconv.FormatInt(*plaidAccountID, 10)
	}
	var out map[string]any
	_, err := c.DoMultipart("POST", "/transactions/file/pdf", fields, "file", filename, fileContent, "application/pdf", &out)
	return out, err
}

// GetPDFStatus → POST /transactions/file/pdf/status.
// Body: {uuids:[...]}. Returns {data:{processing, transactions?}}.
func (c *Client) GetPDFStatus(uuids []string) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("POST", "/transactions/file/pdf/status", map[string]any{"uuids": uuids}, &out)
	return out, err
}

// BulkInsertTransactionsCheck → PUT /transactions/bulk_insert/check.
// Dry-run a CSV/JSON import. Returns prepared transactions + per-row errors
// without persisting anything.
type BulkInsertOptions struct {
	ApplyRules          bool `json:"apply_rules"`
	SkipDuplicates      bool `json:"skip_duplicates"`
	CreateNewCategories bool `json:"create_new_categories"`
}

func (c *Client) BulkInsertTransactionsCheck(txns []TransactionCreate, opts BulkInsertOptions) (map[string]any, error) {
	// PATCH: Implement the captured CSV-import dry-run check endpoint.
	body := map[string]any{
		"transactions":          txns,
		"apply_rules":           opts.ApplyRules,
		"skip_duplicates":       opts.SkipDuplicates,
		"create_new_categories": opts.CreateNewCategories,
	}
	var out map[string]any
	_, err := c.Do("PUT", "/transactions/bulk_insert/check", body, &out)
	return out, err
}

// BulkInsertTransactions → PUT /transactions/bulk_insert. Commits prepared
// transactions. Returns {transactions:[...], total_count, asset_update?} or
// {background_job:...} for large imports.
func (c *Client) BulkInsertTransactions(txns []TransactionCreate) (map[string]any, error) {
	body := map[string]any{"transactions": txns}
	var out map[string]any
	_, err := c.Do("PUT", "/transactions/bulk_insert", body, &out)
	return out, err
}

// ListImportConfigs → GET /import_configs. Returns saved CSV column-mapping
// presets.
func (c *Client) ListImportConfigs() ([]map[string]any, error) {
	// PATCH: Implement captured CSV import preset listing.
	var out []map[string]any
	_, err := c.Do("GET", "/import_configs", nil, &out)
	return out, err
}

// SaveImportConfig → PUT /import_configs. Persists a column-mapping preset.
func (c *Client) SaveImportConfig(cfg map[string]any) (map[string]any, error) {
	var out map[string]any
	_, err := c.Do("PUT", "/import_configs", cfg, &out)
	return out, err
}

// See rules.go for the RuleActions type. Captures on 2026-05-13 surfaced the
// full action enum surface ({set_uncategorized, payee, notes, add_tag_ids,
// mark_as_reviewed, mark_as_unreviewed, should_send_email, should_delete,
// skip_recurring, dont_run_rules, stop_processing_others}); extend rules.go's
// RuleActions to add those fields before wiring CLI commands.
