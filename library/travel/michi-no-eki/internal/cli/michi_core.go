// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
	"github.com/spf13/cobra"
	"math"
	"strings"
	"sync"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, cmd := range []*cobra.Command{newMichiFindCmd(flags, false), newMichiStationCmd(flags, false), newMichiNoticesCmd(flags), newMichiNoticeCmd(flags), newMichiCatalogCmd(flags), newMichiGuidanceCmd(flags), newMichiSnapshotCmd(flags)} {
			addNovelCommandIfAbsent(root, cmd)
		}
	})
}
func michiAnnotations(strategy, happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": strategy, "pp:happy-args": happy}
}
func michiSource(flags *rootFlags) (*michi.Source, error) {
	if flags.dataSource == "local" {
		return nil, usageErr(fmt.Errorf("no local data source for this command; use changes for saved observations"))
	}
	c, e := flags.newClient()
	if e != nil {
		return nil, e
	}
	flags.agentSource = "live"
	var lock sync.Mutex
	return &michi.Source{BaseURL: c.RequestBaseURL(), Fetch: func(ctx context.Context, path string) ([]byte, error) {
		lock.Lock()
		defer lock.Unlock()
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		body, e := c.GetWithHeadersNoCache(ctx, path, nil, map[string]string{client.HTMLResponseHeader: "true"})
		return []byte(body), e
	}}, nil
}
func michiOutput(cmd *cobra.Command, flags *rootFlags, v any, e error) error {
	if e != nil {
		return classifyAPIErrorOnly(e)
	}
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}
func queryFlags(cmd *cobra.Command, q *michi.Query) {
	cmd.Flags().StringVar(&q.Prefecture, "prefecture", "", "Prefecture slugs, Japanese labels or official IDs, comma-separated; run catalog")
	cmd.Flags().StringVar(&q.Region, "region", "", "Provider region slugs, comma-separated; choose this or --prefecture")
	cmd.Flags().StringVar(&q.Facility, "facility", "", "Facility slugs, Japanese labels or official IDs, comma-separated; presence only")
	cmd.Flags().StringVar(&q.Keyword, "keyword", "", "Provider keyword, including Japanese station names; at most 120 characters")
	cmd.Flags().StringVar(&q.Match, "match", "all", "Facility match: all required categories (default) or any; upstream native search is any")
	cmd.Flags().IntVar(&q.Limit, "limit", 10, "Maximum station rows returned,1–50; this does not cap candidate scanning")
	cmd.Flags().IntVar(&q.MaxCandidates, "max-candidates", 2000, "Maximum parsed candidates examined,1–5000; coverage reports any cap")
}
func newMichiFindCmd(flags *rootFlags, near bool) *cobra.Command {
	var q michi.Query
	var lat, lon, radius float64
	name := "find"
	short := "Find official roadside stations by area and source-listed facilities"
	long := "Find official roadside-station service stops. Multiple prefectures are a union. The provider facility query is any-of; --match all applies a local required-facility filter. Active icons mean source-listed presence; unlisted icons do not prove physical absence. Listing never establishes overnight lodging/camping permission or live opening/charger availability."
	happy := "--prefecture=nagano;--facility=onsen;--limit=3"
	example := "  michi-no-eki-pp-cli find --prefecture nagano --facility onsen,restaurant --match all --limit 5 --agent"
	if near {
		name = "nearby"
		short = "Rank source stations by straight-line kilometers from required latitude/longitude, scoped by area and facilities"
		long = "Use this command for straight-line proximity within an explicit source-search scope. Do NOT use it to compare an existing ordered shortlist; use 'compare' instead. Distances are kilometers, not driving routes; source map coordinates do not establish vehicle access. Candidate coverage and missing coordinates are reported."
		happy += ";--lat=35.8632686;--lon=138.2769108"
		example = "  michi-no-eki-pp-cli nearby --prefecture nagano --facility onsen --lat 35.8632686 --lon 138.2769108 --limit 3 --agent"
	}
	cmd := &cobra.Command{Use: "find", Short: "Find official roadside stations by area and source-listed facilities", Long: long, Example: example, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": happy}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, name)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("%s takes filters, not positional arguments", name))
		}
		if e := michi.ValidateQuery(q); e != nil {
			return usageErr(e)
		}
		if near && (!cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon")) {
			return usageErr(fmt.Errorf("--lat and --lon are required for nearby; supply explicit itinerary coordinates"))
		}
		if near && (math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 || math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 || math.IsNaN(radius) || math.IsInf(radius, 0) || radius < 0 || radius > 2000) {
			return usageErr(fmt.Errorf("nearby requires finite --lat within -90..90, --lon within -180..180 and --radius-km within 0..2000"))
		}
		src, e := michiSource(flags)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		var result michi.Search
		if near {
			result, e = src.Nearby(ctx, q, lat, lon, radius)
		} else {
			result, e = src.Find(ctx, q)
		}
		return michiOutput(cmd, flags, result, e)
	}}
	cmd.Use = name
	cmd.Short = short
	queryFlags(cmd, &q)
	if near {
		cmd.Flags().Float64Var(&lat, "lat", 0, "Explicit latitude in decimal degrees, within -90..90")
		cmd.Flags().Float64Var(&lon, "lon", 0, "Explicit longitude in decimal degrees, within -180..180")
		cmd.Flags().Float64Var(&radius, "radius-km", 0, "Straight-line radius in kilometers,0–2000;0 means no radius cutoff")
	}
	return cmd
}
func newMichiStationCmd(flags *rootFlags, ready bool) *cobra.Command {
	name := "station"
	short := "Inspect station parking, published hours, operators and facility evidence"
	long := "Inspect canonical official station details with Japanese names, visible phone text, nominal parking capacities, published hours and source-listed facilities. Physical presence, live availability, fees and campervan fit remain separate. Use 'readiness' for an explicit evidence/unknowns checklist."
	if ready {
		name = "readiness"
		short = "Inspect source facts and permission or service unknowns for one station ID"
		long = "Use this command for listed stop evidence and explicit permission/status unknowns. Do NOT use it to find dated station notices; use 'station-notices' instead. Do NOT use it for a shortlist table; use 'compare' instead. General MLIT fatigue-recovery rest guidance is attributed separately from station-specific lodging/camping permission, which remains unknown."
	}
	cmd := &cobra.Command{Use: "station [id]", Short: "Inspect station parking, published hours, operators and facility evidence", Long: long, Example: "  michi-no-eki-pp-cli " + name + " 19187 --agent", Annotations: michiAnnotations("live", "id=19187"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, name)
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("%s requires one numeric station ID; for example %s 19187", name, name))
		}
		if e := michi.ValidateIDs(args[0]); e != nil || strings.Contains(args[0], ",") {
			if e == nil {
				e = fmt.Errorf("one station ID is required")
			}
			return usageErr(e)
		}
		src, e := michiSource(flags)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		if ready {
			v, e := src.Readiness(ctx, args[0])
			return michiOutput(cmd, flags, v, e)
		}
		v, e := src.Station(ctx, args[0])
		return michiOutput(cmd, flags, v, e)
	}}
	cmd.Short = short
	return cmd
}
func newMichiNoticesCmd(flags *rootFlags) *cobra.Command {
	var pages, limit int
	var prefecture, keyword string
	cmd := &cobra.Command{Use: "notices", Short: "List dated station notices within explicit page scan coverage", Long: "List Japanese notice titles, publication dates in Asia/Tokyo, prefectures and canonical handoff URLs. --limit caps output and --max-scan-pages caps older-page scanning separately. A zero-match scan does not mean the station has no closures or is open. Use notice for an exact linked station and bounded excerpt.", Example: "  michi-no-eki-pp-cli notices --prefecture hokkaido --max-scan-pages 2 --limit 5 --agent", Annotations: michiAnnotations("live", "--max-scan-pages=1;--limit=3"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "notices")
		}
		if len(args) > 0 || pages < 1 || pages > 5 || limit < 1 || limit > 50 {
			return usageErr(fmt.Errorf("notices requires --max-scan-pages 1–5 and --limit 1–50, with no positional arguments"))
		}
		src, e := michiSource(flags)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		v, e := src.Notices(ctx, pages, limit, prefecture, keyword)
		return michiOutput(cmd, flags, v, e)
	}}
	cmd.Flags().IntVar(&pages, "max-scan-pages", 1, "Maximum source notice pages scanned,1–5; page size is provider-owned")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum matching notice rows returned,1–50; separate from scan cap")
	cmd.Flags().StringVar(&prefecture, "prefecture", "", "Prefecture slugs, Japanese labels or IDs, comma-separated; local index filter")
	cmd.Flags().StringVar(&keyword, "keyword", "", "Case-insensitive substring of source notice title; local index filter")
	return cmd
}
func newMichiNoticeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "notice [id]", Short: "Inspect a dated notice excerpt and explicit canonical station links", Long: "Read a bounded Japanese notice excerpt and exact linked station IDs. Publication date is in Asia/Tokyo and is distinct from service/event dates in prose. The excerpt is capped at 160 characters; follow the canonical URL for full details and attachments.", Example: "  michi-no-eki-pp-cli notice 23237 --agent", Annotations: michiAnnotations("live", "id=23237"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "notice")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("notice requires one numeric notice ID, for example 23237"))
		}
		if e := michi.ValidateIDs(args[0]); e != nil || strings.Contains(args[0], ",") {
			if e == nil {
				e = fmt.Errorf("one notice ID is required")
			}
			return usageErr(e)
		}
		src, e := michiSource(flags)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		v, e := src.Notice(ctx, args[0])
		return michiOutput(cmd, flags, v, e)
	}}
}
func newMichiCatalogCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "catalog", Short: "Show official filter IDs, Japanese labels, aliases and region groupings", Long: "Read the embedded observed public filter catalog. Region groupings are the provider's, including Yamanashi in Kanto and Nagano in Chubu. Catalog observation time is preserved; it does not claim the directory is complete or unchanged since observation.", Example: "  michi-no-eki-pp-cli catalog --agent --select prefectures,facilities", Annotations: michiAnnotations("computed", ""), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "catalog")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("catalog takes no positional arguments"))
		}
		flags.agentSource = "computed"
		return printJSONFiltered(cmd.OutOrStdout(), michi.CatalogData(), flags)
	}}
}
func newMichiGuidanceCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "guidance", Short: "Read attributed MLIT rest, lodging and camping distinctions", Long: "Read general roadside-station parking guidance verified from MLIT, with source and verification date. Rest/napping for fatigue recovery, lodging use and outdoor camping are distinct. This reference gives no per-station lodging/camping permission; confirm station-specific spaces and rules.", Example: "  michi-no-eki-pp-cli guidance --agent", Annotations: michiAnnotations("computed", ""), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "guidance")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("guidance takes no positional arguments"))
		}
		flags.agentSource = "computed"
		return printJSONFiltered(cmd.OutOrStdout(), michi.GeneralGuidance(), flags)
	}}
}
func newMichiBatchCmd(flags *rootFlags, snapshot bool) *cobra.Command {
	var ids string
	name, kind, short := "compare", "comparison", "Compare up to six ordered station IDs with source facts and per-ID failures"
	long := "Use this command to compare an ordered station shortlist. Do NOT use it for a stop-evidence checklist; use 'readiness' instead. Do NOT use it for historical changes; use 'changes' instead. Fetch at most 6 unique numeric IDs, preserve input order and report failed IDs separately; nominal parking is not current space availability."
	if snapshot {
		name, kind, short = "snapshot", "snapshot", "Emit a factual ordered observation as JSON for user-directed local saving"
		long = "Emit a bounded factual observation with schema_version 1, kind snapshot, stable IDs, source URLs and observation time. The CLI writes to stdout; the user can redirect JSON to a local file. Use 'changes' to compare two saved snapshots. At most6 station IDs; incomplete fetches stay explicit."
	}
	cmd := &cobra.Command{Use: name, Short: short, Long: long, Example: "  michi-no-eki-pp-cli " + name + " --ids 19187,19189 --json", Annotations: michiAnnotations("live", "--ids=19187,19189"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, name)
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("use --ids 19187,19189 rather than positional arguments"))
		}
		if e := michi.ValidateIDs(ids); e != nil {
			return usageErr(e)
		}
		src, e := michiSource(flags)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		v, fetchErr := src.Compare(ctx, ids, kind)
		if len(v.FetchFailures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d station fetches failed; comparison includes %d successful station records\n", len(v.FetchFailures), v.RequestedCount, v.SuccessfulCount)
		}
		if e := printJSONFiltered(cmd.OutOrStdout(), v, flags); e != nil {
			return e
		}
		if fetchErr != nil {
			return classifyAPIErrorOnly(fetchErr)
		}
		return nil
	}}
	cmd.Flags().StringVar(&ids, "ids", "", "Ordered comma-separated unique official station IDs, at most 6; find supplies IDs")
	return cmd
}

