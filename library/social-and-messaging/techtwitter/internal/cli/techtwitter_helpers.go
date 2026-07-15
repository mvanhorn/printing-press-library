// Tech Twitter novel-command helpers. Hand-authored; shared by the offline
// store-backed commands (since, momentum, narrative, author-rank, digest,
// evidence) and the live-or-offline time-travel command.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/cliutil"
)

// ttUUIDRe mirrors the site's tweet-detail allowlist pattern. A non-matching id
// would be redirected to the homepage (307 -> /), so validating up front turns a
// 200-HTML leak into a clean usage error.
var ttUUIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

// ttDateRe matches YYYY-MM-DD.
var ttDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ttIsUUID reports whether s is a tweet UUID accepted by the public API.
func ttIsUUID(s string) bool { return ttUUIDRe.MatchString(s) }

// ttIsValidTravelDate reports whether s is an accepted time-travel date token.
func ttIsValidTravelDate(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "today", "yesterday", "latest":
		return true
	}
	return ttDateRe.MatchString(strings.TrimSpace(s))
}

// ttDefaultWindow is the fallback window for the time-bounded novel commands.
const ttDefaultWindow = 24 * time.Hour

// ttParseWindow parses a window string such as "24h", "48h", "7d", "30d", or
// "1w". An empty string yields ttDefaultWindow.
func ttParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ttDefaultWindow, nil
	}
	d, err := cliutil.ParseDurationLoose(s)
	if err != nil {
		return 0, fmt.Errorf("invalid window %q (try 24h, 48h, 7d, 30d): %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("window must be positive, got %q", s)
	}
	return d, nil
}

// ttCutoff returns the RFC3339 UTC timestamp for (now - window). Stored tweet
// timestamps are ISO8601, so lexical string comparison against this value is a
// valid chronological filter.
func ttCutoff(window time.Duration) string {
	return time.Now().UTC().Add(-window).Format(time.RFC3339)
}

// ttEngagement weights metrics the same way the Tech Twitter ranking does:
// bookmark*4 + comment*3 + retweet*2 + like.
func ttEngagement(like, retweet, comment, bookmark int) int {
	return bookmark*4 + comment*3 + retweet*2 + like
}

// ttResolveDB returns the provided db path or the framework default, matching
// the path `sync` writes to.
func ttResolveDB(dbPath string) string {
	if strings.TrimSpace(dbPath) != "" {
		return dbPath
	}
	return defaultDBPath("techtwitter-pp-cli")
}

// ttMissingMirror prints the standard "no local mirror" guidance and returns
// true when the SQLite file is absent. Machine callers receive an empty result
// rather than an open error.
func ttMissingMirror(cmd *cobra.Command, flags *rootFlags, dbPath, emptyJSON string) bool {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: techtwitter-pp-cli sync --db %s\n", dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), emptyJSON)
		}
		return true
	}
	return false
}

// ttTweet is the projection the offline novel commands return.
type ttTweet struct {
	ID          string  `json:"id"`
	Author      string  `json:"author_handle,omitempty"`
	AuthorName  string  `json:"author_name,omitempty"`
	Text        string  `json:"tweet_text,omitempty"`
	Summary     string  `json:"summary,omitempty"`
	URL         string  `json:"tweet_url,omitempty"`
	Likes       int     `json:"like_count"`
	Retweets    int     `json:"retweet_count"`
	Comments    int     `json:"comment_count"`
	Bookmarks   int     `json:"bookmark_count"`
	Quality     float64 `json:"quality_score"`
	Engagement  int     `json:"engagement"`
	Timestamp   string  `json:"timestamp,omitempty"`
	ContentType string  `json:"content_type,omitempty"`
}

// ttScanTweets runs a SELECT against the typed `tweets` table and returns the
// projected rows. The SELECT must return the column order used below; callers
// build only the WHERE/ORDER/LIMIT tail. COALESCE in the query keeps every scan
// target a bare type so a NULL column never drops a row.
const ttTweetSelect = `SELECT id,
	COALESCE(author_handle,''), COALESCE(author_name,''),
	COALESCE(tweet_text,''), COALESCE(summary,''), COALESCE(tweet_url,''),
	COALESCE(like_count,0), COALESCE(retweet_count,0),
	COALESCE(comment_count,0), COALESCE(bookmark_count,0),
	COALESCE(quality_score,0), COALESCE(timestamp,''), COALESCE(content_type,'')
	FROM tweets `

