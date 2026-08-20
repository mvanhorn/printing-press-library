// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-authored body.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

// pp:data-source local
//
// watch digest makes ZERO API requests, deliberately.
//
// Sensor Tower throttles at roughly 13 requests before returning a bare HTTP
// 429 with no Retry-After and a ~240s recovery, so a per-app live fetch would
// blow the entire budget on a ten-app watchlist. The digest instead reads the
// rank_snapshot rows that `movers` writes. An app with no stored snapshot is
// reported with a null rank and told which command to run — never a fabricated
// or silently-omitted rank.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/marketing/sensortower/internal/store"
	"github.com/spf13/cobra"
)

type watchDigestRow struct {
	AppID string `json:"app_id"`
	OS    string `json:"os"`
	Label string `json:"label,omitempty"`
	// Rank is null when the local mirror holds no snapshot for this app on the
	// requested chart/country.
	Rank         *int   `json:"rank"`
	PreviousRank *int   `json:"previous_rank"`
	Delta        *int   `json:"delta"`
	Chart        string `json:"chart"`
	Category     string `json:"category,omitempty"`
	AsOf         string `json:"as_of,omitempty"`
}

type watchDigestResult struct {
	Apps []watchDigestRow `json:"apps"`
	Note string           `json:"note,omitempty"`
}

func newNovelWatchDigestCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagChart string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Print every tracked app's current rank per chart plus its delta since the previous sync.",
		Long: "Report each watched app's latest stored rank and its move since the previous snapshot.\n\n" +
			"This command reads only the local store — it makes no API requests. Snapshots come from\n" +
			"'sensortower-pp-cli movers <category>'; run that for the categories you care about to\n" +
			"populate the digest. Apps with no snapshot are listed with a null rank rather than\n" +
			"dropped, so the watchlist and the digest always line up.",
		Example:     "  sensortower-pp-cli watch digest --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read local rank snapshots for every watched app (no API requests)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if err := validateChoice("chart", flagChart, "free", "grossing", "paid"); err != nil {
				return err
			}

			// readWatchlist hints on stderr for a cold store and returns no rows;
			// a cold store is an empty watchlist, so it lands in the same
			// empty-envelope branch below as an empty-but-warm one. The digest's
			// payload shape must not depend on whether the store file exists.
			entries, err := readWatchlist(ctx, cmd)
			if err != nil {
				return err
			}

			result := watchDigestResult{Apps: make([]watchDigestRow, 0, len(entries))}
			if len(entries) == 0 {
				result.Note = "the watchlist is empty. Add an app with 'sensortower-pp-cli watch add <app-id>'."
				return emitNovelResult(cmd, flags, result, "apps")
			}

			// One store handle for the whole digest: readWatchlist already
			// established that the mirror exists, and reopening it per watched
			// app would cost an open/close cycle per row for no benefit.
			db, err := store.OpenReadOnlyContext(ctx, defaultDBPath("sensortower-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			missing := 0
			for _, e := range entries {
				row := watchDigestRow{AppID: e.AppID, OS: e.OS, Label: e.Label, Chart: flagChart}
				snaps, err := readLatestSnapshots(ctx, db, e.OS, e.AppID, flagCountry, flagChart)
				if err != nil {
					return err
				}
				if len(snaps) == 0 {
					missing++
					result.Apps = append(result.Apps, row)
					continue
				}
				row.Rank = intPtr(snaps[0].Rank)
				row.Category = snaps[0].Category
				row.AsOf = snaps[0].Date
				if len(snaps) > 1 {
					row.PreviousRank = intPtr(snaps[1].Rank)
					row.Delta = intPtr(snaps[1].Rank - snaps[0].Rank)
				}
				result.Apps = append(result.Apps, row)
			}

			switch {
			case missing == len(result.Apps):
				result.Note = fmt.Sprintf("no local snapshots for any watched app on the %s/%s chart, so every rank is null. Run 'sensortower-pp-cli movers <category> --country %s --chart %s' for each app's category first; this digest never fetches per app because the API throttles at ~13 requests.", flagCountry, flagChart, flagCountry, flagChart)
			case missing > 0:
				result.Note = fmt.Sprintf("%d of %d watched apps have no local snapshot on the %s/%s chart (rank is null). Run 'sensortower-pp-cli movers <category>' for their categories to cover them.", missing, len(result.Apps), flagCountry, flagChart)
			default:
				result.Note = "delta is previous_rank - rank against this CLI's own snapshot history, so a positive delta means the app climbed since the last stored snapshot."
			}

			return emitNovelResult(cmd, flags, result, "apps")
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code of the stored snapshots (e.g. US, GB, JP)")
	cmd.Flags().StringVar(&flagChart, "chart", "free", "Which stored chart to digest (one of: free, grossing, paid)")
	return cmd
}

// readLatestSnapshots returns the two most recent stored snapshots for one app
// on one chart, newest first. Zero rows means the mirror has nothing for this
// app — the caller must report a null rank, not omit the app.
//
// Both rows are scoped to ONE category: the one the newest snapshot belongs to,
// which is also the category the digest row reports. An app can sit on several
// category charts at once (`movers 6016` and `movers 36` both snapshot it), and
// a rank on one chart is not comparable to a rank on another. Without this scope
// the two newest rows could be the same date in different categories, and their
// difference would be printed as a move the app never made.
//
// Scoping by category also makes the pair inherently distinct-dated: the row id
// is os:category:country:chart:date:app, so one category holds at most one row
// per date.
//
// db is caller-owned: the digest reads once per watched app, so it opens the
// store once and lends the handle rather than paying an open/close per row.
func readLatestSnapshots(ctx context.Context, db *store.Store, osName, appID, country, chart string) ([]rankSnapshot, error) {
	// captured_at breaks the tie when two categories share the newest date, so
	// the chosen category is deterministic rather than whatever SQLite returns.
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data
		   FROM resources
		  WHERE resource_type = ?
		    AND json_extract(data, '$.os') = ?
		    AND json_extract(data, '$.app_id_key') = ?
		    AND json_extract(data, '$.country') = ?
		    AND json_extract(data, '$.chart') = ?
		    AND json_extract(data, '$.category') = (
		          SELECT json_extract(latest.data, '$.category')
		            FROM resources AS latest
		           WHERE latest.resource_type = ?
		             AND json_extract(latest.data, '$.os') = ?
		             AND json_extract(latest.data, '$.app_id_key') = ?
		             AND json_extract(latest.data, '$.country') = ?
		             AND json_extract(latest.data, '$.chart') = ?
		           ORDER BY json_extract(latest.data, '$.date') DESC,
		                    json_extract(latest.data, '$.captured_at') DESC
		           LIMIT 1
		        )
		  ORDER BY json_extract(data, '$.date') DESC
		  LIMIT 2`,
		rankSnapshotResource, osName, appID, country, chart,
		rankSnapshotResource, osName, appID, country, chart,
	)
	if err != nil {
		if syncHintMissingTable(err) {
			return nil, nil
		}
		return nil, err
	}
	// Drain fully, then close.
	var out []rankSnapshot
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if !raw.Valid {
			continue
		}
		var snap rankSnapshot
		if err := json.Unmarshal([]byte(raw.String), &snap); err != nil {
			continue
		}
		out = append(out, snap)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	return out, nil
}
