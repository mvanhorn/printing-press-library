// Cookie-authenticated Position Tracking subcommands (add-keywords, rankings,
// annotate). These hit the same www.semrush.com UI host as the API-key PT
// commands in pt.go, but the endpoints below are gated by the user's Chrome
// session — not by the SEMRUSH_API_KEY query param — so they reuse the cookie
// jar imported by `auth login --chrome`.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/semrush/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/semrush/internal/config"
)

// ptCookieDo issues a request to www.semrush.com using the imported Chrome
// cookie jar. Mirrors the keyword_magic.go pattern: Origin/Referer/User-Agent
// headers that look UI-shaped keep the session-validation logic happy.
func ptCookieDo(ctx context.Context, method, path string, query url.Values, body any, timeout time.Duration, flags *rootFlags) ([]byte, error) {
	jar, jarErr := loadSemrushCookieJar()
	// The www.semrush.com PT/notes endpoints require ?api_key= even when valid
	// session cookies are present (the param gates server-side rate limiting and
	// quota accounting). Pull the key from config and inject it.
	cfg, cfgErr := config.Load(flags.configPath)
	if cfgErr != nil {
		return nil, cfgErr
	}
	apiKey := cfg.SemrushApiKey
	if apiKey == "" && jarErr != nil {
		return nil, fmt.Errorf("no auth available — set SEMRUSH_API_KEY (or run 'semrush-pp-cli auth set-token <key>'), and optionally 'auth login --chrome' for richer endpoints")
	}

	if query == nil {
		query = url.Values{}
	}
	if apiKey != "" {
		query.Set("api_key", apiKey)
	}

	u := "https://www.semrush.com" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.semrush.com")
	req.Header.Set("Referer", "https://www.semrush.com/position-tracking/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	if jar != nil {
		jar.applyCookiesToRequest(req)
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		hint := ""
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			hint = " — cookies may be expired; re-run 'semrush-pp-cli auth login --chrome'"
		}
		return nil, fmt.Errorf("%s %s returned HTTP %d%s: %s", method, path, resp.StatusCode, hint, truncatePT(respBody, 300))
	}
	return respBody, nil
}

// splitCSV splits a comma-separated flag value, trims whitespace, and drops empties.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ptParseCampaign extracts the user/project id (left of underscore) from a PT
// campaign id like "29008758_4495670". Returns the project id as int.
func ptParseProjectFromCampaign(campaignID string) (int, error) {
	parts := strings.SplitN(campaignID, "_", 2)
	if len(parts) != 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid PT campaign id %q — expected '<project>_<campaign>' form (get one from 'semrush-pp-cli pt campaigns')", campaignID)
	}
	projectID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid PT campaign id %q — project portion %q is not an integer", campaignID, parts[0])
	}
	return projectID, nil
}

// --- pt add-keywords ---

type ptAddKeywordsItem struct {
	Keyword string   `json:"keyword"`
	Tags    []string `json:"tags,omitempty"`
}

type ptAddKeywordsBody struct {
	Create []ptAddKeywordsItem `json:"create"`
}

