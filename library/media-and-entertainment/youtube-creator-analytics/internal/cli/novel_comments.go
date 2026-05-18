package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
)

type commentRow struct {
	ID        string
	VideoID   string
	Author    string
	Text      string
	LikeCount int64
}

func loadComments(db *store.Store, limit int) ([]commentRow, error) {
	q := `SELECT id, data FROM resources WHERE resource_type IN ('commentThreads','comments')`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()
	var out []commentRow
	for rows.Next() {
		var id string
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		c, ok := decodeComment(id, data.String)
		if !ok {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func decodeComment(id, raw string) (commentRow, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return commentRow{}, false
	}
	c := commentRow{ID: id}
	// commentThread has snippet.topLevelComment.snippet
	var snippet map[string]json.RawMessage
	_ = json.Unmarshal(obj["snippet"], &snippet)
	if snippet != nil {
		var top map[string]json.RawMessage
		_ = json.Unmarshal(snippet["topLevelComment"], &top)
		if top != nil {
			var inner map[string]json.RawMessage
			_ = json.Unmarshal(top["snippet"], &inner)
			if inner != nil {
				_ = json.Unmarshal(inner["videoId"], &c.VideoID)
				_ = json.Unmarshal(inner["authorDisplayName"], &c.Author)
				_ = json.Unmarshal(inner["textDisplay"], &c.Text)
				c.LikeCount = parseInt64(inner["likeCount"])
				return c, true
			}
		}
		// plain comment
		_ = json.Unmarshal(snippet["videoId"], &c.VideoID)
		_ = json.Unmarshal(snippet["authorDisplayName"], &c.Author)
		_ = json.Unmarshal(snippet["textDisplay"], &c.Text)
		c.LikeCount = parseInt64(snippet["likeCount"])
		if c.Text == "" {
			return c, false
		}
		return c, true
	}
	return c, false
}

// ---- comment faq: top question-shaped comments by likes ----

type faqRow struct {
	Question  string `json:"question"`
	LikeCount int64  `json:"like_count"`
	VideoID   string `json:"video_id,omitempty"`
	Author    string `json:"author,omitempty"`
}

func newCommentFAQCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "comment-faq",
		Short:       "Surface most-liked question-shaped comments across cached videos",
		Example:     "  youtube-creator-analytics-pp-cli comment-faq --limit 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("youtube-creator-analytics-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			comments, err := loadComments(db, 0)
			if err != nil {
				return err
			}
			out := []faqRow{}
			for _, c := range comments {
				txt := stripHTML(c.Text)
				if !looksLikeQuestion(txt) {
					continue
				}
				out = append(out, faqRow{
					Question:  truncate(txt, 200),
					LikeCount: c.LikeCount,
					VideoID:   c.VideoID,
					Author:    c.Author,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].LikeCount > out[j].LikeCount })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max questions")
	return cmd
}

// ---- theme-mine: recurring n-grams from comments ----

type themeRow struct {
	Phrase string `json:"phrase"`
	Count  int    `json:"count"`
}

func newThemeMineCmd(flags *rootFlags) *cobra.Command {
	var dbPath, lang string
	var n, limit int
	cmd := &cobra.Command{
		Use:   "theme-mine",
		Short: "Extract most-repeated bigrams/trigrams from cached comments (FAQ surface area)",
		Example: `  youtube-creator-analytics-pp-cli theme-mine --n 2 --limit 30 --json
  youtube-creator-analytics-pp-cli theme-mine --lang en --limit 30 --json
  youtube-creator-analytics-pp-cli theme-mine --lang es --n 3 --json`,
		Long: `Extract most-repeated bigrams/trigrams from cached comments.

--lang controls the stoplist:
  both (default) — bilingual ES+EN, original behavior
  en             — English-only stoplist; short Spanish words survive
  es             — Spanish-only stoplist; short English words survive

Statistical floor: with fewer than 30 videos worth of comments the n-gram
counts are noisy and ranking should be treated as directional.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if lang != "both" && lang != "en" && lang != "es" {
				return fmt.Errorf("--lang must be 'both', 'en', or 'es', got %q", lang)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("youtube-creator-analytics-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			comments, err := loadComments(db, 0)
			if err != nil {
				return err
			}
			stop := stopwordsFor(lang)
			counts := map[string]int{}
			for _, c := range comments {
				words := tokenizeWithStop(stripHTML(c.Text), stop)
				if len(words) < n {
					continue
				}
				for i := 0; i+n <= len(words); i++ {
					phrase := strings.Join(words[i:i+n], " ")
					if !isMeaningfulPhrase(phrase) {
						continue
					}
					counts[phrase]++
				}
			}
			out := make([]themeRow, 0, len(counts))
			for p, c := range counts {
				if c < 2 {
					continue
				}
				out = append(out, themeRow{Phrase: p, Count: c})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&lang, "lang", "both", "Stoplist language: 'both' (ES+EN, default), 'en', or 'es'")
	cmd.Flags().IntVar(&n, "n", 2, "N-gram length (2 = bigram, 3 = trigram)")
	cmd.Flags().IntVar(&limit, "limit", 40, "Max phrases")
	return cmd
}

// ---- text utils ----

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)
var wordRe = regexp.MustCompile(`[\p{L}\p{N}']+`)
var englishStop = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "you": {}, "are": {}, "this": {}, "that": {}, "with": {},
	"from": {}, "your": {}, "have": {}, "but": {}, "not": {}, "all": {}, "what": {}, "out": {},
	"about": {}, "just": {},
}

var spanishStop = map[string]struct{}{
	"como": {}, "para": {}, "que": {}, "los": {}, "las": {}, "del": {},
	"con": {}, "por": {}, "una": {}, "uno": {}, "este": {}, "esta": {}, "esto": {}, "mas": {},
	"más": {}, "muy": {}, "todo": {}, "todos": {}, "todas": {}, "soy": {}, "ser": {}, "fue": {},
	"era": {}, "han": {}, "hay": {}, "sin": {}, "sus": {},
}

// stopwordsFor returns the active stoplist for the requested language.
// "both" (default) preserves the original bilingual behavior; "en" or "es"
// scope the stoplist for single-language channels so culturally meaningful
// short words in the *other* language survive into the n-gram counts.
func stopwordsFor(lang string) map[string]struct{} {
	switch lang {
	case "en":
		return englishStop
	case "es":
		return spanishStop
	default:
		merged := make(map[string]struct{}, len(englishStop)+len(spanishStop))
		for k, v := range englishStop {
			merged[k] = v
		}
		for k, v := range spanishStop {
			merged[k] = v
		}
		return merged
	}
}

func stripHTML(s string) string {
	return htmlTagRe.ReplaceAllString(s, " ")
}

func tokenize(s string) []string {
	return tokenizeWithStop(s, stopwordsFor("both"))
}

func tokenizeWithStop(s string, stop map[string]struct{}) []string {
	out := wordRe.FindAllString(strings.ToLower(s), -1)
	clean := out[:0]
	for _, w := range out {
		if len(w) <= 2 {
			continue
		}
		if _, isStop := stop[w]; isStop {
			continue
		}
		clean = append(clean, w)
	}
	return clean
}

func isMeaningfulPhrase(p string) bool {
	parts := strings.Fields(p)
	for _, w := range parts {
		if len(w) <= 2 {
			return false
		}
	}
	return true
}

var questionMark = regexp.MustCompile(`[?¿]`)

func looksLikeQuestion(s string) bool {
	if questionMark.MatchString(s) {
		return true
	}
	low := strings.ToLower(strings.TrimSpace(s))
	for _, p := range []string{"how ", "why ", "what ", "when ", "where ", "cómo ", "como ", "por qué ", "porque ", "qué ", "que ", "cuándo ", "dónde ", "donde "} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}
