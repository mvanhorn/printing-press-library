package chrome

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
	_ "modernc.org/sqlite"
)

const (
	chromeEpochOffsetSeconds  int64 = 11644473600
	TestedSchemaVersion       int   = 70
	MinSupportedSchemaVersion int   = 66
)

var transitionCoreTypes = map[uint8]string{0: "link", 1: "typed", 2: "auto_bookmark", 3: "auto_subframe", 4: "manual_subframe", 5: "generated", 6: "start_page", 7: "form_submit", 8: "reload", 9: "keyword", 10: "keyword_generated"}

type Source struct{}

func New() *Source             { return &Source{} }
func (s *Source) Name() string { return "chrome" }
func (s *Source) Capabilities() source.Capabilities {
	return source.Capabilities{Journeys: true, SearchTerms: true, Downloads: true, Transitions: true, PerDeviceOrigin: true}
}
func (s *Source) TestedVersion() int       { return TestedSchemaVersion }
func (s *Source) MinSupportedVersion() int { return MinSupportedSchemaVersion }

func chromeMicrosToTime(chromeMicros int64) time.Time {
	if chromeMicros <= 0 {
		return time.Time{}
	}
	unixSeconds := chromeMicros/1_000_000 - chromeEpochOffsetSeconds
	remMicros := chromeMicros % 1_000_000
	return time.Unix(unixSeconds, remMicros*1_000).UTC()
}

func timeToChromeMicros(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	u := t.UTC()
	return (u.Unix()+chromeEpochOffsetSeconds)*1_000_000 + int64(u.Nanosecond()/1_000)
}

func transitionLabel(raw int64) string {
	if v, ok := transitionCoreTypes[uint8(raw&0xff)]; ok {
		return v
	}
	return "unknown"
}

func cleanClusterLabel(label string) string {
	l := strings.TrimSpace(label)
	if l == "" {
		return l
	}
	for {
		if l == "" {
			return l
		}
		r, sz := utf8.DecodeRuneInString(l)
		last, lsz := utf8.DecodeLastRuneInString(l)
		trimmed := false
		if strings.ContainsRune(`"'“‘`, r) {
			l = strings.TrimSpace(l[sz:])
			trimmed = true
		}
		if l != "" && strings.ContainsRune(`"'”’`, last) {
			l = strings.TrimSpace(l[:len(l)-lsz])
			trimmed = true
		}
		if !trimmed {
			break
		}
	}
	return l
}

func normalizedClusterKey(label string) string {
	return strings.ToLower(cleanClusterLabel(label))
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

func (s *Source) LocateHistoryDB(profile string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	pn := strings.TrimSpace(profile)
	if pn == "" {
		pn = "Default"
	}
	p := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", pn, "History")
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("chrome history db not found at %s", p)
		}
		return "", err
	}
	return p, nil
}

func copySnapshot(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// Byte copy, not VACUUM INTO: Chrome holds an exclusive lock on History
	// while running, so opening it (which VACUUM INTO requires) fails with
	// SQLITE_BUSY. cp copies the file without opening the DB. Chrome's History
	// is not in WAL mode (no -wal sidecar), so the single file is self-contained.
	cmd := exec.Command("cp", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp snapshot: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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
	rows, err := db.Query(`SELECT key, value FROM meta WHERE key IN ('version','last_compatible_version')`)
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
		var n int
		if _, err := fmt.Sscanf(v.String, "%d", &n); err != nil {
			continue
		}
		if k.String == "version" {
			version = n
		}
		if k.String == "last_compatible_version" {
			lastCompatible = n
		}
	}
	return version, lastCompatible, rows.Err()
}

