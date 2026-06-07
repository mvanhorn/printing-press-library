// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
//
// Brand registry + the live `pull` collector that populates the local
// multi-brand snapshot store. The novel analytics commands read from the
// tables this file writes. The Instagram Graph API is per-account and only
// exposes a short window; this collector is what turns it into cross-account,
// historical analytics on disk.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

// resolveDBPath returns the explicit --db value if set, otherwise the
// canonical default path for the local store.
func resolveDBPath(dbFlag string) string {
	if strings.TrimSpace(dbFlag) != "" {
		return dbFlag
	}
	return defaultDBPath("instagram-pp-cli")
}

// ensureAnalyticsSchema opens the store at dbPath and creates the analytics
// tables if missing. The caller owns Close().
func ensureAnalyticsSchema(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.EnsureAnalyticsSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// parseLooseDuration accepts 7d/8w/30d/24h/15m style windows, delegating to
// cliutil.ParseDurationLoose (which extends time.ParseDuration with d/w).
func parseLooseDuration(s string) (time.Duration, error) {
	return cliutil.ParseDurationLoose(s)
}

var slugifyRE = regexp.MustCompile(`[^a-z0-9]+`)

var validIGUsername = regexp.MustCompile(`^[A-Za-z0-9._]+$`)

// slugify lowercases and collapses non-alphanumeric runs to hyphens.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugifyRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// ---------------------------------------------------------------------------
// brands command tree
// ---------------------------------------------------------------------------

func newBrandsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brands",
		Short: "Manage the local registry of Instagram Business brand accounts the analytics commands read from.",
		Long: `Register the Instagram Business accounts (brands) you own or want to track.

The registry lives in the local store and drives the 'pull' collector: each
registered brand is fetched per sync, building the cross-account, historical
snapshot data the compare/growth/top-posts/formats/rivals/hashtag-perf
commands analyze.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBrandsAddCmd(flags))
	cmd.AddCommand(newBrandsListCmd(flags))
	cmd.AddCommand(newBrandsRmCmd(flags))
	cmd.AddCommand(newBrandsDiscoverCmd(flags))
	cmd.AddCommand(newBrandsTrackRivalCmd(flags))
	cmd.AddCommand(newBrandsTrackHashtagCmd(flags))
	return cmd
}

func newBrandsAddCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, nameFlag string
	cmd := &cobra.Command{
		Use:     "add <slug> <ig_user_id>",
		Short:   "Register a brand by slug and Instagram Business account id.",
		Example: "  instagram-pp-cli brands add acme 17841400000000001 --name \"Acme\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would register brand in local store")
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("brands add requires <slug> and <ig_user_id>"))
			}
			slug := slugify(args[0])
			igUserID := strings.TrimSpace(args[1])
			if slug == "" || igUserID == "" {
				return usageErr(fmt.Errorf("slug and ig_user_id must be non-empty"))
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			_, err = db.DB().ExecContext(cmd.Context(),
				`INSERT OR REPLACE INTO ig_brands(slug, ig_user_id, name, username, added_at) VALUES (?,?,?,?,?)`,
				slug, igUserID, nameFlag, "", nowRFC3339())
			if err != nil {
				return apiErr(err)
			}
			out := map[string]any{"slug": slug, "ig_user_id": igUserID, "name": nameFlag}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "registered brand %q (ig_user_id=%s)\n", slug, igUserID)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "Human-readable brand name")
	return cmd
}

type brandRow struct {
	Slug     string `json:"slug"`
	IGUserID string `json:"ig_user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	AddedAt  string `json:"added_at"`
}

func newBrandsListCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List tracked brands with slug, IG user id, name, and date added",
		Example:     "  instagram-pp-cli brands list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list brands from local store")
				return nil
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT slug, ig_user_id, COALESCE(name,''), COALESCE(username,''), COALESCE(added_at,'')
				 FROM ig_brands ORDER BY slug`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			out := make([]brandRow, 0)
			for rows.Next() {
				var b brandRow
				if err := rows.Scan(&b.Slug, &b.IGUserID, &b.Name, &b.Username, &b.AddedAt); err != nil {
					return apiErr(err)
				}
				out = append(out, b)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No brands registered. Add one with 'instagram-pp-cli brands add <slug> <ig_user_id>' or 'brands discover'.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SLUG\tIG_USER_ID\tNAME\tADDED_AT")
			for _, b := range out {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", b.Slug, b.IGUserID, b.Name, b.AddedAt)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	return cmd
}

func newBrandsRmCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	cmd := &cobra.Command{
		Use:     "rm <slug>",
		Short:   "Remove a brand from the registry.",
		Example: "  instagram-pp-cli brands rm acme",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would remove brand from local store")
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("brands rm requires <slug>"))
			}
			slug := slugify(args[0])
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			res, err := db.DB().ExecContext(cmd.Context(), `DELETE FROM ig_brands WHERE slug = ?`, slug)
			if err != nil {
				return apiErr(err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("no registered brand matching %q", slug))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"slug": slug, "removed": n}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d brand(s) matching %q\n", n, slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	return cmd
}

func newBrandsDiscoverCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Auto-register brands from the Instagram Business accounts linked to your Facebook Pages.",
		Long: `Walk /me/accounts, and for every Facebook Page with a linked Instagram
Business account, fetch its profile and register a brand (slug = lowercased
username). Live call — requires a valid access token.`,
		Example: `  # Register every IG Business account linked to your Facebook Pages
  instagram-pp-cli brands discover

  # Preview without writing to the local store
  instagram-pp-cli brands discover --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would discover brands from /me/accounts")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			raw, err := c.Get(cmd.Context(), "/me/accounts", map[string]string{"fields": "id,name,instagram_business_account"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var pages struct {
				Data []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					IGBA struct {
						ID string `json:"id"`
					} `json:"instagram_business_account"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &pages); err != nil {
				return apiErr(fmt.Errorf("parsing /me/accounts: %w", err))
			}
			discovered := make([]brandRow, 0)
			for _, p := range pages.Data {
				if p.IGBA.ID == "" {
					continue
				}
				prof, perr := c.Get(cmd.Context(), "/"+p.IGBA.ID, map[string]string{"fields": "id,username,name"})
				slug := ""
				name := p.Name
				username := ""
				if perr == nil {
					var pr struct {
						ID       string `json:"id"`
						Username string `json:"username"`
						Name     string `json:"name"`
					}
					if json.Unmarshal(prof, &pr) == nil {
						username = pr.Username
						if pr.Name != "" {
							name = pr.Name
						}
						if pr.Username != "" {
							slug = slugify(pr.Username)
						}
					}
				}
				if slug == "" {
					slug = slugify(p.Name)
				}
				if slug == "" {
					continue
				}
				_, werr := db.DB().ExecContext(cmd.Context(),
					`INSERT OR REPLACE INTO ig_brands(slug, ig_user_id, name, username, added_at) VALUES (?,?,?,?,?)`,
					slug, p.IGBA.ID, name, username, nowRFC3339())
				if werr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not register %q: %v\n", slug, werr)
					continue
				}
				discovered = append(discovered, brandRow{Slug: slug, IGUserID: p.IGBA.ID, Name: name, Username: username})
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"discovered": discovered, "count": len(discovered)}, flags)
			}
			if len(discovered) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No Instagram Business accounts found on your linked Pages.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SLUG\tIG_USER_ID\tNAME")
			for _, b := range discovered {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", b.Slug, b.IGUserID, b.Name)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	return cmd
}

func newBrandsTrackRivalCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	cmd := &cobra.Command{
		Use:     "track-rival <slug> <username>",
		Short:   "Track a rival public Instagram account for one of your brands.",
		Example: "  instagram-pp-cli brands track-rival acme competitorhandle",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would track rival in local store")
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("brands track-rival requires <slug> and <username>"))
			}
			slug := slugify(args[0])
			username := strings.TrimPrefix(strings.TrimSpace(args[1]), "@")
			if slug == "" || username == "" {
				return usageErr(fmt.Errorf("slug and username must be non-empty"))
			}
			if !validIGUsername.MatchString(username) {
				return usageErr(fmt.Errorf("invalid username %q: only letters, digits, '.' and '_' are allowed", username))
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			_, err = db.DB().ExecContext(cmd.Context(),
				`INSERT OR REPLACE INTO ig_tracked_competitors(owner_slug, username) VALUES (?,?)`, slug, username)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"owner_slug": slug, "username": username}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tracking rival @%s for brand %q\n", username, slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	return cmd
}

func newBrandsTrackHashtagCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	cmd := &cobra.Command{
		Use:   "track-hashtag <slug> <hashtag>",
		Short: "Track a hashtag's top-media performance for one of your brands.",
		Example: `  # Start tracking #coffee top media for the brand "acme"
  instagram-pp-cli brands track-hashtag acme coffee

  # Preview the tracking write without touching the store
  instagram-pp-cli brands track-hashtag acme coffee --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would track hashtag in local store")
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("brands track-hashtag requires <slug> and <hashtag>"))
			}
			slug := slugify(args[0])
			hashtag := strings.TrimPrefix(strings.TrimSpace(args[1]), "#")
			if slug == "" || hashtag == "" {
				return usageErr(fmt.Errorf("slug and hashtag must be non-empty"))
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			_, err = db.DB().ExecContext(cmd.Context(),
				`INSERT OR REPLACE INTO ig_tracked_hashtags(slug, hashtag) VALUES (?,?)`, slug, hashtag)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"slug": slug, "hashtag": hashtag}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tracking #%s for brand %q\n", hashtag, slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	return cmd
}

