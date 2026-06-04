package research

import (
	"encoding/json"
	"time"
)

type ScopeKind string

const (
	ScopeQuery   ScopeKind = "query"
	ScopeKeyword ScopeKind = "keyword"
	ScopeShop    ScopeKind = "shop"
	ScopeListing ScopeKind = "listing_id"
)

type ResearchScope struct {
	Kind  ScopeKind `json:"kind"`
	Value string    `json:"value"`
}

type DataSource string

const (
	DataSourceLocal              DataSource = "local"
	DataSourceRefreshed          DataSource = "refreshed"
	DataSourceStaleLocalFallback DataSource = "stale-local-fallback"
	DataSourceNone               DataSource = "none"
)

type Decision string

const (
	DecisionUseLocal         Decision = "use_local"
	DecisionRefreshTargeted  Decision = "refresh_targeted"
	DecisionFallbackLocal    Decision = "fallback_local"
	DecisionInsufficientData Decision = "insufficient_data"
)

type PlanOptions struct {
	Now        time.Time
	MaxAge     time.Duration
	CanRefresh bool
	NoRefresh  bool
}

type ResearchPlan struct {
	Scope      ResearchScope `json:"scope"`
	Decision   Decision      `json:"decision"`
	DataSource DataSource    `json:"data_source"`
	Snapshot   *Snapshot     `json:"snapshot,omitempty"`
	Freshness  Freshness     `json:"freshness"`
	Warnings   []string      `json:"warnings,omitempty"`
}

type Freshness struct {
	FetchedAt     time.Time `json:"fetched_at,omitempty"`
	AgeSeconds    int64     `json:"age_seconds"`
	MaxAgeSeconds int64     `json:"max_age_seconds"`
	Fresh         bool      `json:"fresh"`
}

type Snapshot struct {
	ID         int64             `json:"id,omitempty"`
	Scope      ResearchScope     `json:"scope"`
	Resources  []string          `json:"resources,omitempty"`
	FetchedAt  time.Time         `json:"fetched_at"`
	FreshFor   time.Duration     `json:"fresh_for,omitempty"`
	RawRecords []json.RawMessage `json:"raw_records,omitempty"`
	Evidence   []EvidenceRecord  `json:"evidence,omitempty"`
	Coverage   Coverage          `json:"coverage"`
	Warnings   []string          `json:"warnings,omitempty"`
}

type Coverage struct {
	ResourceCounts      map[string]int `json:"resource_counts,omitempty"`
	RawRecordCount      int            `json:"raw_record_count"`
	EvidenceRecordCount int            `json:"evidence_record_count"`
}

type EvidenceRecord struct {
	ID               string    `json:"id,omitempty"`
	Resource         string    `json:"resource,omitempty"`
	Title            string    `json:"title,omitempty"`
	ShopName         string    `json:"shop_name,omitempty"`
	ListingID        string    `json:"listing_id,omitempty"`
	Keywords         []string  `json:"keywords,omitempty"`
	Tags             []string  `json:"tags,omitempty"`
	Price            *float64  `json:"price,omitempty"`
	EstimatedSales   *float64  `json:"estimated_sales,omitempty"`
	EstimatedRevenue *float64  `json:"estimated_revenue,omitempty"`
	Rank             int       `json:"rank,omitempty"`
	SearchableText   string    `json:"searchable_text,omitempty"`
	CapturedAt       time.Time `json:"captured_at,omitempty"`
}

type InsightRecord struct {
	ID          string   `json:"id,omitempty"`
	Label       string   `json:"label"`
	Score       float64  `json:"score"`
	Reasons     []string `json:"reasons,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type ResponseEnvelope struct {
	Scope       ResearchScope    `json:"scope"`
	DataSource  DataSource       `json:"data_source"`
	Freshness   Freshness        `json:"freshness"`
	Summary     string           `json:"summary"`
	Records     []InsightRecord  `json:"records"`
	Evidence    []EvidenceRecord `json:"evidence"`
	Confidence  float64          `json:"confidence"`
	Coverage    Coverage         `json:"coverage"`
	Warnings    []string         `json:"warnings"`
	NextActions []string         `json:"next_actions"`
}

func (e ResponseEnvelope) MarshalJSON() ([]byte, error) {
	type responseEnvelopeJSON ResponseEnvelope
	if e.Records == nil {
		e.Records = []InsightRecord{}
	}
	if e.Evidence == nil {
		e.Evidence = []EvidenceRecord{}
	}
	if e.Warnings == nil {
		e.Warnings = []string{}
	}
	if e.NextActions == nil {
		e.NextActions = []string{}
	}
	return json.Marshal(responseEnvelopeJSON(e))
}