func newMichiReadinessCmd(flags *rootFlags) *cobra.Command {
	delegate := newMichiStationCmd(flags, true)
	cmd := &cobra.Command{Use: "readiness [id]", Short: "Inspect source facts and permission or service unknowns for one station ID", Long: delegate.Long, Example: delegate.Example, Annotations: delegate.Annotations, RunE: delegate.RunE}
	cmd.Flags().AddFlagSet(delegate.Flags())
	return cmd
}

func newMichiCompareCmd(flags *rootFlags) *cobra.Command {
	delegate := newMichiBatchCmd(flags, false)
	cmd := &cobra.Command{Use: "compare", Short: "Compare up to six ordered station IDs with source facts and per-ID failures", Long: delegate.Long, Example: delegate.Example, Annotations: delegate.Annotations, RunE: delegate.RunE}
	cmd.Flags().AddFlagSet(delegate.Flags())
	return cmd
}

func newMichiNearbyCmd(flags *rootFlags) *cobra.Command {
	delegate := newMichiFindCmd(flags, true)
	cmd := &cobra.Command{Use: "nearby", Short: "Rank source stations by straight-line kilometers from required latitude/longitude, scoped by area and facilities", Long: delegate.Long, Example: delegate.Example, Annotations: delegate.Annotations, RunE: delegate.RunE}
	cmd.Flags().AddFlagSet(delegate.Flags())
	return cmd
}

func newMichiSnapshotCmd(flags *rootFlags) *cobra.Command {
	delegate := newMichiBatchCmd(flags, true)
	cmd := &cobra.Command{Use: "snapshot", Short: "Emit a factual ordered observation as JSON for user-directed local saving", Long: delegate.Long, Example: delegate.Example, Annotations: delegate.Annotations, RunE: delegate.RunE}
	cmd.Flags().AddFlagSet(delegate.Flags())
	return cmd
}