func ttScanTweets(db *sql.DB, tail string, args ...any) ([]ttTweet, error) {
	rows, err := db.Query(ttTweetSelect+tail, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ttTweet, 0)
	for rows.Next() {
		var t ttTweet
		if err := rows.Scan(&t.ID, &t.Author, &t.AuthorName, &t.Text, &t.Summary,
			&t.URL, &t.Likes, &t.Retweets, &t.Comments, &t.Bookmarks,
			&t.Quality, &t.Timestamp, &t.ContentType); err != nil {
			return nil, err
		}
		t.Engagement = ttEngagement(t.Likes, t.Retweets, t.Comments, t.Bookmarks)
		t.Summary = cliutil.CleanText(t.Summary)
		t.Text = cliutil.CleanText(t.Text)
		out = append(out, t)
	}
	return out, rows.Err()
}

// ttArticle is the projection digest/evidence return for long-form rows.
type ttArticle struct {
	ID        string  `json:"id"`
	Title     string  `json:"article_title,omitempty"`
	Summary   string  `json:"summary,omitempty"`
	Author    string  `json:"author_handle,omitempty"`
	URL       string  `json:"tweet_url,omitempty"`
	Quality   float64 `json:"quality_score"`
	Timestamp string  `json:"timestamp,omitempty"`
}

func ttScanArticles(db *sql.DB, tail string, args ...any) ([]ttArticle, error) {
	const sel = `SELECT id, COALESCE(article_title,''), COALESCE(summary,''),
		COALESCE(author_handle,''), COALESCE(tweet_url,''),
		COALESCE(quality_score,0), COALESCE(timestamp,'')
		FROM articles `
	rows, err := db.Query(sel+tail, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ttArticle, 0)
	for rows.Next() {
		var a ttArticle
		if err := rows.Scan(&a.ID, &a.Title, &a.Summary, &a.Author, &a.URL,
			&a.Quality, &a.Timestamp); err != nil {
			return nil, err
		}
		a.Summary = cliutil.CleanText(a.Summary)
		a.Title = cliutil.CleanText(a.Title)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ttTopic is a heatmap keyword row.
type ttTopic struct {
	Keyword    string `json:"keyword"`
	Slug       string `json:"slug,omitempty"`
	Count      int    `json:"count"`
	Engagement int    `json:"engagement"`
}

// ttRenderTweetPanel writes a Time-Travel-panel-style listing for humans.
func ttRenderTweetPanel(cmd *cobra.Command, header string, tweets []ttTweet) {
	w := cmd.OutOrStdout()
	if header != "" {
		fmt.Fprintf(w, "%s\n\n", bold(header))
	}
	if len(tweets) == 0 {
		fmt.Fprintln(w, "  (no tweets)")
		return
	}
	for _, t := range tweets {
		date := t.Timestamp
		if len(date) >= 10 {
			date = date[:10]
		}
		body := t.Text
		if body == "" {
			body = t.Summary
		}
		fmt.Fprintf(w, "  @%s  %s  ♥%d 💬%d\n    %s\n", t.Author, date, t.Likes, t.Comments, truncate(strings.ReplaceAll(body, "\n", " "), 120))
		if t.URL != "" {
			fmt.Fprintf(w, "    %s\n", t.URL)
		}
		fmt.Fprintln(w)
	}
}

// ttEmitJSON writes v through the shared filtered-JSON path so --select,
// --compact, --csv, and --quiet all work for hand-authored commands.
func ttEmitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// ttWantsJSON reports whether the command should emit machine JSON: explicit
// --json/--agent, or a non-terminal stdout without a competing format flag.
func ttWantsJSON(cmd *cobra.Command, flags *rootFlags) bool {
	if flags.asJSON || flags.agent {
		return true
	}
	return !isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain
}
