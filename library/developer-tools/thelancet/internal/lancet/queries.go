package lancet

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// AuthorRank is one row of the rank-authors output.
type AuthorRank struct {
	AuthorID       string  `json:"author_id"`
	AuthorName     string  `json:"author_name"`
	Works          int     `json:"works"`
	TotalCitations int     `json:"total_citations"`
	AvgCitations   float64 `json:"avg_citations"`
}

// RankAuthors ranks authors by total citations across the local store,
// optionally scoped to a journal ISSN and/or an institution substring.
func RankAuthors(ctx context.Context, db *sql.DB, issn, institution string, limit int) ([]AuthorRank, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	q := `
		SELECT a.author_id, a.author_name,
		       COUNT(DISTINCT w.work_id) AS works,
		       COALESCE(SUM(w.cited_count), 0) AS total_cites,
		       COALESCE(AVG(w.cited_count), 0) AS avg_cites
		FROM lancet_authorships a
		JOIN lancet_works w ON w.work_id = a.work_id`
	var where []string
	var args []any
	if issn != "" {
		where = append(where, "w.journal_issn = ?")
		args = append(args, issn)
	}
	if institution != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM lancet_affiliations af
			WHERE af.work_id = a.work_id AND af.author_id = a.author_id
			  AND af.institution_name LIKE ?
		)`)
		args = append(args, "%"+institution+"%")
	}
	q += whereClause(where) + `
		GROUP BY a.author_id, a.author_name
		ORDER BY total_cites DESC
		LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuthorRank
	for rows.Next() {
		var r AuthorRank
		var name sql.NullString
		if err := rows.Scan(&r.AuthorID, &name, &r.Works, &r.TotalCitations, &r.AvgCitations); err != nil {
			continue
		}
		r.AuthorName = name.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// CoAuthorEdge is one collaboration pair in the mesh output.
type CoAuthorEdge struct {
	AuthorA     string `json:"author_a"`
	AuthorB     string `json:"author_b"`
	SharedWorks int    `json:"shared_works"`
}

// CoAuthorMesh finds co-authorship pairs where both authors have published from
// the given institution, ranked by number of shared works.
//
// The institution's authors are collected into an indexed temporary table
// rather than matched with `author_id IN (SELECT ... )`. That IN-list is what
// made this query unusable on a real store: SQLite materialises the first
// reference but replays the second as a LINEAR SCAN of the list for every
// candidate row, so the cost is (rows of the institution's authors) x (number
// of those authors).
//
// Measured on the Bibliovera mirror (506k authorships, 610k affiliations,
// "oxford" -> 3,649 authors and 12,020 authorship rows, so ~44M list
// comparisons):
//
//	IN (SELECT ...)            29.9s   <- the form this replaces
//	CTE narrowing the rows     28.4s
//	COUNT(*) instead of DISTINCT 28.7s
//	EXISTS against the index   >60s, abandoned
//	indexed temp table          0.98s  <- this form
//
// All of the completed variants returned byte-identical output: 28,963 pairs,
// verified with diff. The speedup is in how the set is probed, not in what is
// counted, and the callers see exactly the same rows in the same order.
//
// Two details are load-bearing:
//
//   - The work runs on an explicit *sql.Conn. A TEMP table belongs to one
//     SQLite connection, and database/sql is free to hand the next statement a
//     different pooled connection, on which the table would simply not exist.
//     One caller (ensureLancetStore) already pins the pool to a single
//     connection, but this function cannot see that and must not depend on it.
//   - The table is dropped on the way out. The connection goes back to the
//     pool afterwards, so a leftover table would meet the next call's CREATE
//     and fail it.
//
// COUNT(*) is correct in place of COUNT(DISTINCT a1.work_id) because
// lancet_authorships has PRIMARY KEY (work_id, author_id): within one
// (a1.author_id, a2.author_id) group a work_id cannot repeat, so there is
// nothing for DISTINCT to remove. It is also the form the 0.98s figure above
// was measured with.
func CoAuthorMesh(ctx context.Context, db *sql.DB, institution string, limit int) ([]CoAuthorEdge, error) {
	if institution == "" {
		return nil, fmt.Errorf("institution is required")
	}
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// A previous call on this same pooled connection should have dropped its
	// table, but a cancelled context can cut the drop short. Clearing first
	// makes the function safe to retry rather than dependent on the last run
	// having finished cleanly.
	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp.mesh_inst_authors`); err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TEMP TABLE mesh_inst_authors (
			author_id TEXT PRIMARY KEY
		)`); err != nil {
		return nil, err
	}
	defer func() {
		// Not ctx: if the caller's context is already cancelled this is exactly
		// when the drop matters most, and leaving the table behind would break
		// the NEXT call rather than this one.
		_, _ = conn.ExecContext(context.Background(), `DROP TABLE IF EXISTS temp.mesh_inst_authors`)
	}()

	if _, err := conn.ExecContext(ctx, `
		INSERT OR IGNORE INTO mesh_inst_authors(author_id)
		SELECT DISTINCT author_id FROM lancet_affiliations
		WHERE institution_name LIKE ?`, "%"+institution+"%"); err != nil {
		return nil, err
	}

	q := `
		SELECT a1.author_name, a2.author_name, COUNT(*) AS shared
		FROM lancet_authorships a1
		JOIN mesh_inst_authors i1 ON i1.author_id = a1.author_id
		JOIN lancet_authorships a2
		  ON a2.work_id = a1.work_id AND a2.author_id > a1.author_id
		JOIN mesh_inst_authors i2 ON i2.author_id = a2.author_id
		GROUP BY a1.author_id, a2.author_id
		ORDER BY shared DESC
		LIMIT ?`
	rows, err := conn.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoAuthorEdge
	for rows.Next() {
		var e CoAuthorEdge
		var a, b sql.NullString
		if err := rows.Scan(&a, &b, &e.SharedWorks); err != nil {
			continue
		}
		e.AuthorA, e.AuthorB = a.String, b.String
		out = append(out, e)
	}
	return out, rows.Err()
}