func newPTAddKeywordsCmd(flags *rootFlags) *cobra.Command {
	var keywordsFlag, tagsFlag string
	cmd := &cobra.Command{
		Use:   "add-keywords <campaign-id>",
		Short: "Add keywords (with optional tags) to a Position Tracking campaign",
		Long: "Adds one or more keywords to a Position Tracking campaign, optionally " +
			"applying tags to all of them. Tags can be arbitrary strings — Semrush " +
			"groups keywords by tag in the PT UI. Cookie-authenticated; requires " +
			"'semrush-pp-cli auth login --chrome' first.",
		Example: strings.Trim(`
  # Add 2 keywords with no tags
  semrush-pp-cli pt add-keywords 29008758_4495670 --keywords "ethical investing,green super fund"

  # Add keywords and tag them
  semrush-pp-cli pt add-keywords 29008758_4495670 \
    --keywords "tax deductible donations australia,donate to charity eofy" \
    --tags "#articles,article - tips for donating to charity this eofy"

  # See the request without sending it
  semrush-pp-cli pt add-keywords 29008758_4495670 --keywords "test" --dry-run
`, "\n"),
		Annotations: map[string]string{
			// Mutating endpoint — no read-only annotation, so MCP tooling will
			// surface it as a write tool requiring per-call permission.
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				// Show the planned mutation but never send it.
				keywords := splitCSV(keywordsFlag)
				tags := splitCSV(tagsFlag)
				items := make([]ptAddKeywordsItem, 0, len(keywords))
				for _, kw := range keywords {
					items = append(items, ptAddKeywordsItem{Keyword: kw, Tags: tags})
				}
				body := ptAddKeywordsBody{Create: items}
				if cliutil.IsVerifyEnv() {
					fmt.Fprintln(cmd.OutOrStdout(), "would POST: /tracking/web-api/management/"+args[0]+"/update_keywords_state")
					return nil
				}
				raw, _ := json.MarshalIndent(body, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), "POST /tracking/web-api/management/"+args[0]+"/update_keywords_state")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - no request sent)")
				return nil
			}

			campaignID := args[0]
			keywords := splitCSV(keywordsFlag)
			if len(keywords) == 0 {
				return fmt.Errorf("at least one keyword is required — pass --keywords \"kw1,kw2\"")
			}
			tags := splitCSV(tagsFlag)

			items := make([]ptAddKeywordsItem, 0, len(keywords))
			for _, kw := range keywords {
				items = append(items, ptAddKeywordsItem{Keyword: kw, Tags: tags})
			}
			body := ptAddKeywordsBody{Create: items}

			path := "/tracking/web-api/management/" + campaignID + "/update_keywords_state"
			data, err := ptCookieDo(cmd.Context(), "POST", path, nil, body, flags.timeout, flags)
			if err != nil {
				return err
			}
			return ptPrint(cmd, flags, data)
		},
	}
	cmd.Flags().StringVar(&keywordsFlag, "keywords", "", "Comma-separated keywords to add (required)")
	cmd.Flags().StringVar(&tagsFlag, "tags", "", "Comma-separated tags applied to ALL added keywords")
	return cmd
}

// --- pt rankings ---

