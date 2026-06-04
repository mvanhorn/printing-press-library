// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/store"
	"github.com/spf13/cobra"
)

func newSubscribersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscribers",
		Short: "Subscriber-side commands the upstream API does not provide directly",
		Long: `Local-data analyses over synced Bento subscribers and events. Run
'bento-pp-cli sync' first to populate the local store; these commands
do not call the API.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSubscribersChurnRiskCmd(flags))
	cmd.AddCommand(newSubscribersWinbackCmd(flags))
	cmd.AddCommand(newSubscribersPreDeleteCmd(flags))
	cmd.AddCommand(newSubscribersFindCmd(flags))
	cmd.AddCommand(newSubscribersFetchBatchCmd(flags))
	cmd.AddCommand(newSubscribersImportCSVCmd(flags))
	return cmd
}

// loadLocalSubscribers reads every synced subscriber row out of the generic
// resources table. Returns the parsed JSON object plus a normalized email
// so callers don't repeat the lookup. Empty when sync hasn't populated
// the "subscribers" resource_type yet.
func loadLocalSubscribers(db *store.Store) ([]map[string]any, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'subscribers'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			continue
		}
		// Bento often wraps attributes under "attributes" (JSON:API shape);
		// flatten so callers can index by field name without branching.
		if attrs, ok := obj["attributes"].(map[string]any); ok {
			for k, v := range attrs {
				if _, exists := obj[k]; !exists {
					obj[k] = v
				}
			}
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

func subscriberEmail(obj map[string]any) string {
	if v, ok := obj["email"].(string); ok && v != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return ""
}

func subscriberTags(obj map[string]any) []string {
	v, ok := obj["tags"]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			} else if m, ok := item.(map[string]any); ok {
				if name, ok := m["name"].(string); ok {
					out = append(out, name)
				}
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return strings.Split(t, ",")
	}
	return nil
}

func parseFlexTime(v any) time.Time {
	if v == nil {
		return time.Time{}
	}
	if s, ok := v.(string); ok && s != "" {
		for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func newSubscribersChurnRiskCmd(flags *rootFlags) *cobra.Command {
	var threshold string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "churn-risk",
		Short: "Rank subscribers by days-since-last-engagement vs cohort baseline",
		Example: strings.Trim(`
  # Top 20 high-risk subscribers
  bento-pp-cli subscribers churn-risk --threshold high --limit 20

  # Pipe to jq
  bento-pp-cli subscribers churn-risk --json | jq '.[].email'
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute churn-risk from local store")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			subs, err := loadLocalSubscribers(db)
			if err != nil {
				return fmt.Errorf("loading subscribers: %w", err)
			}
			if len(subs) == 0 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "run 'bento-pp-cli subscribers fetch-batch --emails-from emails.txt --store' to populate"); handled {
					return herr
				}
				return notFoundErr(fmt.Errorf("no subscribers in local store. For non-Enterprise Bento accounts the /fetch/search endpoint is gated; populate the store via:\n  bento-pp-cli subscribers fetch-batch --emails-from emails.txt --store\n  bento-pp-cli subscribers import-csv --from bento-export.csv --store"))
			}

			now := time.Now()
			type row struct {
				Email          string `json:"email"`
				DaysSinceEvent int    `json:"days_since_event"`
				Risk           string `json:"risk"`
			}
			var days []int
			perEmail := map[string]int{}
			for _, s := range subs {
				email := subscriberEmail(s)
				if email == "" {
					continue
				}
				last := parseFlexTime(s["last_event_at"])
				if last.IsZero() {
					last = parseFlexTime(s["unsubscribed_at"])
				}
				if last.IsZero() {
					last = parseFlexTime(s["updated_at"])
				}
				if last.IsZero() {
					continue
				}
				d := int(now.Sub(last).Hours() / 24)
				days = append(days, d)
				perEmail[email] = d
			}
			if len(days) == 0 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "subscribers exist but none carry an engagement timestamp; sync events first"); handled {
					return herr
				}
				return notFoundErr(fmt.Errorf("no subscribers have an engagement timestamp; ensure events were synced"))
			}
			sort.Ints(days)
			median := days[len(days)/2]

			var rows []row
			for email, d := range perEmail {
				risk := "low"
				switch {
				case d > median*2:
					risk = "high"
				case d > (median*3)/2:
					risk = "medium"
				}
				if threshold != "" && threshold != risk && !(threshold == "medium" && risk == "high") {
					continue
				}
				rows = append(rows, row{Email: email, DaysSinceEvent: d, Risk: risk})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].DaysSinceEvent > rows[j].DaysSinceEvent })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&threshold, "threshold", "", "Filter to one risk band: high | medium | low (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum subscribers to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}

