package yclocal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// StatsCell is one aggregated row.
type StatsCell struct {
	Key         string  `json:"key"` // batch slug or industry name
	Count       int     `json:"count"`
	AvgTeamSize float64 `json:"avg_team_size"`
	PctHiring   float64 `json:"pct_hiring"`
	PctTop      float64 `json:"pct_top_company"`
	PctActive   float64 `json:"pct_active"`
	PctAcquired float64 `json:"pct_acquired"`
	PctPublic   float64 `json:"pct_public"`
	PctInactive float64 `json:"pct_inactive"`
}

// StatsQuery filters the aggregation.
type StatsQuery struct {
	GroupBy  string // "batch" or "industry"
	Industry string // optional filter
	Batch    string // optional filter
	Tag      string // optional filter (matches substring inside tags JSON)
	Region   string // optional filter (matches substring inside regions JSON)
}

// Stats computes counts/percentages grouped by batch or industry.
func Stats(ctx context.Context, db *sql.DB, q StatsQuery) ([]StatsCell, error) {
	groupCol := ""
	switch strings.ToLower(q.GroupBy) {
	case "batch":
		groupCol = "batch"
	case "industry":
		groupCol = "industry"
	default:
		return nil, fmt.Errorf("group-by must be 'batch' or 'industry'")
	}

	var (
		where []string
		args  []any
	)
	if q.Industry != "" {
		where = append(where, "industry = ?")
		args = append(args, q.Industry)
	}
	if q.Batch != "" {
		where = append(where, "batch = ?")
		args = append(args, q.Batch)
	}
	if q.Tag != "" {
		where = append(where, "LOWER(tags) LIKE ?")
		args = append(args, "%"+strings.ToLower(q.Tag)+"%")
	}
	if q.Region != "" {
		where = append(where, "LOWER(regions) LIKE ?")
		args = append(args, "%"+strings.ToLower(q.Region)+"%")
	}
	where = append(where, groupCol+" IS NOT NULL AND "+groupCol+" <> ''")

	query := `
SELECT ` + groupCol + ` AS key,
       COUNT(*),
       COALESCE(AVG(NULLIF(team_size, 0)), 0),
       100.0 * SUM(CASE WHEN is_hiring = 1 THEN 1 ELSE 0 END) / COUNT(*),
       100.0 * SUM(CASE WHEN top_company = 1 THEN 1 ELSE 0 END) / COUNT(*),
       100.0 * SUM(CASE WHEN LOWER(status) = 'active' THEN 1 ELSE 0 END) / COUNT(*),
       100.0 * SUM(CASE WHEN LOWER(status) = 'acquired' THEN 1 ELSE 0 END) / COUNT(*),
       100.0 * SUM(CASE WHEN LOWER(status) = 'public' THEN 1 ELSE 0 END) / COUNT(*),
       100.0 * SUM(CASE WHEN LOWER(status) = 'inactive' THEN 1 ELSE 0 END) / COUNT(*)
FROM companies
WHERE ` + strings.Join(where, " AND ") + `
GROUP BY ` + groupCol + `
ORDER BY ` + sortOrderFor(groupCol)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StatsCell
	for rows.Next() {
		var c StatsCell
		if err := rows.Scan(&c.Key, &c.Count, &c.AvgTeamSize, &c.PctHiring, &c.PctTop,
			&c.PctActive, &c.PctAcquired, &c.PctPublic, &c.PctInactive); err != nil {
			return nil, err
		}
		c.AvgTeamSize = round3(c.AvgTeamSize)
		c.PctHiring = round3(c.PctHiring)
		c.PctTop = round3(c.PctTop)
		c.PctActive = round3(c.PctActive)
		c.PctAcquired = round3(c.PctAcquired)
		c.PctPublic = round3(c.PctPublic)
		c.PctInactive = round3(c.PctInactive)
		out = append(out, c)
	}
	return out, rows.Err()
}

func sortOrderFor(col string) string {
	if col == "batch" {
		// Order batches chronologically — Winter < Spring < Summer < Fall, by year.
		return `(CAST(SUBSTR(batch, INSTR(batch, ' ')+1) AS INTEGER) * 10) +
                CASE LOWER(SUBSTR(batch, 1, INSTR(batch, ' ')-1))
                  WHEN 'winter' THEN 0
                  WHEN 'spring' THEN 1
                  WHEN 'summer' THEN 2
                  WHEN 'fall' THEN 3
                  WHEN 'late' THEN 4
                  ELSE 5 END ASC`
	}
	return "COUNT(*) DESC, " + col + " ASC"
}

