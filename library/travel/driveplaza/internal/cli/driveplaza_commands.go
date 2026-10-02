package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/driveplaza"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, dpRoads(flags))
		addNovelCommandIfAbsent(root, dpConditions(flags))
	})
}

func dpAnnotations(source, happy string) map[string]string {
	a := map[string]string{"pp:data-source": source, "mcp:read-only": "true"}
	if happy != "" {
		a["pp:happy-args"] = happy
	}
	return a
}
func dpEmit(cmd *cobra.Command, flags *rootFlags, v any) error {
	// Project the domain payload while retaining its provenance envelope.
	// Every model field is already a bounded summary or requested detail.
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	var selectErr error
	if flags.selectFields != "" {
		paths := strings.Split(flags.selectFields, ",")
		for i, p := range paths {
			paths[i] = strings.TrimPrefix(strings.TrimSpace(p), "results.")
		}
		envelope["results"], selectErr = filterFieldsChecked(envelope["results"], strings.Join(paths, ","))
	}
	raw, err = json.Marshal(envelope)
	if err != nil {
		return err
	}
	var meta map[string]any
	_ = json.Unmarshal(envelope["meta"], &meta)
	was, wasCompact, wasAgent := flags.selectFields, flags.compact, flags.agent
	flags.selectFields = ""
	flags.compact = false
	// The domain already owns {meta,results}. Preserve its collection shape
	// when a projection leaves only "items"; the generic wrapper flattens it.
	flags.agent = false
	defer func() { flags.selectFields = was; flags.compact = wasCompact; flags.agent = wasAgent }()
	var output bytes.Buffer
	formatPayload := raw
	if flags.csv || flags.plain || flags.quiet {
		formatPayload = envelope["results"]
		var payload map[string]json.RawMessage
		if json.Unmarshal(formatPayload, &payload) == nil {
			if rows, ok := payload["items"]; ok {
				formatPayload = rows
			} else if rows, ok := payload["alternatives"]; ok {
				formatPayload = rows
			}
		}
		if bytes.Equal(bytes.TrimSpace(formatPayload), []byte("[]")) {
			return selectErr
		}
	}
	if err = printOutputWithFlagsMeta(&output, formatPayload, flags, meta); err != nil {
		return err
	}
	if !flags.csv && !flags.plain && !flags.quiet {
		var compact bytes.Buffer
		if err = json.Compact(&compact, output.Bytes()); err != nil {
			return err
		}
		compact.WriteByte('\n')
		output = compact
	}
	if _, err = cmd.OutOrStdout().Write(output.Bytes()); err != nil {
		return err
	}
	return selectErr
}
func dpRun(cmd *cobra.Command, args []string, flags *rootFlags, source string, required bool, fn func(context.Context, *driveplaza.Client) (any, error)) error {
	if dryRunOK(flags) {
		return writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
	}
	if required && len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.agent && !flags.asJSON {
		return cmd.Help()
	}
	if len(args) > 0 {
		return usageErr(fmt.Errorf("unexpected positional arguments; use the named flags in %s --help", cmd.CommandPath()))
	}
	if err := validateDataSourceStrategy(flags, source); err != nil {
		return usageErr(err)
	}
	if source == "computed" && flags.dataSource == "live" {
		return usageErr(fmt.Errorf("--data-source live is not supported for embedded handoff/condition catalogs; use auto or local"))
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	v, err := fn(ctx, driveplaza.New(flags.rateLimit))
	if err != nil {
		s := err.Error()
		if strings.HasPrefix(s, "--") || strings.HasPrefix(s, "use exactly") || strings.HasPrefix(s, "unknown --facility") {
			return usageErr(err)
		}
		return classifyAPIErrorOnly(err)
	}
	return dpEmit(cmd, flags, v)
}
func dpPageFlags(cmd *cobra.Command, limit, offset *int) {
	cmd.Flags().IntVar(limit, "limit", 10, "Maximum results to return (1..30); output pagination only")
	cmd.Flags().IntVar(offset, "offset", 0, "Skip this many matching records from the current source response")
}

// pp:data-source live
func dpInterchanges(flags *rootFlags) *cobra.Command {
	o := driveplaza.ICOptions{}
	cmd := &cobra.Command{Use: "interchanges", Short: "Resolve IC name queries by language/start-arrival role, or Japanese codes; return stable IDs and source names", Long: "Resolve exact source names before requesting a route. English results are joined by stable IC ID to the Japanese name endpoint. Code lookup is Japanese only and may omit road information. Partial name enrichment appears in meta.warnings; missing values are null.", Example: "  driveplaza-pp-cli interchanges --query nerima --language en --agent\n  driveplaza-pp-cli interchanges --query 練馬 --language ja --agent\n  driveplaza-pp-cli interchanges --code 1800001 --agent", Annotations: dpAnnotations("live", "--query=nerima;--language=en;--limit=3")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "live", true, func(ctx context.Context, c *driveplaza.Client) (any, error) { return c.Interchanges(ctx, o) })
	}
	cmd.Flags().StringVar(&o.Query, "query", "", "Source name query; use canonical English spelling for --language en")
	cmd.Flags().StringVar(&o.Language, "language", "ja", "Lookup language: ja or en; English rows also request Japanese names")
	cmd.Flags().StringVar(&o.Kind, "kind", "start", "Interchange use: start or arrive; source suggestions can differ")
	cmd.Flags().StringVar(&o.Code, "code", "", "Exact 7-digit interchange ID; Japanese code lookup, exclusive with --query")
	cmd.Flags().StringVar(&o.Road, "road", "", "Restrict name results to this 4-digit source road ID")
	dpPageFlags(cmd, &o.Limit, &o.Offset)
	return cmd
}