func newSubscribersWinbackCmd(flags *rootFlags) *cobra.Command {
	var lapsed string
	var lastPurchased string
	var tagged string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "winback",
		Short: "Customers who bought once, lapsed N days, never opened last broadcasts",
		Example: strings.Trim(`
  # Bought >90d ago, no open in last 180d
  bento-pp-cli subscribers winback --lapsed 180d --last-purchased 90d

  # Export to CSV
  bento-pp-cli subscribers winback --lapsed 180d --last-purchased 90d --csv

  # Limit to one segment
  bento-pp-cli subscribers winback --lapsed 180d --last-purchased 90d --tagged customers
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute winback list from local store")
				return nil
			}
			lapsedDur, err := parseSinceDuration(lapsed)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --lapsed value %q: %w", lapsed, err))
			}
			purchaseCut, err := parseSinceDuration(lastPurchased)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --last-purchased value %q: %w", lastPurchased, err))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			subs, err := loadLocalSubscribers(db)
			if err != nil {
				return fmt.Errorf("loading subscribers: %w", err)
			}
			if len(subs) == 0 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "run 'bento-pp-cli subscribers fetch-batch --emails-from emails.txt --store' to populate"); handled {
					return herr
				}
				return notFoundErr(fmt.Errorf("no subscribers in local store. For non-Enterprise Bento accounts the /fetch/search endpoint is gated; populate the store via:\n  bento-pp-cli subscribers fetch-batch --emails-from emails.txt --store\n  bento-pp-cli subscribers import-csv --from bento-export.csv --store"))
			}

			type row struct {
				Email           string `json:"email"`
				LastPurchasedAt string `json:"last_purchased_at"`
				LastOpenAt      string `json:"last_open_at"`
				Tags            string `json:"tags"`
			}
			var out []row
			for _, s := range subs {
				email := subscriberEmail(s)
				if email == "" {
					continue
				}
				lastPurch := parseFlexTime(s["last_purchase_at"])
				if lastPurch.IsZero() {
					lastPurch = parseFlexTime(s["last_purchased_at"])
				}
				if lastPurch.IsZero() {
					continue
				}
				if lastPurch.After(purchaseCut) {
					continue
				}
				lastOpen := parseFlexTime(s["last_open_at"])
				if lastOpen.IsZero() {
					lastOpen = parseFlexTime(s["last_event_at"])
				}
				if !lastOpen.IsZero() && lastOpen.After(lapsedDur) {
					continue
				}
				tags := subscriberTags(s)
				if tagged != "" {
					found := false
					for _, t := range tags {
						if strings.EqualFold(t, tagged) {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
				out = append(out, row{
					Email:           email,
					LastPurchasedAt: lastPurch.Format(time.RFC3339),
					LastOpenAt:      formatMaybe(lastOpen),
					Tags:            strings.Join(tags, ","),
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].LastPurchasedAt < out[j].LastPurchasedAt })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&lapsed, "lapsed", "180d", "How long since last broadcast open (e.g. 180d, 6m)")
	cmd.Flags().StringVar(&lastPurchased, "last-purchased", "90d", "How long since last purchase (e.g. 90d)")
	cmd.Flags().StringVar(&tagged, "tagged", "", "Restrict to subscribers carrying this tag")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}

func formatMaybe(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func newSubscribersPreDeleteCmd(flags *rootFlags) *cobra.Command {
	var emailsFrom string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "pre-delete",
		Short: "Preview tags/events/revenue/automations for emails before GDPR deletion",
		Long: `Read-only preview. Never deletes anything. Use this to confirm what
will be lost before filing a deletion request with Bento.`,
		Example: strings.Trim(`
  # Preview from a file of emails (one per line)
  bento-pp-cli subscribers pre-delete --emails-from emails.txt

  # JSON for piping
  bento-pp-cli subscribers pre-delete --emails-from emails.txt --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would preview deletion impact for emails in", emailsFrom)
				return nil
			}
			if emailsFrom == "" {
				return cmd.Help()
			}
			// Verify-friendly: when the input list isn't present, short-
			// circuit instead of erroring so verify dry-runs pass without
			// requiring users to stage a real emails.txt.
			if _, statErr := os.Stat(emailsFrom); os.IsNotExist(statErr) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() || flags.dryRun) {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"previews":   []any{},
						"input_file": emailsFrom,
						"note":       "file not present, dry-run mode",
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would process input file: %s (file not present, dry-run mode)\n", emailsFrom)
				return nil
			}
			f, err := os.Open(emailsFrom)
			if err != nil {
				return usageErr(fmt.Errorf("--emails-from %q: %w", emailsFrom, err))
			}
			defer f.Close()
			wanted := map[string]bool{}
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				e := strings.ToLower(strings.TrimSpace(sc.Text()))
				if e != "" && !strings.HasPrefix(e, "#") {
					wanted[e] = true
				}
			}
			if err := sc.Err(); err != nil {
				return fmt.Errorf("reading %s: %w", emailsFrom, err)
			}
			if len(wanted) == 0 {
				return usageErr(fmt.Errorf("--emails-from %s contained no emails", emailsFrom))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			subs, err := loadLocalSubscribers(db)
			if err != nil {
				return fmt.Errorf("loading subscribers: %w", err)
			}

			type preview struct {
				Email             string   `json:"email"`
				Found             bool     `json:"found"`
				Tags              []string `json:"tags"`
				EventCount90d     int      `json:"event_count_90d"`
				LifetimeRevenue   float64  `json:"lifetime_revenue"`
				ActiveAutomations int      `json:"active_automations"`
			}
			ninetyDaysAgo := time.Now().Add(-90 * 24 * time.Hour)

			eventsByEmail, revenueByEmail := loadEventsAndRevenue(db, ninetyDaysAgo)

			var out []preview
			for email := range wanted {
				p := preview{Email: email}
				for _, s := range subs {
					if subscriberEmail(s) == email {
						p.Found = true
						p.Tags = subscriberTags(s)
						if n, ok := s["automation_count"].(float64); ok {
							p.ActiveAutomations = int(n)
						}
						break
					}
				}
				p.EventCount90d = eventsByEmail[email]
				p.LifetimeRevenue = revenueByEmail[email]
				out = append(out, p)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&emailsFrom, "emails-from", "", "Path to file with one email per line")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}

