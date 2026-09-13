// pp:data-source local

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newHideCmd(flags))
		addNovelCommandIfAbsent(root, newShortlistCmd(flags))
		addNovelCommandIfAbsent(root, newDumpCmd(flags))
	})
}

func parseIDs(args []string) ([]int64, error) {
	ids := make([]int64, 0, len(args))
	for _, a := range args {
		id, err := immo.ParseListingID(a)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func newHideCmd(flags *rootFlags) *cobra.Command {
	var undo bool
	var reason, dbPath string
	cmd := &cobra.Command{
		Use:   "hide [id-or-url...]",
		Short: "Hide listings you have dismissed so find, watch and triage skip them (--undo to restore)",
		Example: strings.Trim(`
  immoweb-pp-cli hide 21834193 21833605 --reason "too dark"
  immoweb-pp-cli hide 21834193 --undo`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "local",
			"pp:happy-args":  "id=21834193",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hide listings locally")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one listing ID or URL is required"))
			}
			ids, err := parseIDs(args)
			if err != nil {
				return usageErr(err)
			}
			db, err := openImmoStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			for _, id := range ids {
				if err := db.SetHidden(cmd.Context(), id, !undo, reason); err != nil {
					return err
				}
			}
			out := map[string]any{"hidden": !undo, "ids": ids}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			verb := "Hid"
			if undo {
				verb = "Restored"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d listing(s)\n", verb, len(ids))
			return nil
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "Un-hide the listings")
	cmd.Flags().StringVar(&reason, "reason", "", "Optional reason, kept locally")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

func newShortlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shortlist",
		Short: "Local shortlist of listings with notes (no Immoweb login needed)",
		Example: strings.Trim(`
  immoweb-pp-cli shortlist add 21828249 --note "visit Saturday"
  immoweb-pp-cli shortlist list
  immoweb-pp-cli shortlist remove 21828249`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	var dbPath string
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.PersistentFlags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files

	add := &cobra.Command{
		Use:         "add [id-or-url...]",
		Short:       "Add listings to the shortlist",
		Example:     "  immoweb-pp-cli shortlist add 21828249 --note \"visit Saturday\"",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=21828249"},
	}
	var note string
	add.Flags().StringVar(&note, "note", "", "Note to keep with the listing")
	add.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 && c.Flags().NFlag() == 0 {
			return c.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(c.OutOrStdout(), flags, "add to shortlist")
		}
		if len(args) == 0 {
			_ = c.Usage()
			return usageErr(fmt.Errorf("at least one listing ID or URL is required"))
		}
		return shortlistMutate(c, flags, dbPath, args, true, note)
	}

	remove := &cobra.Command{
		Use:         "remove [id-or-url...]",
		Aliases:     []string{"rm"},
		Short:       "Remove listings from the shortlist",
		Example:     "  immoweb-pp-cli shortlist remove 21828249",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "id=21828249"},
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 && c.Flags().NFlag() == 0 {
				return c.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(c.OutOrStdout(), flags, "remove from shortlist")
			}
			if len(args) == 0 {
				_ = c.Usage()
				return usageErr(fmt.Errorf("at least one listing ID or URL is required"))
			}
			return shortlistMutate(c, flags, dbPath, args, false, "")
		},
	}

	list := &cobra.Command{
		Use:         "list",
		Short:       "Show shortlisted listings with their latest stored price and notes",
		Example:     "  immoweb-pp-cli shortlist list --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(c *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(c.OutOrStdout(), flags, "list shortlist")
			}
			type row struct {
				store.ShortlistEntry
				Listing *store.StoredListing `json:"listing,omitempty"`
			}
			rows := make([]row, 0)
			if path, ok := localStoreExists(dbPath); ok {
				db, err := openImmoStore(c.Context(), path)
				if err != nil {
					return err
				}
				defer db.Close()
				entries, err := db.ListShortlist(c.Context())
				if err != nil {
					return err
				}
				ids := make([]int64, 0, len(entries))
				for _, e := range entries {
					ids = append(ids, e.ID)
				}
				byID := map[int64]store.StoredListing{}
				if len(ids) > 0 {
					ls, err := db.QueryListings(c.Context(), store.ListingFilter{IDs: ids, IncludeGone: true})
					if err != nil {
						return err
					}
					for _, l := range ls {
						byID[l.ID] = l
					}
				}
				for _, e := range entries {
					r := row{ShortlistEntry: e}
					if l, ok := byID[e.ID]; ok {
						l := l
						r.Listing = &l
					}
					rows = append(rows, r)
				}
			}
			if !wantsHumanTable(c.OutOrStdout(), flags) {
				return printJSONFiltered(c.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(c.OutOrStdout(), "Shortlist is empty. Add with: immoweb-pp-cli shortlist add <id>")
				return nil
			}
			tw := newTabWriter(c.OutOrStdout())
			fmt.Fprintln(tw, "ID\tPRICE\tLOCALITY\tSTATUS\tNOTE")
			for _, r := range rows {
				price, loc, status := "", "", ""
				if r.Listing != nil {
					if r.Listing.Price != nil {
						price = fmtEUR(*r.Listing.Price)
					}
					loc = locLabel(r.Listing.Listing)
					if r.Listing.GoneAt != "" {
						status = "gone"
					} else if r.Listing.UnderOption {
						status = "under option"
					}
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", r.ID, price, loc, status, r.Note)
			}
			return tw.Flush()
		},
	}
	cmd.AddCommand(add, remove, list)
	return cmd
}

