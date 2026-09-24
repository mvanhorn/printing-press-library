// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newHideCmd(flags))
		addNovelCommandIfAbsent(root, newShortlistCmd(flags))
		addNovelCommandIfAbsent(root, newDumpCmd(flags))
		addNovelCommandIfAbsent(root, newLocationsCmd(flags))
	})
}

func newHideCmd(flags *rootFlags) *cobra.Command {
	var undo bool
	var reason, dbPath string
	cmd := &cobra.Command{
		Use:   "hide <reference> [reference...]",
		Short: "Hide listings from find, watch and rankings (local only)",
		Example: strings.Trim(`
  immovlan-pp-cli hide vbe69761 --reason "visited, too dark"
  immovlan-pp-cli hide vbe69761 --undo`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:local-write": "true", "pp:happy-args": "reference=vbe69761"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hide listings locally")
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("give at least one listing reference"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			ids := []string{}
			for _, a := range args {
				ref, err := immovlan.ParseReference(a)
				if err != nil {
					return usageErr(err)
				}
				if err := db.SetHidden(ctx, ref, !undo, reason); err != nil {
					return err
				}
				ids = append(ids, ref)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), map[string]any{"ids": ids, "hidden": !undo}, flags)
			}
			verb := "Hidden"
			if undo {
				verb = "Unhidden"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s.\n", verb, strings.Join(ids, ", "))
			return nil
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "Show the listing again")
	cmd.Flags().StringVar(&reason, "reason", "", "Why (kept locally)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

func newShortlistCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "shortlist",
		Short:       "Keep a local shortlist of listings with notes: add, list, remove",
		Example:     "  immovlan-pp-cli shortlist add vbe69761 --note \"visit Saturday\"\n  immovlan-pp-cli shortlist list --agent",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE:        func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.PersistentFlags().MarkHidden("db")
	var note string
	add := &cobra.Command{
		Use:         "add <reference>",
		Short:       "Add a stored listing to the local shortlist by reference, with an optional --note",
		Example:     "  immovlan-pp-cli shortlist add vbe69761 --note \"visit Saturday\"",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:local-write": "true", "pp:happy-args": "reference=vbe69761"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "add to the shortlist")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("give one listing reference"))
			}
			ref, err := immovlan.ParseReference(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.SetShortlist(ctx, ref, true, note); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), map[string]any{"id": ref, "note": note, "shortlisted": true}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Shortlisted %s.\n", ref)
			return nil
		},
	}
	add.Flags().StringVar(&note, "note", "", "A note to keep with the listing")
	list := &cobra.Command{
		Use:         "list",
		Short:       "Show the shortlist with the stored listing fields",
		Example:     "  immovlan-pp-cli shortlist list --agent",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list the shortlist")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := db.ListShortlist(ctx)
			if err != nil {
				return err
			}
			type row struct {
				store.ShortlistEntry
				Listing *store.StoredListing `json:"listing,omitempty"`
			}
			out := make([]row, 0, len(entries))
			ids := make([]string, 0, len(entries))
			for _, e := range entries {
				ids = append(ids, e.ID)
			}
			byID := map[string]store.StoredListing{}
			if len(ids) > 0 {
				rows, err := db.QueryVlanListings(ctx, store.ListingFilter{IDs: ids, IncludeGone: true})
				if err != nil {
					return err
				}
				for _, r := range rows {
					byID[r.ID] = r
				}
			}
			for _, e := range entries {
				r := row{ShortlistEntry: e}
				if l, ok := byID[e.ID]; ok {
					l := l
					r.Listing = &l
				}
				out = append(out, r)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				note := ""
				if len(out) == 0 {
					note = "shortlist is empty; add with: immovlan-pp-cli shortlist add <ref>"
				}
				return printView(cmd.OutOrStdout(), struct {
					Results []row  `json:"results"`
					Note    string `json:"note,omitempty"`
				}{out, note}, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Shortlist is empty. Add with: immovlan-pp-cli shortlist add <ref>")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tPRICE\tLOCALITY\tPEB\tADDED\tNOTE")
			for _, r := range out {
				price, loc, epc := "", "", ""
				if r.Listing != nil {
					price, loc, epc = fmtPtrEUR(r.Listing.Price), locLabel(r.Listing.Listing), termSafe(epcLabel(r.Listing.Listing))
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, price, loc, epc, shortDate(r.AddedAt), termSafe(r.Note))
			}
			return tw.Flush()
		},
	}
	remove := &cobra.Command{
		Use:         "remove <reference>",
		Short:       "Remove a listing from the local shortlist by reference (its note is dropped too)",
		Example:     "  immovlan-pp-cli shortlist remove vbe69761",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:local-write": "true", "pp:happy-args": "reference=vbe69761"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "remove from the shortlist")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("give one listing reference"))
			}
			ref, err := immovlan.ParseReference(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.SetShortlist(ctx, ref, false, ""); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), map[string]any{"id": ref, "shortlisted": false}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from the shortlist.\n", ref)
			return nil
		},
	}
	cmd.AddCommand(add, list, remove)
	return cmd
}