// InstGrowth is one row of the affiliation-growth output.
type InstGrowth struct {
	Institution string `json:"institution"`
	RecentCount int    `json:"recent_count"`
	PriorCount  int    `json:"prior_count"`
	Growth      int    `json:"growth"`
}

// AffiliationGrowth compares institutional publication counts between the most
// recent `years` and the equal-length window before it, returning institutions
// whose recent count meets `threshold`, ranked by growth.
func AffiliationGrowth(ctx context.Context, db *sql.DB, issn string, years, threshold, limit int) ([]InstGrowth, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	var maxYear sql.NullInt64
	yq := `SELECT MAX(pub_year) FROM lancet_works`
	yargs := []any{}
	if issn != "" {
		yq += ` WHERE journal_issn = ?`
		yargs = append(yargs, issn)
	}
	if err := db.QueryRowContext(ctx, yq, yargs...).Scan(&maxYear); err != nil {
		return nil, err
	}
	if !maxYear.Valid {
		return nil, nil
	}
	top := int(maxYear.Int64)
	recentStart := top - years + 1
	priorStart := recentStart - years
	priorEnd := recentStart - 1

	q := `
		SELECT af.institution_name,
		       COUNT(DISTINCT CASE WHEN w.pub_year BETWEEN ? AND ? THEN w.work_id END) AS recent,
		       COUNT(DISTINCT CASE WHEN w.pub_year BETWEEN ? AND ? THEN w.work_id END) AS prior
		FROM lancet_affiliations af
		JOIN lancet_works w ON w.work_id = af.work_id`
	var where []string
	args := []any{recentStart, top, priorStart, priorEnd}
	if issn != "" {
		where = append(where, "w.journal_issn = ?")
		args = append(args, issn)
	}
	q += whereClause(where) + `
		GROUP BY af.institution_name
		HAVING recent >= ?
		ORDER BY (recent - prior) DESC
		LIMIT ?`
	args = append(args, threshold, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InstGrowth
	for rows.Next() {
		var g InstGrowth
		var name sql.NullString
		if err := rows.Scan(&name, &g.RecentCount, &g.PriorCount); err != nil {
			continue
		}
		g.Institution = name.String
		g.Growth = g.RecentCount - g.PriorCount
		out = append(out, g)
	}
	return out, rows.Err()
}

// TopicShift is one row of the drift output.
type TopicShift struct {
	Topic        string  `json:"topic"`
	Window1Count int     `json:"window1_count"`
	Window2Count int     `json:"window2_count"`
	Window1Share float64 `json:"window1_share"`
	Window2Share float64 `json:"window2_share"`
	DeltaShare   float64 `json:"delta_share"`
}

// TopicDrift compares topic share between two publication-year windows for a
// journal, returning the topics with the largest share change (positive =
// rising in window2).
func TopicDrift(ctx context.Context, db *sql.DB, issn string, w1s, w1e, w2s, w2e, topN int) ([]TopicShift, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	counts := map[string]*TopicShift{}
	total1, total2 := 0, 0

	load := func(start, end int, assign func(*TopicShift, int)) error {
		q := `SELECT COALESCE(topic,'(untagged)'), COUNT(*) FROM lancet_works
		      WHERE pub_year BETWEEN ? AND ?`
		args := []any{start, end}
		if issn != "" {
			q += ` AND journal_issn = ?`
			args = append(args, issn)
		}
		q += ` GROUP BY topic`
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var topic string
			var n int
			if err := rows.Scan(&topic, &n); err != nil {
				continue
			}
			if counts[topic] == nil {
				counts[topic] = &TopicShift{Topic: topic}
			}
			assign(counts[topic], n)
		}
		return rows.Err()
	}

	if err := load(w1s, w1e, func(t *TopicShift, n int) { t.Window1Count = n; total1 += n }); err != nil {
		return nil, err
	}
	if err := load(w2s, w2e, func(t *TopicShift, n int) { t.Window2Count = n; total2 += n }); err != nil {
		return nil, err
	}
	var out []TopicShift
	for _, t := range counts {
		if total1 > 0 {
			t.Window1Share = float64(t.Window1Count) / float64(total1)
		}
		if total2 > 0 {
			t.Window2Share = float64(t.Window2Count) / float64(total2)
		}
		t.DeltaShare = t.Window2Share - t.Window1Share
		out = append(out, *t)
	}
	// Sort by absolute delta share descending.
	sortByAbsDelta(out)
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out, nil
}

