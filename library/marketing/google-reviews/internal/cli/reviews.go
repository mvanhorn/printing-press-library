// Copyright 2026 dahlia. Licensed under Apache-2.0. See LICENSE.
// Google Reviews commands: reviews, summary, search

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Review holds a single parsed Google review.
type Review struct {
	ReviewID    string `json:"review_id"`
	Rating      int    `json:"rating"`
	Author      string `json:"author"`
	AuthorID    string `json:"author_id"`
	Date        string `json:"date"`
	TimestampMs int64  `json:"timestamp_ms"`
	Text        string `json:"text"`
	Language    string `json:"language"`
	ReviewURL   string `json:"review_url"`
}

// RatingSummary holds the overall rating and distribution for a place.
type RatingSummary struct {
	Stars5 int `json:"stars_5"`
	Stars4 int `json:"stars_4"`
	Stars3 int `json:"stars_3"`
	Stars2 int `json:"stars_2"`
	Stars1 int `json:"stars_1"`
	Total  int `json:"total"`
}

var cidRe = regexp.MustCompile(`0x([0-9a-fA-F]+):0x([0-9a-fA-F]+)`)

// parseCID extracts lo/hi uint64 from a Google Maps URL or raw CID string.
// Accepts:
//   - "0xHEX:0xHEX"
//   - Full Google Maps URL containing the CID in data=!...!1s0x...
func parseCID(input string) (lo, hi uint64, err error) {
	m := cidRe.FindStringSubmatch(input)
	if m == nil {
		return 0, 0, fmt.Errorf("no CID found in %q (expected 0xHEX:0xHEX)", input)
	}
	lo, err = strconv.ParseUint(m[1], 16, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid CID lo hex %q: %w", m[1], err)
	}
	hi, err = strconv.ParseUint(m[2], 16, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid CID hi hex %q: %w", m[2], err)
	}
	return lo, hi, nil
}

// sortCode maps --sort flag to Google's internal sort code.
func sortCode(sort string) int {
	switch strings.ToLower(sort) {
	case "newest":
		return 2
	case "highest":
		return 3
	case "lowest":
		return 4
	default:
		return 1 // relevant
	}
}

// buildPB constructs the pb= parameter for listentitiesreviews.
func buildPB(lo, hi uint64, count, offset, sort int) string {
	return fmt.Sprintf("!1m2!1y%d!2y%d!2m2!1i%d!2i%d!3e%d!5m2!1sGOOGLE_REVIEWS_CLI!7e81", lo, hi, count, offset, sort)
}

// extractChromeCookies tries to get NID and __Secure-STRP from Chrome via agent-browser.
// Returns an empty string on failure (caller proceeds without cookies).
func extractChromeCookies() string {
	// Check env override first
	if nid := os.Getenv("GOOGLE_NID"); nid != "" {
		return "NID=" + nid
	}

	// Use agent-browser cookies get (reads HttpOnly cookies that JS can't access)
	out, err := exec.Command("agent-browser", "cookies", "get", "--domain", ".google.com").Output()
	if err != nil || len(out) < 5 {
		return ""
	}

	var nid, strp string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NID=") {
			nid = line
		} else if strings.HasPrefix(line, "__Secure-STRP=") {
			strp = line
		}
	}

	if nid == "" {
		return ""
	}
	if strp != "" {
		return nid + "; " + strp
	}
	return nid
}

