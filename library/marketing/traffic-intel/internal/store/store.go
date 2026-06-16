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
	Name       string    `json:"name"`
	SiteURL    string    `json:"site_url,omitempty"`
	GAProperty string    `json:"ga_property,omitempty"`
	Ahrefs     string    `json:"ahrefs_project,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ProfilesFile struct {
	Profiles map[string]Profile `json:"profiles"`
}

type PageMetrics struct {
	URL              string        `json:"url"`
	Title            string        `json:"title,omitempty"`
	Clicks           int           `json:"clicks"`
	Impressions      int           `json:"impressions"`
	CTR              float64       `json:"ctr"`
	Position         float64       `json:"position"`
	Sessions         int           `json:"sessions"`
	Conversions      int           `json:"conversions"`
	Revenue          float64       `json:"revenue"`
	PreviousClicks   int           `json:"previous_clicks"`
	PreviousSessions int           `json:"previous_sessions"`
	PreviousRevenue  float64       `json:"previous_revenue"`
	Backlinks        int           `json:"backlinks"`
	RefDomains       int           `json:"ref_domains"`
	Sources          MetricSources `json:"sources"`
	UpdatedAt        string        `json:"updated_at,omitempty"`
}

type MetricSources struct {
	GSC    GSCMetrics    `json:"gsc"`
	GA4    GA4Metrics    `json:"ga4"`
	Ahrefs AhrefsMetrics `json:"ahrefs"`
}

type GSCMetrics struct {
	Clicks          int     `json:"clicks"`
	Impressions     int     `json:"impressions"`
	CTR             float64 `json:"ctr"`
	Position        float64 `json:"position"`
	PreviousClicks  int     `json:"previous_clicks"`
	QuerySample     string  `json:"query_sample,omitempty"`
	ChildCLICommand string  `json:"child_cli_command,omitempty"`
}

type GA4Metrics struct {
	Sessions         int     `json:"sessions"`
	Conversions      int     `json:"conversions"`
	Revenue          float64 `json:"revenue"`
	PreviousSessions int     `json:"previous_sessions"`
	PreviousRevenue  float64 `json:"previous_revenue"`
	ChildCLICommand  string  `json:"child_cli_command,omitempty"`
}

type AhrefsMetrics struct {
	Backlinks       int    `json:"backlinks"`
	RefDomains      int    `json:"ref_domains"`
	TopKeyword      string `json:"top_keyword,omitempty"`
	ChildCLICommand string `json:"child_cli_command,omitempty"`
}

type DataSet struct {
	Profile    string         `json:"profile"`
	SyncedAt   time.Time      `json:"synced_at"`
	Source     string         `json:"source"`
	Provenance DataProvenance `json:"provenance,omitempty"`
	Pages      []PageMetrics  `json:"pages"`
}

type DataProvenance = intelcli.DataProvenance
type DateRange = intelcli.DateRange

type Store struct{ Dir string }

func New(dir string) *Store { return &Store{Dir: dir} }
func DefaultDir() string {
	if v := os.Getenv("TRAFFIC_INTEL_HOME"); v != "" {
		return v
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".traffic-intel-pp-cli")
	}
	return ".traffic-intel-pp-cli"
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
	return DataSet{Profile: profile, SyncedAt: now, Source: "embedded-fixture", Pages: []PageMetrics{
		page("/collections/winter-jackets", "Winter Jackets Collection", 760, 19000, .04, 4.6, 1430, 32, 8960, 610, 1090, 7020, 112, 51, "winter jackets", now),
		page("/products/waterproof-hiking-boot", "Waterproof Hiking Boot", 420, 8200, .0512, 3.8, 980, 44, 18480, 510, 1200, 22600, 87, 34, "waterproof hiking boots", now),
		page("/blog/how-to-choose-running-shoes", "How to Choose Running Shoes", 310, 7700, .0403, 7.2, 690, 9, 1710, 455, 820, 2460, 28, 18, "running shoe guide", now),
		page("/pages/shipping-returns", "Shipping and Returns", 205, 4500, .0455, 5.5, 510, 16, 4320, 240, 540, 4100, 45, 26, "shipping policy", now),
	}}
}

func page(url, title string, clicks, impressions int, ctr, position float64, sessions, conversions int, revenue float64, previousClicks, previousSessions int, previousRevenue float64, backlinks, refDomains int, keyword string, now time.Time) PageMetrics {
	return PageMetrics{
		URL: url, Title: title, Clicks: clicks, Impressions: impressions, CTR: ctr, Position: position,
		Sessions: sessions, Conversions: conversions, Revenue: revenue, PreviousClicks: previousClicks,
		PreviousSessions: previousSessions, PreviousRevenue: previousRevenue, Backlinks: backlinks, RefDomains: refDomains,
		Sources: MetricSources{
			GSC:    GSCMetrics{Clicks: clicks, Impressions: impressions, CTR: ctr, Position: position, PreviousClicks: previousClicks, QuerySample: keyword, ChildCLICommand: "google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions [\\\"page\\\"]"},
			GA4:    GA4Metrics{Sessions: sessions, Conversions: conversions, Revenue: revenue, PreviousSessions: previousSessions, PreviousRevenue: previousRevenue, ChildCLICommand: "google-analytics-pp-cli top-pages --agent"},
			Ahrefs: AhrefsMetrics{Backlinks: backlinks, RefDomains: refDomains, TopKeyword: keyword, ChildCLICommand: "ahrefs-pp-cli site-explorer top-pages --agent"},
		},
		UpdatedAt: now.Format(time.RFC3339),
	}
}