// WorkRow is one row of the curate output.
type WorkRow struct {
	Title   string `json:"title"`
	DOI     string `json:"doi"`
	Journal string `json:"journal"`
	Year    int    `json:"year"`
	Cited   int    `json:"cited_by_count"`
	Topic   string `json:"topic"`
	// PubDate is YYYY-MM-DD, empty when unknown.
	PubDate          string  `json:"pub_date"`
	CitationsPerYear float64 `json:"citations_per_year"`
	// Velocity is the recency-weighted mean of citations in the last complete
	// calendar years after publication (see velocitySQL); null while the work
	// has no complete year yet. CitationsLastYear and Acceleration are display
	// only, as are FWCI and CitationNormalizedPercentile (OpenAlex's values,
	// null when it has none). None of them is a sort key except Velocity.
	Velocity                     *float64 `json:"velocity"`
	CitationsLastYear            *int     `json:"citations_last_year"`
	Acceleration                 *float64 `json:"acceleration"`
	FWCI                         *float64 `json:"fwci"`
	CitationNormalizedPercentile *float64 `json:"citation_normalized_percentile"`
}

// minAgeYears floors a work's age so a paper published days ago does not get an
// absurd citations-per-year figure.
const minAgeYears = 0.25

// CitationsPerYear is cited / age in years, rounded to one decimal. Age is
// (now - pubDate) / 365.25 days; an empty or invalid pubDate falls back to
// July 1 of year; age is floored at 0.25 years. ok is false when neither a
// valid date nor a year is available.
func CitationsPerYear(now time.Time, pubDate string, year, cited int) (cpy float64, ok bool) {
	d, err := time.Parse("2006-01-02", pubDate)
	if err != nil {
		if year <= 0 {
			return 0, false
		}
		d = time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC)
	}
	age := now.Sub(d).Hours() / 24 / 365.25
	if age < minAgeYears {
		age = minAgeYears
	}
	return math.Round(float64(cited)/age*10) / 10, true
}