// fetchReviews calls the listentitiesreviews endpoint and returns raw JSON response body.
func fetchReviews(ctx context.Context, lo, hi uint64, count, offset, sc int, lang, country, cookieOverride string, timeout time.Duration) ([]byte, error) {
	pb := buildPB(lo, hi, count, offset, sc)
	apiURL := fmt.Sprintf(
		"https://www.google.com/maps/preview/review/listentitiesreviews?authuser=0&hl=%s&gl=%s&pb=%s",
		url.QueryEscape(lang), url.QueryEscape(country), url.QueryEscape(pb),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.google.com/maps/")
	req.Header.Set("Accept", "*/*")
	if cookieOverride != "" {
		req.Header.Set("Cookie", cookieOverride)
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		preview := body
		if len(preview) > 200 {
			preview = body[:200]
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(preview))
	}
	return body, nil
}

// stripGooglePrefix strips the )]}'\n anti-XSSI prefix.
func stripGooglePrefix(b []byte) []byte {
	s := string(b)
	if strings.HasPrefix(s, ")]}'") {
		s = strings.TrimPrefix(s, ")]}'")
		s = strings.TrimPrefix(s, "\n")
		return []byte(s)
	}
	return b
}

// parseReviewsResponse parses the Google Reviews response array.
func parseReviewsResponse(body []byte) ([]Review, *RatingSummary, error) {
	clean := stripGooglePrefix(body)

	var data []json.RawMessage
	if err := json.Unmarshal(clean, &data); err != nil {
		return nil, nil, fmt.Errorf("parse response: %w", err)
	}

	var reviews []Review
	if len(data) > 2 && data[2] != nil {
		var rawReviews [][]json.RawMessage
		if err := json.Unmarshal(data[2], &rawReviews); err != nil {
			return nil, nil, fmt.Errorf("parse reviews array: %w", err)
		}
		for _, r := range rawReviews {
			review := Review{}
			if len(r) > 0 && r[0] != nil {
				var authorInfo []json.RawMessage
				if json.Unmarshal(r[0], &authorInfo) == nil && len(authorInfo) > 1 {
					json.Unmarshal(authorInfo[1], &review.Author)
				}
			}
			if len(r) > 1 && r[1] != nil {
				json.Unmarshal(r[1], &review.Date)
			}
			if len(r) > 3 && r[3] != nil {
				json.Unmarshal(r[3], &review.Text)
			}
			if len(r) > 4 && r[4] != nil {
				json.Unmarshal(r[4], &review.Rating)
			}
			if len(r) > 6 && r[6] != nil {
				json.Unmarshal(r[6], &review.AuthorID)
			}
			if len(r) > 10 && r[10] != nil {
				json.Unmarshal(r[10], &review.ReviewID)
			}
			if len(r) > 18 && r[18] != nil {
				json.Unmarshal(r[18], &review.ReviewURL)
			}
			if len(r) > 27 && r[27] != nil {
				json.Unmarshal(r[27], &review.TimestampMs)
			}
			if len(r) > 32 && r[32] != nil {
				json.Unmarshal(r[32], &review.Language)
			}
			reviews = append(reviews, review)
		}
	}

	var summary *RatingSummary
	if len(data) > 5 && data[5] != nil {
		var dist []int
		if json.Unmarshal(data[5], &dist) == nil && len(dist) == 5 {
			summary = &RatingSummary{
				Stars5: dist[4],
				Stars4: dist[3],
				Stars3: dist[2],
				Stars2: dist[1],
				Stars1: dist[0],
				Total:  dist[0] + dist[1] + dist[2] + dist[3] + dist[4],
			}
		}
	}

	return reviews, summary, nil
}

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-3]) + "..."
}