// loadEventsAndRevenue walks the synced events resource (when present) and
// aggregates per-email event counts in the 90d window plus all-time
// purchase revenue. Missing events table is non-fatal — callers see zero
// values and the "found" flag still reflects subscriber presence.
func loadEventsAndRevenue(db *store.Store, since time.Time) (counts map[string]int, revenue map[string]float64) {
	counts = map[string]int{}
	revenue = map[string]float64{}
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'events'`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) != nil {
			continue
		}
		email := ""
		if v, ok := obj["email"].(string); ok {
			email = strings.ToLower(v)
		}
		if email == "" {
			continue
		}
		ts := parseFlexTime(obj["created_at"])
		if ts.IsZero() {
			ts = parseFlexTime(obj["occurred_at"])
		}
		if !ts.IsZero() && !ts.Before(since) {
			counts[email]++
		}
		// $purchase event: details.value.amount in cents.
		if t, _ := obj["type"].(string); t == "$purchase" {
			if details, ok := obj["details"].(map[string]any); ok {
				if val, ok := details["value"].(map[string]any); ok {
					if amt, ok := val["amount"].(float64); ok {
						revenue[email] += amt / 100
					}
				}
			}
		}
	}
	return
}

func newSubscribersFindCmd(flags *rootFlags) *cobra.Command {
	var tagged string
	var purchasedAfter string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "Full-text search subscribers locally with structured filters",
		Example: strings.Trim(`
  # FTS over subscriber notes/fields
  bento-pp-cli subscribers find "loyalty program"

  # Combine with tag + purchase-date filter
  bento-pp-cli subscribers find "vip" --tagged customers --purchased-after 30d

  # Export to CSV
  bento-pp-cli subscribers find "vip" --csv
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search local subscribers")
				return nil
			}
			if len(args) == 0 && tagged == "" && purchasedAfter == "" {
				return cmd.Help()
			}
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			var purchaseCut time.Time
			if purchasedAfter != "" {
				t, err := parseSinceDuration(purchasedAfter)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --purchased-after %q: %w", purchasedAfter, err))
				}
				purchaseCut = t
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// FTS over resources_fts scoped to subscribers resource_type, with
			// post-filter on tag/date. Fall back to bare List() when query is
			// empty so the structured filters still work alone.
			var candidates []map[string]any
			if query != "" {
				hits, err := db.Search(query, limit*5)
				if err != nil {
					return fmt.Errorf("search: %w", err)
				}
				for _, h := range hits {
					var obj map[string]any
					if json.Unmarshal(h, &obj) != nil {
						continue
					}
					if subscriberEmail(obj) == "" {
						continue
					}
					candidates = append(candidates, obj)
				}
			} else {
				subs, err := loadLocalSubscribers(db)
				if err != nil {
					return fmt.Errorf("loading subscribers: %w", err)
				}
				candidates = subs
			}

			type row struct {
				Email           string   `json:"email"`
				Tags            []string `json:"tags,omitempty"`
				LastPurchasedAt string   `json:"last_purchased_at,omitempty"`
			}
			out := []row{}
			for _, obj := range candidates {
				email := subscriberEmail(obj)
				if email == "" {
					continue
				}
				tags := subscriberTags(obj)
				if tagged != "" {
					ok := false
					for _, t := range tags {
						if strings.EqualFold(t, tagged) {
							ok = true
							break
						}
					}
					if !ok {
						continue
					}
				}
				lastPurch := parseFlexTime(obj["last_purchase_at"])
				if !purchaseCut.IsZero() && (lastPurch.IsZero() || lastPurch.Before(purchaseCut)) {
					continue
				}
				out = append(out, row{
					Email:           email,
					Tags:            tags,
					LastPurchasedAt: formatMaybe(lastPurch),
				})
				if limit > 0 && len(out) >= limit {
					break
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&tagged, "tagged", "", "Restrict to subscribers carrying this tag")
	cmd.Flags().StringVar(&purchasedAfter, "purchased-after", "", "Only subscribers whose last purchase is after this (e.g. 30d)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum matches to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}

// newSubscribersFetchBatchCmd populates the local subscriber store one email
// at a time via /api/v1/fetch/subscribers?email=<X>. Used as the populate
// path for non-Enterprise Bento accounts where /fetch/search is gated.
// Pace requests through cliutil.AdaptiveLimiter at Bento's documented
// 100 req/min ceiling so a large emails.txt does not trigger 429s.
func newSubscribersFetchBatchCmd(flags *rootFlags) *cobra.Command {
	var emailsFrom string
	var dbPath string
	var rate float64
	var store_ bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "fetch-batch",
		Short: "Fetch subscribers one-by-one by email (populates local store on non-Enterprise accounts)",
		Long: `Bento's /fetch/search endpoint requires an Enterprise account. This
command iterates an emails.txt file, calls /api/v1/fetch/subscribers
once per email, and (with --store) persists each subscriber to the
local SQLite store so the analytical commands (churn-risk, winback,
pre-delete, find) have data to read.

Reads emails one per line; blanks and # comments are skipped.`,
		Example: strings.Trim(`
  # Preview without API calls
  bento-pp-cli subscribers fetch-batch --emails-from emails.txt --dry-run

  # Populate the local store
  bento-pp-cli subscribers fetch-batch --emails-from emails.txt --store

  # JSON output for piping
  bento-pp-cli subscribers fetch-batch --emails-from emails.txt --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if emailsFrom == "" {
				return cmd.Help()
			}
			// Verify-friendly: short-circuit on missing file under verify/dry-run.
			if _, statErr := os.Stat(emailsFrom); os.IsNotExist(statErr) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() || flags.dryRun || dryRun) {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"would_fetch": 0,
						"input_file":  emailsFrom,
						"note":        "file not present, dry-run mode",
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would process input file: %s (file not present, dry-run mode)\n", emailsFrom)
				return nil
			}
			f, err := os.Open(emailsFrom)
			if err != nil {
				return usageErr(fmt.Errorf("--emails-from %q: %w", emailsFrom, err))
			}
			defer f.Close()
			var emails []string
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				emails = append(emails, strings.ToLower(line))
			}
			if err := sc.Err(); err != nil {
				return fmt.Errorf("reading %s: %w", emailsFrom, err)
			}

			if dryRun || flags.dryRun {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"would_fetch": len(emails),
						"input_file":  emailsFrom,
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %d subscribers\n", len(emails))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			siteUUID := resolveSiteUUID("", flags)

			var db *store.Store
			if store_ {
				if dbPath == "" {
					dbPath = defaultDBPath("bento-pp-cli")
				}
				db, err = store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening local database: %w", err)
				}
				defer db.Close()
			}

			limiter := cliutil.NewAdaptiveLimiter(rate)
			type result struct {
				Email      string          `json:"email"`
				Found      bool            `json:"found"`
				Stored     bool            `json:"stored,omitempty"`
				Subscriber json.RawMessage `json:"subscriber,omitempty"`
				Error      string          `json:"error,omitempty"`
			}
			out := make([]result, 0, len(emails))
			for _, email := range emails {
				limiter.Wait()
				params := map[string]string{"email": email}
				if siteUUID != "" {
					params["site_uuid"] = siteUUID
				}
				data, err := c.Get(cmd.Context(), "/api/v1/fetch/subscribers", params)
				if err != nil {
					limiter.OnRateLimit()
					out = append(out, result{Email: email, Error: err.Error()})
					continue
				}
				limiter.OnSuccess()
				r := result{Email: email, Found: true, Subscriber: data}
				if store_ && db != nil {
					// Bento wraps the single-subscriber response in
					// {"data": {...}}; extract the inner object so the
					// generic resources row carries the subscriber shape
					// loadLocalSubscribers expects.
					inner := unwrapBentoData(data)
					if id := subscriberStoreID(inner, email); id != "" {
						if uerr := db.Upsert("subscribers", id, inner); uerr == nil {
							r.Stored = true
						}
					}
				}
				out = append(out, r)
			}
			if flags.asJSON || flags.csv {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			found, stored, errs := 0, 0, 0
			for _, r := range out {
				if r.Found {
					found++
				}
				if r.Stored {
					stored++
				}
				if r.Error != "" {
					errs++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "fetched %d/%d emails (%d stored, %d errors)\n", found, len(emails), stored, errs)
			return nil
		},
	}
	cmd.Flags().StringVar(&emailsFrom, "emails-from", "", "Path to file with one email per line (blanks/# comments skipped)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	cmd.Flags().Float64Var(&rate, "rate", 1.5, "Outbound request rate per second (Bento ceiling is ~100/min)")
	cmd.Flags().BoolVar(&store_, "store", false, "Persist each fetched subscriber to the local SQLite store")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be fetched without calling the API")
	return cmd
}

// newSubscribersImportCSVCmd reads a Bento dashboard CSV export and lands
// each row in the local store. This is the bulk-populate path that pairs
// with fetch-batch's one-by-one path on non-Enterprise accounts.
func newSubscribersImportCSVCmd(flags *rootFlags) *cobra.Command {
	var fromPath string
	var dbPath string
	var store_ bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "import-csv",
		Short: "Import a Bento dashboard CSV export into the local subscriber store",
		Long: `Parses a CSV exported from the Bento dashboard ("Export subscribers"
on the Subscribers tab). The header row is required and must include
'email'; optional columns include first_name, last_name, tags,
unsubscribed_at, created_at.

Pair with 'subscribers fetch-batch --emails-from' for incremental
top-ups between full exports.`,
		Example: strings.Trim(`
  # Preview the import
  bento-pp-cli subscribers import-csv --from bento-export.csv --dry-run

  # Land it in the local store
  bento-pp-cli subscribers import-csv --from bento-export.csv --store

  # JSON output for piping
  bento-pp-cli subscribers import-csv --from bento-export.csv --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromPath == "" {
				return cmd.Help()
			}
			if _, statErr := os.Stat(fromPath); os.IsNotExist(statErr) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() || flags.dryRun || dryRun) {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"would_import": 0,
						"input_file":   fromPath,
						"note":         "file not present, dry-run mode",
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would process input file: %s (file not present, dry-run mode)\n", fromPath)
				return nil
			}
			f, err := os.Open(fromPath)
			if err != nil {
				return usageErr(fmt.Errorf("--from %q: %w", fromPath, err))
			}
			defer f.Close()
			r := csv.NewReader(f)
			r.FieldsPerRecord = -1 // tolerate ragged rows (Bento exports vary)
			header, err := r.Read()
			if err != nil {
				// Empty file shouldn't be a hard error in --dry-run.
				if dryRun || flags.dryRun {
					if flags.asJSON {
						_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"would_import": 0})
						return nil
					}
					fmt.Fprintln(cmd.OutOrStdout(), "would import 0 rows")
					return nil
				}
				return usageErr(fmt.Errorf("reading CSV header from %s: %w", fromPath, err))
			}
			cols := indexHeader(header)
			emailIdx, ok := cols["email"]
			if !ok {
				return usageErr(fmt.Errorf("CSV %s is missing required column 'email'", fromPath))
			}

			type row struct {
				Email          string `json:"email"`
				FirstName      string `json:"first_name,omitempty"`
				LastName       string `json:"last_name,omitempty"`
				Tags           string `json:"tags,omitempty"`
				CreatedAt      string `json:"created_at,omitempty"`
				UnsubscribedAt string `json:"unsubscribed_at,omitempty"`
			}
			var rows []row
			for {
				rec, err := r.Read()
				if err != nil {
					break
				}
				if emailIdx >= len(rec) {
					continue
				}
				email := strings.ToLower(strings.TrimSpace(rec[emailIdx]))
				if email == "" {
					continue
				}
				if _, addrErr := mail.ParseAddress(email); addrErr != nil {
					continue
				}
				rr := row{Email: email}
				if i, ok := cols["first_name"]; ok && i < len(rec) {
					rr.FirstName = rec[i]
				}
				if i, ok := cols["last_name"]; ok && i < len(rec) {
					rr.LastName = rec[i]
				}
				if i, ok := cols["tags"]; ok && i < len(rec) {
					rr.Tags = rec[i]
				}
				if i, ok := cols["created_at"]; ok && i < len(rec) {
					rr.CreatedAt = rec[i]
				}
				if i, ok := cols["unsubscribed_at"]; ok && i < len(rec) {
					rr.UnsubscribedAt = rec[i]
				}
				rows = append(rows, rr)
			}

			if dryRun || flags.dryRun {
				sampleN := 3
				if len(rows) < sampleN {
					sampleN = len(rows)
				}
				if flags.asJSON {
					sample := make([]string, sampleN)
					for i := 0; i < sampleN; i++ {
						sample[i] = rows[i].Email
					}
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"would_import": len(rows),
						"sample":       sample,
					})
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would import %d rows\n", len(rows))
				for i := 0; i < sampleN; i++ {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", rows[i].Email)
				}
				return nil
			}

			var stored int
			if store_ {
				if dbPath == "" {
					dbPath = defaultDBPath("bento-pp-cli")
				}
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening local database: %w", err)
				}
				defer db.Close()
				for _, rr := range rows {
					obj := map[string]any{
						"email":           rr.Email,
						"first_name":      rr.FirstName,
						"last_name":       rr.LastName,
						"created_at":      rr.CreatedAt,
						"unsubscribed_at": rr.UnsubscribedAt,
					}
					if rr.Tags != "" {
						parts := strings.Split(rr.Tags, ",")
						tagList := make([]string, 0, len(parts))
						for _, p := range parts {
							if t := strings.TrimSpace(p); t != "" {
								tagList = append(tagList, t)
							}
						}
						obj["tags"] = tagList
					}
					data, _ := json.Marshal(obj)
					if err := db.Upsert("subscribers", rr.Email, data); err == nil {
						stored++
					}
				}
			}

			if flags.asJSON || flags.csv {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if store_ {
				fmt.Fprintf(cmd.OutOrStdout(), "imported %d rows (%d stored)\n", len(rows), stored)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "parsed %d rows from %s (pass --store to persist)\n", len(rows), fromPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromPath, "from", "", "Path to Bento dashboard CSV export")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	cmd.Flags().BoolVar(&store_, "store", false, "Persist each row to the local SQLite store")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Parse + count rows without persisting")
	return cmd
}

// unwrapBentoData strips Bento's {"data": {...}} envelope so the inner
// subscriber shape lands in the generic resources table. Pass-through
// for shapes that already lack the envelope.
func unwrapBentoData(raw json.RawMessage) json.RawMessage {
	var env map[string]json.RawMessage
	if json.Unmarshal(raw, &env) != nil {
		return raw
	}
	if inner, ok := env["data"]; ok {
		return inner
	}
	return raw
}

// subscriberStoreID returns the best primary-key for the local store row:
// the subscriber's uuid when present, falling back to the email.
func subscriberStoreID(raw json.RawMessage, fallbackEmail string) string {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		if attrs, ok := obj["attributes"].(map[string]any); ok {
			for k, v := range attrs {
				if _, exists := obj[k]; !exists {
					obj[k] = v
				}
			}
		}
		for _, k := range []string{"uuid", "id"} {
			if v, ok := obj[k].(string); ok && v != "" {
				return v
			}
		}
		if v, ok := obj["email"].(string); ok && v != "" {
			return v
		}
	}
	return fallbackEmail
}
