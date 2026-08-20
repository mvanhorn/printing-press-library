// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/pdok-location/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/pdok-location/internal/store"
	"github.com/spf13/cobra"
)

// gemeenteRow / provincieRow are the lightweight projections used by the
// gazetteer commands. The full Locatieserver doc is kept in the resources
// table's `data` column; these structs are just the queryable subset.
type gemeenteRow struct {
	Code        string   `json:"gemeentecode"`
	Naam        string   `json:"gemeentenaam"`
	ProvCode    string   `json:"provinciecode"`
	ProvNaam    string   `json:"provincienaam"`
	CentroideLL *coord   `json:"centroide_ll,omitempty"`
	CentroideRD *rdCoord `json:"centroide_rd,omitempty"`
	Score       float64  `json:"score,omitempty"`
}

type provincieRow struct {
	Code        string `json:"provinciecode"`
	Naam        string `json:"provincienaam"`
	Afkorting   string `json:"provincieafkorting,omitempty"`
	CentroideLL *coord `json:"centroide_ll,omitempty"`
}

// newGemeenteCmd parents the gemeente sub-commands.
func newGemeenteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gemeente",
		Short: "Look up Dutch gemeenten (municipalities) — offline gazetteer with API fallback",
		Long: "After a first call seeds the local store, every later gemeente lookup " +
			"is served from SQLite — no API round-trip. The full set is ~342 rows " +
			"and stable enough to cache.",
	}
	cmd.AddCommand(newGemeenteGetCmd(flags))
	cmd.AddCommand(newGemeenteListCmd(flags))
	cmd.AddCommand(newGemeenteOfPointCmd(flags))
	return cmd
}

func newProvincieCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provincie",
		Short: "List Dutch provincies (12 rows total, fully cached after first call)",
	}
	cmd.AddCommand(newProvincieListCmd(flags))
	return cmd
}

func newGemeenteGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "get [name]",
		Short: "Look up a gemeente by name (case-insensitive)",
		Long: "Returns the gemeente record (name, code, provincie) for the given " +
			"name. Tries the local SQLite cache first; on miss, calls " +
			"Locatieserver `/free?fq=type:gemeente` and caches the result.",
		Example:     "  pdok-location-pp-cli gemeente get amsterdam --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.Join(args, " ")
			if dbPath == "" {
				dbPath = defaultDBPath("pdok-location-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			row, err := lookupGemeenteLocal(cmd.Context(), db, name)
			if err == nil && row != nil {
				return flags.printJSON(cmd, row)
			}
			// Cache miss — fetch via API and cache.
			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}
			row, err = fetchAndCacheGemeente(cmd.Context(), c, db, name)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if row == nil {
				return notFoundErr(fmt.Errorf("no gemeente named %q", name))
			}
			return flags.printJSON(cmd, row)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default ~/.local/share/pdok-location-pp-cli/data.db)")
	return cmd
}

func newGemeenteListCmd(flags *rootFlags) *cobra.Command {
	var provincie string
	var dbPath string
	var refresh bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all gemeenten, optionally filtered to one provincie",
		Long: "Lists every gemeente from the local gazetteer (seeded automatically " +
			"on first call). Pass --refresh to re-fetch from Locatieserver.",
		Example: "  pdok-location-pp-cli gemeente list --json\n" +
			"  pdok-location-pp-cli gemeente list --provincie 'Noord-Holland' --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("pdok-location-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			needSeed := refresh
			if !needSeed {
				n, _ := countResourceRows(cmd.Context(), db, "gemeente")
				needSeed = n < 100
			}
			if needSeed {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				if err := seedGemeenten(cmd.Context(), c, db); err != nil {
					return classifyAPIError(err, flags)
				}
			}

			rows, err := listGemeentenLocal(cmd.Context(), db, provincie)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&provincie, "provincie", "", "Filter to one provincie (name match, case-insensitive)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-fetch the full gemeente list from PDOK before listing")
	return cmd
}