// pp:data-source live
func dpRoads(flags *rootFlags) *cobra.Command {
	var query string
	var limit, offset int
	cmd := &cobra.Command{Use: "roads", Short: "List source road IDs and English/Japanese names, optionally filtered by --query", Example: "  driveplaza-pp-cli roads --query tohoku --agent", Annotations: dpAnnotations("live", "--query=tohoku")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "live", false, func(ctx context.Context, c *driveplaza.Client) (any, error) {
			return c.Roads(ctx, query, limit, offset)
		})
	}
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive local match against road ID, English or Japanese name")
	dpPageFlags(cmd, &limit, &offset)
	return cmd
}

// pp:data-source live
func dpRoute(flags *rootFlags) *cobra.Command {
	o := driveplaza.RouteOptions{}
	var detail bool
	cmd := &cobra.Command{Use: "route", Short: "Compare source tolls, km and minutes for --from/--to English IC names and --at JST time, with vehicle and conditional ETC assumptions", Long: "Request source estimates using exact English IC names and an explicit JST departure or arrival time. Minutes must be 00,10,20,30,40,50. --payment selects the source standard/ETC/ETC2.0 column and does not recompute discounts. ETC prices depend on actual eligibility and travel. Considering-traffic times are source estimates for the requested schedule. Use --detail for directional stop and forecast links.", Example: "  driveplaza-pp-cli route --from nerima --to sendai-minami --at 2026-10-10T08:00 --vehicle standard --payment etc --agent\n  driveplaza-pp-cli route --from nerima --to sendai-minami --at 2026-10-10T08:00 --detail --agent", Annotations: dpAnnotations("live", "--from=nerima;--to=sendai-minami;--at=2026-10-10T08:00")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "live", true, func(ctx context.Context, c *driveplaza.Client) (any, error) { return c.Route(ctx, o, detail) })
	}
	cmd.Flags().StringVar(&o.From, "from", "", "Exact English departure IC name from interchanges --language en")
	cmd.Flags().StringVar(&o.To, "to", "", "Exact English arrival IC name from interchanges --kind arrive --language en")
	cmd.Flags().StringVar(&o.At, "at", "", "Planned JST time YYYY-MM-DDTHH:MM; ten-minute increments only")
	cmd.Flags().StringVar(&o.Vehicle, "vehicle", "standard", "Source class: light, standard, medium, large or extra-large")
	cmd.Flags().StringVar(&o.Priority, "priority", "time", "Source alternative ordering: time, distance or toll")
	cmd.Flags().StringVar(&o.TimeKind, "time-kind", "departure", "Meaning of --at: departure or arrival in JST")
	cmd.Flags().StringVar(&o.Payment, "payment", "standard", "Selected source toll column: standard, etc or etc2")
	cmd.Flags().StringSliceVar(&o.Via, "via", nil, "Up to five exact English waypoint IC names, in route order")
	cmd.Flags().BoolVar(&o.ExcludeUrban, "exclude-urban", false, "Set the source form option to exclude urban expressways")
	cmd.Flags().BoolVar(&o.ExcludeOrdinary, "exclude-ordinary", false, "Set the source form option to exclude ordinary roads")
	cmd.Flags().BoolVar(&detail, "detail", false, "Include directional rest-stop identities and predicted-traffic handoff URLs")
	return cmd
}