func shortlistMutate(c *cobra.Command, flags *rootFlags, dbPath string, args []string, add bool, note string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return usageErr(err)
	}
	db, err := openImmoStore(c.Context(), dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, id := range ids {
		if err := db.SetShortlist(c.Context(), id, add, note); err != nil {
			return err
		}
	}
	out := map[string]any{"shortlisted": add, "ids": ids}
	if !wantsHumanTable(c.OutOrStdout(), flags) {
		return printJSONFiltered(c.OutOrStdout(), out, flags)
	}
	fmt.Fprintf(c.OutOrStdout(), "Updated %d listing(s)\n", len(ids))
	return nil
}

func newDumpCmd(flags *rootFlags) *cobra.Command {
	var format, communes, postcodes, types, deal, dbPath string
	var includeGone bool
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Export stored listings as flattened CSV, GeoJSON or JSONL",
		Example: strings.Trim(`
  immoweb-pp-cli dump --format csv --postcode 1050 > ixelles.csv
  immoweb-pp-cli dump --format geojson --deal rent > rentals.geojson
  immoweb-pp-cli dump --format jsonl --include-gone`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export stored listings")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			format = strings.ToLower(format)
			if format != "csv" && format != "geojson" && format != "jsonl" {
				return usageErr(fmt.Errorf("--format must be csv, geojson or jsonl"))
			}
			f := store.ListingFilter{IncludeGone: includeGone}
			if deal != "" {
				d, err := immo.NormalizeDeal(deal)
				if err != nil {
					return usageErr(err)
				}
				f.Deal = d
			}
			if types != "" {
				ts, err := immo.NormalizeTypes(types)
				if err != nil {
					return usageErr(err)
				}
				f.Types = ts
			}
			for _, p := range immo.SplitCSV(postcodes) {
				pc := immo.NormalizePostalCode(p)
				if pc == "" {
					return usageErr(fmt.Errorf("--postcode %q is not a 4-digit Belgian postal code", p))
				}
				f.PostalCodes = append(f.PostalCodes, pc)
			}
			communeNames := immo.SplitCSV(communes)
			rows := make([]store.StoredListing, 0)
			path, ok := localStoreExists(dbPath)
			if ok {
				db, err := openImmoStore(cmd.Context(), path)
				if err != nil {
					return err
				}
				defer db.Close()
				for _, name := range communeNames {
					pcs, err := localPostcodesFor(cmd.Context(), db, name)
					if err != nil {
						return err
					}
					if len(pcs) == 0 {
						return notFoundErr(fmt.Errorf("no stored listings in %q; pull it first: immoweb-pp-cli pull --commune %s --type <type> --deal <sale|rent>", name, name))
					}
					f.PostalCodes = append(f.PostalCodes, pcs...)
				}
				if rows, err = db.QueryListings(cmd.Context(), f); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local store at %s; run: immoweb-pp-cli pull --commune <name> --type <type> --deal <sale|rent>\n", path)
			}
			w := cmd.OutOrStdout()
			if (flags.asJSON || flags.agent) && format != "geojson" {
				// --json/--agent ask for one JSON document; they win over csv
				// and jsonl (geojson is JSON already and is kept).
				return printJSONFiltered(w, rows, flags)
			}
			switch format {
			case "jsonl":
				enc := json.NewEncoder(w)
				for _, r := range rows {
					if err := enc.Encode(r); err != nil {
						return err
					}
				}
			case "geojson":
				type feature struct {
					Type       string         `json:"type"`
					Geometry   map[string]any `json:"geometry"`
					Properties any            `json:"properties"`
				}
				fc := struct {
					Type     string    `json:"type"`
					Features []feature `json:"features"`
				}{Type: "FeatureCollection", Features: make([]feature, 0)}
				for _, r := range rows {
					if r.Lat == nil || r.Lng == nil {
						continue
					}
					fc.Features = append(fc.Features, feature{Type: "Feature", Geometry: map[string]any{"type": "Point", "coordinates": []float64{*r.Lng, *r.Lat}}, Properties: r})
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(fc)
			case "csv":
				cw := csv.NewWriter(w)
				_ = cw.Write([]string{"id", "url", "deal", "type", "subtype", "title", "postal_code", "locality", "province", "street", "lat", "lng", "price", "rent_costs", "bedrooms", "surface_m2", "land_m2", "price_per_m2", "epc", "agency", "private_seller", "under_option", "created_at", "first_seen", "last_seen", "gone_at"})
				fs := func(p *float64) string {
					if p == nil {
						return ""
					}
					return strconv.FormatFloat(*p, 'f', -1, 64)
				}
				for _, r := range rows {
					_ = cw.Write([]string{strconv.FormatInt(r.ID, 10), r.URL, csvSafe(r.Deal), csvSafe(r.Type), csvSafe(r.Subtype), csvSafe(r.Title), csvSafe(r.PostalCode), csvSafe(r.Locality), csvSafe(r.Province), csvSafe(r.Street),
						fs(r.Lat), fs(r.Lng), fs(r.Price), fs(r.RentCosts), intStr(r.Bedrooms), fs(r.Surface), fs(r.Land), fs(r.PricePerSqm), csvSafe(r.EPC), csvSafe(r.Agency),
						strconv.FormatBool(r.Private), strconv.FormatBool(r.UnderOption), r.CreatedAt, r.FirstSeen, r.LastSeen, r.GoneAt})
				}
				cw.Flush()
				return cw.Error()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "Output format: csv, geojson or jsonl (--json/--agent print a JSON array instead of csv or jsonl)")
	cmd.Flags().StringVar(&postcodes, "postcode", "", "Only these postal codes (comma-separated)")
	cmd.Flags().StringVar(&communes, "commune", "", "Commune name(s) as stored locally (e.g. ixelles) or postal codes")
	cmd.Flags().StringVar(&types, "type", "", "Only these property types")
	cmd.Flags().StringVar(&deal, "deal", "", "Only sale or rent")
	cmd.Flags().BoolVar(&includeGone, "include-gone", false, "Include listings that disappeared from Immoweb")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

// csvSafe neutralises spreadsheet formula injection in advertiser-written text.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}