func (s *Source) RecentVisits(db *sql.DB, f source.VisitFilter) ([]source.VisitRow, error) {
	since := timeToChromeMicros(f.Since)
	until := timeToChromeMicros(f.Until)
	if until == 0 {
		until = timeToChromeMicros(time.Now().UTC())
	}
	// Keep a bounded over-fetch so domain/transition filtering can happen in Go without starvation.
	guidToDevice, err := s.deviceIDMap(db)
	if err != nil {
		return nil, err
	}
	guidExpr, err := s.originatorGUIDExpr(db, "v", "vs")
	if err != nil {
		return nil, err
	}
	deviceWhere, deviceArgs, err := s.deviceFilterClause(db, f.Device, guidToDevice, "vs", guidExpr)
	if err != nil {
		return nil, err
	}
	q := `SELECT v.id, COALESCE(u.url,''), COALESCE(u.title,''), COALESCE(v.visit_time,0), COALESCE(v.from_visit,0), COALESCE((v.transition & 255),0), COALESCE(v.visit_duration,0), COALESCE(u.typed_count,0), COALESCE(u.visit_count,0), COALESCE(vs.source,-1), ` + guidExpr + `
	FROM visits v JOIN urls u ON u.id=v.url LEFT JOIN visit_source vs ON vs.id=v.id
	WHERE v.visit_time > 0 AND v.visit_time BETWEEN ? AND ? AND COALESCE(u.visit_count,0) >= ?`
	args := []any{since, until, f.MinVisits}
	if deviceWhere != "" {
		q += " AND " + deviceWhere
		args = append(args, deviceArgs...)
	}
	q += " ORDER BY v.visit_time DESC LIMIT ?"
	args = append(args, max(10000, f.Limit*500))
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []source.VisitRow{}
	for rows.Next() {
		var id, vt, fv, tr, vd, tc, vc int64
		var srcVal int64
		var guid string
		var u, t string
		if err := rows.Scan(&id, &u, &t, &vt, &fv, &tr, &vd, &tc, &vc, &srcVal, &guid); err != nil {
			return nil, err
		}
		r := source.VisitRow{VisitID: id, URL: u, Title: t, VisitTime: chromeMicrosToTime(vt), FromVisit: fv, Transition: transitionLabel(tr), VisitDuration: time.Duration(vd) * time.Microsecond, TypedCount: tc, VisitCount: vc, Origin: visitOrigin(srcVal, guid, guidToDevice)}
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
	since := timeToChromeMicros(f.Since)
	until := timeToChromeMicros(f.Until)
	if until == 0 {
		until = timeToChromeMicros(time.Now().UTC())
	}
	guidToDevice, err := s.deviceIDMap(db)
	if err != nil {
		return nil, err
	}
	guidExpr, err := s.originatorGUIDExpr(db, "v", "vs")
	if err != nil {
		return nil, err
	}
	deviceWhere, deviceArgs, err := s.deviceFilterClause(db, f.Device, guidToDevice, "vs", guidExpr)
	if err != nil {
		return nil, err
	}
	sqlQ := `SELECT f.url, f.title, COALESCE(u.visit_count,0), COALESCE(u.last_visit_time,0), bm25(history_fts)
	FROM history_fts f LEFT JOIN urls u ON u.url=f.url
	WHERE history_fts MATCH ? AND EXISTS (
		SELECT 1 FROM urls ux JOIN visits v ON v.url=ux.id LEFT JOIN visit_source vs ON vs.id=v.id
		WHERE ux.url=f.url AND v.visit_time BETWEEN ? AND ?`
	args := []any{match, since, until}
	if deviceWhere != "" {
		sqlQ += " AND " + deviceWhere
		args = append(args, deviceArgs...)
	}
	sqlQ += `)
	ORDER BY bm25(history_fts) ASC LIMIT ?`
	args = append(args, max(1, f.Limit))
	rows, err := db.Query(sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []source.HistoryRow{}
	for rows.Next() {
		var r source.HistoryRow
		var lv int64
		if err := rows.Scan(&r.URL, &r.Title, &r.VisitCount, &lv, &r.Rank); err != nil {
			return nil, err
		}
		r.LastVisit = chromeMicrosToTime(lv)
		if f.Domain != "" && source.DomainFromURL(r.URL) != source.NormalizeTargetDomain(f.Domain) {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Source) DomainStats(db *sql.DB, f source.VisitFilter) ([]source.DomainStat, error) {
	since := timeToChromeMicros(f.Since)
	until := timeToChromeMicros(f.Until)
	if until == 0 {
		until = timeToChromeMicros(time.Now().UTC())
	}
	guidToDevice, err := s.deviceIDMap(db)
	if err != nil {
		return nil, err
	}
	guidExpr, err := s.originatorGUIDExpr(db, "v", "vs")
	if err != nil {
		return nil, err
	}
	deviceWhere, deviceArgs, err := s.deviceFilterClause(db, f.Device, guidToDevice, "vs", guidExpr)
	if err != nil {
		return nil, err
	}
	q := `SELECT COALESCE(u.url,''), COUNT(*), COALESCE(MAX(v.visit_time),0)
	FROM visits v JOIN urls u ON u.id=v.url LEFT JOIN visit_source vs ON vs.id=v.id
	WHERE u.hidden=0 AND v.visit_time > 0 AND v.visit_time BETWEEN ? AND ?`
	args := []any{since, until}
	if deviceWhere != "" {
		q += " AND " + deviceWhere
		args = append(args, deviceArgs...)
	}
	// Group by URL with no row cap: the per-domain totals are summed in Go
	// below, so a SQL LIMIT here would truncate URLs before aggregation and
	// undercount breadth-heavy domains (many distinct URLs, few visits each).
	// f.Limit is applied to the domain-level result after aggregation.
	q += ` GROUP BY u.url`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agg := map[string]source.DomainStat{}
	for rows.Next() {
		var u string
		var pc, lv int64
		if err := rows.Scan(&u, &pc, &lv); err != nil {
			return nil, err
		}
		d := source.DomainFromURL(u)
		x := agg[d]
		x.Domain = d
		x.PageCount += pc
		x.VisitSum += pc
		if chromeMicrosToTime(lv).After(x.LastVisit) {
			x.LastVisit = chromeMicrosToTime(lv)
		}
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

func (s *Source) SearchTerms(db *sql.DB, f source.VisitFilter) ([]source.SearchTermRow, error) {
	since := timeToChromeMicros(f.Since)
	until := timeToChromeMicros(f.Until)
	if until == 0 {
		until = timeToChromeMicros(time.Now().UTC())
	}
	guidToDevice, err := s.deviceIDMap(db)
	if err != nil {
		return nil, err
	}
	guidExpr, err := s.originatorGUIDExpr(db, "v", "vs")
	if err != nil {
		return nil, err
	}
	deviceWhere, deviceArgs, err := s.deviceFilterClause(db, f.Device, guidToDevice, "vs", guidExpr)
	if err != nil {
		return nil, err
	}
	q := `SELECT k.term, COALESCE(u.last_visit_time,0), COALESCE(u.url,''), COALESCE(u.title,''), COALESCE(u.visit_count,0)
	FROM keyword_search_terms k JOIN urls u ON u.id=k.url_id
	WHERE EXISTS (
		SELECT 1 FROM visits v LEFT JOIN visit_source vs ON vs.id=v.id
		WHERE v.url=u.id AND v.visit_time BETWEEN ? AND ?`
	args := []any{since, until}
	if deviceWhere != "" {
		q += " AND " + deviceWhere
		args = append(args, deviceArgs...)
	}
	q += `) ORDER BY u.last_visit_time DESC LIMIT ?`
	args = append(args, max(1, f.Limit))
	rows, err := db.Query(q, args...)
	if err != nil {
		return []source.SearchTermRow{}, nil
	}
	defer rows.Close()
	out := []source.SearchTermRow{}
	for rows.Next() {
		var r source.SearchTermRow
		var w int64
		if err := rows.Scan(&r.Term, &w, &r.URL, &r.Title, &r.Visits); err != nil {
			return nil, err
		}
		r.When = chromeMicrosToTime(w)
		if f.Domain != "" && source.DomainFromURL(r.URL) != source.NormalizeTargetDomain(f.Domain) {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Source) Downloads(db *sql.DB, f source.VisitFilter) ([]source.DownloadRow, error) {
	since := timeToChromeMicros(f.Since)
	rows, err := db.Query(`SELECT COALESCE(target_path,''), COALESCE(received_bytes,0), COALESCE(mime_type, original_mime_type, ''), COALESCE(NULLIF(site_url,''), NULLIF(referrer,''), ''), COALESCE(start_time,0), COALESCE(state,0)
	FROM downloads WHERE start_time >= ? ORDER BY start_time DESC LIMIT ?`, since, max(1, f.Limit))
	if err != nil {
		return []source.DownloadRow{}, nil
	}
	defer rows.Close()
	out := []source.DownloadRow{}
	for rows.Next() {
		var r source.DownloadRow
		var w int64
		if err := rows.Scan(&r.TargetPath, &r.Bytes, &r.MIME, &r.Source, &w, &r.State); err != nil {
			return nil, err
		}
		r.When = chromeMicrosToTime(w)
		out = append(out, r)
	}
	return out, rows.Err()
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
	var fs, ls, tv, tc sql.NullInt64
	if err := db.QueryRow(`SELECT MIN(v.visit_time), MAX(v.visit_time), COUNT(*), SUM(u.typed_count) FROM visits v JOIN urls u ON u.id=v.url WHERE u.url LIKE ? ESCAPE '\'`, like).Scan(&fs, &ls, &tv, &tc); err != nil {
		return out, err
	}
	if fs.Valid {
		out.FirstSeen = chromeMicrosToTime(fs.Int64)
	}
	if ls.Valid {
		out.LastSeen = chromeMicrosToTime(ls.Int64)
	}
	if tv.Valid {
		out.TotalVisits = tv.Int64
	}
	if tc.Valid {
		out.TypedCount = tc.Int64
	}
	if out.TotalVisits == 0 {
		return out, nil
	}
	out.Found = true
	tr, err := db.Query(`SELECT (v.transition & 255), COUNT(*) FROM visits v JOIN urls u ON u.id=v.url WHERE u.url LIKE ? ESCAPE '\' GROUP BY 1`, like)
	if err == nil {
		defer tr.Close()
		for tr.Next() {
			var t, c int64
			if tr.Scan(&t, &c) == nil {
				out.TransitionBreakdown[transitionLabel(t)] = c
			}
		}
	}
	rr, err := db.Query(`SELECT DISTINCT u2.url FROM visits v JOIN urls u ON u.id=v.url JOIN visits pv ON pv.id=v.from_visit JOIN urls u2 ON u2.id=pv.url WHERE u.url LIKE ? ESCAPE '\' AND v.from_visit > 0 LIMIT 5`, like)
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

func (s *Source) Clusters(db *sql.DB, f source.ClusterFilter) ([]source.Cluster, string, error) {
	// One-shot probe via QueryRow closes its own connection; db.Query would leak
	// an open *Rows (and its SQLite connection) on this discarded result.
	var probe int
	if err := db.QueryRow(`SELECT 1 FROM clusters LIMIT 1`).Scan(&probe); err != nil {
		return []source.Cluster{}, "clusters tables missing", nil
	}
	since := timeToChromeMicros(f.Since)
	q := `SELECT c.cluster_id, c.label, COALESCE(u.url,''), COALESCE(v.visit_time,0)
	FROM clusters c JOIN clusters_and_visits cav ON cav.cluster_id=c.cluster_id JOIN visits v ON v.id=cav.visit_id JOIN urls u ON u.id=v.url
	WHERE TRIM(COALESCE(c.label,'')) <> ''`
	args := []any{}
	if since > 0 {
		q += ` AND v.visit_time >= ?`
		args = append(args, since)
	}
	q += ` ORDER BY c.cluster_id, v.visit_time DESC LIMIT ?`
	args = append(args, max(1000, f.Limit*500))
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	type agg struct {
		label string
		pages map[string]int64
		last  time.Time
		id    int64
	}
	m := map[string]*agg{}
	for rows.Next() {
		var id, wt int64
		var lbl, u string
		if err := rows.Scan(&id, &lbl, &u, &wt); err != nil {
			return nil, "", err
		}
		clean := cleanClusterLabel(lbl)
		key := normalizedClusterKey(clean)
		a := m[key]
		if a == nil {
			a = &agg{label: clean, pages: map[string]int64{}, id: id}
			m[key] = a
		}
		a.pages[u]++
		t := chromeMicrosToTime(wt)
		if t.After(a.last) {
			a.last = t
		}
		if id > a.id {
			a.id = id
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ai, aj := m[keys[i]], m[keys[j]]
		if ai.last.Equal(aj.last) {
			return len(ai.pages) > len(aj.pages)
		}
		return ai.last.After(aj.last)
	})
	if f.Limit > 0 && len(keys) > f.Limit {
		keys = keys[:f.Limit]
	}
	out := make([]source.Cluster, 0, len(keys))
	for _, k := range keys {
		a := m[k]
		c := source.Cluster{ClusterID: a.id, Label: a.label, PageCount: int64(len(a.pages)), LastVisit: a.last}
		type kv struct {
			u string
			c int64
		}
		arr := []kv{}
		for u, cnt := range a.pages {
			arr = append(arr, kv{u, cnt})
		}
		sort.Slice(arr, func(i, j int) bool { return arr[i].c > arr[j].c })
		for i := 0; i < len(arr) && i < 3; i++ {
			c.TopPages = append(c.TopPages, source.TopPage{URL: arr[i].u, Count: arr[i].c})
		}
		out = append(out, c)
	}
	return out, "", nil
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
	uniquePages := map[string]struct{}{}
	for _, e := range events {
		uniquePages[e.URL] = struct{}{}
	}
	pd.Pages = int64(len(uniquePages))
	pd.Visits = int64(len(events))
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
	top, _ := s.DomainStats(db, source.VisitFilter{Since: f.Since, Until: f.Until, Limit: 5, Device: f.Device})
	for _, d := range top {
		pd.TopDomains = append(pd.TopDomains, map[string]any{"domain": d.Domain, "visits": d.VisitSum})
	}
	termRows, _ := s.SearchTerms(db, source.VisitFilter{Since: f.Since, Until: f.Until, Limit: 50000, Device: f.Device})
	termCounts := map[string]int64{}
	for _, tr := range termRows {
		termCounts[tr.Term]++
	}
	type termKV struct {
		term  string
		count int64
	}
	sortedTerms := make([]termKV, 0, len(termCounts))
	for t, c := range termCounts {
		sortedTerms = append(sortedTerms, termKV{term: t, count: c})
	}
	sort.Slice(sortedTerms, func(i, j int) bool { return sortedTerms[i].count > sortedTerms[j].count })
	for i := 0; i < len(sortedTerms) && i < 5; i++ {
		pd.TopSearchTerms = append(pd.TopSearchTerms, map[string]any{"term": sortedTerms[i].term, "count": sortedTerms[i].count})
	}
	return pd, nil
}

func (s *Source) Devices(db *sql.DB) ([]source.DeviceInfo, error) {
	guidToDevice, err := s.deviceIDMap(db)
	if err != nil {
		return nil, err
	}
	guidExpr, err := s.originatorGUIDExpr(db, "v", "vs")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT COALESCE(vs.source,-1), ` + guidExpr + `, COALESCE(v.visit_time,0), COALESCE(u.url,'')
		FROM visits v JOIN urls u ON u.id=v.url LEFT JOIN visit_source vs ON vs.id=v.id
		WHERE v.visit_time > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type agg struct {
		info   source.DeviceInfo
		domain map[string]int64
	}
	byID := map[string]*agg{}
	get := func(id, kind string) *agg {
		a := byID[id]
		if a == nil {
			a = &agg{info: source.DeviceInfo{ID: id, Kind: kind}, domain: map[string]int64{}}
			byID[id] = a
		}
		return a
	}
	for rows.Next() {
		var srcVal, vt int64
		var guid, rawURL string
		if err := rows.Scan(&srcVal, &guid, &vt, &rawURL); err != nil {
			return nil, err
		}
		id, kind := deviceBucket(srcVal, guid, guidToDevice)
		a := get(id, kind)
		a.info.Visits++
		ts := chromeMicrosToTime(vt)
		if a.info.FirstSeen.IsZero() || ts.Before(a.info.FirstSeen) {
			a.info.FirstSeen = ts
		}
		if ts.After(a.info.LastSeen) {
			a.info.LastSeen = ts
		}
		a.domain[source.DomainFromURL(rawURL)]++
	}
	out := make([]source.DeviceInfo, 0, len(byID))
	for _, a := range byID {
		type dv struct {
			d string
			c int64
		}
		dom := make([]dv, 0, len(a.domain))
		for d, c := range a.domain {
			dom = append(dom, dv{d: d, c: c})
		}
		sort.Slice(dom, func(i, j int) bool { return dom[i].c > dom[j].c })
		for i := 0; i < len(dom) && i < 3; i++ {
			a.info.TopDomains = append(a.info.TopDomains, dom[i].d)
		}
		out = append(out, a.info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID == "this" {
			return true
		}
		if out[j].ID == "this" {
			return false
		}
		if out[i].Kind == out[j].Kind {
			return out[i].Visits > out[j].Visits
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

func (s *Source) deviceIDMap(db *sql.DB) (map[string]string, error) {
	guidExpr, err := s.originatorGUIDExpr(db, "v", "vs")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT ` + guidExpr + ` AS guid, COUNT(*) c
		FROM visits v LEFT JOIN visit_source vs ON vs.id=v.id
		WHERE COALESCE(vs.source,-1)=0 AND TRIM(` + guidExpr + `) <> ''
		GROUP BY guid ORDER BY c DESC, guid ASC`)
	if err != nil {
		return map[string]string{}, nil
	}
	defer rows.Close()
	out := map[string]string{}
	i := 1
	for rows.Next() {
		var guid string
		var c int64
		if err := rows.Scan(&guid, &c); err != nil {
			return nil, err
		}
		out[guid] = fmt.Sprintf("device-%d", i)
		i++
	}
	return out, rows.Err()
}

func visitOrigin(srcVal int64, guid string, guidToDevice map[string]string) string {
	c := classifyOrigin(srcVal, guid, guidToDevice)
	return c.id
}

func deviceBucket(srcVal int64, guid string, guidToDevice map[string]string) (id, kind string) {
	c := classifyOrigin(srcVal, guid, guidToDevice)
	return c.id, c.kind
}

type originClass struct {
	id   string
	kind string
}

func classifyOrigin(srcVal int64, guid string, guidToDevice map[string]string) originClass {
	switch srcVal {
	case -1, 1:
		return originClass{id: "this", kind: "this"}
	case 0:
		if id, ok := guidToDevice[guid]; ok && strings.TrimSpace(guid) != "" {
			return originClass{id: id, kind: "synced"}
		}
		return originClass{id: "synced", kind: "synced"}
	case 2:
		return originClass{id: "extension", kind: "extension"}
	case 3, 4, 5:
		return originClass{id: "imported", kind: "imported"}
	default:
		return originClass{id: "this", kind: "this"}
	}
}

func (s *Source) deviceFilterClause(db *sql.DB, device string, guidToDevice map[string]string, alias string, guidExpr string) (string, []any, error) {
	d := strings.TrimSpace(strings.ToLower(device))
	if d == "" || d == "all" {
		return "", nil, nil
	}
	local := fmt.Sprintf("(COALESCE(%s.source,-1) IN (-1,1))", alias)
	if d == "this" {
		return local, nil, nil
	}
	if d == "synced" {
		return fmt.Sprintf("(%s.source=0)", alias), nil, nil
	}
	if d == "extension" {
		return fmt.Sprintf("(%s.source=2)", alias), nil, nil
	}
	if d == "imported" {
		return fmt.Sprintf("(%s.source IN (3,4,5))", alias), nil, nil
	}
	for guid, id := range guidToDevice {
		if d == id {
			return fmt.Sprintf("(%s.source=0 AND %s=?)", alias, guidExpr), []any{guid}, nil
		}
	}
	return "", nil, fmt.Errorf("unknown device id: %s", device)
}

func (s *Source) originatorGUIDExpr(db *sql.DB, visitAlias, visitSourceAlias string) (string, error) {
	vsHas, err := tableHasColumn(db, "visit_source", "originator_cache_guid")
	if err != nil {
		return "", err
	}
	vHas, err := tableHasColumn(db, "visits", "originator_cache_guid")
	if err != nil {
		return "", err
	}
	switch {
	case vsHas && vHas:
		return fmt.Sprintf("COALESCE(%s.originator_cache_guid,%s.originator_cache_guid,'')", visitSourceAlias, visitAlias), nil
	case vsHas:
		return fmt.Sprintf("COALESCE(%s.originator_cache_guid,'')", visitSourceAlias), nil
	case vHas:
		return fmt.Sprintf("COALESCE(%s.originator_cache_guid,'')", visitAlias), nil
	default:
		return "''", nil
	}
}

func tableHasColumn(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