// BatchCard is the projection of `batches show <slug>`.
type BatchCard struct {
	Batch           string         `json:"batch"`
	CompanyCount    int            `json:"company_count"`
	TopIndustries   []KeyCount     `json:"top_industries"`
	TopTags         []KeyCount     `json:"top_tags"`
	PctHiring       float64        `json:"pct_hiring"`
	PctTop          float64        `json:"pct_top_company"`
	PctAcquired     float64        `json:"pct_acquired"`
	PctPublic       float64        `json:"pct_public"`
	PctInactive     float64        `json:"pct_inactive"`
	MedianTeamSize  int            `json:"median_team_size"`
	StatusBreakdown map[string]int `json:"status_breakdown"`
}

// KeyCount is a name/count pair.
type KeyCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// BatchSummary computes the summary card for one batch.
// The batch arg may be the full label ("Winter 2025") or a slug-like form ("w25", "winter-2025");
// the function resolves the canonical batch label by case-insensitive prefix matching.
func BatchSummary(ctx context.Context, db *sql.DB, batchArg string) (*BatchCard, error) {
	batch, err := resolveBatchLabel(ctx, db, batchArg)
	if err != nil {
		return nil, err
	}
	if batch == "" {
		return nil, fmt.Errorf("batch %q not found (try `yc-companies-pp-cli sql \"SELECT DISTINCT batch FROM companies\"` to list batches)", batchArg)
	}
	card := &BatchCard{Batch: batch, StatusBreakdown: map[string]int{}}

	// Aggregates
	row := db.QueryRowContext(ctx, `
SELECT COUNT(*),
       100.0 * SUM(CASE WHEN is_hiring = 1 THEN 1 ELSE 0 END) / NULLIF(COUNT(*),0),
       100.0 * SUM(CASE WHEN top_company = 1 THEN 1 ELSE 0 END) / NULLIF(COUNT(*),0),
       100.0 * SUM(CASE WHEN LOWER(status) = 'acquired' THEN 1 ELSE 0 END) / NULLIF(COUNT(*),0),
       100.0 * SUM(CASE WHEN LOWER(status) = 'public' THEN 1 ELSE 0 END) / NULLIF(COUNT(*),0),
       100.0 * SUM(CASE WHEN LOWER(status) = 'inactive' THEN 1 ELSE 0 END) / NULLIF(COUNT(*),0)
FROM companies WHERE batch = ?`, batch)
	var pctHiring, pctTop, pctAcq, pctPub, pctInac sql.NullFloat64
	if err := row.Scan(&card.CompanyCount, &pctHiring, &pctTop, &pctAcq, &pctPub, &pctInac); err != nil {
		return nil, err
	}
	card.PctHiring = round3(pctHiring.Float64)
	card.PctTop = round3(pctTop.Float64)
	card.PctAcquired = round3(pctAcq.Float64)
	card.PctPublic = round3(pctPub.Float64)
	card.PctInactive = round3(pctInac.Float64)

	// Status breakdown
	srows, err := db.QueryContext(ctx, `SELECT COALESCE(status,''), COUNT(*) FROM companies WHERE batch = ? GROUP BY status`, batch)
	if err != nil {
		return nil, err
	}
	for srows.Next() {
		var s string
		var n int
		if err := srows.Scan(&s, &n); err != nil {
			srows.Close()
			return nil, err
		}
		if s == "" {
			s = "Unknown"
		}
		card.StatusBreakdown[s] = n
	}
	srows.Close()

	// Top 5 industries
	card.TopIndustries, err = topByColumn(ctx, db, "industry", batch, 5)
	if err != nil {
		return nil, err
	}

	// Top 10 tags — tags is a JSON string; expand at the application layer.
	tagCount := make(map[string]int)
	trows, err := db.QueryContext(ctx, `SELECT COALESCE(tags,'') FROM companies WHERE batch = ?`, batch)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		var raw string
		if err := trows.Scan(&raw); err != nil {
			trows.Close()
			return nil, err
		}
		for _, t := range parseTags(raw) {
			if t == "" {
				continue
			}
			tagCount[t]++
		}
	}
	trows.Close()
	card.TopTags = topKFromMap(tagCount, 10)

	// Median team_size
	mrow := db.QueryRowContext(ctx, `
SELECT AVG(team_size) FROM (
  SELECT team_size FROM companies
  WHERE batch = ? AND team_size IS NOT NULL AND team_size > 0
  ORDER BY team_size
  LIMIT 2 - (
    SELECT COUNT(*) FROM companies WHERE batch = ? AND team_size IS NOT NULL AND team_size > 0
  ) % 2
  OFFSET (
    SELECT (COUNT(*) - 1) / 2 FROM companies WHERE batch = ? AND team_size IS NOT NULL AND team_size > 0
  )
)`, batch, batch, batch)
	var med sql.NullFloat64
	if err := mrow.Scan(&med); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	card.MedianTeamSize = int(med.Float64 + 0.5)

	return card, nil
}

