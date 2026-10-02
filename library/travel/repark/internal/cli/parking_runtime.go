// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/repark"
	"github.com/spf13/cobra"
)

func parkingAnnotations(happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": happy}
}
func parkingContext(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	bounded, stop := context.WithTimeout(ctx, 60*time.Second)
	return bounded, func() { stop(); cancel() }
}
func parkingError(cmd *cobra.Command, flags *rootFlags, err error) error {
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		err = rateLimitErr(err)
	} else if errors.Is(err, repark.ErrNotFound) {
		err = notFoundErr(err)
	} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		err = &cliError{code: 6, err: err}
	} else {
		err = apiErr(err)
	}
	return classifyAPIError(cmd.OutOrStdout(), err, flags)
}
func parkingClient(flags *rootFlags, budget int) *repark.Client {
	return repark.New(flags.timeout, flags.rateLimit, budget)
}

// Parking owns a provenance envelope whose results may be a detail object with
// nested arrays. Project the results before wrapping so the generic list-
// envelope fallback cannot mistake tariff arrays for the command's row list.
func parkingPrint(cmd *cobra.Command, flags *rootFlags, meta repark.Meta, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	copyFlags := *flags
	var selectErr error
	if flags.selectFields != "" {
		raw, selectErr = filterFieldsChecked(raw, flags.selectFields)
		copyFlags.selectFields = ""
		copyFlags.compact = false
	}
	if err := copyFlags.printJSON(cmd, struct {
		Meta    repark.Meta     `json:"meta"`
		Results json.RawMessage `json:"results"`
	}{meta, raw}); err != nil {
		return err
	}
	return selectErr
}

type parkingDimensions struct{ height, length, width, weight float64 }

func (d *parkingDimensions) add(cmd *cobra.Command) {
	cmd.Flags().Float64Var(&d.height, "height", 0, "Supplied vehicle height in metres; compares published limits only")
	cmd.Flags().Float64Var(&d.length, "length", 0, "Supplied vehicle length in metres; compares published limits only")
	cmd.Flags().Float64Var(&d.width, "width", 0, "Supplied vehicle width in metres; compares published limits only")
	cmd.Flags().Float64Var(&d.weight, "weight", 0, "Supplied vehicle weight in tonnes; compares published limits only")
}
func (d *parkingDimensions) vehicle(cmd *cobra.Command) (repark.Vehicle, error) {
	v := repark.Vehicle{}
	for _, x := range []struct {
		name   string
		value  *float64
		target **float64
	}{{"height", &d.height, &v.HeightM}, {"length", &d.length, &v.LengthM}, {"width", &d.width, &v.WidthM}, {"weight", &d.weight, &v.WeightT}} {
		if cmd.Flags().Changed(x.name) {
			*x.target = x.value
		}
	}
	return v, repark.ValidateVehicle(v)
}

type parkingDiscoveryFlags struct {
	radius, limit, maxScan int
	available, within      bool
	dimensions             parkingDimensions
}

func (d *parkingDiscoveryFlags) add(cmd *cobra.Command) {
	cmd.Flags().IntVar(&d.radius, "radius", 1000, "Search radius in metres, 50..2000; provider coverage remains unverified")
	cmd.Flags().IntVar(&d.limit, "limit", 10, "Maximum matching lots returned, 1..50")
	cmd.Flags().IntVar(&d.maxScan, "max-scan-records", 500, "Maximum source marker rows examined, 1..1000; independent of --limit")
	cmd.Flags().BoolVar(&d.available, "available-only", false, "Keep source available/crowded categories; remaining-bay fit stays unknown")
	cmd.Flags().BoolVar(&d.within, "within-limits-only", false, "Keep lots within supplied published limits; does not guarantee any suitable bay")
	d.dimensions.add(cmd)
}
func (d *parkingDiscoveryFlags) options(cmd *cobra.Command) (repark.Options, error) {
	v, err := d.dimensions.vehicle(cmd)
	if err != nil {
		return repark.Options{}, err
	}
	o := repark.Options{RadiusM: d.radius, Limit: d.limit, MaxScanRecords: d.maxScan, AvailableOnly: d.available, WithinLimitsOnly: d.within, Vehicle: v}
	return o, repark.ValidateOptions(o)
}

func newReparkDetailCmd(flags *rootFlags) *cobra.Command {
	var dims parkingDimensions
	cmd := &cobra.Command{Use: "detail [lot-id-or-url]", Short: "Read source occupancy, opening hours, vehicle limits and full tariff conditions", Long: "Inspect a Repark lot by canonical REP ID or detail URL. Day-type labels and cap conditions come from the provider. Vacancy and supplied published-limit checks remain separate; source import times are not claimed as measurement times.", Example: "  repark-pp-cli parking detail REP0022209 --agent\n  repark-pp-cli parking detail REP0022209 --height 2.1 --json", Annotations: parkingAnnotations("lot=REP0022209"), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "parking detail: one public source GET")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("detail requires one REP lot ID or canonical detail URL"))
		}
		id, err := repark.CanonicalID(args[0])
		if err != nil {
			return usageErr(err)
		}
		v, err := dims.vehicle(cmd)
		if err != nil {
			return usageErr(err)
		}
		ctx, cancel := parkingContext(cmd, flags)
		defer cancel()
		c := parkingClient(flags, 1)
		lot, err := c.Detail(ctx, id)
		if err != nil {
			return parkingError(cmd, flags, err)
		}
		lot.Fit = repark.AssessFit(lot.Limits, v)
		return parkingPrint(cmd, flags, c.Meta(), lot)
	}}
	dims.add(cmd)
	return cmd
}

