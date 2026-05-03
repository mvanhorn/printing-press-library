// `watch save / list / show / delete / run / tail` is the saved-search +
// diff-event surface that turns a periodic sapi search into typed NEW /
// PRICE-DROP / DUP events. seen_listings carries per-watch alert state so
// re-running a watch never re-emits the same NEW twice.
//
// Side-effect rule: watch run mutates seen_listings (local state) and watch
// tail loops forever. Both honor cliutil.IsVerifyEnv() per the side-effect
// command convention; mcp:read-only is true because no external state is
// touched.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"
	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/store"

	"github.com/spf13/cobra"
)

// watchRow is the typed saved_searches shape we emit in list/show.
type watchRow struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Query          string `json:"query"`
	SitesCSV       string `json:"sites"`
	Category       string `json:"category"`
	MinPrice       int    `json:"minPrice,omitempty"`
	MaxPrice       int    `json:"maxPrice,omitempty"`
	HasPic         bool   `json:"hasPic,omitempty"`
	Postal         string `json:"postal,omitempty"`
	SearchDistance int    `json:"distanceMi,omitempty"`
	TitleOnly      bool   `json:"titleOnly,omitempty"`
	NegateCSV      string `json:"negate,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
	LastRunAt      int64  `json:"lastRunAt,omitempty"`
}

// watchEvent is the typed event the run/tail loop emits. Kind ∈ {NEW, PRICE-DROP, DUP, SEED}.
type watchEvent struct {
	Kind     string `json:"kind"`
	Site     string `json:"site"`
	PID      int64  `json:"pid"`
	UUID     string `json:"uuid,omitempty"`
	Title    string `json:"title"`
	Price    int    `json:"price"`
	PriceOld int    `json:"priceOld,omitempty"`
	URL      string `json:"url,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Saved searches with NEW / PRICE-DROP / DUP event diffs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newWatchSaveCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchShowCmd(flags))
	cmd.AddCommand(newWatchDeleteCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	cmd.AddCommand(newWatchTailCmd(flags))
	return cmd
}

func newWatchSaveCmd(flags *rootFlags) *cobra.Command {
	var query, sites, category, postal, negate string
	var minPrice, maxPrice, distanceMi int
	var hasPic, titleOnly bool
	cmd := &cobra.Command{
		Use:         "save [name]",
		Short:       "Save a watch (named saved search)",
		Example:     "  craigslist-pp-cli watch save apartments --query 1BR --negate furnished,sublet --sites sfbay --category apa --max-price 2500\n  craigslist-pp-cli watch save deals --query \"eames lounge\" --sites sfbay,nyc,seattle --category fua --max-price 1500",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.TrimSpace(args[0])
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			now := time.Now().Unix()
			res, err := db.DB().ExecContext(ctx, `
				INSERT INTO saved_searches(name, query, sites_csv, category, min_price, max_price, has_pic, postal, search_distance, title_only, negate_csv, created_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
				ON CONFLICT(name) DO UPDATE SET
					query=excluded.query, sites_csv=excluded.sites_csv, category=excluded.category,
					min_price=excluded.min_price, max_price=excluded.max_price, has_pic=excluded.has_pic,
					postal=excluded.postal, search_distance=excluded.search_distance,
					title_only=excluded.title_only, negate_csv=excluded.negate_csv
				`,
				name, query, sites, category, minPrice, maxPrice, boolToInt(hasPic), postal, distanceMi, boolToInt(titleOnly), negate, now,
			)
			if err != nil {
				return fmt.Errorf("save watch: %w", err)
			}
			id, _ := res.LastInsertId()
			row, err := loadWatch(ctx, db, name)
			if err != nil {
				return err
			}
			if row.ID == 0 {
				row.ID = id
			}
			return printJSONFiltered(cmd.OutOrStdout(), row, flags)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Search query string")
	cmd.Flags().StringVar(&sites, "sites", "sfbay", "Comma-separated sites to poll")
	cmd.Flags().StringVar(&category, "category", "sss", "Category abbreviation")
	cmd.Flags().IntVar(&minPrice, "min-price", 0, "Minimum price filter")
	cmd.Flags().IntVar(&maxPrice, "max-price", 0, "Maximum price filter")
	cmd.Flags().BoolVar(&hasPic, "has-pic", false, "Require listings with a picture")
	cmd.Flags().StringVar(&postal, "postal", "", "ZIP / postal code anchor")
	cmd.Flags().IntVar(&distanceMi, "distance-mi", 0, "Distance in miles from --postal")
	cmd.Flags().BoolVar(&titleOnly, "title-only", false, "Match titles only")
	cmd.Flags().StringVar(&negate, "negate", "", "Comma-separated NOT keywords")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all saved watches with their query, sites, and category (local saved_searches table)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := loadAllWatches(ctx, db)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					items = append(items, map[string]any{
						"name":     r.Name,
						"sites":    r.SitesCSV,
						"category": r.Category,
						"query":    r.Query,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	return cmd
}

func newWatchShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show [name]",
		Short:       "Show one saved watch",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			row, err := loadWatch(ctx, db, args[0])
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), row, flags)
		},
	}
	return cmd
}

func newWatchDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "delete [name]",
		Short:       "Delete a saved watch and its alert history",
		Example:     "  craigslist-pp-cli watch delete apartments",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			row, err := loadWatch(ctx, db, args[0])
			if err != nil {
				return err
			}
			if row.ID > 0 {
				_, _ = db.DB().ExecContext(ctx, `DELETE FROM seen_listings WHERE watch_id = ?`, row.ID)
			}
			_, err = db.DB().ExecContext(ctx, `DELETE FROM saved_searches WHERE name = ?`, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted watch %q\n", args[0])
			return nil
		},
	}
	return cmd
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var seedOnly bool
	cmd := &cobra.Command{
		Use:         "run [name]",
		Short:       "Poll a watch once and emit NEW / PRICE-DROP / DUP events",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would run watch (verify env)")
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			row, err := loadWatch(ctx, db, args[0])
			if err != nil {
				return err
			}
			if row.ID == 0 {
				return fmt.Errorf("watch %q not found", args[0])
			}
			events, err := runWatchOnce(ctx, db, row, seedOnly)
			if err != nil {
				return err
			}
			_, _ = db.DB().ExecContext(ctx, `UPDATE saved_searches SET last_run_at = ? WHERE id = ?`, time.Now().Unix(), row.ID)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			for _, e := range events {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %d %s — $%d\n", e.Kind, e.PID, e.Title, e.Price)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&seedOnly, "seed-only", false, "Mark current results as seen without emitting NEW events")
	return cmd
}

func newWatchTailCmd(flags *rootFlags) *cobra.Command {
	var interval string
	cmd := &cobra.Command{
		Use:         "tail [name]",
		Short:       "Long-running poll loop, one event per stdout line",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would tail watch (verify env)")
				return nil
			}
			d, err := parseDuration(interval)
			if err != nil {
				return err
			}
			if d <= 0 {
				d = 5 * time.Minute
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			row, err := loadWatch(ctx, db, args[0])
			if err != nil {
				return err
			}
			if row.ID == 0 {
				return fmt.Errorf("watch %q not found", args[0])
			}
			ticker := time.NewTicker(d)
			defer ticker.Stop()
			for {
				events, err := runWatchOnce(ctx, db, row, false)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: tail iteration: %v\n", err)
				}
				for _, e := range events {
					_ = printJSONFiltered(cmd.OutOrStdout(), e, flags)
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		},
	}
	cmd.Flags().StringVar(&interval, "interval", "5m", "Poll interval (e.g. 5m, 30s)")
	return cmd
}

// runWatchOnce executes one poll cycle: fan out across sites, diff against
// seen_listings, return events. seen_listings is updated as a side effect.
func runWatchOnce(ctx context.Context, db *store.Store, watch watchRow, seedOnly bool) ([]watchEvent, error) {
	sites := splitCSV(watch.SitesCSV)
	if len(sites) == 0 {
		sites = []string{"sfbay"}
	}
	q := craigslist.SearchQuery{
		Query:          watch.Query,
		SearchPath:     watch.Category,
		MinPrice:       watch.MinPrice,
		MaxPrice:       watch.MaxPrice,
		HasPic:         watch.HasPic,
		Postal:         watch.Postal,
		SearchDistance: watch.SearchDistance,
		TitleOnly:      watch.TitleOnly,
	}
	c := craigslist.New(1.0)
	results, _ := cliutil.FanoutRun[string, []craigslist.Listing](
		ctx,
		sites,
		func(s string) string { return s },
		func(ctx context.Context, s string) ([]craigslist.Listing, error) {
			sr, err := c.Search(ctx, s, q)
			if err != nil {
				return nil, err
			}
			return sr.Items, nil
		},
		cliutil.WithConcurrency(5),
	)

	negate := splitNegate(watch.NegateCSV)
	var events []watchEvent
	for _, r := range results {
		for _, item := range r.Value {
			if matchesNegate(item.Title+" "+item.CanonicalURL, negate) {
				continue
			}
			seen, err := lookupSeen(ctx, db, watch.ID, item.PostingID)
			if err != nil {
				return events, err
			}
			if seen == nil {
				_ = upsertSeen(ctx, db, watch.ID, item.PostingID, item.Price, item.Title)
				kind := "NEW"
				if seedOnly {
					kind = "SEED"
				}
				events = append(events, watchEvent{Kind: kind, Site: r.Source, PID: item.PostingID, UUID: item.UUID, Title: item.Title, Price: item.Price, URL: item.CanonicalURL})
				continue
			}
			if seen.LastPrice > 0 && item.Price > 0 && item.Price < seen.LastPrice {
				_ = upsertSeen(ctx, db, watch.ID, item.PostingID, item.Price, item.Title)
				events = append(events, watchEvent{Kind: "PRICE-DROP", Site: r.Source, PID: item.PostingID, UUID: item.UUID, Title: item.Title, Price: item.Price, PriceOld: seen.LastPrice, URL: item.CanonicalURL})
			}
		}
	}
	return events, nil
}

// seenRow is the typed shape from seen_listings.
type seenRow struct {
	WatchID      int64
	PID          int64
	FirstAlertAt int64
	LastPrice    int
	LastTitle    string
}

func lookupSeen(ctx context.Context, db *store.Store, watchID, pid int64) (*seenRow, error) {
	row := db.DB().QueryRowContext(ctx, `
		SELECT watch_id, pid, first_alert_at, COALESCE(last_price,0), COALESCE(last_title,'')
		FROM seen_listings WHERE watch_id = ? AND pid = ?`, watchID, pid)
	var r seenRow
	if err := row.Scan(&r.WatchID, &r.PID, &r.FirstAlertAt, &r.LastPrice, &r.LastTitle); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func upsertSeen(ctx context.Context, db *store.Store, watchID, pid int64, price int, title string) error {
	now := time.Now().Unix()
	_, err := db.DB().ExecContext(ctx, `
		INSERT INTO seen_listings(watch_id, pid, first_alert_at, last_price, last_title)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(watch_id, pid) DO UPDATE SET last_price=excluded.last_price, last_title=excluded.last_title`,
		watchID, pid, now, price, title)
	return err
}

func loadWatch(ctx context.Context, db *store.Store, name string) (watchRow, error) {
	row := db.DB().QueryRowContext(ctx, `
		SELECT id, name, COALESCE(query,''), sites_csv, category, COALESCE(min_price,0), COALESCE(max_price,0),
		       COALESCE(has_pic,0), COALESCE(postal,''), COALESCE(search_distance,0), COALESCE(title_only,0),
		       COALESCE(negate_csv,''), created_at, COALESCE(last_run_at,0)
		FROM saved_searches WHERE name = ?`, name)
	var r watchRow
	var hasPic, titleOnly int
	if err := row.Scan(&r.ID, &r.Name, &r.Query, &r.SitesCSV, &r.Category, &r.MinPrice, &r.MaxPrice, &hasPic, &r.Postal, &r.SearchDistance, &titleOnly, &r.NegateCSV, &r.CreatedAt, &r.LastRunAt); err != nil {
		if err == sql.ErrNoRows {
			return watchRow{}, nil
		}
		return watchRow{}, err
	}
	r.HasPic = hasPic != 0
	r.TitleOnly = titleOnly != 0
	return r, nil
}

func loadAllWatches(ctx context.Context, db *store.Store) ([]watchRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, name, COALESCE(query,''), sites_csv, category, COALESCE(min_price,0), COALESCE(max_price,0),
		       COALESCE(has_pic,0), COALESCE(postal,''), COALESCE(search_distance,0), COALESCE(title_only,0),
		       COALESCE(negate_csv,''), created_at, COALESCE(last_run_at,0)
		FROM saved_searches ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []watchRow
	for rows.Next() {
		var r watchRow
		var hasPic, titleOnly int
		if err := rows.Scan(&r.ID, &r.Name, &r.Query, &r.SitesCSV, &r.Category, &r.MinPrice, &r.MaxPrice, &hasPic, &r.Postal, &r.SearchDistance, &titleOnly, &r.NegateCSV, &r.CreatedAt, &r.LastRunAt); err != nil {
			return out, err
		}
		r.HasPic = hasPic != 0
		r.TitleOnly = titleOnly != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// splitCSV returns trimmed non-empty parts of a comma-separated string.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// matchesNegate returns true if hay (lowercased internally) contains any of
// the negate tokens. Empty negate is always false.
func matchesNegate(hay string, negate []string) bool {
	if len(negate) == 0 {
		return false
	}
	hay = strings.ToLower(hay)
	for _, n := range negate {
		if strings.Contains(hay, n) {
			return true
		}
	}
	return false
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
