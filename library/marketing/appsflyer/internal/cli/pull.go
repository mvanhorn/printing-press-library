package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/channels"
	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/sources"

	"github.com/spf13/cobra"
)

// PullArgs captures the user-facing flags for `pull`. Kept as a struct so the
// underlying routing logic is testable without a Cobra context.
type pullArgs struct {
	appID        string
	from         string
	to           string
	source       string
	channelGroup string
	campaign     string
	breakdown    string
	metrics      string
	currency     string
	timezone     string
	maxRows      int
}

// pullRow is the unified row shape returned by `pull`. AppsFlyer's Pull V2
// endpoints all return CSV with column overlap on (date, media_source,
// campaign, country, installs, clicks, impressions, cost, revenue, roas);
// pullRow is a flat projection of those columns.
type pullRow struct {
	AppID       string  `json:"app_id"`
	Date        string  `json:"date,omitempty"`
	MediaSource string  `json:"media_source,omitempty"`
	Campaign    string  `json:"campaign,omitempty"`
	CampaignID  string  `json:"campaign_id,omitempty"`
	Country     string  `json:"country,omitempty"`
	Installs    int     `json:"installs"`
	Clicks      int     `json:"clicks"`
	Impressions int     `json:"impressions"`
	Cost        float64 `json:"cost"`
	Revenue     float64 `json:"revenue"`
	ROAS        float64 `json:"roas"`
	Group       string  `json:"channel_group,omitempty"`
}

type pullResponse struct {
	Endpoint string    `json:"endpoint"`
	AppID    string    `json:"app_id"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	Rows     []pullRow `json:"rows"`
	RowCount int       `json:"row_count"`
	Budget   int       `json:"budget_remaining,omitempty"`
	Source   string    `json:"resolved_source,omitempty"`
	Group    string    `json:"resolved_channel_group,omitempty"`
}

func newPullCmd(flags *rootFlags) *cobra.Command {
	a := &pullArgs{}
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Ad-hoc pull facade: dates, source/channel-group, campaign, breakdown, metrics — one command",
		Long: `pull is a high-level facade over the AppsFlyer Pull V2 endpoints.

Given a date range and an optional source (canonical or friendly), channel
group (social, programmatic, OEM, rewarded), or campaign, the CLI routes to
the right underlying endpoint:

  --breakdown media_source         → agg/partners_report
  --breakdown date,media_source    → agg/partners_by_date_report
  --breakdown country              → agg/geo_report
  --breakdown date,country         → agg/geo_by_date_report
  --breakdown date (no source)     → agg/daily_report

Channel groups are resolved from ~/.config/appsflyer-pp-cli/channels.yaml
(falling back to the built-in mapping). Source names are resolved to
canonical _int IDs ('facebook' → 'facebook_int') and applied as a row
filter on the CSV response.`,
		Example: strings.Trim(`
  appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --json
  appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --source facebook --breakdown campaign --json
  appsflyer-pp-cli pull --app-id id123456 --from 2026-05-04 --to 2026-05-10 --channel-group social --breakdown media_source --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.appID == "" || a.from == "" || a.to == "" {
				if !dryRunOK(flags) {
					return fmt.Errorf("pull requires --app-id, --from, --to (run 'appsflyer-pp-cli pull --help' for examples)")
				}
			}
			if dryRunOK(flags) && (a.appID == "" || a.from == "" || a.to == "") {
				return nil
			}
			return runPull(cmd, flags, a)
		},
	}
	cmd.Flags().StringVar(&a.appID, "app-id", "", "AppsFlyer app id (required)")
	cmd.Flags().StringVar(&a.from, "from", "", "Start date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&a.to, "to", "", "End date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&a.source, "source", "", "Filter to one media source (canonical _int or friendly name, e.g. facebook)")
	cmd.Flags().StringVar(&a.channelGroup, "channel-group", "", "Filter to a channel group (social, programmatic, oem, rewarded, video); use 'sources groups' to list")
	cmd.Flags().StringVar(&a.campaign, "campaign", "", "Filter to one campaign name")
	cmd.Flags().StringVar(&a.breakdown, "breakdown", "media_source", "Breakdown dimension(s): media_source, campaign, date, country, or comma-separated combinations")
	cmd.Flags().StringVar(&a.metrics, "metrics", "", "Reserved for future use; currently the CLI returns the standard column set")
	cmd.Flags().StringVar(&a.currency, "currency", "", "Override account currency (USD, EUR, ...)")
	cmd.Flags().StringVar(&a.timezone, "timezone", "", "Override account timezone (UTC, America/New_York, ...)")
	cmd.Flags().IntVar(&a.maxRows, "max-rows", 0, "Maximum rows to return after filtering (0 = no cap)")
	return cmd
}

