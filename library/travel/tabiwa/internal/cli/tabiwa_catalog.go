// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/tabiwa"
	"github.com/spf13/cobra"
	"path/filepath"
)

const catalogNote = "Catalog summaries and area tags do not confirm included routes, passenger-specific fares, complete redemption terms, operating status, seats or purchasability. Open the canonical product page for final terms."

type catalogOptions struct {
	region, query, kind, category, prefecture, area, on string
	limit, maxRecords                                   int
	save                                                bool
}
type catalogCoverage struct {
	Scanned       int  `json:"scanned_records"`
	ResponseCount int  `json:"source_records"`
	ScanCap       int  `json:"scan_cap"`
	Complete      bool `json:"scan_complete"`
	Returned      int  `json:"returned_products"`
	Truncated     bool `json:"output_truncated"`
}
type catalogResult struct {
	Products          []tabiwa.Product `json:"products"`
	Coverage          catalogCoverage  `json:"coverage"`
	SourceURL         string           `json:"source_url"`
	ObservedAt        string           `json:"observed_at"`
	RequestedDate     string           `json:"requested_date,omitempty"`
	MembershipMeaning string           `json:"date_filter_meaning"`
	Note              string           `json:"note"`
	Saved             bool             `json:"saved"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, _ := root.Find([]string{"catalog"})
		if parent == nil {
			return
		}
		// The promoted endpoint at `catalog` is also bounded and normalized.
		bindCatalogSearch(parent, flags, true)
		geo, _, _ := root.Find([]string{"geography"})
		if geo != nil {
			bindGeography(geo, flags)
			geo.AddCommand(newGeographyList(flags))
		}
		bindLocalFrameworkSearch(root, flags)
		// Per-CLI metadata overrides preserve the generated framework callbacks.
		for _, metadata := range []struct {
			path  []string
			short string
		}{
			{[]string{"analytics"}, "Count generic synced records by --type or group by --group-by with --limit; returns counts from the framework store, not selected catalog evidence"},
			{[]string{"learnings", "candidates"}, "List quarantined local improvement candidates filtered by --class, --status and --limit; returns IDs, payloads and observation clocks for later judgment"},
		} {
			if command, _, err := root.Find(metadata.path); err == nil && command != nil {
				command.Short = metadata.short
			}
		}
	})
}
func catalogClient(flags *rootFlags) (*tabiwa.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	return tabiwa.New(cfg.BaseURL, flags.rateLimit), nil
}
func savedPath() (string, error) {
	dir, err := cliutil.CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tabiwa.SavedFilename), nil
}
func catalogAnnotate(cmd *cobra.Command, happy string, local bool) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	delete(cmd.Annotations, "pp:novel-scaffold")
	cmd.Annotations["pp:happy-args"] = happy
	cmd.Annotations["pp:data-source"] = "live"
	cmd.Annotations["mcp:read-only"] = "true"
	delete(cmd.Annotations, "mcp:local-write")
	delete(cmd.Annotations, "pp:live-happy-path")
	if cmd.Flags().Lookup("save") != nil {
		// Provider operations remain GET-only, but the accepted save=true
		// argument can change persistent selected evidence in the private cache.
		cmd.Annotations["mcp:read-only"] = "false"
		cmd.Annotations["mcp:local-write"] = "true"
		cmd.Annotations["pp:live-happy-path"] = "true"
		cmd.Annotations["pp:happy-args"] = happy + ";--save=true"
	}
	if local {
		cmd.Annotations["pp:data-source"] = "local"
	}
}
func addRegion(cmd *cobra.Command, o *catalogOptions) {
	if cmd.Flags().Lookup("region") != nil {
		return
	}
	cmd.Flags().StringVar(&o.region, "region", "10", "Regional catalog: 10 せとうち, 20 北陸, 30 山陰, 40 九州; a public display preference")
}
func addSave(cmd *cobra.Command, o *catalogOptions) {
	cmd.Flags().BoolVar(&o.save, "save", false, "Save only returned normalized evidence locally; never purchases a ticket")
}
func query(o *catalogOptions) tabiwa.Query {
	return tabiwa.Query{Region: o.region, Type: o.kind, Category: o.category, Prefecture: o.prefecture, Area: o.area, Date: o.on}
}
func checkLive(cmd *cobra.Command, flags *rootFlags, o *catalogOptions) error {
	if err := validateDataSourceStrategy(flags, "live"); err != nil {
		return catalogUsage(err.Error())
	}
	if o.save && flags.noCache {
		return catalogUsage("--save conflicts with --no-cache; remove one")
	}
	return nil
}
func saveProducts(ctx context.Context, flags *rootFlags, o *catalogOptions, p []tabiwa.Product) error {
	if !o.save {
		return nil
	}
	path, err := savedPath()
	if err != nil {
		return err
	}
	return tabiwa.Save(ctx, path, p)
}

func bindCatalogSearch(cmd *cobra.Command, flags *rootFlags, promoted bool) {
	o := &catalogOptions{region: "10", limit: 10, maxRecords: 500}
	if promoted {
		cmd.Short = "Discover regional catalog summaries with optional private evidence saves; date filters are not availability"
		cmd.Example = "  tabiwa-pp-cli catalog --region 20 --category transportation --limit 3 --save=true --agent"
	}
	if !promoted {
		cmd.Use = "search [query]"
		cmd.Short = "Discover bounded catalog summaries with optional private evidence saves; a date filter is not availability"
		cmd.Example = "  tabiwa-pp-cli catalog search --region 20 --category transportation --on 2026-10-28 --limit 3 --save=true --agent"
	}
	addRegion(cmd, o)
	addSave(cmd, o)
	cmd.Flags().StringVar(&o.query, "query", "", "Case-insensitive text contained in the Japanese name or catalog overview; at most 100 characters")
	cmd.Flags().IntVar(&o.limit, "limit", 10, "Maximum returned normalized products, from 1 to 50")
	cmd.Flags().IntVar(&o.maxRecords, "max-records", 500, "Maximum source catalog records scanned locally, from 1 to 1000; independent of --limit")
	if !promoted {
		cmd.Flags().StringVar(&o.kind, "type", "", "Provider product type: freepass, ticket, multi-coupon or discount-coupon")
		cmd.Flags().StringVar(&o.category, "category", "", "Provider category: transportation, tourism_experience, gourmet or other")
		cmd.Flags().StringVar(&o.prefecture, "prefecture", "", "Provider prefecture ID from geography list; not a national code")
		cmd.Flags().StringVar(&o.area, "area", "", "Provider area ID from geography list; area tags do not guarantee pass coverage")
		cmd.Flags().StringVar(&o.on, "on", "", "Requested Japan date YYYY-MM-DD; only filters the published catalog")
	}
	happy := "--region=20;--category=transportation;--limit=3"
	if !promoted {
		// The live matrix needs a real optional positional query fixture.
		happy = "query=立山;--region=20;--category=transportation;--on=2026-10-28;--limit=3"
	}
	catalogAnnotate(cmd, happy, false)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bounded public catalog search")
		}
		o.region, _ = cmd.Flags().GetString("region")
		if promoted {
			for flag, target := range map[string]*string{"type": &o.kind, "category": &o.category, "prefecture": &o.prefecture, "area": &o.area, "on": &o.on} {
				*target, _ = cmd.Flags().GetString(flag)
			}
		}
		if len(args) > 1 {
			return catalogUsage("catalog search accepts at most one text query")
		}
		if len(args) == 1 {
			if o.query != "" {
				return catalogUsage("use either a positional query or --query")
			}
			o.query = args[0]
		}
		if o.limit < 1 || o.limit > 50 || o.maxRecords < 1 || o.maxRecords > tabiwa.MaxRecords {
			return catalogUsage("--limit must be 1..50 and --max-records must be 1..1000")
		}
		if err := tabiwa.Validate(query(o)); err != nil {
			return catalogUsage(err.Error())
		}
		if err := tabiwa.ValidateQuery(o.query); err != nil {
			return catalogUsage(err.Error())
		}
		if err := checkLive(cmd, flags, o); err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := catalogClient(flags)
		if err != nil {
			return err
		}
		rows, u, observed, err := c.Search(ctx, query(o))
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		scanned := min(len(rows), o.maxRecords)
		products := []tabiwa.Product{}
		matches := 0
		for _, r := range rows[:scanned] {
			if !tabiwa.Match(r, o.query) {
				continue
			}
			matches++
			if len(products) < o.limit {
				products = append(products, tabiwa.Normalize(r, o.region, o.on, u, observed))
			}
		}
		if err = saveProducts(ctx, flags, o, products); err != nil {
			return err
		}
		result := catalogResult{Products: products, Coverage: catalogCoverage{scanned, len(rows), o.maxRecords, scanned == len(rows), len(products), matches > len(products)}, SourceURL: u, ObservedAt: observed, RequestedDate: o.on, MembershipMeaning: "published_catalog_filter_only; availability_unknown", Note: catalogNote, Saved: o.save}
		if len(products) == 0 {
			result.Note = "No matching catalog summary was found within the scanned records and selected filters. " + catalogNote
		}
		return flags.printJSON(cmd, result)
	}
}

func newCatalogInspect(flags *rootFlags) *cobra.Command {
	o := &catalogOptions{}
	cmd := &cobra.Command{Use: "inspect [product-id]", Short: "Read exact catalog identity, price units and overview restriction cues; optionally save private evidence; full terms stay unknown", Example: "  tabiwa-pp-cli catalog inspect J0000900 --region 10 --save=true --agent"}
	addRegion(cmd, o)
	addSave(cmd, o)
	cmd.Flags().StringVar(&o.on, "on", "", "Requested Japan date YYYY-MM-DD; presence only means dated catalog membership")
	catalogAnnotate(cmd, "id=J0000900;--region=10", false)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "public catalog inspect")
		}
		if len(args) == 0 && !hasChangedLocalFlags(cmd) {
			return cmd.Help()
		}
		if len(args) != 1 {
			return catalogUsage("catalog inspect needs one product ID from catalog search")
		}
		q := query(o)
		q.IDs = args
		if err := tabiwa.Validate(q); err != nil {
			return catalogUsage(err.Error())
		}
		if err := checkLive(cmd, flags, o); err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := catalogClient(flags)
		if err != nil {
			return err
		}
		rows, u, observed, err := c.Search(ctx, q)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		products := []tabiwa.Product{}
		for _, r := range rows {
			if r.ID == args[0] {
				products = append(products, tabiwa.Normalize(r, o.region, o.on, u, observed))
			}
		}
		if err = saveProducts(ctx, flags, o, products); err != nil {
			return err
		}
		state := "not_listed_in_selected_catalog"
		if len(products) > 0 {
			state = "listed"
		}
		return flags.printJSON(cmd, map[string]any{"products": products, "requested_id": args[0], "catalog_membership": state, "requested_date": o.on, "source_url": u, "observed_at": observed, "availability": "unknown", "note": catalogNote, "saved": o.save})
	}
	return cmd
}

func newCatalogCompare(flags *rootFlags) *cobra.Command {
	o := &catalogOptions{}
	var ids []string
	cmd := &cobra.Command{Use: "compare [product-ids...]", Short: "Compare at most five catalog products with separate price units and dated membership; optionally save private evidence", Example: "  tabiwa-pp-cli catalog compare J0001900 J0000900 --region 10 --on 2026-10-28 --save=true --agent"}
	addRegion(cmd, o)
	addSave(cmd, o)
	cmd.Flags().StringVar(&o.on, "on", "", "Requested Japan date YYYY-MM-DD; dated absence is not sold-out, closed or unavailable")
	cmd.Flags().StringSliceVar(&ids, "ids", nil, "Two to five distinct product IDs separated by commas; use instead of positional IDs")
	catalogAnnotate(cmd, "first=J0001900;second=J0000900;--region=10;--on=2026-10-28", false)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bounded public catalog comparison")
		}
		if len(ids) > 0 {
			if len(args) > 0 {
				return catalogUsage("use positional product IDs or --ids, not both")
			}
			args = ids
		}
		if len(args) == 0 && !hasChangedLocalFlags(cmd) {
			return cmd.Help()
		}
		if len(args) < 2 || len(args) > 5 {
			return catalogUsage("catalog compare needs two to five distinct product IDs")
		}
		seen := map[string]bool{}
		for _, id := range args {
			if seen[id] {
				return catalogUsage("catalog compare requires distinct product IDs")
			}
			seen[id] = true
		}
		q := query(o)
		q.IDs = args
		if err := tabiwa.Validate(q); err != nil {
			return catalogUsage(err.Error())
		}
		if err := checkLive(cmd, flags, o); err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := catalogClient(flags)
		if err != nil {
			return err
		}
		q.Date = ""
		rows, u, observed, err := c.Search(ctx, q)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		indexed := map[string]tabiwa.Product{}
		for _, r := range rows {
			if seen[r.ID] {
				indexed[r.ID] = tabiwa.Normalize(r, o.region, "", u, observed)
			}
		}
		dated := map[string]bool{}
		dateObserved := ""
		dateSource := ""
		if o.on != "" {
			q.Date = o.on
			dr, du, dt, e := c.Search(ctx, q)
			if e != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("requested-date catalog failed; comparison is incomplete: %w", e), flags)
			}
			dateObserved = dt
			dateSource = du
			for _, r := range dr {
				dated[r.ID] = true
			}
		}
		comparisons := []map[string]any{}
		products := []tabiwa.Product{}
		for _, id := range args {
			p, ok := indexed[id]
			membership := "not_listed_in_selected_catalog"
			var product any = nil
			if ok {
				membership = "listed"
				product = p
				products = append(products, p)
			}
			dm := "not_requested"
			if o.on != "" {
				dm = "not_listed"
				if dated[id] {
					dm = "listed"
				}
			}
			comparisons = append(comparisons, map[string]any{"id": id, "product": product, "catalog_membership": membership, "requested_date_membership": dm, "availability": "unknown"})
		}
		if err = saveProducts(ctx, flags, o, products); err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"comparisons": comparisons, "requested_date": o.on, "source_url": u, "observed_at": observed, "dated_source_url": dateSource, "dated_observed_at": dateObserved, "comparison_rule": "Units and unknown price bases are preserved; no points/cash conversion, savings estimate or price ranking.", "note": catalogNote, "saved": o.save})
	}
	return cmd
}

func bindGeography(cmd *cobra.Command, flags *rootFlags) {
	o := &catalogOptions{}
	addRegion(cmd, o)
	catalogAnnotate(cmd, "--region=20", false)
	cmd.Annotations["mcp:read-only"] = "true"
	delete(cmd.Annotations, "mcp:local-write")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "public regional geography")
		}
		if len(args) > 0 {
			return catalogUsage("geography list does not accept positional arguments")
		}
		if _, err := tabiwa.Region(o.region); err != nil {
			return catalogUsage(err.Error())
		}
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return catalogUsage(err.Error())
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := catalogClient(flags)
		if err != nil {
			return err
		}
		rows, u, observed, err := c.Geography(ctx, o.region)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		reg, _ := tabiwa.Region(o.region)
		return flags.printJSON(cmd, map[string]any{"region": reg, "prefectures": rows, "source_url": u, "observed_at": observed, "note": "These are provider identifiers and geographic tags; they do not prove included pass routes."})
	}
}
func newGeographyList(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List provider prefecture and area identifiers for --region; use IDs as discovery filters, not as proof of pass coverage", Example: "  tabiwa-pp-cli geography list --region 20 --agent"}
	bindGeography(cmd, flags)
	return cmd
}

func catalogUsage(message string) error { return usageErr(fmt.Errorf("%s", message)) }
