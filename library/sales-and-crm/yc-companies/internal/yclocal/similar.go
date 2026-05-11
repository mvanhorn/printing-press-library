package yclocal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SimilarHit is one peer returned by Similar().
type SimilarHit struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Batch        string   `json:"batch"`
	Industry     string   `json:"industry"`
	OneLiner     string   `json:"one_liner"`
	SharedTags   []string `json:"shared_tags"`
	Score        float64  `json:"score"`
	TagOverlap   float64  `json:"tag_overlap"`
	SameIndustry bool     `json:"same_industry"`
}

// Similar ranks peers of slug by Jaccard tag overlap, with a same-industry
// bonus and a batch-proximity bonus.
func Similar(ctx context.Context, db *sql.DB, slug string, limit int) ([]SimilarHit, error) {
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	if limit <= 0 {
		limit = 10
	}

	var (
		anchorTagsJSON sql.NullString
		anchorIndustry sql.NullString
		anchorBatch    sql.NullString
	)
	err := db.QueryRowContext(ctx, `SELECT tags, industry, batch FROM companies WHERE slug = ? LIMIT 1`, slug).Scan(&anchorTagsJSON, &anchorIndustry, &anchorBatch)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("company %q not found in local store (run sync first)", slug)
	}
	if err != nil {
		return nil, err
	}
	anchorTags := parseTags(anchorTagsJSON.String)
	if len(anchorTags) == 0 && !anchorIndustry.Valid {
		return nil, nil
	}
	anchorSet := make(map[string]bool, len(anchorTags))
	for _, t := range anchorTags {
		anchorSet[strings.ToLower(t)] = true
	}

	rows, err := db.QueryContext(ctx, `
SELECT slug, COALESCE(name,''), COALESCE(batch,''), COALESCE(industry,''),
       COALESCE(one_liner,''), COALESCE(tags,'')
FROM companies WHERE slug <> ?`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	anchorBatchKey := batchSortKey(anchorBatch.String)
	var hits []SimilarHit
	for rows.Next() {
		var h SimilarHit
		var tagsJSON string
		if err := rows.Scan(&h.Slug, &h.Name, &h.Batch, &h.Industry, &h.OneLiner, &tagsJSON); err != nil {
			return nil, err
		}
		peerTags := parseTags(tagsJSON)
		if len(peerTags) == 0 && len(anchorTags) == 0 {
			continue
		}
		shared, union := jaccardSets(anchorSet, peerTags)
		if len(shared) == 0 && !strings.EqualFold(h.Industry, anchorIndustry.String) {
			continue
		}
		var overlap float64
		if union > 0 {
			overlap = float64(len(shared)) / float64(union)
		}
		h.TagOverlap = overlap
		h.SharedTags = shared
		h.SameIndustry = strings.EqualFold(h.Industry, anchorIndustry.String) && anchorIndustry.Valid && anchorIndustry.String != ""

		score := overlap
		if h.SameIndustry {
			score += 0.2
		}
		score += batchProximityBonus(anchorBatchKey, batchSortKey(h.Batch))
		h.Score = round3(score)
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Slug < hits[j].Slug
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// parseTags accepts the JSON-encoded tags field (the synced store keeps the
// upstream JSON array as a string) and returns the slice.
func parseTags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return nil
	}
	if strings.HasPrefix(s, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return arr
		}
	}
	// Fall back to comma-separated
	out := strings.Split(s, ",")
	clean := out[:0]
	for _, v := range out {
		v = strings.TrimSpace(v)
		if v != "" {
			clean = append(clean, v)
		}
	}
	return clean
}

func jaccardSets(anchor map[string]bool, peer []string) (shared []string, unionCount int) {
	seen := make(map[string]bool, len(peer))
	for _, p := range peer {
		key := strings.ToLower(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		if anchor[key] {
			shared = append(shared, p)
		}
	}
	unionCount = len(anchor)
	for key := range seen {
		if !anchor[key] {
			unionCount++
		}
	}
	return shared, unionCount
}

// batchSortKey turns "Winter 2024" / "Summer 2024" into a sortable int.
// Winter=0, Spring=1, Summer=2, Fall=3, Late=4 etc.
func batchSortKey(batch string) int {
	parts := strings.Fields(batch)
	if len(parts) < 2 {
		return 0
	}
	season := strings.ToLower(parts[0])
	year := 0
	fmt.Sscanf(parts[len(parts)-1], "%d", &year)
	seasonRank := map[string]int{"winter": 0, "spring": 1, "summer": 2, "fall": 3, "late": 4}
	r, ok := seasonRank[season]
	if !ok {
		return year * 10
	}
	return year*10 + r
}

func batchProximityBonus(anchor, peer int) float64 {
	if anchor == 0 || peer == 0 {
		return 0
	}
	diff := anchor - peer
	if diff < 0 {
		diff = -diff
	}
	// Keys are year*10+season, so consecutive seasons differ by 1 inside a
	// year and same-season-next-year differs by 10. Group "within ~1 year"
	// and "within ~3 years" buckets accordingly.
	switch {
	case diff == 0:
		return 0.10
	case diff <= 10:
		return 0.05
	case diff <= 30:
		return 0.02
	default:
		return 0
	}
}

func round3(x float64) float64 {
	return float64(int64(x*1000+0.5)) / 1000
}