func topByColumn(ctx context.Context, db *sql.DB, col, batch string, limit int) ([]KeyCount, error) {
	q := `SELECT ` + col + `, COUNT(*) FROM companies WHERE batch = ? AND ` + col + ` IS NOT NULL AND ` + col + ` <> '' GROUP BY ` + col + ` ORDER BY COUNT(*) DESC, ` + col + ` ASC LIMIT ?`
	rows, err := db.QueryContext(ctx, q, batch, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KeyCount
	for rows.Next() {
		var kc KeyCount
		if err := rows.Scan(&kc.Name, &kc.Count); err != nil {
			return nil, err
		}
		out = append(out, kc)
	}
	return out, rows.Err()
}

func topKFromMap(m map[string]int, k int) []KeyCount {
	out := make([]KeyCount, 0, len(m))
	for name, count := range m {
		out = append(out, KeyCount{Name: name, Count: count})
	}
	// Simple insertion sort suffices; the list is small.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && (out[j].Count > out[j-1].Count || (out[j].Count == out[j-1].Count && out[j].Name < out[j-1].Name)); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if len(out) > k {
		out = out[:k]
	}
	return out
}

// resolveBatchLabel maps user-friendly batch names to the canonical "Winter 2025" form.
// Accepts: "Winter 2025", "winter 2025", "winter-2025", "w25", "W25", "summer-2024", "s24".
func resolveBatchLabel(ctx context.Context, db *sql.DB, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	// Exact match
	var got sql.NullString
	err := db.QueryRowContext(ctx, `SELECT batch FROM companies WHERE LOWER(batch) = LOWER(?) LIMIT 1`, input).Scan(&got)
	if err == nil && got.Valid {
		return got.String, nil
	}

	// Slug form: replace hyphens with spaces
	if dashed := strings.ReplaceAll(input, "-", " "); dashed != input {
		err = db.QueryRowContext(ctx, `SELECT batch FROM companies WHERE LOWER(batch) = LOWER(?) LIMIT 1`, dashed).Scan(&got)
		if err == nil && got.Valid {
			return got.String, nil
		}
	}

	// Short-code form: w25/s24/spring22/fall23
	short := strings.ToLower(input)
	seasonMap := map[byte]string{'w': "Winter", 's': "Summer", 'f': "Fall"}
	if len(short) >= 3 {
		first := short[0]
		var season string
		var yearStart int
		if s, ok := seasonMap[first]; ok && short[1] >= '0' && short[1] <= '9' {
			season = s
			yearStart = 1
		} else {
			// Try "winterXX", "summerXX", "fallXX", "springXX"
			for _, name := range []string{"winter", "summer", "spring", "fall", "late"} {
				if strings.HasPrefix(short, name) && len(short) > len(name) {
					season = strings.Title(name) // nolint:staticcheck
					yearStart = len(name)
					break
				}
			}
		}
		if season != "" {
			yr := short[yearStart:]
			yr = strings.TrimLeft(yr, " -")
			yi := 0
			for _, c := range yr {
				if c < '0' || c > '9' {
					break
				}
				yi = yi*10 + int(c-'0')
			}
			if yi > 0 {
				if yi < 100 {
					yi += 2000
				}
				cand := fmt.Sprintf("%s %d", season, yi)
				err = db.QueryRowContext(ctx, `SELECT batch FROM companies WHERE batch = ? LIMIT 1`, cand).Scan(&got)
				if err == nil && got.Valid {
					return got.String, nil
				}
			}
		}
	}

	// Try LIKE fallback
	err = db.QueryRowContext(ctx, `SELECT batch FROM companies WHERE LOWER(batch) LIKE LOWER(?) || '%' LIMIT 1`, input).Scan(&got)
	if err == nil && got.Valid {
		return got.String, nil
	}
	return "", nil
}
