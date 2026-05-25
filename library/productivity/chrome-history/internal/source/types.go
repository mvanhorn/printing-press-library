package source

import (
	"database/sql"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

type Capabilities struct {
	Journeys, SearchTerms, Downloads, Transitions, PerDeviceOrigin bool
}

type SnapshotInfo struct {
	SnapshotPath          string
	Version               int
	LastCompatibleVersion int
}

type VisitFilter struct {
	Since     time.Time
	Until     time.Time
	Limit     int
	Domain    string
	MinVisits int64
	Device    string
}

type ClusterFilter struct {
	Since time.Time
	Limit int
}

type VisitRow struct {
	VisitID       int64
	URL           string
	Title         string
	VisitTime     time.Time
	FromVisit     int64
	Transition    string
	VisitDuration time.Duration
	TypedCount    int64
	VisitCount    int64
	Origin        string
}

type DeviceInfo struct {
	ID         string
	Kind       string // this|synced|extension|imported
	Visits     int64
	FirstSeen  time.Time
	LastSeen   time.Time
	TopDomains []string
}

type HistoryRow struct {
	URL        string
	Title      string
	VisitCount int64
	LastVisit  time.Time
	Rank       float64
}

type DomainStat struct {
	Domain    string
	PageCount int64
	VisitSum  int64
	LastVisit time.Time
}

type SearchTermRow struct {
	Term   string
	When   time.Time
	URL    string
	Title  string
	Visits int64
}

type DownloadRow struct {
	TargetPath string
	Bytes      int64
	MIME       string
	Source     string
	When       time.Time
	State      int64
}

type VisitedSummary struct {
	Target              string
	Found               bool
	FirstSeen           time.Time
	LastSeen            time.Time
	TotalVisits         int64
	TypedCount          int64
	TransitionBreakdown map[string]int64
	Referrers           []string
}

type Cluster struct {
	ClusterID int64
	Label     string
	PageCount int64
	LastVisit time.Time
	TopPages  []TopPage
}

type TopPage struct {
	URL   string
	Count int64
}

type ProfileData struct {
	Hourly         []map[string]any
	Daily          []map[string]any
	Weekday        []map[string]any
	TopDomains     []map[string]any
	TopSearchTerms []map[string]any
	Pages          int64
	Visits         int64
}

type Source interface {
	Name() string
	Capabilities() Capabilities
	LocateHistoryDB(profile string) (string, error)
	Snapshot(dstDir string, profile string) (SnapshotInfo, error)
	SchemaVersion(db *sql.DB) (version, lastCompatible int, err error)
	TestedVersion() int
	MinSupportedVersion() int
	RecentVisits(db *sql.DB, f VisitFilter) ([]VisitRow, error)
	FullTextSearch(db *sql.DB, query string, f VisitFilter) ([]HistoryRow, error)
	DomainStats(db *sql.DB, f VisitFilter) ([]DomainStat, error)
	SearchTerms(db *sql.DB, f VisitFilter) ([]SearchTermRow, error)
	Downloads(db *sql.DB, f VisitFilter) ([]DownloadRow, error)
	VisitedSummary(db *sql.DB, target string) (VisitedSummary, error)
	Clusters(db *sql.DB, f ClusterFilter) ([]Cluster, string, error)
	ProfileAggregates(db *sql.DB, f VisitFilter) (ProfileData, error)
	Devices(db *sql.DB) ([]DeviceInfo, error)
}

func DomainFromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return normalizeDomain(raw)
	}
	return normalizeDomain(u.Hostname())
}

func NormalizeTargetDomain(raw string) string {
	r := strings.TrimSpace(raw)
	if strings.Contains(r, "://") {
		return DomainFromURL(r)
	}
	return normalizeDomain(r)
}

func normalizeDomain(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimPrefix(h, "www.")
	// publicsuffix handles country-code second-level TLDs (co.uk, com.au,
	// co.jp, ...) that a naive "last two labels" split would collapse into the
	// same bucket. Fall back to the host on error (single labels, localhost).
	if reg, err := publicsuffix.EffectiveTLDPlusOne(h); err == nil {
		return reg
	}
	return h
}
