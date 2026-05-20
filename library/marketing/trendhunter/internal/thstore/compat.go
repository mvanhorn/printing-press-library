package thstore

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/internal/thparse"
)

type Store struct {
	db *sql.DB
}

type ScoredTrend struct {
	thparse.Trend
	MatchScore float64 `json:"match_score"`
	Why        string  `json:"why,omitempty"`
}

func DefaultPath() string {
	if v := os.Getenv("TRENDHUNTER_DB"); v != "" {
		return v
	}
	// PATCH: Keep novel-command writes inside this generated working tree.
	return filepath.Join(".", ".trendhunter-pp-cli.db")
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, err
	}
	if err := EnsureSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) UpsertTrends(ctx context.Context, trends []thparse.Trend) (int, error) {
	count := 0
	for _, trend := range trends {
		if strings.TrimSpace(trend.Slug) == "" {
			continue
		}
		if err := UpsertTrend(ctx, s.db, trend); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Store) Latest(ctx context.Context, limit int) ([]thparse.Trend, error) {
	return ListTrendsByCategory(ctx, s.db, "", 3650*24*time.Hour, limit)
}

func (s *Store) BySlug(ctx context.Context, slug string) (thparse.Trend, error) {
	trend, ok, err := GetTrend(ctx, s.db, slug)
	if err != nil {
		return thparse.Trend{}, err
	}
	if !ok {
		return thparse.Trend{}, sql.ErrNoRows
	}
	return *trend, nil
}

func (s *Store) FAQ(ctx context.Context, slug string) ([]thparse.FAQ, error) {
	trend, ok, err := GetTrend(ctx, s.db, slug)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, sql.ErrNoRows
	}
	return trend.FAQ, nil
}

func (s *Store) Search(ctx context.Context, query, category string, limit int) ([]ScoredTrend, error) {
	if limit <= 0 {
		limit = 10
	}
	trends, err := s.Latest(ctx, 500)
	if err != nil {
		return nil, err
	}
	terms := scoreTerms(query)
	var scored []ScoredTrend
	for _, trend := range trends {
		if category != "" && !strings.EqualFold(trend.Category, category) {
			continue
		}
		haystack := strings.ToLower(strings.Join([]string{
			trend.Title,
			trend.Description,
			trend.BodyText,
			strings.Join(trend.Keywords, " "),
		}, " "))
		score := 0.0
		var hits []string
		for _, term := range terms {
			if strings.Contains(haystack, term) {
				score++
				hits = append(hits, term)
			}
		}
		if query != "" && score == 0 {
			continue
		}
		scored = append(scored, ScoredTrend{Trend: trend, MatchScore: score, Why: strings.Join(hits, ", ")})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].MatchScore == scored[j].MatchScore {
			return scored[i].Title < scored[j].Title
		}
		return scored[i].MatchScore > scored[j].MatchScore
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

func (s *Store) Authors(ctx context.Context, limit int) ([]AuthorRow, error) {
	return ListAuthorVelocity(ctx, s.db, 3650*24*time.Hour, limit)
}

func (s *Store) Clusters(ctx context.Context, limit int) ([]KeywordRow, error) {
	return KeywordCounts(ctx, s.db, 3650*24*time.Hour, limit)
}

func (s *Store) SetCursor(ctx context.Context, _ string, value string) error {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t = time.Now()
	}
	return UpdateCursor(ctx, s.db, t)
}

func scoreTerms(query string) []string {
	stop := map[string]bool{"and": true, "for": true, "the": true, "with": true, "your": true}
	seen := map[string]bool{}
	var out []string
	for _, term := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if len(term) < 3 || stop[term] || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out
}