func newDumpCmd(flags *rootFlags) *cobra.Command {
	var format, deal, types, postcodes, communes, dbPath string
	var includeGone bool
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Export stored listings as CSV, GeoJSON or JSON lines",
		Long: `Export the local store (every listing find/watch/enrich saw). Columns mirror
immoweb-pp-cli dump so both portals load into the same spreadsheet or map.`,
		Example: strings.Trim(`
  immovlan-pp-cli dump --format csv --postcode 1030
  immovlan-pp-cli dump --format geojson --deal sale
  immovlan-pp-cli dump --format jsonl --include-gone
  immovlan-pp-cli dump --agent --postcode 1030`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--format=csv;--postcode=1030"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export the local store")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			f := store.ListingFilter{IncludeGone: includeGone}
			var err error
			if f.Deal, err = parseDealFlag(deal); err != nil {
				return err
			}
			for _, t := range immovlan.SplitCSV(types) {
				v, err := immovlan.NormalizeType(t)
				if err != nil {
					return usageErr(err)
				}
				f.Types = append(f.Types, v)
			}
			if f.PostalCodes, err = parsePostcodes(postcodes); err != nil {
				return err
			}
			path, ok := localStoreExists(dbPath)
			rows := []store.StoredListing{}
			if ok {
				db, err := openVlanStore(ctx, path)
				if err != nil {
					return err
				}
				defer db.Close()
				for _, name := range immovlan.SplitCSV(communes) {
					pcs, town, err := resolveCommune(ctx, db, name)
					if err != nil {
						return err
					}
					if town != "" {
						// the store keys on postcodes; a <postcode>-<slug> town resolves to its postcode
						pcs = immovlan.TownsPostcodes([]string{town})
					}
					f.PostalCodes = append(f.PostalCodes, pcs...)
				}
				if rows, err = db.QueryVlanListings(ctx, f); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), noStoreNote(path, "find --type maison --deal sale --postcode 1030"))
			}
			w := cmd.OutOrStdout()
			// --json / --agent always win: an agent that also passes --format gets the JSON array.
			if flags.asJSON || flags.agent {
				return printView(w, rows, flags)
			}
			switch format {
			case "csv":
				return writeVlanCSV(w, rows)
			case "jsonl":
				enc := json.NewEncoder(w)
				for _, r := range rows {
					if err := enc.Encode(r); err != nil {
						return err
					}
				}
				return nil
			case "geojson":
				feats := []map[string]any{}
				for _, r := range rows {
					if r.Lat == nil || r.Lng == nil {
						continue
					}
					feats = append(feats, map[string]any{"type": "Feature", "geometry": map[string]any{"type": "Point", "coordinates": []float64{*r.Lng, *r.Lat}}, "properties": r})
				}
				return json.NewEncoder(w).Encode(map[string]any{"type": "FeatureCollection", "features": feats})
			}
			return usageErr(fmt.Errorf("unknown --format %q (csv, geojson or jsonl)", format))
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "Output format: csv, geojson or jsonl (--json/--agent print a JSON array unless --format is set)")
	cmd.Flags().StringVar(&deal, "deal", "", "Only this deal (sale, rent, public-sale, colocation)")
	cmd.Flags().StringVar(&types, "type", "", "Only these property types, comma-separated")
	cmd.Flags().StringVar(&postcodes, "postcode", "", "Only these postal codes, comma-separated")
	cmd.Flags().StringVar(&communes, "commune", "", "Only these communes (Brussels names or stored localities)")
	cmd.Flags().BoolVar(&includeGone, "include-gone", false, "Include listings that left Immovlan")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

func newLocationsCmd(flags *rootFlags) *cobra.Command {
	var query, dbPath string
	cmd := &cobra.Command{
		Use:   "locations",
		Short: "Resolve a commune name to postal codes (Brussels offline; elsewhere from stored listings) or list what the store knows",
		Example: strings.Trim(`
  immovlan-pp-cli locations --query "saint-josse" --agent
  immovlan-pp-cli locations --agent`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--query=schaerbeek"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "resolve commune names")
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			type loc struct {
				Name      string   `json:"name"`
				Postcodes []string `json:"postal_codes"`
				Towns     []string `json:"towns,omitempty"`
				Source    string   `json:"source"`
			}
			out := []loc{}
			if query != "" {
				if pcs := immovlan.BrusselsPostcodes(query); pcs != nil {
					towns := []string{}
					for _, pc := range pcs {
						towns = append(towns, immovlan.TownSlug(pc, immovlan.BrusselsCommune(pc)))
					}
					out = append(out, loc{Name: immovlan.BrusselsCommune(pcs[0]), Postcodes: pcs, Towns: towns, Source: "brussels-table"})
				} else if path, ok := localStoreExists(dbPath); ok {
					db, err := openVlanStore(ctx, path)
					if err != nil {
						return err
					}
					defer db.Close()
					pcs, err := db.LocalPostcodesFor(ctx, query)
					if err != nil {
						return err
					}
					if len(pcs) > 0 {
						towns := []string{}
						for _, pc := range pcs {
							towns = append(towns, immovlan.TownSlug(pc, query))
						}
						out = append(out, loc{Name: query, Postcodes: pcs, Towns: towns, Source: "local-store"})
					}
				}
				if len(out) == 0 {
					return notFoundErr(fmt.Errorf("unknown commune %q: use --postcode, or --commune <postcode>-<slug> (e.g. 4000-liege)", query))
				}
			} else {
				for _, pc := range immovlan.BrusselsPostcodeList() {
					out = append(out, loc{Name: immovlan.BrusselsCommune(pc), Postcodes: []string{pc}, Towns: []string{immovlan.TownSlug(pc, immovlan.BrusselsCommune(pc))}, Source: "brussels-table"})
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), struct {
					Results []loc `json:"results"`
				}{out}, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tPOSTCODES\tTOWN SLUGS\tSOURCE")
			for _, l := range out {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", l.Name, strings.Join(l.Postcodes, ","), strings.Join(l.Towns, ","), l.Source)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Commune name to resolve")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}
