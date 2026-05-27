package safari

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

type Source struct{}

func New() *Source             { return &Source{} }
func (s *Source) Name() string { return "safari" }
func (s *Source) Capabilities() source.Capabilities {
	return source.Capabilities{Journeys: false, SearchTerms: false, Downloads: false, Transitions: false, PerDeviceOrigin: false}
}
func (s *Source) TestedVersion() int       { return 0 }
func (s *Source) MinSupportedVersion() int { return 0 }

func timeToSafariSeconds(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	u := t.UTC()
	return float64(u.Unix()) + float64(u.Nanosecond())/float64(time.Second) - source.SafariEpochOffsetSeconds
}

func sanitizeFTSQuery(input string) string {
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return `""`
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ReplaceAll(p, `"`, `""`)
		out = append(out, `"`+p+`"`)
	}
	return strings.Join(out, " ")
}

func (s *Source) LocateHistoryDB(_ string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(home, "Library", "Safari", "History.db")
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("safari history db not found at %s", p)
		}
		return "", err
	}
	return p, nil
}

func copySnapshot(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// Safari runs History.db in WAL mode, so prefer VACUUM INTO: opened
	// read-only (the live DB is never modified), it reads through the WAL and
	// writes a consistent single-file snapshot that a raw cp would leave
	// incomplete (cp omits History.db-wal). If the open/vacuum fails — e.g.
	// Safari holds an exclusive lock — fall back to a byte copy so sync still
	// succeeds.
	if err := vacuumIntoReadOnly(src, dst); err == nil {
		return nil
	}
	_ = os.Remove(dst)
	cmd := exec.Command("cp", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp snapshot: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func vacuumIntoReadOnly(src, dst string) error {
	srcDB, err := sql.Open("sqlite", "file:"+src+"?mode=ro")
	if err != nil {
		return err
	}
	defer srcDB.Close()
	_, err = srcDB.Exec("VACUUM INTO '" + strings.ReplaceAll(dst, "'", "''") + "'")
	return err
}

func (s *Source) Snapshot(dstDir string, profile string) (source.SnapshotInfo, error) {
	src, err := s.LocateHistoryDB(profile)
	if err != nil {
		return source.SnapshotInfo{}, err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return source.SnapshotInfo{}, err
	}
	tmp := filepath.Join(dstDir, fmt.Sprintf("snapshot-tmp-%d.db", time.Now().UnixNano()))
	if err := copySnapshot(src, tmp); err != nil {
		return source.SnapshotInfo{}, err
	}
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return source.SnapshotInfo{}, err
	}
	defer db.Close()
	v, lv, _ := s.SchemaVersion(db)
	return source.SnapshotInfo{SnapshotPath: tmp, Version: v, LastCompatibleVersion: lv}, nil
}

func (s *Source) SchemaVersion(db *sql.DB) (version, lastCompatible int, err error) {
	if !tableExists(db, "metadata") {
		return 0, 0, nil
	}
	rows, err := db.Query(`SELECT key, CAST(value AS TEXT) FROM metadata`)
	if err != nil {
		return 0, 0, nil
	}
	defer rows.Close()
	for rows.Next() {
		var k, v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return 0, 0, err
		}
		if !k.Valid || !v.Valid {
			continue
		}
		kl := strings.ToLower(k.String)
		if !strings.Contains(kl, "version") && !strings.Contains(kl, "schema") {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(v.String, "%d", &n); err != nil {
			continue
		}
		if version == 0 {
			version = n
		}
		if strings.Contains(kl, "compatible") {
			lastCompatible = n
		}
	}
	return version, lastCompatible, rows.Err()
}

func (s *Source) PopulateFTS(db *sql.DB) error {
	_, err := db.Exec(`INSERT INTO history_fts(url, title, search_terms)
	SELECT COALESCE(hi.url,''), COALESCE(hv.title,''), ''
	FROM history_items hi
	LEFT JOIN history_visits hv ON hv.id = (
		SELECT h2.id FROM history_visits h2
		WHERE h2.history_item = hi.id
		ORDER BY h2.visit_time DESC, h2.id DESC
		LIMIT 1
	)`)
	return err
}

func (s *Source) SnapshotCounts(db *sql.DB) (pages, visits, terms int64, err error) {
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_items`).Scan(&pages); err != nil {
		return 0, 0, 0, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_visits`).Scan(&visits); err != nil {
		return 0, 0, 0, err
	}
	return pages, visits, 0, nil
}