func newReviewsCmd(flags *rootFlags) *cobra.Command {
	var flagSort string
	var flagCount int
	var flagAll bool
	var flagLang string
	var flagCountry string
	var flagOffset int

	cmd := &cobra.Command{
		Use:   "reviews <maps-url-or-cid>",
		Short: "Fetch reviews for a Google Maps business (no API key required)",
		Long: `Fetch Google Reviews for a business identified by its Google Maps URL or CID.

The CID can be found in a Google Maps URL like:
  https://www.google.com/maps/place/.../@.../data=!3m5!1s0xHEXLO:0xHEXHI!...

Examples:
  # By Google Maps URL
  google-reviews-pp-cli reviews "https://www.google.com/maps/place/Shake+Shack/...data=!3m5!1s0x89c258bc949d58cf:0x84ac8a2dc2535dc2"

  # By CID directly
  google-reviews-pp-cli reviews "0x89c258bc949d58cf:0x84ac8a2dc2535dc2"

  # Newest reviews, JSON output
  google-reviews-pp-cli reviews --sort newest --json "0x89c258bc949d58cf:0x84ac8a2dc2535dc2"

  # Fetch all reviews
  google-reviews-pp-cli reviews --all "0x89c258bc949d58cf:0x84ac8a2dc2535dc2"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flagDryRun(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.google.com/maps/preview/review/listentitiesreviews?authuser=0&hl=%s&gl=%s&pb=<cid-required>\n", flagLang, flagCountry)
					return nil
				}
				return cmd.Help()
			}

			sc := sortCode(flagSort)
			if flagCount <= 0 || flagCount > 20 {
				flagCount = 20
			}

			// Emit a dry-run URL even when the CID is not a valid hex pair.
			if flagDryRun(flags) {
				lo, hi, err := parseCID(args[0])
				if err != nil {
					// Show a placeholder URL so verify can confirm dry-run mode.
					fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.google.com/maps/preview/review/listentitiesreviews?authuser=0&hl=%s&gl=%s&pb=<cid=%s>\n", flagLang, flagCountry, url.QueryEscape(args[0]))
					return nil
				}
				pb := buildPB(lo, hi, flagCount, flagOffset, sc)
				fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.google.com/maps/preview/review/listentitiesreviews?authuser=0&hl=%s&gl=%s&pb=%s\n", flagLang, flagCountry, url.QueryEscape(pb))
				return nil
			}

			lo, hi, err := parseCID(args[0])
			if err != nil {
				return fmt.Errorf("invalid input: %w\nProvide a Google Maps URL or CID (0xHEX:0xHEX)", err)
			}

			cookies := extractChromeCookies()
			allReviews := make([]Review, 0)
			offset := flagOffset

			for {
				body, err := fetchReviews(cmd.Context(), lo, hi, flagCount, offset, sc, flagLang, flagCountry, cookies, flags.timeout)
				if err != nil {
					if flagAll && len(allReviews) > 0 {
						// Google's API uses cursor-based pagination internally;
						// offset > 0 may return 500 — stop with what we have.
						fmt.Fprintf(os.Stderr, "warning: stopped paginating after %d reviews: %v\n", len(allReviews), err)
						break
					}
					return fmt.Errorf("fetch reviews: %w", err)
				}

				batch, _, err := parseReviewsResponse(body)
				if err != nil {
					return fmt.Errorf("parse reviews: %w", err)
				}

				allReviews = append(allReviews, batch...)

				if !flagAll || len(batch) == 0 {
					break
				}
				offset += len(batch)
				// Rate limit between paginated requests
				time.Sleep(500 * time.Millisecond)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(allReviews)
			}

			// Human-readable table
			printReviewsTable(cmd.OutOrStdout(), allReviews)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagSort, "sort", "relevant", "Sort order: relevant, newest, highest, lowest")
	cmd.Flags().IntVar(&flagCount, "count", 20, "Number of reviews per request (max 20)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all available reviews (auto-paginate)")
	cmd.Flags().StringVar(&flagLang, "lang", "en", "Language code (e.g. en, fr, de)")
	cmd.Flags().StringVar(&flagCountry, "country", "us", "Country code (e.g. us, gb, de)")
	cmd.Flags().IntVar(&flagOffset, "offset", 0, "Starting offset for pagination")

	return cmd
}

func flagDryRun(flags *rootFlags) bool {
	return flags.dryRun
}

func printReviewsTable(w io.Writer, reviews []Review) {
	fmt.Fprintf(w, "%-6s  %-20s  %-16s  %s\n", "Rating", "Author", "Date", "Review")
	fmt.Fprintf(w, "%-6s  %-20s  %-16s  %s\n", "------", "--------------------", "----------------", "------")
	for _, r := range reviews {
		stars := strings.Repeat("★", r.Rating) + strings.Repeat("☆", 5-r.Rating)
		author := truncateRunes(r.Author, 20)
		date := truncateRunes(r.Date, 16)
		text := truncateRunes(r.Text, 60)
		fmt.Fprintf(w, "%-6s  %-20s  %-16s  %s\n", stars, author, date, text)
	}
	if len(reviews) > 0 {
		fmt.Fprintf(w, "\n%d reviews\n", len(reviews))
	}
}

func newSummaryCmd(flags *rootFlags) *cobra.Command {
	var flagLang string
	var flagCountry string

	cmd := &cobra.Command{
		Use:   "summary <maps-url-or-cid>",
		Short: "Show rating summary and distribution for a Google Maps business",
		Long: `Show the overall rating distribution (star counts) for a business.

Examples:
  google-reviews-pp-cli summary "0x89c258bc949d58cf:0x84ac8a2dc2535dc2"
  google-reviews-pp-cli summary --json "https://www.google.com/maps/place/.../data=!...!1s0x89c258bc949d58cf:0x84ac8a2dc2535dc2"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flagDryRun(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.google.com/maps/preview/review/listentitiesreviews?authuser=0&hl=%s&gl=%s&pb=<cid-required>\n", flagLang, flagCountry)
					return nil
				}
				return cmd.Help()
			}

			// Emit a dry-run URL even when the CID is not a valid hex pair.
			if flagDryRun(flags) {
				lo, hi, err := parseCID(args[0])
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.google.com/maps/preview/review/listentitiesreviews?authuser=0&hl=%s&gl=%s&pb=<cid=%s>\n", flagLang, flagCountry, url.QueryEscape(args[0]))
					return nil
				}
				pb := buildPB(lo, hi, 1, 0, 1)
				fmt.Fprintf(cmd.OutOrStdout(), "GET https://www.google.com/maps/preview/review/listentitiesreviews?authuser=0&hl=%s&gl=%s&pb=%s\n", flagLang, flagCountry, url.QueryEscape(pb))
				return nil
			}

			lo, hi, err := parseCID(args[0])
			if err != nil {
				return fmt.Errorf("invalid input: %w", err)
			}

			cookies := extractChromeCookies()
			body, err := fetchReviews(cmd.Context(), lo, hi, 1, 0, 1, flagLang, flagCountry, cookies, flags.timeout)
			if err != nil {
				return fmt.Errorf("fetch summary: %w", err)
			}

			_, summary, err := parseReviewsResponse(body)
			if err != nil {
				return fmt.Errorf("parse summary: %w", err)
			}
			if summary == nil {
				return fmt.Errorf("no rating summary in response")
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(summary)
			}

			printRatingSummary(cmd.OutOrStdout(), summary)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagLang, "lang", "en", "Language code")
	cmd.Flags().StringVar(&flagCountry, "country", "us", "Country code")

	return cmd
}

func printRatingSummary(w io.Writer, s *RatingSummary) {
	if s.Total == 0 {
		fmt.Fprintln(w, "No reviews")
		return
	}

	type row struct {
		stars int
		count int
	}
	rows := []row{{5, s.Stars5}, {4, s.Stars4}, {3, s.Stars3}, {2, s.Stars2}, {1, s.Stars1}}

	fmt.Fprintf(w, "Total: %s reviews\n\n", fmtNum(s.Total))
	for _, r := range rows {
		pct := float64(r.count) * 100 / float64(s.Total)
		bar := strings.Repeat("█", int(pct/5))
		fmt.Fprintf(w, "%d ★  %5.1f%%  %-20s  %s\n", r.stars, pct, bar, fmtNum(r.count))
	}
}

func fmtNum(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