func newGemeenteOfPointCmd(flags *rootFlags) *cobra.Command {
	var lat, lon float64
	var rdX, rdY float64
	cmd := &cobra.Command{
		Use:   "of-point",
		Short: "Return the gemeente containing a given point",
		Long: "Given any lat/lon (WGS84) or RD x/y, return which gemeente (and " +
			"provincie) contains the point. Calls Locatieserver `/reverse` with " +
			"type=gemeente to find the enclosing municipality.",
		Example: "  pdok-location-pp-cli gemeente of-point --lat 52.3731 --lon 4.8922 --json\n" +
			"  pdok-location-pp-cli gemeente of-point --rd-x 121200 --rd-y 488000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			haveLL := cmd.Flags().Changed("lat") && cmd.Flags().Changed("lon")
			haveRD := cmd.Flags().Changed("rd-x") && cmd.Flags().Changed("rd-y")
			partialLL := cmd.Flags().Changed("lat") != cmd.Flags().Changed("lon")
			partialRD := cmd.Flags().Changed("rd-x") != cmd.Flags().Changed("rd-y")
			if partialLL || partialRD {
				return usageErr(fmt.Errorf("both --lat and --lon are required (or both --rd-x and --rd-y)"))
			}
			if !haveLL && !haveRD {
				return cmd.Help()
			}
			if haveLL && haveRD {
				return usageErr(fmt.Errorf("specify --lat/--lon OR --rd-x/--rd-y, not both"))
			}
			if haveRD {
				lon, lat = rdToWGS84(rdX, rdY)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"lat":  fmt.Sprintf("%v", lat),
				"lon":  fmt.Sprintf("%v", lon),
				"type": "gemeente",
				"rows": "1",
				"fl":   "id type weergavenaam gemeentecode gemeentenaam provinciecode provincienaam centroide_ll score afstand",
			}
			data, err := c.Get("/bzk/locatieserver/search/v3_1/reverse", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp lsResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(err)
			}
			if resp.Response.NumFound == 0 {
				return notFoundErr(fmt.Errorf("no gemeente near (%.6f, %.6f)", lat, lon))
			}
			doc := enrichLSDoc(resp.Response.Docs[0], false)
			out := map[string]any{
				"query":           map[string]float64{"lat": lat, "lon": lon},
				"gemeentecode":    doc.Gemeentecode,
				"gemeentenaam":    doc.Gemeentenaam,
				"provinciecode":   doc.Provinciecode,
				"provincienaam":   doc.Provincienaam,
				"distance_meters": doc.Afstand,
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "WGS84 latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "WGS84 longitude")
	cmd.Flags().Float64Var(&rdX, "rd-x", 0, "RD X coordinate")
	cmd.Flags().Float64Var(&rdY, "rd-y", 0, "RD Y coordinate")
	return cmd
}

func newProvincieListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var refresh bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all 12 Dutch provincies",
		Long: "Returns the 12 Dutch provincies from the local cache (seeded " +
			"automatically on first call).",
		Example: "  pdok-location-pp-cli provincie list --json\n" +
			"  pdok-location-pp-cli provincie list --refresh",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("pdok-location-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			needSeed := refresh
			if !needSeed {
				n, _ := countResourceRows(cmd.Context(), db, "provincie")
				needSeed = n < 12
			}
			if needSeed {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				if err := seedProvincies(cmd.Context(), c, db); err != nil {
					return classifyAPIError(err, flags)
				}
			}
			rows, err := listProvinciesLocal(cmd.Context(), db)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-fetch the full provincie list from PDOK before listing")
	return cmd
}

// ---------------- Store helpers ----------------

func countResourceRows(ctx context.Context, db *store.Store, resourceType string) (int, error) {
	var n int
	err := db.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM resources WHERE resource_type = ?`,
		resourceType,
	).Scan(&n)
	return n, err
}

func lookupGemeenteLocal(ctx context.Context, db *store.Store, name string) (*gemeenteRow, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type = 'gemeente' AND lower(json_extract(data, '$.gemeentenaam')) = lower(?) LIMIT 1`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var raw sql.NullString
	if err := rows.Scan(&raw); err != nil {
		return nil, err
	}
	if !raw.Valid {
		return nil, nil
	}
	r := projectGemeente([]byte(raw.String))
	return &r, nil
}

func listGemeentenLocal(ctx context.Context, db *store.Store, provincie string) ([]gemeenteRow, error) {
	query := `SELECT data FROM resources WHERE resource_type = 'gemeente'`
	args := []any{}
	if provincie != "" {
		query += ` AND lower(json_extract(data, '$.provincienaam')) = lower(?)`
		args = append(args, provincie)
	}
	query += ` ORDER BY json_extract(data, '$.gemeentenaam')`
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []gemeenteRow
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if !raw.Valid {
			continue
		}
		out = append(out, projectGemeente([]byte(raw.String)))
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Naam) < strings.ToLower(out[j].Naam) })
	return out, nil
}

