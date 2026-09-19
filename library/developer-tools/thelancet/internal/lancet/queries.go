package lancet

import (
	"context"
	"database/sql"
	"fmt"
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
}

// Curate selects works matching a topic/keyword (title or topic substring),
// scoped optionally to a journal, sorted by "citations" or "date".
func Curate(ctx context.Context, db *sql.DB, topic, issn, sort string, openAccessOnly bool, limit int) ([]WorkRow, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	q := `SELECT title, doi, journal_name, pub_year, cited_count, COALESCE(topic,'')
	      FROM lancet_works WHERE (title LIKE ? OR topic LIKE ?)`
	args := []any{"%" + topic + "%", "%" + topic + "%"}
	if issn != "" {
		q += ` AND journal_issn = ?`
		args = append(args, issn)
	}
	if openAccessOnly {
		q += ` AND is_oa = 1`
	}
	switch sort {
	case "date":
		q += ` ORDER BY pub_date DESC`
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
		var title, doi, jn, tp sql.NullString
		if err := rows.Scan(&title, &doi, &jn, &w.Year, &w.Cited, &tp); err != nil {
			continue
		}
		w.Title, w.DOI, w.Journal, w.Topic = title.String, doi.String, jn.String, tp.String
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