// Same age rule as CitationsPerYear, evaluated in SQL so it can run before LIMIT.
// validDateSQL accepts pub_date only when it is exactly a real YYYY-MM-DD: the
// GLOB rejects other shapes ("2024-2-5", timestamps) and the date() round trip
// rejects day overflow that SQLite would normalise ("2024-02-31"), matching
// time.Parse in the Go helper. Anything else falls back to July 1 of pub_year.
const (
	validDateSQL = `CASE WHEN length(pub_date) = 10 AND pub_date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]' AND date(pub_date) = pub_date THEN pub_date END`
	effDateSQL   = `COALESCE(` + validDateSQL + `, CASE WHEN pub_year > 0 THEN printf('%04d-07-01', pub_year) END)`
	// rawCpySQL is the unrounded rate, used for ordering; cpySQL rounds it for output.
	rawCpySQL = `cited_count / MAX(0.25, (julianday('now') - julianday(` + effDateSQL + `)) / 365.25)`
	cpySQL    = `CASE WHEN ` + effDateSQL + ` IS NULL THEN 0 ELSE ROUND(` + rawCpySQL + `, 1) END`
)

// The velocity window: the last three complete calendar years (the current year
// is partial and excluded), keeping only years after the publication year, with
// weights 0.5, 0.3, 0.2 for Y-1, Y-2, Y-3 renormalised over the kept years. A
// year missing from lancet_work_year_counts counts as 0. The year comes from
// the store clock, like the age in cpySQL. c1..c3 are the LEFT JOINed counts.
const (
	curYearSQL = `CAST(strftime('%Y', 'now') AS INTEGER)`
	c1SQL      = `COALESCE(y1.cited_by_count, 0)`
	c2SQL      = `COALESCE(y2.cited_by_count, 0)`
	c3SQL      = `COALESCE(y3.cited_by_count, 0)`
	// currentSyncSQL: the counts were fetched during the current UTC year, so
	// every scored year (Y-1..Y-3) was already complete at sync time. A work
	// synced earlier (or never) has no current counts.
	currentSyncSQL = `(counts_synced_at IS NOT NULL AND counts_synced_at >= strftime('%Y', 'now') || '-01-01')`
	// keepN is true when year Y-N is a complete year after the publication year.
	keep1SQL = `(` + currentSyncSQL + ` AND pub_year > 0 AND pub_year < ` + curYearSQL + ` - 1)`
	keep2SQL = `(` + currentSyncSQL + ` AND pub_year > 0 AND pub_year < ` + curYearSQL + ` - 2)`
	keep3SQL = `(` + currentSyncSQL + ` AND pub_year > 0 AND pub_year < ` + curYearSQL + ` - 3)`
	// velocitySQL is the unrounded velocity, used for ordering; NULL when no
	// complete year follows the publication year.
	velocitySQL = `CASE WHEN ` + keep1SQL + ` THEN
		(0.5 * ` + c1SQL + ` + CASE WHEN ` + keep2SQL + ` THEN 0.3 * ` + c2SQL + ` ELSE 0 END + CASE WHEN ` + keep3SQL + ` THEN 0.2 * ` + c3SQL + ` ELSE 0 END) /
		(0.5 + CASE WHEN ` + keep2SQL + ` THEN 0.3 ELSE 0 END + CASE WHEN ` + keep3SQL + ` THEN 0.2 ELSE 0 END) END`
	lastYearSQL = `CASE WHEN ` + keep1SQL + ` THEN ` + c1SQL + ` END`
	// Acceleration needs all three years to be complete years after publication.
	accelSQL = `CASE WHEN ` + keep3SQL + ` THEN ` + c1SQL + ` - (` + c2SQL + ` + ` + c3SQL + `) / 2.0 END`
	// curateFromSQL joins the three yearly counts the velocity window reads.
	curateFromSQL = `FROM lancet_works
	      LEFT JOIN lancet_work_year_counts y1 ON y1.work_id = lancet_works.work_id AND y1.year = ` + curYearSQL + ` - 1
	      LEFT JOIN lancet_work_year_counts y2 ON y2.work_id = lancet_works.work_id AND y2.year = ` + curYearSQL + ` - 2
	      LEFT JOIN lancet_work_year_counts y3 ON y3.work_id = lancet_works.work_id AND y3.year = ` + curYearSQL + ` - 3`
)