func newReparkNearbyCmd(flags *rootFlags) *cobra.Command {
	var lat, lon float64
	var park string
	var opts parkingDiscoveryFlags
	var excludeAnchor bool
	cmd := &cobra.Command{Use: "nearby", Short: "Find source parking near explicit coordinates or a known lot", Long: "Supply both --lat and --lon, or --park for a lot's source coordinates. Distances are calculated straight-line metres, not walking/driving routes. --available-only accepts source available and crowded categories; size-restricted remaining bays may still be unsuitable.", Example: "  repark-pp-cli parking nearby --park REP0022209 --limit 3 --agent\n  repark-pp-cli parking nearby --lat 34.663534 --lon 135.516310 --available-only --height 2.1 --agent", Annotations: parkingAnnotations("--park=REP0022209;--limit=3"), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "parking nearby: one marker GET, plus one detail GET when --park anchors the search")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("nearby accepts --lat/--lon or --park, with no positional arguments"))
		}
		a, b := cmd.Flags().Changed("lat"), cmd.Flags().Changed("lon")
		if a != b || ((a || b) && (park != "")) || (!a && park == "") {
			return usageErr(fmt.Errorf("supply exactly both --lat and --lon, or --park"))
		}
		o, err := opts.options(cmd)
		if err != nil {
			return usageErr(err)
		}
		if a {
			if err = repark.ValidateCoordinates(repark.Coordinates{Latitude: lat, Longitude: lon}); err != nil {
				return usageErr(err)
			}
		}
		id := ""
		if park != "" {
			id, err = repark.CanonicalID(park)
			if err != nil {
				return usageErr(err)
			}
		}
		ctx, cancel := parkingContext(cmd, flags)
		defer cancel()
		budget := 1
		if id != "" {
			budget = 2
		}
		c := parkingClient(flags, budget)
		p := repark.Coordinates{Latitude: lat, Longitude: lon}
		if id != "" {
			l, err := c.Detail(ctx, id)
			if err != nil {
				return parkingError(cmd, flags, err)
			}
			if l.Coordinates == nil {
				return apiErr(fmt.Errorf("source lot has no verified coordinates; provide explicit --lat/--lon"))
			}
			p = *l.Coordinates
			if excludeAnchor {
				o.ExcludeID = id
			}
		}
		v, err := c.Nearby(ctx, p, o)
		if err != nil {
			return parkingError(cmd, flags, err)
		}
		if id != "" {
			v.AnchorKind = "source_lot_coordinates"
		}
		return parkingPrint(cmd, flags, c.Meta(), v)
	}}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Explicit WGS84 latitude of the desired search anchor")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Explicit WGS84 longitude of the desired search anchor")
	cmd.Flags().StringVar(&park, "park", "", "Canonical lot ID or detail URL whose source coordinates anchor the search")
	cmd.Flags().BoolVar(&excludeAnchor, "exclude-anchor", false, "Exclude the anchor lot from a --park search")
	opts.add(cmd)
	return cmd
}

func newReparkSearchCmd(flags *rootFlags) *cobra.Command {
	var opts parkingDiscoveryFlags
	cmd := &cobra.Command{Use: "search [query]", Short: "Resolve a named place, station or area through Repark and discover nearby lots", Long: "Search the provider's freeword interface with a named place or address. Repark may select a coordinate anchor or return choices; unresolved queries return needs_refinement with source candidates. No driver location is inferred.", Example: "  repark-pp-cli parking search 東京駅 --limit 3 --agent\n  repark-pp-cli parking search 大阪市天王寺区上汐 --radius 500 --available-only --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "query=東京駅;--limit=3"}, RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "parking search: source freeword resolution and one bounded marker GET (at most three HTTP requests)")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("search requires one quoted place, station or address"))
		}
		if err := repark.ValidateQuery(args[0]); err != nil {
			return usageErr(err)
		}
		o, err := opts.options(cmd)
		if err != nil {
			return usageErr(err)
		}
		ctx, cancel := parkingContext(cmd, flags)
		defer cancel()
		c := parkingClient(flags, 3)
		v, err := c.Search(ctx, args[0], o)
		if err != nil {
			return parkingError(cmd, flags, err)
		}
		return parkingPrint(cmd, flags, c.Meta(), v)
	}}
	opts.add(cmd)
	return cmd
}