// ---------------------------------------------------------------------------
// pull collector
// ---------------------------------------------------------------------------

type pullBrand struct {
	slug     string
	igUserID string
}

// jsonNum coerces a JSON number-or-string into int64 (the Graph API returns
// some counts as strings).
func jsonNum(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case string:
		var f float64
		if _, err := fmt.Sscanf(n, "%g", &f); err == nil {
			return int64(f)
		}
	}
	return 0
}

// fetchInsightTotals issues an insights GET and returns metric->total_value
// per metric. A whole-call failure returns an empty map (caller treats every
// metric as 0). Missing individual metrics are simply absent.
// fetchInsightTotals issues an insights GET and returns metric->total_value.
// An HTTP 400 is tolerated (returns an empty map, nil error): the Graph API
// 400s when a requested metric is unsupported for the target's type, which is
// expected for some media. Any other error (401/403/429/5xx, transport, or a
// malformed body) is propagated so the caller does NOT persist a zero-filled
// snapshot that would silently corrupt the growth/compare/top-posts series.
func fetchInsightTotals(ctx context.Context, c *client.Client, path string, metrics string) (map[string]int64, error) {
	out := map[string]int64{}
	raw, err := c.Get(ctx, path, map[string]string{
		"metric":      metrics,
		"period":      "day",
		"metric_type": "total_value",
	})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 400 {
			return out, nil // unsupported-metric; tolerate as "no values"
		}
		return out, err
	}
	var resp struct {
		Data []struct {
			Name       string `json:"name"`
			TotalValue struct {
				Value json.Number `json:"value"`
			} `json:"total_value"`
		} `json:"data"`
	}
	if jerr := json.Unmarshal(raw, &resp); jerr != nil {
		return out, fmt.Errorf("parsing insights for %s: %w", path, jerr)
	}
	for _, m := range resp.Data {
		i, _ := m.TotalValue.Value.Int64()
		out[m.Name] = i
	}
	return out, nil
}

func newPullCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, accountFlag string
	var mediaLimit int
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Fetch a fresh snapshot of every registered brand into the local store.",
		Long: `Live collector. For each registered brand (or just --account <slug>):
fetch profile counts, account insights, recent media + per-media insights,
tracked-rival business-discovery, and tracked-hashtag top media — then write
snapshot rows the analytics commands read.

Run this on a schedule to build the historical series the growth/rivals
commands need. Per-metric API errors are tolerated (counted as 0) so a single
unavailable insight never aborts the whole pull.`,
		Example:     "  instagram-pp-cli pull --media-limit 25",
		Annotations: map[string]string{"pp:method": "GET"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would pull snapshots for all registered brands")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			brands, err := loadBrands(cmd.Context(), db.DB(), accountFlag)
			if err != nil {
				return apiErr(err)
			}
			if len(brands) == 0 {
				note := "no brands registered; run 'instagram-pp-cli brands add <slug> <ig_user_id>' or 'brands discover' first"
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"brands_pulled": 0, "media_upserted": 0, "snapshots": 0,
						"errors": []string{}, "note": note,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}

			// Dogfood: cap work to fit the 30s timeout.
			trackedCap := -1
			if cliutil.IsDogfoodEnv() {
				if mediaLimit > 3 {
					mediaLimit = 3
				}
				trackedCap = 1
			}

			var (
				brandsPulled  int
				mediaUpserted int
				snapshots     int
				pullErrors    = make([]string, 0)
				fetchFailures = make([]string, 0)
			)

			for _, b := range brands {
				bm, bs, errs, ffs := pullBrandSnapshot(cmd.Context(), c, db.DB(), b, mediaLimit, trackedCap)
				mediaUpserted += bm
				snapshots += bs
				pullErrors = append(pullErrors, errs...)
				fetchFailures = append(fetchFailures, ffs...)
				brandsPulled++
				if !flags.asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %d media, %d snapshot row(s)\n", b.slug, bm, bs)
				}
			}

			if len(fetchFailures) > 0 {
				fmt.Fprintf(os.Stderr, "warning: %d media-insight fetch(es) failed and were excluded from counts\n", len(fetchFailures))
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"brands_pulled":  brandsPulled,
					"media_upserted": mediaUpserted,
					"snapshots":      snapshots,
					"errors":         pullErrors,
					"fetch_failures": fetchFailures,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pulled %d brand(s), %d media, %d snapshot row(s)\n", brandsPulled, mediaUpserted, snapshots)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&accountFlag, "account", "", "Limit the pull to a single brand slug")
	cmd.Flags().IntVar(&mediaLimit, "media-limit", 25, "Max recent media per brand to fetch")
	return cmd
}