func (s *Source) RecentVisits(db *sql.DB, f source.VisitFilter) ([]source.VisitRow, error) {
	since := timeToSafariSeconds(f.Since)
	until := timeToSafariSeconds(f.Until)
	if until == 0 {
		until = timeToSafariSeconds(time.Now().UTC())
	}
	originClause, originArgs, err := safariOriginFilterClause("hv", f.Device)
	if err != nil {
		return nil, err
	}
	args := []any{since, until, f.MinVisits}
	args = append(args, originArgs...)
	args = append(args, max(10000, f.Limit*500))
	rows, err := db.Query(`SELECT hv.id, COALESCE(hi.url,''), COALESCE(hv.title,''), COALESCE(hv.visit_time,0), COALESCE(hv.redirect_source,0), COALESCE(hi.visit_count,0), COALESCE(hv.origin,0)
	FROM history_visits hv
	JOIN history_items hi ON hi.id = hv.history_item
	WHERE hv.visit_time > 0 AND hv.visit_time BETWEEN ? AND ? AND COALESCE(hi.visit_count,0) >= ?`+originClause+`
	ORDER BY hv.visit_time DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []source.VisitRow{}
	for rows.Next() {
		var id, from, vc int64
		var u, t string
		var vt float64
		var origin int64
		if err := rows.Scan(&id, &u, &t, &vt, &from, &vc, &origin); err != nil {
			return nil, err
		}
		r := source.VisitRow{VisitID: id, URL: u, Title: t, VisitTime: source.SafariSecondsToTime(vt), FromVisit: from, Transition: "", TypedCount: 0, VisitCount: vc, Origin: safariOriginLabel(origin)}
		if f.Domain != "" && source.DomainFromURL(r.URL) != source.NormalizeTargetDomain(f.Domain) {
			continue
		}
		out = append(out, r)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out, rows.Err()
}

func (s *Source) FullTextSearch(db *sql.DB, query string, f source.VisitFilter) ([]source.HistoryRow, error) {
	match := sanitizeFTSQuery(query)
	since := timeToSafariSeconds(f.Since)
	until := timeToSafariSeconds(f.Until)
	if until == 0 {
		until = timeToSafariSeconds(time.Now().UTC())
	}
	originClause, originArgs, err := safariOriginFilterClause("hv", f.Device)
	if err != nil {
		return nil, err
	}
	args := []any{match, since, until}
	args = append(args, originArgs...)
	args = append(args, max(1, f.Limit))
	// Keep bm25() in a CTE that touches only the FTS table — SQLite rejects the
	// FTS5 ranking functions when the matched table sits inside a JOIN. The
	// history_items / history_visits joins happen in the outer query.
	rows, err := db.Query(`WITH matches AS MATERIALIZED (
		SELECT f.url, f.title, bm25(history_fts) AS rank
		FROM history_fts f
		WHERE history_fts MATCH ?
	)
	SELECT m.url, m.title, COALESCE(hi.visit_count,0), COALESCE(MAX(hv.visit_time),0), m.rank
	FROM matches m
	JOIN history_items hi ON hi.url = m.url
	JOIN history_visits hv ON hv.history_item = hi.id
	WHERE hv.visit_time BETWEEN ? AND ?`+originClause+`
	GROUP BY m.url, m.title, hi.visit_count, m.rank
	ORDER BY m.rank ASC, m.url ASC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []source.HistoryRow{}
	for rows.Next() {
		var r source.HistoryRow
		var lv float64
		if err := rows.Scan(&r.URL, &r.Title, &r.VisitCount, &lv, &r.Rank); err != nil {
			return nil, err
		}
		r.LastVisit = source.SafariSecondsToTime(lv)
		if f.Domain != "" && source.DomainFromURL(r.URL) != source.NormalizeTargetDomain(f.Domain) {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Source) DomainStats(db *sql.DB, f source.VisitFilter) ([]source.DomainStat, error) {
	since := timeToSafariSeconds(f.Since)
	originClause, originArgs, err := safariOriginFilterClause("hv", f.Device)
	if err != nil {
		return nil, err
	}
	args := []any{since}
	args = append(args, originArgs...)
	rows, err := db.Query(`SELECT COALESCE(hi.url,''), COALESCE(hv.visit_time,0)
	FROM history_visits hv
	JOIN history_items hi ON hi.id = hv.history_item
	WHERE hv.visit_time >= ?`+originClause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agg := map[string]source.DomainStat{}
	domainURLs := map[string]map[string]struct{}{}
	for rows.Next() {
		var u string
		var lv float64
		if err := rows.Scan(&u, &lv); err != nil {
			return nil, err
		}
		d := source.DomainFromURL(u)
		x := agg[d]
		x.Domain = d
		x.VisitSum++
		t := source.SafariSecondsToTime(lv)
		if t.After(x.LastVisit) {
			x.LastVisit = t
		}
		agg[d] = x
		if _, ok := domainURLs[d]; !ok {
			domainURLs[d] = map[string]struct{}{}
		}
		domainURLs[d][u] = struct{}{}
	}
	for d, urls := range domainURLs {
		x := agg[d]
		x.PageCount = int64(len(urls))
		agg[d] = x
	}
	arr := make([]source.DomainStat, 0, len(agg))
	for _, v := range agg {
		arr = append(arr, v)
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].VisitSum > arr[j].VisitSum })
	if f.Limit > 0 && len(arr) > f.Limit {
		arr = arr[:f.Limit]
	}
	return arr, nil
}

func (s *Source) SearchTerms(_ *sql.DB, _ source.VisitFilter) ([]source.SearchTermRow, error) {
	return []source.SearchTermRow{}, nil
}

func (s *Source) Downloads(_ *sql.DB, _ source.VisitFilter) ([]source.DownloadRow, error) {
	return []source.DownloadRow{}, nil
}

// escapeLike escapes the SQLite LIKE metacharacters (\, %, _) so a literal
// occurrence in user input matches itself instead of acting as a wildcard.
// Must be paired with an ESCAPE '\' clause on the LIKE.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *Source) VisitedSummary(db *sql.DB, target string) (source.VisitedSummary, error) {
	like := "%" + escapeLike(target) + "%"
	out := source.VisitedSummary{Target: target, TransitionBreakdown: map[string]int64{}}
	var fs, ls sql.NullFloat64
	var tv sql.NullInt64
	if err := db.QueryRow(`SELECT MIN(hv.visit_time), MAX(hv.visit_time), COUNT(*)
	FROM history_visits hv
	JOIN history_items hi ON hi.id=hv.history_item
	WHERE hi.url LIKE ? ESCAPE '\' OR hi.domain_expansion LIKE ? ESCAPE '\'`, like, like).Scan(&fs, &ls, &tv); err != nil {
		return out, err
	}
	if fs.Valid {
		out.FirstSeen = source.SafariSecondsToTime(fs.Float64)
	}
	if ls.Valid {
		out.LastSeen = source.SafariSecondsToTime(ls.Float64)
	}
	if tv.Valid {
		out.TotalVisits = tv.Int64
	}
	if out.TotalVisits == 0 {
		return out, nil
	}
	out.Found = true
	rr, err := db.Query(`SELECT DISTINCT COALESCE(src.url,'')
	FROM history_visits hv
	JOIN history_items hi ON hi.id=hv.history_item
	JOIN history_visits p ON p.id = hv.redirect_source
	JOIN history_items src ON src.id = p.history_item
	WHERE (hi.url LIKE ? ESCAPE '\' OR hi.domain_expansion LIKE ? ESCAPE '\') AND hv.redirect_source > 0
	LIMIT 5`, like, like)
	if err == nil {
		defer rr.Close()
		for rr.Next() {
			var rs sql.NullString
			if rr.Scan(&rs) == nil && rs.Valid {
				out.Referrers = append(out.Referrers, rs.String)
			}
		}
	}
	return out, nil
}

func (s *Source) Clusters(_ *sql.DB, _ source.ClusterFilter) ([]source.Cluster, string, error) {
	return []source.Cluster{}, "journeys not available for safari", nil
}

func (s *Source) ProfileAggregates(db *sql.DB, f source.VisitFilter) (source.ProfileData, error) {
	pd := source.ProfileData{}
	events, err := s.RecentVisits(db, source.VisitFilter{Since: f.Since, Until: f.Until, Limit: 50000, Device: f.Device})
	if err != nil {
		return pd, err
	}
	hc := map[int]int64{}
	wd := map[time.Weekday]int64{}
	dc := map[string]int64{}
	for _, e := range events {
		lt := e.VisitTime.In(time.Local)
		hc[lt.Hour()]++
		wd[lt.Weekday()]++
		dc[lt.Format("2006-01-02")]++
	}
	type hkv struct {
		h int
		c int64
	}
	hours := make([]hkv, 0, len(hc))
	for h, c := range hc {
		hours = append(hours, hkv{h: h, c: c})
	}
	sort.Slice(hours, func(i, j int) bool {
		if hours[i].c == hours[j].c {
			return hours[i].h < hours[j].h
		}
		return hours[i].c > hours[j].c
	})
	for i, kv := range hours {
		if i >= 5 {
			break
		}
		pd.Hourly = append(pd.Hourly, map[string]any{"hour": kv.h, "count": kv.c})
	}
	type wkv struct {
		w time.Weekday
		c int64
	}
	week := make([]wkv, 0, len(wd))
	for w, c := range wd {
		week = append(week, wkv{w: w, c: c})
	}
	sort.Slice(week, func(i, j int) bool {
		if week[i].c == week[j].c {
			return week[i].w < week[j].w
		}
		return week[i].c > week[j].c
	})
	for _, kv := range week {
		pd.Weekday = append(pd.Weekday, map[string]any{"weekday": kv.w.String(), "count": kv.c})
	}
	for d, c := range dc {
		pd.Daily = append(pd.Daily, map[string]any{"day": d, "count": c})
	}
	sort.Slice(pd.Daily, func(i, j int) bool { return pd.Daily[i]["day"].(string) < pd.Daily[j]["day"].(string) })
	top, _ := s.DomainStats(db, source.VisitFilter{Since: f.Since, Limit: 5, Device: f.Device})
	for _, d := range top {
		pd.TopDomains = append(pd.TopDomains, map[string]any{"domain": d.Domain, "visits": d.VisitSum})
	}
	pd.TopSearchTerms = []map[string]any{}
	pd.Pages, pd.Visits, _, _ = s.SnapshotCounts(db)
	return pd, nil
}

func tableExists(db *sql.DB, table string) bool {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&name)
	return err == nil && name == table
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func safariOriginLabel(origin int64) string {
	if origin == 1 {
		return "synced"
	}
	return "this"
}

func safariOriginFilterClause(alias, device string) (string, []any, error) {
	d := strings.TrimSpace(strings.ToLower(device))
	switch d {
	case "", "all":
		return "", nil, nil
	case "this":
		return " AND COALESCE(" + alias + `.origin,0) = ?`, []any{0}, nil
	case "synced":
		return " AND COALESCE(" + alias + `.origin,0) = ?`, []any{1}, nil
	default:
		return "", nil, fmt.Errorf("device filtering by specific device not available for safari: Safari only distinguishes this-device vs synced")
	}
}

func (s *Source) Devices(db *sql.DB) ([]source.DeviceInfo, error) {
	rows, err := db.Query(`SELECT COALESCE(hv.origin,0), COUNT(*), COALESCE(MIN(hv.visit_time),0), COALESCE(MAX(hv.visit_time),0)
	FROM history_visits hv
	WHERE hv.visit_time > 0
	GROUP BY COALESCE(hv.origin,0)
	ORDER BY COALESCE(hv.origin,0)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	visitsByOrigin := map[int64]int64{0: 0, 1: 0}
	firstByOrigin := map[int64]time.Time{}
	lastByOrigin := map[int64]time.Time{}
	for rows.Next() {
		var origin int64
		var visits int64
		var firstRaw, lastRaw float64
		if err := rows.Scan(&origin, &visits, &firstRaw, &lastRaw); err != nil {
			return nil, err
		}
		if origin != 1 {
			origin = 0
		}
		visitsByOrigin[origin] += visits
		first := source.SafariSecondsToTime(firstRaw)
		last := source.SafariSecondsToTime(lastRaw)
		if prev, ok := firstByOrigin[origin]; !ok || (first.Before(prev) && !first.IsZero()) {
			firstByOrigin[origin] = first
		}
		if last.After(lastByOrigin[origin]) {
			lastByOrigin[origin] = last
		}
	}
	topDomainsByOrigin, err := s.deviceTopDomains(db)
	if err != nil {
		return nil, err
	}
	return []source.DeviceInfo{
		{ID: "this", Kind: "this", Visits: visitsByOrigin[0], FirstSeen: firstByOrigin[0], LastSeen: lastByOrigin[0], TopDomains: topDomainsByOrigin[0]},
		{ID: "synced", Kind: "synced", Visits: visitsByOrigin[1], FirstSeen: firstByOrigin[1], LastSeen: lastByOrigin[1], TopDomains: topDomainsByOrigin[1]},
	}, nil
}

func (s *Source) deviceTopDomains(db *sql.DB) (map[int64][]string, error) {
	rows, err := db.Query(`SELECT COALESCE(hv.origin,0), COALESCE(hi.url,'')
	FROM history_visits hv
	JOIN history_items hi ON hi.id = hv.history_item
	WHERE hv.visit_time > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[int64]map[string]int64{0: {}, 1: {}}
	for rows.Next() {
		var origin int64
		var rawURL string
		if err := rows.Scan(&origin, &rawURL); err != nil {
			return nil, err
		}
		if origin != 1 {
			origin = 0
		}
		d := source.DomainFromURL(rawURL)
		if strings.TrimSpace(d) == "" {
			continue
		}
		counts[origin][d]++
	}
	out := map[int64][]string{0: {}, 1: {}}
	for _, origin := range []int64{0, 1} {
		type kv struct {
			d string
			c int64
		}
		list := make([]kv, 0, len(counts[origin]))
		for d, c := range counts[origin] {
			list = append(list, kv{d: d, c: c})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].c == list[j].c {
				return list[i].d < list[j].d
			}
			return list[i].c > list[j].c
		})
		n := 5
		if len(list) < n {
			n = len(list)
		}
		top := make([]string, 0, n)
		for i := 0; i < n; i++ {
			top = append(top, list[i].d)
		}
		out[origin] = top
	}
	return out, nil
}
