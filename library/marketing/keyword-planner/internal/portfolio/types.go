// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

// Package portfolio stores immutable Google Keyword Planner evidence and its
// normalized, provenance-linked projections. The package deliberately owns a
// separate SQLite database from the generated resource/learning store.
package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const (
	EndpointIdeas       = "ideas"
	EndpointHistorical  = "historical"
	EndpointAccount     = "account"
	NetworkGoogleSearch = "GOOGLE_SEARCH"

	StatusInProgress = "in_progress"
	StatusComplete   = "complete"
	StatusIncomplete = "incomplete"
	StatusFailed     = "failed"

	EligibilityEligible = "eligible"
	EligibilityRawOnly  = "raw_only"
)

var (
	ErrNotFound           = errors.New("portfolio object not found")
	ErrAlreadyNormalized  = errors.New("portfolio receipt already normalized")
	ErrFinalized          = errors.New("portfolio snapshot is finalized")
	ErrInvalidInput       = errors.New("invalid portfolio input")
	ErrInvalidResponse    = errors.New("invalid portfolio response")
	ErrIncompleteSnapshot = errors.New("portfolio snapshot is incomplete")
)

// Store is an append-only evidence portfolio backed by one SQLite file.
// Evidence rows are never replaced. Snapshot status fields are mutable only
// while a collection is in progress; Finish seals the snapshot.
type Store struct {
	db       *sql.DB
	path     string
	readOnly bool
	writeMu  chan struct{}
}

// Month is a calendar month with a one-based month number.
type Month struct {
	Year   int `json:"year"`
	Number int `json:"number"`
}

// String returns the ISO month-start representation used by SQLite and the
// DuckDB compatibility relation.
func (m Month) String() string {
	return formatMonth(m)
}

// SnapshotInput is the non-secret request context shared by all pages/batches
// in a collection. Slices are copied at the write boundary and their original
// order is retained for seeds and submitted keywords.
type SnapshotInput struct {
	RunID             string `json:"run_id"`
	Endpoint          string `json:"endpoint"`
	APIVersion        string `json:"api_version"`
	DiscoveryRevision string `json:"discovery_revision"`

	CustomerID      string `json:"customer_id"`
	LoginCustomerID string `json:"login_customer_id"`

	Language       string `json:"language"`
	Network        string `json:"network"`
	CurrencyCode   string `json:"currency_code"`
	CurrencySource string `json:"currency_source"`

	GeoTargetConstants []string `json:"geo_target_constants"`
	SubmittedSeeds     []string `json:"submitted_seeds"`
	SubmittedKeywords  []string `json:"submitted_keywords"`

	RequestedStart string `json:"requested_start"` // YYYY-MM
	RequestedEnd   string `json:"requested_end"`   // YYYY-MM

	RequestBody   json.RawMessage `json:"request_body"`
	SourceVariant string          `json:"source_variant"`
	Metadata      json.RawMessage `json:"metadata"`
}

// Snapshot identifies one collection. It remains distinct even when the
// request is byte-for-byte identical to an earlier collection.
type Snapshot struct {
	ID                 string          `json:"id"`
	RunID              string          `json:"run_id"`
	Endpoint           string          `json:"endpoint"`
	APIVersion         string          `json:"api_version"`
	DiscoveryRevision  string          `json:"discovery_revision"`
	CustomerID         string          `json:"customer_id"`
	LoginCustomerID    string          `json:"login_customer_id"`
	Language           string          `json:"language"`
	Network            string          `json:"network"`
	CurrencyCode       string          `json:"currency_code"`
	CurrencySource     string          `json:"currency_source"`
	GeoSetID           string          `json:"geo_set_id"`
	GeoTargetConstants []string        `json:"geo_target_constants"`
	SubmittedSeeds     []string        `json:"submitted_seeds"`
	SubmittedKeywords  []string        `json:"submitted_keywords"`
	RequestedStart     string          `json:"requested_start"`
	RequestedEnd       string          `json:"requested_end"`
	RequestBody        json.RawMessage `json:"request_body"`
	SourceVariant      string          `json:"source_variant"`
	Metadata           json.RawMessage `json:"metadata"`
	RequestBodyHash    string          `json:"request_body_sha256"`
	FetchedAt          time.Time       `json:"fetched_at"`
	Status             string          `json:"status"`
	Complete           bool            `json:"complete"`
	Finalized          bool            `json:"finalized"`
	FinishedAt         *time.Time      `json:"finished_at"`
	WarningFlags       []string        `json:"warning_flags"`
	FailedReceipts     []string        `json:"failed_receipts"`
	MissingBatches     []string        `json:"missing_batches"`
}