func loadBrands(ctx context.Context, db *sql.DB, account string) ([]pullBrand, error) {
	q := `SELECT slug, ig_user_id FROM ig_brands`
	var rows *sql.Rows
	var err error
	if strings.TrimSpace(account) != "" {
		rows, err = db.QueryContext(ctx, q+` WHERE slug = ? ORDER BY slug`, slugify(account))
	} else {
		rows, err = db.QueryContext(ctx, q+` ORDER BY slug`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]pullBrand, 0)
	for rows.Next() {
		var b pullBrand
		if err := rows.Scan(&b.slug, &b.igUserID); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// pullBrandSnapshot performs the full collection for a single brand. It
// returns media-upserted count, snapshot-row count, hard errors, and
// per-media fetch failures (excluded from counts).
func pullBrandSnapshot(ctx context.Context, c *client.Client, db *sql.DB, b pullBrand, mediaLimit, trackedCap int) (int, int, []string, []string) {
	errs := make([]string, 0)
	fetchFailures := make([]string, 0)
	snapshots := 0
	mediaUpserted := 0

	// 1. profile counts.
	var followers, follows, mediaCount int64
	profileOK := false
	if raw, err := c.Get(ctx, "/"+b.igUserID, map[string]string{"fields": "followers_count,follows_count,media_count"}); err == nil {
		var prof map[string]any
		if json.Unmarshal(raw, &prof) == nil {
			followers = jsonNum(prof["followers_count"])
			follows = jsonNum(prof["follows_count"])
			mediaCount = jsonNum(prof["media_count"])
			profileOK = true
		} else {
			errs = append(errs, fmt.Sprintf("%s profile: parse failure", b.slug))
		}
	} else {
		errs = append(errs, fmt.Sprintf("%s profile: %v", b.slug, err))
	}

	// 2. account insights. A 400 tolerates (unsupported metric); a real
	// auth/transport error is recorded and the insight columns are written
	// NULL so a bad token can never persist fake-zero reach/interactions.
	ins, insErr := fetchInsightTotals(ctx, c, "/"+b.igUserID+"/insights", "reach,accounts_engaged,total_interactions,views")
	if insErr != nil {
		errs = append(errs, fmt.Sprintf("%s account-insights: %v", b.slug, insErr))
	}

	// 3. account snapshot row. Only persist when the profile resolved — a
	// snapshot with no real followers count would corrupt the growth series.
	if profileOK {
		nullable := func(k string) sql.NullInt64 {
			if insErr != nil {
				return sql.NullInt64{} // insights unavailable -> NULL, not 0
			}
			return sql.NullInt64{Int64: ins[k], Valid: true}
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO ig_account_snapshots(slug, ig_user_id, followers_count, follows_count, media_count, reach, total_interactions, accounts_engaged, views, captured_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?)`,
			b.slug, b.igUserID, followers, follows, mediaCount,
			nullable("reach"), nullable("total_interactions"), nullable("accounts_engaged"), nullable("views"), nowRFC3339()); err == nil {
			snapshots++
		} else {
			errs = append(errs, fmt.Sprintf("%s account-snapshot: %v", b.slug, err))
		}
	} else {
		errs = append(errs, fmt.Sprintf("%s account-snapshot: skipped (profile unavailable)", b.slug))
	}

	// 4. media + per-media insights.
	mu, mfails := pullBrandMedia(ctx, c, db, b, mediaLimit)
	mediaUpserted += mu
	fetchFailures = append(fetchFailures, mfails...)

	// 5. tracked competitors.
	cs, cerrs := pullBrandRivals(ctx, c, db, b, trackedCap)
	snapshots += cs
	errs = append(errs, cerrs...)

	// 6. tracked hashtags.
	hs, herrs := pullBrandHashtags(ctx, c, db, b, trackedCap)
	snapshots += hs
	errs = append(errs, herrs...)

	return mediaUpserted, snapshots, errs, fetchFailures
}

type mediaInsightResult struct {
	media        map[string]any
	reach        int64
	views        int64
	saved        int64
	shares       int64
	interactions int64
	reelsWatch   float64
	fetchFailure string
}

func pullBrandMedia(ctx context.Context, c *client.Client, db *sql.DB, b pullBrand, mediaLimit int) (int, []string) {
	fetchFailures := make([]string, 0)
	raw, err := c.Get(ctx, "/"+b.igUserID+"/media", map[string]string{
		"fields": "id,caption,media_type,media_product_type,permalink,timestamp,like_count,comments_count",
		"limit":  fmt.Sprintf("%d", mediaLimit),
	})
	if err != nil {
		return 0, []string{fmt.Sprintf("%s media-list: %v", b.slug, err)}
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if jerr := json.Unmarshal(raw, &resp); jerr != nil {
		return 0, []string{fmt.Sprintf("%s media-list: parse failure: %v", b.slug, jerr)}
	}
	if len(resp.Data) == 0 {
		return 0, fetchFailures
	}

	// Fan out per-media insight calls in parallel; preserve per-fetch errors.
	results := make([]mediaInsightResult, len(resp.Data))
	var wg sync.WaitGroup
	for i, m := range resp.Data {
		wg.Add(1)
		go func(i int, m map[string]any) {
			defer wg.Done()
			r := mediaInsightResult{media: m}
			mediaID, _ := m["id"].(string)
			if mediaID == "" {
				r.fetchFailure = fmt.Sprintf("%s media: missing id", b.slug)
				results[i] = r
				return
			}
			metrics := "reach,views,saved,shares,total_interactions"
			if pt, _ := m["media_product_type"].(string); strings.EqualFold(pt, "REELS") {
				metrics += ",ig_reels_avg_watch_time"
			}
			ins, insErr := fetchInsightTotals(ctx, c, "/"+mediaID+"/insights", metrics)
			if insErr != nil {
				// A real auth/transport error (not a tolerated 400) — record
				// it as a fetch failure rather than upserting fake-zero metrics.
				r.fetchFailure = fmt.Sprintf("%s media %s insights: %v", b.slug, mediaID, insErr)
				results[i] = r
				return
			}
			r.reach = ins["reach"]
			r.views = ins["views"]
			r.saved = ins["saved"]
			r.shares = ins["shares"]
			r.interactions = ins["total_interactions"]
			r.reelsWatch = float64(ins["ig_reels_avg_watch_time"])
			results[i] = r
		}(i, m)
	}
	wg.Wait()

	upserted := 0
	for _, r := range results {
		if r.fetchFailure != "" {
			fetchFailures = append(fetchFailures, r.fetchFailure)
			continue
		}
		m := r.media
		mediaID, _ := m["id"].(string)
		caption, _ := m["caption"].(string)
		mediaType, _ := m["media_type"].(string)
		productType, _ := m["media_product_type"].(string)
		permalink, _ := m["permalink"].(string)
		timestamp, _ := m["timestamp"].(string)
		_, err := db.ExecContext(ctx,
			`INSERT OR REPLACE INTO ig_brand_media(
				slug, ig_user_id, media_id, caption, media_type, media_product_type, permalink, posted_at,
				like_count, comments_count, reach, views, saved, shares, total_interactions, reels_avg_watch_time, captured_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			b.slug, b.igUserID, mediaID, caption, mediaType, productType, permalink, timestamp,
			jsonNum(m["like_count"]), jsonNum(m["comments_count"]),
			r.reach, r.views, r.saved, r.shares, r.interactions, r.reelsWatch, nowRFC3339())
		if err != nil {
			fetchFailures = append(fetchFailures, fmt.Sprintf("%s media %s upsert: %v", b.slug, mediaID, err))
			continue
		}
		upserted++
	}
	return upserted, fetchFailures
}

func pullBrandRivals(ctx context.Context, c *client.Client, db *sql.DB, b pullBrand, trackedCap int) (int, []string) {
	errs := make([]string, 0)
	rows, err := db.QueryContext(ctx, `SELECT username FROM ig_tracked_competitors WHERE owner_slug = ? ORDER BY username`, b.slug)
	if err != nil {
		return 0, []string{fmt.Sprintf("%s rivals-load: %v", b.slug, err)}
	}
	var usernames []string
	for rows.Next() {
		var u string
		if rows.Scan(&u) == nil {
			usernames = append(usernames, u)
		}
	}
	rerr := rows.Err()
	_ = rows.Close()
	if rerr != nil {
		errs = append(errs, fmt.Sprintf("%s rivals-load: %v", b.slug, rerr))
	}

	snapshots := 0
	for i, u := range usernames {
		if trackedCap >= 0 && i >= trackedCap {
			break
		}
		field := fmt.Sprintf("business_discovery.username(%s){followers_count,media_count,media.limit(10){like_count,comments_count}}", u)
		raw, err := c.Get(ctx, "/"+b.igUserID, map[string]string{"fields": field})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s rival %s: %v", b.slug, u, err))
			continue
		}
		var resp struct {
			BD struct {
				FollowersCount int64 `json:"followers_count"`
				MediaCount     int64 `json:"media_count"`
				Media          struct {
					Data []struct {
						LikeCount     int64 `json:"like_count"`
						CommentsCount int64 `json:"comments_count"`
					} `json:"data"`
				} `json:"media"`
			} `json:"business_discovery"`
		}
		if json.Unmarshal(raw, &resp) != nil {
			errs = append(errs, fmt.Sprintf("%s rival %s: parse failure", b.slug, u))
			continue
		}
		var sum, n float64
		for _, m := range resp.BD.Media.Data {
			sum += float64(m.LikeCount + m.CommentsCount)
			n++
		}
		avg := 0.0
		if n > 0 {
			avg = sum / n
		}
		if _, werr := db.ExecContext(ctx,
			`INSERT INTO ig_competitor_snapshots(owner_slug, username, followers_count, media_count, recent_avg_engagement, captured_at)
			 VALUES (?,?,?,?,?,?)`,
			b.slug, u, resp.BD.FollowersCount, resp.BD.MediaCount, avg, nowRFC3339()); werr == nil {
			snapshots++
		} else {
			errs = append(errs, fmt.Sprintf("%s rival %s upsert: %v", b.slug, u, werr))
		}
	}
	return snapshots, errs
}

func pullBrandHashtags(ctx context.Context, c *client.Client, db *sql.DB, b pullBrand, trackedCap int) (int, []string) {
	errs := make([]string, 0)
	rows, err := db.QueryContext(ctx, `SELECT hashtag FROM ig_tracked_hashtags WHERE slug = ? ORDER BY hashtag`, b.slug)
	if err != nil {
		return 0, []string{fmt.Sprintf("%s hashtags-load: %v", b.slug, err)}
	}
	var tags []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			tags = append(tags, t)
		}
	}
	rerr := rows.Err()
	_ = rows.Close()
	if rerr != nil {
		errs = append(errs, fmt.Sprintf("%s hashtags-load: %v", b.slug, rerr))
	}

	snapshots := 0
	for i, tag := range tags {
		if trackedCap >= 0 && i >= trackedCap {
			break
		}
		searchRaw, serr := c.Get(ctx, "/ig_hashtag_search", map[string]string{"user_id": b.igUserID, "q": tag})
		if serr != nil {
			errs = append(errs, fmt.Sprintf("%s hashtag %s search: %v", b.slug, tag, serr))
			continue
		}
		var search struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.Unmarshal(searchRaw, &search) != nil || len(search.Data) == 0 {
			errs = append(errs, fmt.Sprintf("%s hashtag %s: no id", b.slug, tag))
			continue
		}
		hashtagID := search.Data[0].ID
		topRaw, terr := c.Get(ctx, "/"+hashtagID+"/top_media", map[string]string{
			"user_id": b.igUserID,
			"fields":  "like_count,comments_count",
		})
		if terr != nil {
			errs = append(errs, fmt.Sprintf("%s hashtag %s top_media: %v", b.slug, tag, terr))
			continue
		}
		var top struct {
			Data []struct {
				LikeCount     int64 `json:"like_count"`
				CommentsCount int64 `json:"comments_count"`
			} `json:"data"`
		}
		if json.Unmarshal(topRaw, &top) != nil {
			errs = append(errs, fmt.Sprintf("%s hashtag %s top_media: parse failure", b.slug, tag))
			continue
		}
		var engagement int64
		for _, m := range top.Data {
			engagement += m.LikeCount + m.CommentsCount
		}
		if _, werr := db.ExecContext(ctx,
			`INSERT INTO ig_hashtag_snapshots(slug, hashtag, hashtag_id, top_media_reach, top_media_engagement, top_media_count, captured_at)
			 VALUES (?,?,?,?,?,?,?)`,
			b.slug, tag, hashtagID, 0, engagement, int64(len(top.Data)), nowRFC3339()); werr == nil {
			snapshots++
		} else {
			errs = append(errs, fmt.Sprintf("%s hashtag %s upsert: %v", b.slug, tag, werr))
		}
	}
	return snapshots, errs
}
