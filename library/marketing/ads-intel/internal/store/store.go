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

type DataProvenance = intelcli.DataProvenance
type DateRange = intelcli.DateRange

type Profile struct {
	Name      string    `json:"name"`
	AccountID string    `json:"account_id,omitempty"`
	Brand     string    `json:"brand,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProfilesFile struct {
	Profiles map[string]Profile `json:"profiles"`
}

type DataSet struct {
	Profile       string         `json:"profile"`
	SyncedAt      time.Time      `json:"synced_at"`
	Source        string         `json:"source"`
	Provenance    DataProvenance `json:"provenance,omitempty"`
	Account       AccountStatus  `json:"account"`
	Campaigns     []Campaign     `json:"campaigns"`
	SearchTerms   []SearchTerm   `json:"search_terms"`
	Keywords      []Keyword      `json:"keywords"`
	NegativeLists []NegativeList `json:"negative_lists"`
}

type AccountStatus struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status"`
	Currency  string `json:"currency,omitempty"`
}

type Campaign struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Platform      string  `json:"platform"`
	Status        string  `json:"status"`
	Spend         float64 `json:"spend"`
	Conversions   float64 `json:"conversions"`
	Revenue       float64 `json:"revenue"`
	Clicks        int     `json:"clicks"`
	Impressions   int     `json:"impressions"`
	Frequency     float64 `json:"frequency,omitempty"`
	CTR           float64 `json:"ctr,omitempty"`
	ROAS          float64 `json:"roas,omitempty"`
	LearningPhase bool    `json:"learning_phase"`
}

type SearchTerm struct {
	Platform     string  `json:"platform"`
	CampaignID   string  `json:"campaign_id"`
	CampaignName string  `json:"campaign_name,omitempty"`
	AdGroupID    string  `json:"ad_group_id,omitempty"`
	AdGroupName  string  `json:"ad_group_name,omitempty"`
	Term         string  `json:"term"`
	Spend        float64 `json:"spend"`
	Conversions  float64 `json:"conversions"`
	Clicks       int     `json:"clicks"`
}

type Keyword struct {
	Platform     string  `json:"platform"`
	CampaignID   string  `json:"campaign_id"`
	Text         string  `json:"text"`
	MatchType    string  `json:"match_type"`
	Bidding      string  `json:"bidding"`
	Clicks       int     `json:"clicks"`
	Conversions  float64 `json:"conversions"`
	NegativeList string  `json:"negative_list,omitempty"`
}

type NegativeList struct {
	Platform  string   `json:"platform"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Shared    bool     `json:"shared"`
	Campaigns []string `json:"campaigns"`
}

type Store struct{ Dir string }

func New(dir string) *Store { return &Store{Dir: dir} }
func DefaultDir() string {
	if v := os.Getenv("ADS_INTEL_HOME"); v != "" {
		return v
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".ads-intel-pp-cli")
	}
	return ".ads-intel-pp-cli"
}
func (s *Store) ensure() error        { return os.MkdirAll(s.Dir, 0o755) }
func (s *Store) profilesPath() string { return filepath.Join(s.Dir, "profiles.json") }
func (s *Store) dataPath(profile string) string {
	return filepath.Join(s.Dir, intelcli.SafeName(profile)+"-data.json")
}

func (s *Store) LoadProfiles() (ProfilesFile, error) {
	pf := ProfilesFile{Profiles: map[string]Profile{}}
	b, err := os.ReadFile(s.profilesPath())
	if errors.Is(err, os.ErrNotExist) {
		return pf, nil
	}
	if err != nil || len(b) == 0 {
		return pf, err
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
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	pf.Profiles[p.Name] = p
	return writeJSON(s.profilesPath(), pf)
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
	return s.SaveSnapshot(d)
}

func (s *Store) LoadData(profile string) (DataSet, error) {
	var d DataSet
	b, err := os.ReadFile(s.dataPath(profile))
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(b, &d)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