func newReparkQuoteCmd(flags *rootFlags) *cobra.Command {
	var bay int
	var from, to string
	cmd := &cobra.Command{Use: "quote [lot-id-or-url]", Short: "Ask the verified provider calculator for an explicit bay and exact JST interval", Long: "Fetch the public lot and calculator form, then submit only the source's fee simulation. A quote uses current source tariff information, excludes partner discounts, and does not guarantee the final billed charge. The usual source stay limit is 48 hours. Use an actual marked bay; calculation does not establish its vacancy or suitability.", Example: "  repark-pp-cli parking quote REP0022209 --bay 1 --start 2026-10-03T08:00 --end 2026-10-03T12:00 --agent", Annotations: parkingAnnotations("lot=REP0022209;--bay=1;--start=2026-10-03T08:00;--end=2026-10-03T12:00"), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "parking quote: two public GETs and one read-only provider simulation POST; no reservation or payment")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("quote requires one lot ID plus --bay, --start and --end"))
		}
		id, err := repark.CanonicalID(args[0])
		if err != nil {
			return usageErr(err)
		}
		start, err := repark.ParseJST(from)
		if err != nil {
			return usageErr(fmt.Errorf("--start: %w", err))
		}
		end, err := repark.ParseJST(to)
		if err != nil {
			return usageErr(fmt.Errorf("--end: %w", err))
		}
		if err = repark.ValidateQuote(bay, start, end, time.Now()); err != nil {
			return usageErr(err)
		}
		ctx, cancel := parkingContext(cmd, flags)
		defer cancel()
		c := parkingClient(flags, 3)
		v, err := c.Quote(ctx, id, bay, start, end)
		if err != nil {
			return parkingError(cmd, flags, err)
		}
		return parkingPrint(cmd, flags, c.Meta(), v)
	}}
	cmd.Flags().IntVar(&bay, "bay", 0, "Actual marked bay number, required; source choices range from 1..999")
	cmd.Flags().StringVar(&from, "start", "", "Exact entry YYYY-MM-DDTHH:MM in JST, or RFC3339 with zero seconds")
	cmd.Flags().StringVar(&to, "end", "", "Exact exit YYYY-MM-DDTHH:MM in JST, or RFC3339 with zero seconds")
	return cmd
}

func newReparkCompareCmd(flags *rootFlags) *cobra.Command {
	var dims parkingDimensions
	cmd := &cobra.Command{Use: "compare [lot-id-or-url] [lot-id-or-url] ...", Short: "Compare source rates, caps, hours and vehicle limits across 2..5 chosen lots", Long: "Compare full source facts for known lots. Each failed fetch is reported separately, and no guessed price total or phantom zero row enters the comparison. Use parking quote for a specified bay and interval.", Example: "  repark-pp-cli parking compare REP0022209 REP0029431 --height 2.1 --agent", Annotations: parkingAnnotations("first=REP0022209;second=REP0029431"), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "parking compare: at most five public lot detail GETs")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) < 2 || len(args) > 5 {
			return usageErr(fmt.Errorf("compare requires 2..5 distinct lot IDs or canonical detail URLs"))
		}
		ids := []string{}
		seen := map[string]bool{}
		for _, a := range args {
			id, e := repark.CanonicalID(a)
			if e != nil {
				return usageErr(e)
			}
			if seen[id] {
				return usageErr(fmt.Errorf("duplicate lot %s; compare distinct IDs", id))
			}
			seen[id] = true
			ids = append(ids, id)
		}
		v, err := dims.vehicle(cmd)
		if err != nil {
			return usageErr(err)
		}
		ctx, cancel := parkingContext(cmd, flags)
		defer cancel()
		c := parkingClient(flags, len(ids))
		lots := []repark.Lot{}
		failures := []map[string]string{}
		for _, id := range ids {
			l, err := c.Detail(ctx, id)
			if err != nil {
				var rate *cliutil.RateLimitError
				if errors.As(err, &rate) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					return parkingError(cmd, flags, err)
				}
				failures = append(failures, map[string]string{"id": id, "error": err.Error()})
				continue
			}
			l.Fit = repark.AssessFit(l.Limits, v)
			lots = append(lots, l)
		}
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d source fetches failed; comparison contains %d successful lots\n", len(failures), len(ids), len(lots))
		}
		meta := c.Meta()
		if len(failures) > 0 {
			meta.Coverage = "partial: failed lots excluded"
		}
		out := struct {
			Meta    repark.Meta `json:"meta"`
			Results struct {
				Lots       []repark.Lot        `json:"lots"`
				Failures   []map[string]string `json:"fetch_failures"`
				Requested  int                 `json:"requested_lots"`
				Successful int                 `json:"successful_lots"`
			} `json:"results"`
		}{Meta: meta}
		out.Results.Lots = lots
		out.Results.Failures = failures
		out.Results.Requested = len(ids)
		out.Results.Successful = len(lots)
		if err := parkingPrint(cmd, flags, out.Meta, out.Results); err != nil {
			return err
		}
		if len(lots) == 0 {
			return apiErr(fmt.Errorf("every requested source lot failed; inspect fetch_failures"))
		}
		return nil
	}}
	dims.add(cmd)
	return cmd
}
