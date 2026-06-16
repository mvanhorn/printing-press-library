package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Profile struct {
	Name          string    `json:"name"`
	MarketplaceID string    `json:"marketplace_id,omitempty"`
	SellerID      string    `json:"seller_id,omitempty"`
	AdsProfileID  string    `json:"ads_profile_id,omitempty"`
	DefaultDays   int       `json:"default_days,omitempty"`
	COGSFile      string    `json:"cogs_file,omitempty"`
	SellerStoreDB string    `json:"seller_store_db,omitempty"`
	AdsReportDir  string    `json:"ads_report_dir,omitempty"`
	TargetACOS    float64   `json:"target_acos,omitempty"`
	TargetMargin  float64   `json:"target_margin,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProfilesFile struct {
	Profiles map[string]Profile `json:"profiles"`
}

type SourceEvidence struct {
	Present         bool      `json:"present"`
	Source          string    `json:"source,omitempty"`
	ChildCLICommand string    `json:"child_cli_command,omitempty"`
	ImportedFrom    string    `json:"imported_from,omitempty"`
	SyncedAt        time.Time `json:"synced_at,omitempty"`
	Fields          []string  `json:"fields,omitempty"`
}

type MetricSources struct {
	Seller         SourceEvidence `json:"seller,omitempty"`
	Ads            SourceEvidence `json:"ads,omitempty"`
	BrandAnalytics SourceEvidence `json:"brand_analytics,omitempty"`
	Listings       SourceEvidence `json:"listings,omitempty"`
	Reports        SourceEvidence `json:"reports,omitempty"`
	LocalImport    SourceEvidence `json:"local_import,omitempty"`
	VendorFiles    SourceEvidence `json:"vendor_files,omitempty"`
}

type SKU struct {
	SKU                string        `json:"sku"`
	ASIN               string        `json:"asin,omitempty"`
	Title              string        `json:"title,omitempty"`
	MarketplaceID      string        `json:"marketplace_id,omitempty"`
	ParentASIN         string        `json:"parent_asin,omitempty"`
	Brand              string        `json:"brand,omitempty"`
	Category           string        `json:"category,omitempty"`
	Revenue            float64       `json:"revenue,omitempty"`
	UnitsSold          int           `json:"units_sold,omitempty"`
	Sessions           int           `json:"sessions,omitempty"`
	ConversionRate     float64       `json:"conversion_rate,omitempty"`
	BuyBoxPct          float64       `json:"buy_box_pct,omitempty"`
	ReturnRate         float64       `json:"return_rate,omitempty"`
	FBAAvailable       int           `json:"fba_available,omitempty"`
	Reserved           int           `json:"reserved,omitempty"`
	Inbound            int           `json:"inbound,omitempty"`
	DaysOfCover        float64       `json:"days_of_cover,omitempty"`
	LeadTimeDays       int           `json:"lead_time_days,omitempty"`
	AgingDays          int           `json:"aging_days,omitempty"`
	Stranded           bool          `json:"stranded,omitempty"`
	COGS               float64       `json:"cogs,omitempty"`
	ReferralFees       float64       `json:"referral_fees,omitempty"`
	FBAFees            float64       `json:"fba_fees,omitempty"`
	StorageFees        float64       `json:"storage_fees,omitempty"`
	Reimbursements     float64       `json:"reimbursements,omitempty"`
	Profit             float64       `json:"profit,omitempty"`
	ContributionMargin float64       `json:"contribution_margin,omitempty"`
	AdSpend            float64       `json:"ad_spend,omitempty"`
	AdSales            float64       `json:"ad_sales,omitempty"`
	ACOS               float64       `json:"acos,omitempty"`
	TACOS              float64       `json:"tacos,omitempty"`
	BreakEvenACOS      float64       `json:"break_even_acos,omitempty"`
	ListingScore       float64       `json:"listing_score,omitempty"`
	Suppressed         bool          `json:"suppressed,omitempty"`
	Defects            []string      `json:"defects,omitempty"`
	ReimbursementDue   float64       `json:"reimbursement_due,omitempty"`
	Source             MetricSources `json:"source,omitempty"`
}

type Campaign struct {
	CampaignID   string        `json:"campaign_id"`
	Name         string        `json:"name,omitempty"`
	ASIN         string        `json:"asin,omitempty"`
	SKU          string        `json:"sku,omitempty"`
	Spend        float64       `json:"spend,omitempty"`
	Sales        float64       `json:"sales,omitempty"`
	Orders       int           `json:"orders,omitempty"`
	Clicks       int           `json:"clicks,omitempty"`
	Impressions  int           `json:"impressions,omitempty"`
	ACOS         float64       `json:"acos,omitempty"`
	CPC          float64       `json:"cpc,omitempty"`
	CTR          float64       `json:"ctr,omitempty"`
	CVR          float64       `json:"cvr,omitempty"`
	BudgetStatus string        `json:"budget_status,omitempty"`
	Source       MetricSources `json:"source,omitempty"`
}

type SearchTerm struct {
	Term           string        `json:"term"`
	ASIN           string        `json:"asin,omitempty"`
	SKU            string        `json:"sku,omitempty"`
	Spend          float64       `json:"spend,omitempty"`
	Sales          float64       `json:"sales,omitempty"`
	Orders         int           `json:"orders,omitempty"`
	Clicks         int           `json:"clicks,omitempty"`
	Impressions    int           `json:"impressions,omitempty"`
	OrganicRank    int           `json:"organic_rank,omitempty"`
	ClickShare     float64       `json:"click_share,omitempty"`
	ConversionRate float64       `json:"conversion_rate,omitempty"`
	AdAction       string        `json:"ad_action,omitempty"`
	Source         MetricSources `json:"source,omitempty"`
}

type ListingHealth struct {
	SKU            string        `json:"sku,omitempty"`
	ASIN           string        `json:"asin,omitempty"`
	Title          string        `json:"title,omitempty"`
	Score          float64       `json:"score,omitempty"`
	Suppressed     bool          `json:"suppressed,omitempty"`
	Defects        []string      `json:"defects,omitempty"`
	Sessions       int           `json:"sessions,omitempty"`
	ConversionRate float64       `json:"conversion_rate,omitempty"`
	AdSpend        float64       `json:"ad_spend,omitempty"`
	Source         MetricSources `json:"source,omitempty"`
}

type AccountHealth struct {
	Score              float64       `json:"score,omitempty"`
	AtRiskCount        int           `json:"at_risk_count,omitempty"`
	ReturnSpikeCount   int           `json:"return_spike_count,omitempty"`
	ReimbursementFlags int           `json:"reimbursement_flags,omitempty"`
	SettlementGap      float64       `json:"settlement_gap,omitempty"`
	Source             MetricSources `json:"source,omitempty"`
}

type PurchaseOrder struct {
	POID                string        `json:"po_id"`
	SKU                 string        `json:"sku,omitempty"`
	ASIN                string        `json:"asin,omitempty"`
	Units               int           `json:"units,omitempty"`
	UnitCost            float64       `json:"unit_cost,omitempty"`
	ExpectedShipDate    string        `json:"expected_ship_date,omitempty"`
	ExpectedReceiveDate string        `json:"expected_receive_date,omitempty"`
	Status              string        `json:"status,omitempty"`
	Source              MetricSources `json:"source,omitempty"`
}

type VendorDeduction struct {
	ID         string        `json:"id"`
	Type       string        `json:"type,omitempty"`
	SKU        string        `json:"sku,omitempty"`
	ASIN       string        `json:"asin,omitempty"`
	Amount     float64       `json:"amount,omitempty"`
	Reason     string        `json:"reason,omitempty"`
	DisputeBy  string        `json:"dispute_by,omitempty"`
	Confidence float64       `json:"confidence,omitempty"`
	Source     MetricSources `json:"source,omitempty"`
}

type BundleSignal struct {
	PrimaryASIN       string        `json:"primary_asin,omitempty"`
	SecondaryASIN     string        `json:"secondary_asin,omitempty"`
	PrimarySKU        string        `json:"primary_sku,omitempty"`
	SecondarySKU      string        `json:"secondary_sku,omitempty"`
	Confidence        float64       `json:"confidence,omitempty"`
	CombinedMargin    float64       `json:"combined_margin,omitempty"`
	InventoryFeasible bool          `json:"inventory_feasible,omitempty"`
	SuggestedOffer    string        `json:"suggested_offer,omitempty"`
	Source            MetricSources `json:"source,omitempty"`
}

type LaunchPlan struct {
	SKU            string        `json:"sku,omitempty"`
	ASIN           string        `json:"asin,omitempty"`
	TargetACOS     float64       `json:"target_acos,omitempty"`
	LaunchBudget   float64       `json:"launch_budget,omitempty"`
	InventoryUnits int           `json:"inventory_units,omitempty"`
	COGS           float64       `json:"cogs,omitempty"`
	Keywords       []string      `json:"keywords,omitempty"`
	ListingScore   float64       `json:"listing_score,omitempty"`
	Source         MetricSources `json:"source,omitempty"`
}

type DataSet struct {
	Profile          string            `json:"profile"`
	Source           string            `json:"source"`
	SyncedAt         time.Time         `json:"synced_at"`
	SKUs             []SKU             `json:"skus"`
	Campaigns        []Campaign        `json:"campaigns,omitempty"`
	SearchTerms      []SearchTerm      `json:"search_terms,omitempty"`
	Listings         []ListingHealth   `json:"listings,omitempty"`
	PurchaseOrders   []PurchaseOrder   `json:"purchase_orders,omitempty"`
	VendorDeductions []VendorDeduction `json:"vendor_deductions,omitempty"`
	BundleSignals    []BundleSignal    `json:"bundle_signals,omitempty"`
	LaunchPlans      []LaunchPlan      `json:"launch_plans,omitempty"`
	Account          AccountHealth     `json:"account,omitempty"`
}

type Store struct{ Dir string }

func New(dir string) *Store { return &Store{Dir: dir} }

func DefaultDir() string {
	if v := os.Getenv("AMAZON_OPERATOR_INTEL_HOME"); v != "" {
		return v
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".amazon-operator-intel-pp-cli")
	}
	return ".amazon-operator-intel-pp-cli"
}

func (s *Store) ensure() error        { return os.MkdirAll(s.Dir, 0o755) }
func (s *Store) profilesPath() string { return filepath.Join(s.Dir, "profiles.json") }
func (s *Store) dataPath(profile string) string {
	return filepath.Join(s.Dir, safe(profile)+"-data.json")
}

func safe(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return "default"
	}
	r := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return r.Replace(v)
}

func (s *Store) LoadProfiles() (ProfilesFile, error) {
	pf := ProfilesFile{Profiles: map[string]Profile{}}
	b, err := os.ReadFile(s.profilesPath())
	if errors.Is(err, os.ErrNotExist) {
		return pf, nil
	}
	if err != nil {
		return pf, err
	}
	if len(b) == 0 {
		return pf, nil
	}
	if err := json.Unmarshal(b, &pf); err != nil {
		return pf, err
	}
	if pf.Profiles == nil {
		pf.Profiles = map[string]Profile{}
	}
	return pf, nil
}

func (s *Store) SaveProfile(p Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.MarketplaceID == "" {
		p.MarketplaceID = "ATVPDKIKX0DER"
	}
	if p.DefaultDays == 0 {
		p.DefaultDays = 30
	}
	if err := s.ensure(); err != nil {
		return err
	}
	pf, err := s.LoadProfiles()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		if old, ok := pf.Profiles[p.Name]; ok {
			p.CreatedAt = old.CreatedAt
		} else {
			p.CreatedAt = now
		}
	}
	p.UpdatedAt = now
	pf.Profiles[p.Name] = p
	return writeJSON(s.profilesPath(), pf)
}

func (s *Store) DeleteProfile(name string) error {
	pf, err := s.LoadProfiles()
	if err != nil {
		return err
	}
	if _, ok := pf.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(pf.Profiles, name)
	if err := s.ensure(); err != nil {
		return err
	}
	return writeJSON(s.profilesPath(), pf)
}

func (s *Store) GetProfile(name string) (Profile, error) {
	pf, err := s.LoadProfiles()
	if err != nil {
		return Profile{}, err
	}
	p, ok := pf.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found", name)
	}
	return p, nil
}

func (s *Store) ListProfiles() ([]Profile, error) {
	pf, err := s.LoadProfiles()
	if err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(pf.Profiles))
	for _, p := range pf.Profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) SaveData(d DataSet) error {
	if strings.TrimSpace(d.Profile) == "" {
		d.Profile = "default"
	}
	if d.SyncedAt.IsZero() {
		d.SyncedAt = time.Now().UTC()
	}
	if err := s.ensure(); err != nil {
		return err
	}
	return writeJSON(s.dataPath(d.Profile), d)
}

func (s *Store) LoadData(profile string) (DataSet, error) {
	var d DataSet
	b, err := os.ReadFile(s.dataPath(profile))
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return d, err
	}
	return d, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