func newPTRankingsCmd(flags *rootFlags) *cobra.Command {
	var dateBegin, dateEnd, sortField, sortOrder, domain, competitors string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "rankings <campaign-id>",
		Short: "Current organic rankings overview for a PT campaign (the 'Overview' report)",
		Long: "Returns the current ranking snapshot for all tracked keywords in a " +
			"campaign. This is the data behind the PT UI's Overview tab — position, " +
			"position diff, volume, SERP features, traffic estimate, etc. Different " +
			"from 'pt report', which returns the time-series rank history.",
		Example: strings.Trim(`
  # Today's rankings for a tracked domain
  semrush-pp-cli pt rankings 29008758_4495670 --domain client.com

  # JSON for agent consumption, narrowed to the high-gravity fields
  semrush-pp-cli pt rankings 29008758_4495670 --domain client.com --agent \
    --select data.keywords.keyword,data.keywords.position,data.keywords.volume

  # Multi-competitor or non-wildcard patterns
  semrush-pp-cli pt rankings 29008758_4495670 \
    --competitors "*.client.com/*,*.competitor1.com/*,*.competitor2.com/*"

  # Page 2 of results, sorted by volume desc
  semrush-pp-cli pt rankings 29008758_4495670 --domain client.com \
    --sort-field volume --sort-order desc --offset 100
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			campaignID := args[0]
			today := time.Now().UTC().Format("20060102")
			if dateEnd == "" {
				dateEnd = today
			}
			if dateBegin == "" {
				dateBegin = today
			}
			q := url.Values{}
			q.Set("type", "organic")
			q.Set("campaign_id", campaignID)
			q.Set("volume_level", "national")
			q.Set("date_begin", dateBegin)
			q.Set("date_end", dateEnd)
			q.Set("limit", strconv.Itoa(limit))
			q.Set("offset", strconv.Itoa(offset))
			q.Set("sort[field]", sortField)
			q.Set("sort[order]", sortOrder)
			q.Set("sort[date]", "end")
			q.Set("sort[competitor]", "0")

			// Server requires at least one competitors[] entry — typically the
			// tracked domain itself as "*.<domain>/*". --domain is the friendly
			// shortcut; --competitors lets power users override entirely.
			var compList []string
			if competitors != "" {
				compList = splitCSV(competitors)
			} else if domain != "" {
				compList = []string{"*." + domain + "/*"}
			} else {
				return fmt.Errorf("--domain or --competitors is required (e.g. --domain client.com — get the tracked domain from 'semrush-pp-cli pt campaigns')")
			}
			q["competitors[]"] = compList

			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "GET /tracking/web-api/reports/overview?"+q.Encode())
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - no request sent)")
				return nil
			}

			data, err := ptCookieDo(cmd.Context(), "GET", "/tracking/web-api/reports/overview", q, nil, flags.timeout, flags)
			if err != nil {
				return err
			}
			return ptPrint(cmd, flags, data)
		},
	}
	cmd.Flags().StringVar(&dateBegin, "date-begin", "", "Start date YYYYMMDD (default: today)")
	cmd.Flags().StringVar(&dateEnd, "date-end", "", "End date YYYYMMDD (default: today)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max keywords to return (max 99999)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVar(&sortField, "sort-field", "position", "Sort field: position, volume, visibility, traffic, diff")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "asc", "Sort order: asc or desc")
	cmd.Flags().StringVar(&domain, "domain", "", "Tracked domain (e.g. client.com) — expands to '*.client.com/*'")
	cmd.Flags().StringVar(&competitors, "competitors", "", "Comma-separated competitor URL patterns, overrides --domain")
	return cmd
}

// --- pt annotate ---

type ptAnnotationBody struct {
	Datetime  string   `json:"datetime"`
	Category  string   `json:"category"`
	Databases []string `json:"databases"`
	Project   int      `json:"project"`
	Note      string   `json:"note"`
	Title     string   `json:"title"`
	Tools     []string `json:"tools"`
	Labels    []string `json:"labels,omitempty"`
}

func newPTAnnotateCmd(flags *rootFlags) *cobra.Command {
	var title, note, date string
	var projectWide bool
	cmd := &cobra.Command{
		Use:     "annotate <campaign-id>",
		Aliases: []string{"annotation", "note"},
		Short:   "Add an annotation (note) to a Position Tracking campaign",
		Long: "Creates a USER_NOTE annotation in Semrush's Notes service, scoped to " +
			"the campaign by default (campaign-scoped notes appear on that campaign's " +
			"PT charts only). Use --project-wide to attach it to the whole project " +
			"instead. Cookie-authenticated; requires 'semrush-pp-cli auth login --chrome' " +
			"first.",
		Example: strings.Trim(`
  # Quick note on today's date
  semrush-pp-cli pt annotate 29008758_4495670 \
    --title "Algorithm update" --note "Google core update rolled out"

  # Backdated note on a specific date
  semrush-pp-cli pt annotate 29008758_4495670 \
    --title "Published 3 new articles" \
    --note "EOFY donation guide cluster went live" \
    --date 2026-06-08

  # Project-wide note (shows on every PT campaign in the project)
  semrush-pp-cli pt annotate 29008758_4495670 \
    --title "Site migration" --note "Switched to new CDN" --project-wide
`, "\n"),
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			campaignID := args[0]
			projectID, err := ptParseProjectFromCampaign(campaignID)
			if err != nil {
				return err
			}

			// Date defaults to today (UTC midnight). Accept YYYY-MM-DD.
			when := time.Now().UTC().Truncate(24 * time.Hour)
			if date != "" {
				parsed, err := time.Parse("2006-01-02", date)
				if err != nil {
					return fmt.Errorf("invalid --date %q (expected YYYY-MM-DD): %w", date, err)
				}
				when = parsed.UTC()
			}

			body := ptAnnotationBody{
				Datetime:  when.Format("2006-01-02T15:04:05.000Z"),
				Category:  "USER_NOTE",
				Databases: []string{},
				Project:   projectID,
				Note:      note,
				Title:     title,
				Tools:     []string{"POSITION_TRACKING"},
			}
			if !projectWide {
				body.Labels = []string{"pt_campaign_id_" + campaignID}
			}

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				if cliutil.IsVerifyEnv() {
					fmt.Fprintln(cmd.OutOrStdout(), "would POST: /notes/api/notes/v2/notes/")
					return nil
				}
				raw, _ := json.MarshalIndent(body, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), "POST /notes/api/notes/v2/notes/")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - no request sent)")
				return nil
			}

			if title == "" {
				return fmt.Errorf("--title is required")
			}
			if note == "" {
				return fmt.Errorf("--note is required")
			}

			data, err := ptCookieDo(cmd.Context(), "POST", "/notes/api/notes/v2/notes/", nil, body, flags.timeout, flags)
			if err != nil {
				return err
			}
			return ptPrint(cmd, flags, data)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Annotation title (required)")
	cmd.Flags().StringVar(&note, "note", "", "Annotation body text (required; supports multi-line)")
	cmd.Flags().StringVar(&date, "date", "", "Annotation date YYYY-MM-DD (default: today UTC)")
	cmd.Flags().BoolVar(&projectWide, "project-wide", false, "Attach to whole project rather than just this campaign")
	return cmd
}