// ReceiptInput is one exact HTTP response page or historical batch. Body and
// RequestBody are copied as bytes; Body may be nil when transport returned no
// response body, which is itself preserved as a failure receipt.
type ReceiptInput struct {
	SnapshotID  string
	Endpoint    string
	PageToken   string
	PageNumber  int
	BatchNumber int
	Attempt     int

	RequestBody       json.RawMessage
	SubmittedKeywords []string

	HTTPStatus      int
	GoogleRequestID string
	Body            []byte
	FetchedAt       time.Time
	ErrorCode       string
	ErrorMessage    string
}

// ResponseIDs identifies a committed receipt and its exact body hash.
type ResponseIDs struct {
	SnapshotID  string `json:"snapshot_id"`
	ReceiptID   string `json:"receipt_id"`
	PageNumber  int    `json:"page_number"`
	BatchNumber int    `json:"batch_number"`
	Attempt     int    `json:"attempt"`
	BodySHA256  string `json:"body_sha256"`
}

// FinishInput seals a snapshot. Complete may be true only when all pages or
// batches and all normalized writes succeeded.
type FinishInput struct {
	SnapshotID     string   `json:"snapshot_id"`
	Status         string   `json:"status"`
	Complete       bool     `json:"complete"`
	WarningFlags   []string `json:"warning_flags"`
	FailedReceipts []string `json:"failed_receipts"`
	MissingBatches []string `json:"missing_batches"`
}

// NormalizeResult reports the durable projection counts for one receipt.
type NormalizeResult struct {
	SnapshotID   string   `json:"snapshot_id"`
	ReceiptID    string   `json:"receipt_id"`
	KeywordRows  int      `json:"keyword_rows"`
	MonthlyRows  int      `json:"monthly_rows"`
	CoverageRows int      `json:"coverage_rows"`
	VariantRows  int      `json:"variant_rows"`
	Warnings     []string `json:"warnings"`
	Complete     bool     `json:"complete"`
}

// QueryOptions controls read-only portfolio queries. Empty filters mean no
// filter. GeoTargets are canonicalized and compared as an exact sorted set;
// GeoSetID can be used when the caller already has the stored identity.
type QueryOptions struct {
	SnapshotID string   `json:"snapshot_id"`
	Keyword    string   `json:"keyword"`
	StartMonth string   `json:"start_month"`
	EndMonth   string   `json:"end_month"`
	Language   string   `json:"language"`
	GeoSetID   string   `json:"geo_set_id"`
	GeoTargets []string `json:"geo_targets"`
	Endpoint   string   `json:"endpoint"`

	IncludeIncomplete  bool `json:"include_incomplete"`
	IncludeUnavailable bool `json:"include_unavailable"`
	AvailableOnly      bool `json:"available_only"`
	Limit              int  `json:"limit"`
}

// Row is the provenance-rich monthly row returned by Rows and Search.
// MonthlySearches and money fields are pointers so SQL NULL remains distinct
// from the integer zero.
type Row struct {
	SnapshotID       string `json:"snapshot_id"`
	ReceiptID        string `json:"receipt_id"`
	MetricID         string `json:"metric_id"`
	Endpoint         string `json:"endpoint"`
	Keyword          string `json:"keyword"`
	SubmittedKeyword string `json:"submitted_keyword"`
	SubmittedIndex   *int   `json:"submitted_index"`
	VariantGroup     string `json:"variant_group"`
	LinkageStatus    string `json:"linkage_status"`

	GeoSetID       string   `json:"geo_set_id"`
	GeoTargets     []string `json:"geo_targets"`
	Language       string   `json:"language"`
	Network        string   `json:"network"`
	Currency       string   `json:"currency"`
	CurrencySource string   `json:"currency_source"`

	Month                  string `json:"month"`
	RawMonth               string `json:"raw_month"`
	RawYear                string `json:"raw_year"`
	MonthlySearches        *int64 `json:"monthly_searches"`
	ValueState             string `json:"value_state"`
	LowBidMicros           *int64 `json:"low_bid_micros"`
	HighBidMicros          *int64 `json:"high_bid_micros"`
	AverageCPCMicros       *int64 `json:"average_cpc_micros"`
	AverageMonthlySearches *int64 `json:"average_monthly_searches"`
	Competition            string `json:"competition"`
	CompetitionIndex       *int64 `json:"competition_index"`

	Status         string    `json:"status"`
	Flags          []string  `json:"flags"`
	FetchedAt      time.Time `json:"fetched_at"`
	RequestedStart string    `json:"requested_start"`
	RequestedEnd   string    `json:"requested_end"`
	Complete       bool      `json:"complete"`
}