// runPull is the routing core. Decides the underlying endpoint from
// --breakdown, resolves --source / --channel-group, executes one HTTP call,
// then filters the response rows in-memory.
func runPull(cmd *cobra.Command, flags *rootFlags, a *pullArgs) error {
	if err := validatePullDates(a.from, a.to); err != nil {
		return err
	}

	canonicalFilter := ""
	if a.source != "" {
		c, ok := sources.Resolve(a.source)
		if !ok {
			return fmt.Errorf("unknown source %q (try 'appsflyer-pp-cli sources list --query %s')", a.source, a.source)
		}
		canonicalFilter = c
	}

	var groupMembers []string
	if a.channelGroup != "" {
		grp, err := channels.Load()
		if err != nil {
			return err
		}
		members, err := grp.Resolve(a.channelGroup)
		if err != nil {
			return err
		}
		groupMembers = members
	}

	endpoint := chooseAggEndpoint(a.breakdown)

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/api/agg-data/export/app/%s/%s/v5", a.appID, endpoint)
	params := map[string]string{
		"from": a.from,
		"to":   a.to,
	}
	if a.timezone != "" {
		params["timezone"] = a.timezone
	}
	if a.currency != "" {
		params["currency"] = a.currency
	}

	raw, err := c.Get(path, params)
	if err != nil {
		return err
	}
	if c.DryRun {
		// dry-run returns a sentinel JSON; surface it transparently
		_, werr := cmd.OutOrStdout().Write(append([]byte(raw), '\n'))
		return werr
	}

	rows, err := parsePullResponse(raw)
	if err != nil {
		return err
	}
	rows = filterPullRows(rows, canonicalFilter, groupMembers, a.campaign)
	if a.maxRows > 0 && len(rows) > a.maxRows {
		rows = rows[:a.maxRows]
	}
	for i := range rows {
		if rows[i].AppID == "" {
			rows[i].AppID = a.appID
		}
		if len(groupMembers) > 0 {
			rows[i].Group = a.channelGroup
		}
	}

	resp := pullResponse{
		Endpoint: endpoint,
		AppID:    a.appID,
		From:     a.from,
		To:       a.to,
		Rows:     rows,
		RowCount: len(rows),
		Source:   canonicalFilter,
		Group:    a.channelGroup,
	}
	if t := c.Budget(); t != nil {
		resp.Budget = t.Remaining()
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
	}
	tableRows := make([][]string, 0, len(rows))
	for _, r := range rows {
		tableRows = append(tableRows, []string{
			truncStr(r.Date, 10),
			truncStr(r.MediaSource, 24),
			truncStr(r.Campaign, 32),
			truncStr(r.Country, 8),
			itoa(r.Installs),
			ftoa(r.Cost, 2),
			ftoa(r.Revenue, 2),
			ftoa(r.ROAS*100, 1) + "%",
		})
	}
	return flags.printTable(cmd, []string{"DATE", "MEDIA_SOURCE", "CAMPAIGN", "GEO", "INSTALLS", "COST", "REVENUE", "ROAS"}, tableRows)
}

func validatePullDates(from, to string) error {
	const layout = "2006-01-02"
	f, err := time.Parse(layout, from)
	if err != nil {
		return fmt.Errorf("invalid --from %q: expected YYYY-MM-DD", from)
	}
	t, err := time.Parse(layout, to)
	if err != nil {
		return fmt.Errorf("invalid --to %q: expected YYYY-MM-DD", to)
	}
	if t.Before(f) {
		return fmt.Errorf("--to (%s) must not be earlier than --from (%s)", to, from)
	}
	return nil
}

// chooseAggEndpoint maps --breakdown to one of the five Aggregate Pull V2
// report names. Multiple dimensions in --breakdown route to the matching
// *_by_date variant when "date" is present alongside another dimension.
func chooseAggEndpoint(breakdown string) string {
	dims := splitDims(breakdown)
	hasDate := false
	hasGeo := false
	hasSource := false
	for _, d := range dims {
		switch d {
		case "date":
			hasDate = true
		case "country", "geo":
			hasGeo = true
		case "media_source", "campaign":
			hasSource = true
		}
	}
	switch {
	case hasDate && hasGeo:
		return "geo_by_date_report"
	case hasDate && hasSource:
		return "partners_by_date_report"
	case hasGeo:
		return "geo_report"
	case hasDate:
		return "daily_report"
	default:
		return "partners_report"
	}
}