// pp:data-source live
func dpStops(flags *rootFlags) *cobra.Command {
	o := driveplaza.StopOptions{}
	cmd := &cobra.Command{Use: "list", Short: "List SA/PA records for --road with direction, facility and local-name filters; return availability, IDs and scan coverage", Long: "Fetch one road response in English and Japanese, then paginate matching summaries locally. --query is local and case-insensitive, so it avoids the English form's case-sensitive name search. Facility filter semantics are the provider's selection; gray or none icons mean unavailable, missing icons mean unknown. The source's non-East data warning and weekday/holiday caveat stay attached.", Example: "  driveplaza-pp-cli sapa list --road 1040 --direction up --limit 5 --agent\n  driveplaza-pp-cli sapa list --road 1040 --query HASUDA --facility 9010 --agent", Annotations: dpAnnotations("live", "--road=1040;--direction=up;--limit=3")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "live", true, func(ctx context.Context, c *driveplaza.Client) (any, error) { return c.Stops(ctx, o) })
	}
	cmd.Flags().StringVar(&o.Road, "road", "", "Required 4-digit road ID from roads; requests stay road-specific")
	cmd.Flags().StringVar(&o.Direction, "direction", "both", "Provider direction: up, down or both; identities remain distinct")
	cmd.Flags().StringVar(&o.Query, "query", "", "Case-insensitive local match against English or Japanese stop name")
	cmd.Flags().StringSliceVar(&o.Facilities, "facility", nil, "Source facility IDs from sapa facilities, e.g.9010,5220")
	cmd.Flags().BoolVar(&o.Open24, "open-24", false, "Request source selection for a 24-hour facility or service; not every shop")
	cmd.Flags().IntVar(&o.MaxScan, "max-scan-records", 500, "Maximum source records to examine for local matching (1..2000), separate from --limit")
	dpPageFlags(cmd, &o.Limit, &o.Offset)
	return cmd
}

// pp:data-source live
func dpDetail(flags *rootFlags) *cobra.Command {
	var id string
	cmd := &cobra.Command{Use: "detail", Short: "Read source facilities, weekday hours and nearby links for a directional --id from sapa list", Example: "  driveplaza-pp-cli sapa detail --id 1040/1040021/1 --agent", Annotations: dpAnnotations("live", "--id=1040/1040021/1")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "live", true, func(ctx context.Context, c *driveplaza.Client) (any, error) { return c.Detail(ctx, id) })
	}
	cmd.Flags().StringVar(&id, "id", "", "Directional ID from sapa list: road/area/1(up) or road/area/2(down)")
	return cmd
}

// pp:data-source live
func dpFacilities(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "facilities", Short: "List official facility-selector IDs, labels and grouped filters for sapa list --facility", Example: "  driveplaza-pp-cli sapa facilities --agent", Annotations: dpAnnotations("live", "")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "live", false, func(ctx context.Context, c *driveplaza.Client) (any, error) { return c.Facilities(ctx) })
	}
	return cmd
}

// pp:data-source live
func dpNotices(flags *rootFlags) *cobra.Command {
	var query, since string
	var limit, offset int
	cmd := &cobra.Command{Use: "notices", Short: "Read dated official notices using --query/--since and output limits; current restriction status remains unknown", Long: "Read the current official RSS response. Notices may announce plans, postponements or the lifting of a closure. active_restriction is always null because titles and publication dates do not establish present road status. Follow the source link and official live traffic handoff.", Example: "  driveplaza-pp-cli notices --limit 5 --agent\n  driveplaza-pp-cli notices --query 東北 --since 2026-09-01 --agent", Annotations: dpAnnotations("live", "--limit=5")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "live", false, func(ctx context.Context, c *driveplaza.Client) (any, error) {
			return c.Notices(ctx, query, since, limit, offset)
		})
	}
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive local match against the exact advisory title")
	cmd.Flags().StringVar(&since, "since", "", "Include source publication dates on/after YYYY-MM-DD in JST")
	// Keep these declarations local for Press's source-based skill checker.
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum notices to return (1..30); output pagination only")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip this many matching notices from the current RSS response")
	return cmd
}