// SnapshotView is the complete offline view of a snapshot and its evidence.
type SnapshotView struct {
	Snapshot Snapshot       `json:"snapshot"`
	Receipts []ReceiptView  `json:"receipts"`
	Rows     []Row          `json:"rows"`
	Coverage []CoverageView `json:"coverage"`
}

// ReceiptView is safe to display offline; Body is exact raw response bytes.
type ReceiptView struct {
	ResponseIDs
	Endpoint          string          `json:"endpoint"`
	PageToken         string          `json:"page_token"`
	RequestBody       json.RawMessage `json:"request_body"`
	SubmittedKeywords []string        `json:"submitted_keywords"`
	HTTPStatus        int             `json:"http_status"`
	GoogleRequestID   string          `json:"google_request_id"`
	Body              []byte          `json:"raw_body_base64"`
	BodyPresent       bool            `json:"body_present"`
	FetchedAt         time.Time       `json:"fetched_at"`
	ErrorCode         string          `json:"error_code"`
	ErrorMessage      string          `json:"error_message"`
	Normalized        bool            `json:"normalized"`
}

// CoverageView reports requested versus returned coverage for one page or
// batch. JSON arrays preserve order where order is meaningful.
type CoverageView struct {
	SnapshotID         string   `json:"snapshot_id"`
	ReceiptID          string   `json:"receipt_id"`
	Endpoint           string   `json:"endpoint"`
	PageNumber         int      `json:"page_number"`
	BatchNumber        int      `json:"batch_number"`
	Attempt            int      `json:"attempt"`
	RequestedStart     string   `json:"requested_start"`
	RequestedEnd       string   `json:"requested_end"`
	RequestedTerms     []string `json:"requested_terms"`
	ReturnedTerms      []string `json:"returned_terms"`
	ReturnedMonths     []string `json:"returned_months"`
	MissingMonths      []string `json:"missing_months"`
	UnavailableMonths  []string `json:"unavailable_months"`
	RawOnlyMonths      []string `json:"raw_only_months"`
	RequestedCount     int      `json:"requested_count"`
	ReturnedCount      int      `json:"returned_count"`
	ReturnedMonthCount int      `json:"returned_month_count"`
	NoResult           bool     `json:"no_result"`
	CollectionStatus   string   `json:"collection_status"`
	Complete           bool     `json:"complete"`
	NextPageToken      string   `json:"next_page_token"`
	VendorTotalSize    *int64   `json:"vendor_total_size"`
}

// AccountMetadata records verified account context, such as currency, as an
// append-only event tied to the exact account-helper receipt that supplied it.
// It is separate from the original snapshot request because account lookup
// happens after a collection snapshot is opened and before Planner decoding.
type AccountMetadata struct {
	ID           string    `json:"id"`
	SnapshotID   string    `json:"snapshot_id"`
	ReceiptID    string    `json:"receipt_id"`
	CurrencyCode string    `json:"currency_code"`
	Source       string    `json:"source"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// ExportFormat is accepted by Export.
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
)

// Open opens or creates a portfolio database and applies only portfolio
// migrations. The parent directory is created with private permissions.
func Open(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, false)
}

// OpenReadOnly opens an existing portfolio database without creating or
// migrating it. Missing files return ErrNotFound so offline commands can render
// an empty result rather than contacting Google or inventing data.
func OpenReadOnly(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, true)
}
