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

	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
)

type Profile struct {
	Name           string    `json:"name"`
	ShopifyShop    string    `json:"shopify_shop,omitempty"`
	SiteURL        string    `json:"site_url,omitempty"`
	GAProperty     string    `json:"ga_property,omitempty"`
	GSCProperty    string    `json:"gsc_property,omitempty"`
	AhrefsTarget   string    `json:"ahrefs_target,omitempty"`
	KlaviyoAccount string    `json:"klaviyo_account,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProfilesFile struct {
	Profiles map[string]Profile `json:"profiles"`
}

type DataSet struct {
	Profile    string         `json:"profile"`
	SyncedAt   time.Time      `json:"synced_at"`
	Source     string         `json:"source"`
	Provenance DataProvenance `json:"provenance,omitempty"`
	Storefront Storefront     `json:"storefront"`
	Products   []Product      `json:"products"`
	Pages      []Page         `json:"pages"`
	Categories []Category     `json:"categories"`
	Emails     []EmailFlow    `json:"emails"`
	Markets    []Market       `json:"markets"`
}

type DataProvenance = intelcli.DataProvenance
type DateRange = intelcli.DateRange

type Storefront struct {
	Domain              string `json:"domain,omitempty"`
	LLMSTxt             bool   `json:"llms_txt"`
	StructuredData      bool   `json:"structured_data"`
	ProductFacts        bool   `json:"product_facts"`
	BuyingGuides        bool   `json:"buying_guides"`
	Answerability       int    `json:"answerability"`
	Currency            string `json:"currency,omitempty"`
	InventorySyncWindow string `json:"inventory_sync_window,omitempty"`
}

type Product struct {
	ID                  string        `json:"id,omitempty"`
	Handle              string        `json:"handle"`
	Title               string        `json:"title"`
	URL                 string        `json:"url,omitempty"`
	Vendor              string        `json:"vendor,omitempty"`
	Category            string        `json:"category,omitempty"`
	Status              string        `json:"status,omitempty"`
	Price               float64       `json:"price"`
	CompareAtPrice      float64       `json:"compare_at_price,omitempty"`
	GrossMargin         float64       `json:"gross_margin,omitempty"`
	Inventory           int           `json:"inventory"`
	DaysOfStock         float64       `json:"days_of_stock"`
	UnitsSold           int           `json:"units_sold"`
	Revenue             float64       `json:"revenue"`
	PreviousRevenue     float64       `json:"previous_revenue"`
	Sessions            int           `json:"sessions"`
	PreviousSessions    int           `json:"previous_sessions"`
	ConversionRate      float64       `json:"conversion_rate"`
	RefundRate          float64       `json:"refund_rate,omitempty"`
	SearchClicks        int           `json:"search_clicks"`
	PreviousClicks      int           `json:"previous_clicks"`
	SearchPosition      float64       `json:"search_position"`
	Backlinks           int           `json:"backlinks"`
	RefDomains          int           `json:"ref_domains"`
	EmailRevenue        float64       `json:"email_revenue"`
	EmailClicks         int           `json:"email_clicks"`
	HasStructuredData   bool          `json:"has_structured_data"`
	HasProductFacts     bool          `json:"has_product_facts"`
	HasFAQ              bool          `json:"has_faq"`
	HasBuyingGuideLinks bool          `json:"has_buying_guide_links"`
	Source              MetricSources `json:"sources"`
}

type Page struct {
	URL               string        `json:"url"`
	Title             string        `json:"title,omitempty"`
	Type              string        `json:"type,omitempty"`
	Revenue           float64       `json:"revenue"`
	PreviousRevenue   float64       `json:"previous_revenue"`
	Sessions          int           `json:"sessions"`
	PreviousSessions  int           `json:"previous_sessions"`
	SearchClicks      int           `json:"search_clicks"`
	PreviousClicks    int           `json:"previous_clicks"`
	SearchPosition    float64       `json:"search_position"`
	Backlinks         int           `json:"backlinks"`
	RefDomains        int           `json:"ref_domains"`
	StructuredData    bool          `json:"structured_data"`
	Answerable        bool          `json:"answerable"`
	BuyingGuide       bool          `json:"buying_guide"`
	PrimaryCollection string        `json:"primary_collection,omitempty"`
	Source            MetricSources `json:"sources"`
}

type Category struct {
	Handle           string        `json:"handle"`
	Title            string        `json:"title"`
	URL              string        `json:"url,omitempty"`
	Revenue          float64       `json:"revenue"`
	PreviousRevenue  float64       `json:"previous_revenue"`
	Sessions         int           `json:"sessions"`
	SearchClicks     int           `json:"search_clicks"`
	Products         int           `json:"products"`
	OutOfStock       int           `json:"out_of_stock"`
	HasBuyingGuide   bool          `json:"has_buying_guide"`
	HasFAQ           bool          `json:"has_faq"`
	StructuredData   bool          `json:"structured_data"`
	AnswerabilityGap string        `json:"answerability_gap,omitempty"`
	Source           MetricSources `json:"sources"`
}

type EmailFlow struct {
	Name            string        `json:"name"`
	Type            string        `json:"type,omitempty"`
	Revenue         float64       `json:"revenue"`
	Recipients      int           `json:"recipients"`
	OpenRate        float64       `json:"open_rate"`
	ClickRate       float64       `json:"click_rate"`
	ConversionRate  float64       `json:"conversion_rate"`
	AttributedSales int           `json:"attributed_sales"`
	NeedsAction     bool          `json:"needs_action"`
	Source          MetricSources `json:"sources"`
}

type Market struct {
	Country         string  `json:"country"`
	Revenue         float64 `json:"revenue"`
	Sessions        int     `json:"sessions"`
	ConversionRate  float64 `json:"conversion_rate"`
	Localized       bool    `json:"localized"`
	CurrencyPresent bool    `json:"currency_present"`
	ShippingClarity bool    `json:"shipping_clarity"`
	Hreflang        bool    `json:"hreflang"`
}

type MetricSources struct {
	Shopify SourceEvidence `json:"shopify"`
	Klaviyo SourceEvidence `json:"klaviyo"`
	GA4     SourceEvidence `json:"ga4"`
	GSC     SourceEvidence `json:"gsc"`
	Ahrefs  SourceEvidence `json:"ahrefs"`
}

type SourceEvidence struct {
	ChildCLICommand string `json:"child_cli_command,omitempty"`
	Synced          bool   `json:"synced"`
}

type Store struct{ Dir string }

func New(dir string) *Store { return &Store{Dir: dir} }
func DefaultDir() string {
	if v := os.Getenv("ECOMMERCE_INTEL_HOME"); v != "" {
		return v
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".ecommerce-intel-pp-cli")
	}
	return ".ecommerce-intel-pp-cli"
}
func (s *Store) ensure() error        { return os.MkdirAll(s.Dir, 0o755) }
func (s *Store) profilesPath() string { return filepath.Join(s.Dir, "profiles.json") }
func (s *Store) dataPath(profile string) string {
	return filepath.Join(s.Dir, safe(profile)+"-data.json")
}
func safe(v string) string {
	return intelcli.SafeName(v)
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
	if err := writeJSON(s.dataPath(d.Profile), d); err != nil {
		return err
	}
	if err := s.SaveSnapshot(d); err != nil {
		return err
	}
	return s.EnsureLearnings(d.Profile)
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

func Fixture(profile string) DataSet {
	now := time.Now().UTC()
	return DataSet{Profile: profile, SyncedAt: now, Source: "embedded-shopify-commerce-fixture", Storefront: Storefront{Domain: "example-store.myshopify.com", LLMSTxt: false, StructuredData: true, ProductFacts: true, BuyingGuides: false, Answerability: 63, Currency: "USD", InventorySyncWindow: "hourly"}, Products: []Product{
		product("boot-001", "waterproof-hiking-boot", "Waterproof Hiking Boot", "/products/waterproof-hiking-boot", "Footwear", "boots", 189, 94, 124, 18.6, 92, 17388, 22600, 980, 1200, .047, .03, 420, 510, 3.8, 87, 34, 2180, 185, true, true, false, false),
		product("jacket-002", "winter-shell-jacket", "Winter Shell Jacket", "/products/winter-shell-jacket", "Outerwear", "jackets", 249, 131, 38, 9.4, 54, 13446, 11240, 740, 680, .073, .025, 370, 330, 4.4, 64, 29, 1540, 120, true, false, false, false),
		product("pack-003", "ultralight-day-pack", "Ultralight Day Pack", "/products/ultralight-day-pack", "Bags", "packs", 119, 55, 8, 4.2, 41, 4879, 7600, 430, 610, .095, .01, 160, 230, 8.7, 20, 11, 940, 88, false, false, false, true),
	}, Pages: []Page{
		{URL: "/collections/jackets", Title: "Jackets Collection", Type: "collection", Revenue: 23880, PreviousRevenue: 21600, Sessions: 1760, PreviousSessions: 1600, SearchClicks: 760, PreviousClicks: 610, SearchPosition: 4.6, Backlinks: 112, RefDomains: 51, StructuredData: true, Answerable: false, BuyingGuide: false, PrimaryCollection: "jackets", Source: fullEvidence()},
		{URL: "/pages/shipping-returns", Title: "Shipping and Returns", Type: "page", Revenue: 4320, PreviousRevenue: 4100, Sessions: 510, PreviousSessions: 540, SearchClicks: 205, PreviousClicks: 240, SearchPosition: 5.5, Backlinks: 45, RefDomains: 26, StructuredData: false, Answerable: true, Source: fullEvidence()},
	}, Categories: []Category{
		{Handle: "jackets", Title: "Jackets", URL: "/collections/jackets", Revenue: 23880, PreviousRevenue: 21600, Sessions: 1760, SearchClicks: 760, Products: 18, OutOfStock: 2, HasBuyingGuide: false, HasFAQ: false, StructuredData: true, AnswerabilityGap: "missing comparison guide for answer engines", Source: fullEvidence()},
		{Handle: "packs", Title: "Packs", URL: "/collections/packs", Revenue: 4879, PreviousRevenue: 7600, Sessions: 430, SearchClicks: 160, Products: 9, OutOfStock: 4, HasBuyingGuide: true, HasFAQ: false, StructuredData: false, AnswerabilityGap: "product availability and schema gaps", Source: fullEvidence()},
	}, Emails: []EmailFlow{
		{Name: "Abandoned Checkout", Type: "flow", Revenue: 8200, Recipients: 2600, OpenRate: .44, ClickRate: .096, ConversionRate: .032, AttributedSales: 71, Source: MetricSources{Klaviyo: SourceEvidence{Synced: true, ChildCLICommand: "klaviyo-pp-cli report flow-comparison --agent"}}},
		{Name: "Post Purchase Cross-sell", Type: "flow", Revenue: 1700, Recipients: 1800, OpenRate: .37, ClickRate: .024, ConversionRate: .006, AttributedSales: 15, NeedsAction: true, Source: MetricSources{Klaviyo: SourceEvidence{Synced: true, ChildCLICommand: "klaviyo-pp-cli report flow-comparison --agent"}}},
	}, Markets: []Market{{Country: "US", Revenue: 39100, Sessions: 4200, ConversionRate: .039, Localized: true, CurrencyPresent: true, ShippingClarity: true, Hreflang: true}, {Country: "CA", Revenue: 5200, Sessions: 820, ConversionRate: .021, Localized: false, CurrencyPresent: true, ShippingClarity: false, Hreflang: false}}}
}

func fullEvidence() MetricSources {
	return MetricSources{Shopify: SourceEvidence{Synced: true, ChildCLICommand: "shopify-pp-cli products list --agent"}, Klaviyo: SourceEvidence{Synced: true, ChildCLICommand: "klaviyo-pp-cli report flow-comparison --agent"}, GA4: SourceEvidence{Synced: true, ChildCLICommand: "google-analytics-pp-cli top-pages --agent"}, GSC: SourceEvidence{Synced: true, ChildCLICommand: "google-search-console-pp-cli webmasters query-search-analytics <site> --agent"}, Ahrefs: SourceEvidence{Synced: true, ChildCLICommand: "ahrefs-pp-cli site-explorer top-pages --agent"}}
}

func product(id, handle, title, url, vendor, category string, price, margin float64, inventory int, days float64, units int, revenue, prevRevenue float64, sessions, prevSessions int, cr, refund float64, clicks, prevClicks int, pos float64, backlinks, refDomains int, emailRevenue float64, emailClicks int, sd, facts, faq, guide bool) Product {
	return Product{ID: id, Handle: handle, Title: title, URL: url, Vendor: vendor, Category: category, Status: "active", Price: price, GrossMargin: margin, Inventory: inventory, DaysOfStock: days, UnitsSold: units, Revenue: revenue, PreviousRevenue: prevRevenue, Sessions: sessions, PreviousSessions: prevSessions, ConversionRate: cr, RefundRate: refund, SearchClicks: clicks, PreviousClicks: prevClicks, SearchPosition: pos, Backlinks: backlinks, RefDomains: refDomains, EmailRevenue: emailRevenue, EmailClicks: emailClicks, HasStructuredData: sd, HasProductFacts: facts, HasFAQ: faq, HasBuyingGuideLinks: guide, Source: MetricSources{Shopify: SourceEvidence{Synced: true, ChildCLICommand: "shopify-pp-cli products/export --agent"}, Klaviyo: SourceEvidence{Synced: true, ChildCLICommand: "klaviyo-pp-cli reporting flows --agent"}, GA4: SourceEvidence{Synced: true, ChildCLICommand: "google-analytics-pp-cli ecommerce products --agent"}, GSC: SourceEvidence{Synced: true, ChildCLICommand: "google-search-console-pp-cli webmasters query-search-analytics <site> --agent"}, Ahrefs: SourceEvidence{Synced: true, ChildCLICommand: "ahrefs-pp-cli site-explorer top-pages --agent"}}}
}