// curateWhere builds the filter shared by Curate and VelocityCoverage.
func curateWhere(topic, issn string, openAccessOnly bool) (where string, args []any) {
	// Whole-word match (porter unicode61, AND across words). A topic with no
	// searchable word (empty/whitespace/punctuation only) applies no text
	// filter and returns every work, subject to the other filters.
	if m := ftsMatchQuery(topic); m != "" {
		where += ` AND lancet_works.rowid IN (SELECT rowid FROM lancet_works_fts WHERE lancet_works_fts MATCH ?)`
		args = append(args, m)
	}
	if issn != "" {
		where += ` AND journal_issn = ?`
		args = append(args, issn)
	}
	if openAccessOnly {
		where += ` AND is_oa = 1`
	}
	return where, args
}

func countSynced(ctx context.Context, db *sql.DB, where string, args []any) (matches, synced int, err error) {
	err = db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(`+currentSyncSQL+`), 0)
		FROM lancet_works WHERE 1=1`+where, args...).Scan(&matches, &synced)
	return
}

// VelocityCoverage returns how many works match the curate filters and how many
// of them have no current yearly citation counts (never synced, or synced before
// this year); their velocity is no-data.
func VelocityCoverage(ctx context.Context, db *sql.DB, topic, issn string, openAccessOnly bool) (matches, unsynced int, err error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return 0, 0, err
	}
	where, args := curateWhere(topic, issn, openAccessOnly)
	matches, synced, err := countSynced(ctx, db, where, args)
	return matches, matches - synced, err
}

// Curate selects works matching a topic/keyword (whole words in title or topic),
// scoped optionally to a journal, sorted by "citations", "date", "per-year"
// (average citations per year) or "velocity" (recent-window citations). Both
// computed sorts rank over all matching rows before LIMIT.
func Curate(ctx context.Context, db *sql.DB, topic, issn, sort string, openAccessOnly bool, limit int) ([]WorkRow, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	where, args := curateWhere(topic, issn, openAccessOnly)
	if sort == "velocity" {
		// A mirror with no work whose yearly counts were ever fetched would rank
		// an all-null column; fail with the refresh hint instead.
		matches, synced, err := countSynced(ctx, db, where, args)
		if err != nil {
			return nil, err
		}
		if matches > 0 && synced == 0 {
			return nil, fmt.Errorf("--sort velocity needs yearly citation counts, which this local store does not have yet; run 'thelancet-pp-cli refresh' to fetch them, or sort by citations, date or per-year")
		}
	}
	q := `SELECT title, doi, journal_name, pub_year, cited_count, COALESCE(topic,''),
	             COALESCE(` + validDateSQL + `,''), ` + cpySQL + ` AS cpy,
	             ` + velocitySQL + `, ` + lastYearSQL + `, ` + accelSQL + `,
	             fwci, citation_normalized_percentile
	      ` + curateFromSQL + ` WHERE 1=1` + where
	switch sort {
	case "date":
		q += ` ORDER BY pub_date DESC`
	case "per-year":
		q += ` AND ` + effDateSQL + ` IS NOT NULL ORDER BY ` + rawCpySQL + ` DESC, cited_count DESC, title`
	case "velocity":
		q += ` ORDER BY (` + velocitySQL + `) IS NULL, ` + velocitySQL + ` DESC, cited_count DESC, lancet_works.work_id`
	default:
		q += ` ORDER BY cited_count DESC`
	}
	q += ` LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkRow
	for rows.Next() {
		var w WorkRow
		var title, doi, jn, tp, pd sql.NullString
		var vel, accel, fw, pct sql.NullFloat64
		var last sql.NullInt64
		if err := rows.Scan(&title, &doi, &jn, &w.Year, &w.Cited, &tp, &pd, &w.CitationsPerYear, &vel, &last, &accel, &fw, &pct); err != nil {
			continue
		}
		w.Title, w.DOI, w.Journal, w.Topic = title.String, doi.String, jn.String, tp.String
		w.PubDate = pd.String
		if vel.Valid {
			v := math.Round(vel.Float64*100) / 100
			w.Velocity = &v
		}
		if last.Valid {
			n := int(last.Int64)
			w.CitationsLastYear = &n
		}
		if accel.Valid {
			w.Acceleration = &accel.Float64
		}
		if fw.Valid {
			w.FWCI = &fw.Float64
		}
		if pct.Valid {
			w.CitationNormalizedPercentile = &pct.Float64
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// VisibilityRow is one row of the visibility-gap output.
type VisibilityRow struct {
	AuthorID      string  `json:"author_id"`
	AuthorName    string  `json:"author_name"`
	Works         int     `json:"works"`
	AuthorAvgCite float64 `json:"author_avg_citations"`
	JournalAvg    float64 `json:"journal_avg_citations"`
	Gap           float64 `json:"gap"`
}

// VisibilityGap compares each author's average citations against the average
// citations of the journals they publish in (a prestige proxy), surfacing
// authors most out of step with their journals. Scoped optionally to an
// institution substring.
func VisibilityGap(ctx context.Context, db *sql.DB, institution string, minWorks, limit int) ([]VisibilityRow, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	// Per-journal average citations (prestige proxy).
	jq := `SELECT journal_issn, AVG(cited_count) FROM lancet_works GROUP BY journal_issn`
	jrows, err := db.QueryContext(ctx, jq)
	if err != nil {
		return nil, err
	}
	journalAvg := map[string]float64{}
	for jrows.Next() {
		var issn sql.NullString
		var avg sql.NullFloat64
		if err := jrows.Scan(&issn, &avg); err == nil {
			journalAvg[issn.String] = avg.Float64
		}
	}
	if err := jrows.Err(); err != nil {
		jrows.Close()
		return nil, err
	}
	jrows.Close()

	q := `
		SELECT a.author_id, a.author_name, w.journal_issn, w.cited_count
		FROM lancet_authorships a
		JOIN lancet_works w ON w.work_id = a.work_id`
	var args []any
	if institution != "" {
		q += ` WHERE EXISTS (
			SELECT 1 FROM lancet_affiliations af
			WHERE af.work_id = a.work_id AND af.author_id = a.author_id
			  AND af.institution_name LIKE ?
		)`
		args = append(args, "%"+institution+"%")
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type acc struct {
		name        string
		works       int
		sumCite     float64
		sumJournalA float64
	}
	agg := map[string]*acc{}
	for rows.Next() {
		var id, name, issn sql.NullString
		var cited sql.NullInt64
		if err := rows.Scan(&id, &name, &issn, &cited); err != nil {
			continue
		}
		a := agg[id.String]
		if a == nil {
			a = &acc{name: name.String}
			agg[id.String] = a
		}
		a.works++
		a.sumCite += float64(cited.Int64)
		a.sumJournalA += journalAvg[issn.String]
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []VisibilityRow
	for id, a := range agg {
		if a.works < minWorks {
			continue
		}
		authorAvg := a.sumCite / float64(a.works)
		journalMean := a.sumJournalA / float64(a.works)
		out = append(out, VisibilityRow{
			AuthorID:      id,
			AuthorName:    a.name,
			Works:         a.works,
			AuthorAvgCite: authorAvg,
			JournalAvg:    journalMean,
			Gap:           authorAvg - journalMean,
		})
	}
	sortByAbsGap(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func whereClause(where []string) string {
	if len(where) == 0 {
		return ""
	}
	out := " WHERE "
	for i, w := range where {
		if i > 0 {
			out += " AND "
		}
		out += w
	}
	return out
}