func listProvinciesLocal(ctx context.Context, db *store.Store) ([]provincieRow, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type = 'provincie' ORDER BY json_extract(data, '$.provincienaam')`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []provincieRow
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if !raw.Valid {
			continue
		}
		out = append(out, projectProvincie([]byte(raw.String)))
	}
	return out, nil
}

func projectGemeente(data []byte) gemeenteRow {
	d := enrichLSDoc(json.RawMessage(data), false)
	return gemeenteRow{
		Code:        d.Gemeentecode,
		Naam:        d.Gemeentenaam,
		ProvCode:    d.Provinciecode,
		ProvNaam:    d.Provincienaam,
		CentroideLL: d.CentroideLL,
		CentroideRD: d.CentroideRD,
		Score:       d.Score,
	}
}

func projectProvincie(data []byte) provincieRow {
	d := enrichLSDoc(json.RawMessage(data), false)
	r := provincieRow{
		Code:        d.Provinciecode,
		Naam:        d.Provincienaam,
		CentroideLL: d.CentroideLL,
	}
	// provincieafkorting if present
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) == nil {
		if v, ok := m["provincieafkorting"]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				r.Afkorting = s
			}
		}
	}
	return r
}

// ---------------- Seed/fetch helpers ----------------

func seedGemeenten(ctx context.Context, c *client.Client, db *store.Store) error {
	// Locatieserver caps rows at 50; 342 gemeenten fit in ~7 pages.
	const pageSize = 50
	for start := 0; start < 600; start += pageSize {
		data, err := c.Get("/bzk/locatieserver/search/v3_1/free", map[string]string{
			"q":     "*:*",
			"fq":    "type:gemeente",
			"rows":  fmt.Sprintf("%d", pageSize),
			"start": fmt.Sprintf("%d", start),
			"fl":    "id type weergavenaam bron gemeentecode gemeentenaam provinciecode provincienaam centroide_ll centroide_rd identificatie",
		})
		if err != nil {
			return err
		}
		var resp lsResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return err
		}
		if len(resp.Response.Docs) == 0 {
			break
		}
		for _, raw := range resp.Response.Docs {
			id := docID(raw, "gemeentecode", "id", "identificatie")
			if id == "" {
				continue
			}
			_ = db.Upsert("gemeente", id, raw)
		}
		if start+len(resp.Response.Docs) >= resp.Response.NumFound {
			break
		}
	}
	return nil
}

func seedProvincies(ctx context.Context, c *client.Client, db *store.Store) error {
	data, err := c.Get("/bzk/locatieserver/search/v3_1/free", map[string]string{
		"q":    "*:*",
		"fq":   "type:provincie",
		"rows": "20",
		"fl":   "id type weergavenaam bron provinciecode provincienaam provincieafkorting centroide_ll centroide_rd identificatie",
	})
	if err != nil {
		return err
	}
	var resp lsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	for _, raw := range resp.Response.Docs {
		id := docID(raw, "provinciecode", "id", "identificatie")
		if id == "" {
			continue
		}
		_ = db.Upsert("provincie", id, raw)
	}
	return nil
}

// fetchAndCacheGemeente fetches one gemeente by name from Locatieserver
// /free?fq=type:gemeente and caches the row in the local store.
func fetchAndCacheGemeente(ctx context.Context, c *client.Client, db *store.Store, name string) (*gemeenteRow, error) {
	data, err := c.Get("/bzk/locatieserver/search/v3_1/free", map[string]string{
		"q":    fmt.Sprintf("gemeentenaam:%q", name),
		"fq":   "type:gemeente",
		"rows": "1",
		"fl":   "id type weergavenaam bron gemeentecode gemeentenaam provinciecode provincienaam centroide_ll centroide_rd identificatie",
	})
	if err != nil {
		return nil, err
	}
	var resp lsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if len(resp.Response.Docs) == 0 {
		return nil, nil
	}
	raw := resp.Response.Docs[0]
	id := docID(raw, "gemeentecode", "id", "identificatie")
	if id != "" {
		_ = db.Upsert("gemeente", id, raw)
	}
	r := projectGemeente([]byte(raw))
	return &r, nil
}

func docID(raw json.RawMessage, keys ...string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}
