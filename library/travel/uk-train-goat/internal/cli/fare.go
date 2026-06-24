// uk-train-goat hand-authored: fare lookup backed by RJFAF feed.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/fares"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/store"

	"github.com/spf13/cobra"
)

type fareLookupResult struct {
	From        string               `json:"from"`
	To          string               `json:"to"`
	Date        string               `json:"date"`
	PublishDate string               `json:"publish_date,omitempty"`
	Fares       []fares.ResolvedFare `json:"fares"`
	Warning     string               `json:"warning,omitempty"`
	Note        string               `json:"note"`
}

// fareProvenanceNote builds the provenance disclaimer for a fare lookup result.
// When publishDate is non-empty the note includes a "(published <date>)" clause;
// when it is blank the clause is omitted rather than emitting an empty parenthetical.
func fareProvenanceNote(publishDate string) string {
	const suffix = "; confirm the price at point of sale. Walk-up fares only; advance and operator-specific fares are not included."
	if publishDate != "" {
		return "Indicative walk-up fares from the National Rail RJFAF feed (published " + publishDate + ")" + suffix
	}
	return "Indicative walk-up fares from the National Rail RJFAF feed" + suffix
}

func newFareCmd(flags *rootFlags) *cobra.Command {
	var date string
	var offline bool

	cmd := &cobra.Command{
		Use:   "fare <from-crs> <to-crs>",
		Short: "Look up walk-up fares between two stations",
		Long: `Look up walk-up fares between two stations using the local RJFAF fares store.

Run 'fare sync' first to populate the store. The store is sourced from the
National Rail RJFAF feed. Walk-up fares only; advance and operator-specific
fares are not included.`,
		Example: strings.Trim(`
  uk-train-goat-pp-cli fare PAD RDG
  uk-train-goat-pp-cli fare PAD RDG --date 2026-06-01
  uk-train-goat-pp-cli fare PAD RDG --offline
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("fare requires <from-crs> <to-crs>; got %d args", len(args)))
			}
			from := strings.ToUpper(strings.TrimSpace(args[0]))
			to := strings.ToUpper(strings.TrimSpace(args[1]))

			// Parse date or default to today.
			var dateYYYYMMDD string
			if date != "" {
				t, err := time.Parse("2006-01-02", date)
				if err != nil {
					return usageErr(fmt.Errorf("--date must be YYYY-MM-DD, got %q", date))
				}
				dateYYYYMMDD = t.Format("20060102")
			} else {
				dateYYYYMMDD = time.Now().Format("20060102")
			}

			dbPath := defaultDBPath("uk-train-goat-pp-cli")
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("open store: %w", err))
			}
			defer s.Close()
			db := s.DB()

			if err := fares.EnsureSchema(db); err != nil {
				return apiErr(fmt.Errorf("ensure schema: %w", err))
			}

			user := os.Getenv("NR_OPENDATA_USERNAME")
			pass := os.Getenv("NR_OPENDATA_PASSWORD")

			// Without credentials the remote freshness probe can only fail with an
			// auth error, which would surface as a misleading "freshness could not
			// be confirmed" warning on every lookup. Fall back to the age backstop
			// instead, exactly as --offline does.
			probeOffline := offline || user == "" || pass == ""

			result, err := fares.CheckFreshness(cmd.Context(), db, "", user, pass, probeOffline, time.Now())
			if err != nil {
				return apiErr(fmt.Errorf("freshness check: %w", err))
			}
			if !result.OK {
				return usageErr(fmt.Errorf("fares data not ready: %s; run `fare sync`", result.Reason))
			}

			resolved, err := fares.Resolve(db, from, to, dateYYYYMMDD)
			if err != nil {
				return apiErr(fmt.Errorf("resolve fares: %w", err))
			}

			note := fareProvenanceNote(result.PublishDate)
			out := fareLookupResult{
				From:        from,
				To:          to,
				Date:        date,
				PublishDate: result.PublishDate,
				Fares:       resolved,
				Warning:     result.Warning,
				Note:        note,
			}
			if out.Date == "" {
				out.Date = time.Now().Format("2006-01-02")
			}

			data, err := json.Marshal(out)
			if err != nil {
				return apiErr(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Target date (YYYY-MM-DD); defaults to today")
	cmd.Flags().BoolVar(&offline, "offline", false, "Skip freshness probe; serve from local store as-is")
	cmd.AddCommand(newFareSyncCmd(flags))
	return cmd
}

type fareSyncSummary struct {
	Sequence     string `json:"sequence"`
	PublishDate  string `json:"publish_date"`
	LastModified string `json:"last_modified"`
	SyncedAt     string `json:"synced_at"`
	Locations    int    `json:"locations"`
	GroupMembers int    `json:"group_members"`
	Flows        int    `json:"flows"`
	Fares        int    `json:"fares"`
	Clusters     int    `json:"clusters"`
	NDF          int    `json:"ndf"`
	Tickets      int    `json:"tickets"`
	Railcards    int    `json:"railcards"`
	Restrictions int    `json:"restrictions"`
}

func newFareSyncCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Download and load the RJFAF fares feed into the local store",
		Long: `Authenticate with National Rail Open Data and download the latest RJFAF
fares feed. Parses and loads it into the local SQLite store.

Requires NR_OPENDATA_USERNAME and NR_OPENDATA_PASSWORD environment variables.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "verify: skipped sync")
				return nil
			}

			user := os.Getenv("NR_OPENDATA_USERNAME")
			pass := os.Getenv("NR_OPENDATA_PASSWORD")
			if user == "" || pass == "" {
				return usageErr(fmt.Errorf("NR_OPENDATA_USERNAME and NR_OPENDATA_PASSWORD must be set"))
			}

			ctx := cmd.Context()

			token, err := fares.Authenticate(ctx, user, pass)
			if err != nil {
				return apiErr(fmt.Errorf("authenticate: %w", err))
			}

			zipPath, meta, err := fares.FetchFeed(ctx, token)
			if err != nil {
				return apiErr(fmt.Errorf("fetch feed: %w", err))
			}
			defer os.Remove(zipPath)

			data, err := fares.ParseFeedZip(zipPath)
			if err != nil {
				return apiErr(fmt.Errorf("parse feed: %w", err))
			}

			dbPath := defaultDBPath("uk-train-goat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("open store: %w", err))
			}
			defer s.Close()
			db := s.DB()

			if err := fares.EnsureSchema(db); err != nil {
				return apiErr(fmt.Errorf("ensure schema: %w", err))
			}
			if err := fares.Load(db, data); err != nil {
				return apiErr(fmt.Errorf("load data: %w", err))
			}

			meta.SyncedAt = time.Now().UTC().Format(time.RFC3339)
			// Use Last-Modified as the effective publish date when the feed does not
			// carry an explicit PublishDate (RJFAF is a static versioned feed; the
			// HTTP Last-Modified header is the authoritative release timestamp).
			if meta.PublishDate == "" {
				meta.PublishDate = meta.LastModified
			}
			if err := fares.WriteMeta(db, meta); err != nil {
				return apiErr(fmt.Errorf("write meta: %w", err))
			}

			summary := fareSyncSummary{
				Sequence:     meta.Sequence,
				PublishDate:  meta.PublishDate,
				LastModified: meta.LastModified,
				SyncedAt:     meta.SyncedAt,
				Locations:    len(data.Locations),
				GroupMembers: len(data.GroupMembers),
				Flows:        len(data.Flows),
				Fares:        len(data.Fares),
				Clusters:     len(data.Clusters),
				NDF:          len(data.NDF),
				Tickets:      len(data.Tickets),
				Railcards:    len(data.Railcards),
				Restrictions: len(data.Restrictions),
			}
			out, err := json.Marshal(summary)
			if err != nil {
				return apiErr(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
}