func splitDims(s string) []string {
	out := strings.Split(strings.ToLower(strings.TrimSpace(s)), ",")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

// parsePullResponse handles both the JSON shape AppsFlyer returns from some
// endpoints and the CSV body returned by the agg-data reports. Detection is
// content-based, not header-based, because the generated client strips the
// raw HTTP response.
func parsePullResponse(raw json.RawMessage) ([]pullRow, error) {
	body := strings.TrimSpace(string(raw))
	if strings.HasPrefix(body, "[") {
		var rows []pullRow
		if err := json.Unmarshal([]byte(body), &rows); err != nil {
			return nil, fmt.Errorf("parsing JSON response: %w", err)
		}
		return rows, nil
	}
	// CSV path. AppsFlyer agg-data CSVs come back as a quoted JSON string
	// when the client wraps an unknown-content-type response. Unwrap one
	// level of quoting when present, then parse as CSV.
	if strings.HasPrefix(body, "\"") && strings.HasSuffix(body, "\"") {
		var unquoted string
		if err := json.Unmarshal(raw, &unquoted); err == nil {
			body = unquoted
		}
	}
	return parseAggCSV(body)
}

// parseAggCSV interprets the Aggregate Pull V2 CSV columns. The header rows
// AppsFlyer returns vary slightly per report family but always include some
// subset of (Date, Media Source, Campaign, Country, Installs, Clicks,
// Impressions, Total Cost, Total Revenue, ROAS); we project them by column
// name (case-insensitive) and treat missing columns as zero/empty.
func parseAggCSV(body string) ([]pullRow, error) {
	r := csv.NewReader(strings.NewReader(body))
	r.FieldsPerRecord = -1 // tolerate ragged rows
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("parsing CSV header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	cell := func(rec []string, names ...string) string {
		for _, n := range names {
			if i, ok := idx[n]; ok && i < len(rec) {
				return rec[i]
			}
		}
		return ""
	}
	var out []pullRow
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, fmt.Errorf("parsing CSV row: %w", err)
		}
		row := pullRow{
			Date:        cell(rec, "date"),
			MediaSource: cell(rec, "media source", "media_source"),
			Campaign:    cell(rec, "campaign"),
			CampaignID:  cell(rec, "campaign id", "campaign_id"),
			Country:     cell(rec, "country"),
			Installs:    parseInt(cell(rec, "installs")),
			Clicks:      parseInt(cell(rec, "clicks")),
			Impressions: parseInt(cell(rec, "impressions")),
			Cost:        parseFloat(cell(rec, "total cost", "cost")),
			Revenue:     parseFloat(cell(rec, "total revenue", "revenue")),
			ROAS:        parseFloat(cell(rec, "roas")),
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

func filterPullRows(rows []pullRow, source string, group []string, campaign string) []pullRow {
	if source == "" && len(group) == 0 && campaign == "" {
		return rows
	}
	groupSet := make(map[string]struct{}, len(group))
	for _, g := range group {
		groupSet[strings.ToLower(g)] = struct{}{}
	}
	out := make([]pullRow, 0, len(rows))
	for _, r := range rows {
		ms := strings.ToLower(r.MediaSource)
		// match canonical or display name on best-effort substring
		if source != "" {
			if !sourceMatches(ms, source) {
				continue
			}
		}
		if len(groupSet) > 0 {
			matched := false
			for canonical := range groupSet {
				if sourceMatches(ms, canonical) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if campaign != "" && !strings.Contains(strings.ToLower(r.Campaign), strings.ToLower(campaign)) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// sourceMatches checks whether a CSV-reported media-source string corresponds
// to a canonical _int ID. AppsFlyer Pull V2 CSVs report the human-readable
// display name (e.g. "Facebook Ads") in the "Media Source" column, so we
// match against the display name + the canonical prefix.
func sourceMatches(observedLower, canonical string) bool {
	if observedLower == canonical {
		return true
	}
	prefix := strings.TrimSuffix(canonical, "_int")
	if strings.Contains(observedLower, prefix) {
		return true
	}
	// Look up display name for the canonical and check substring
	for _, s := range sources.Catalog() {
		if s.Canonical != canonical {
			continue
		}
		if strings.Contains(observedLower, strings.ToLower(s.Display)) {
			return true
		}
		for _, alias := range s.Aliases {
			if strings.Contains(observedLower, alias) {
				return true
			}
		}
	}
	return false
}
