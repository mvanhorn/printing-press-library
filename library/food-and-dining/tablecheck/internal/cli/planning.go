// Planning commands preserve the normalized public contract across regeneration.
// pp:data-source live
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var newPlanningClient = func(o planner.Options) (planner.PlanningClient, error) { return planner.New(o) }

type planningFlags struct {
	refresh                           bool
	cacheDir                          string
	timeoutText                       string
	maxRequests, concurrency, retries int
	timeout                           time.Duration
}

func init() { registerNovelCommand(registerPlanningCommands) }

func registerPlanningCommands(rootCmd *cobra.Command, flags *rootFlags) {
	removeUnsupportedStoreCommands(rootCmd)
	for _, c := range rootCmd.Commands() {
		if c.Name() == "source" {
			hidePlanningSource(c)
		}
		if c.Name() == "availability" || c.Name() == "courses" || c.Name() == "booking-url" {
			rootCmd.RemoveCommand(c)
		}
	}
	rootCmd.AddCommand(newPlanningVenuesCmd(flags))
	rootCmd.AddCommand(newPlanningCuisinesCmd(flags))
	rootCmd.AddCommand(newPlanningCoursesCmd(flags))
	rootCmd.AddCommand(newPlanningAvailabilityCmd(flags))
	rootCmd.AddCommand(newPlanningBookingURLCmd(flags))
}

// Generated store commands have no supported sync/write resources or planning
// request context here. Keep them out of both CLI and Cobra-derived MCP even
// when a future generation restores their generic root registrations.
func removeUnsupportedStoreCommands(rootCmd *cobra.Command) {
	for _, cmd := range rootCmd.Commands() {
		switch cmd.Name() {
		case "import", "workflow", "sync", "search", "export":
			rootCmd.RemoveCommand(cmd)
		}
	}
}

// pp:data-source live
func newPlanningVenuesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "venues", Short: "Find restaurants by explicit location and filters.", Example: "  tablecheck-pp-cli venues search --lat 35.681236 --lon 139.767125 --radius 3000 --cuisine sushi"}
	planningGroup(cmd, flags)
	cmd.AddCommand(newPlanningVenuesSearchCmd(flags))
	cmd.AddCommand(newPlanningVenuesGetCmd(flags))
	return cmd
}

// pp:data-source live
func newPlanningCuisinesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "cuisines", Short: "Discover stable cuisine filter keys.", Example: "  tablecheck-pp-cli cuisines list --query sushi"}
	planningGroup(cmd, flags)
	cmd.AddCommand(newPlanningCuisinesListCmd(flags))
	return cmd
}

// pp:data-source live
func newPlanningCoursesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "courses", Short: "Inspect quoted courses and source conditions.", Example: "  tablecheck-pp-cli courses list sushi-tokyo81 --limit 5"}
	planningGroup(cmd, flags)
	cmd.AddCommand(newPlanningCoursesListCmd(flags))
	cmd.AddCommand(newPlanningCoursesGetCmd(flags))
	return cmd
}

// pp:data-source live
func newPlanningAvailabilityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "availability", Short: "Check party-specific venue times.", Example: "  tablecheck-pp-cli availability check sushi-tokyo81 --date 2026-09-30 --party 2"}
	planningGroup(cmd, flags)
	cmd.AddCommand(newPlanningAvailabilityCheckCmd(flags))
	cmd.AddCommand(newPlanningAvailabilityScanCmd(flags))
	return cmd
}

func hidePlanningSource(c *cobra.Command) {
	c.Hidden = true
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	c.Annotations["mcp:hidden"] = "true"
	if c.Name() == "venue" && c.Parent() != nil && c.Parent().Name() == "source" {
		// The public raw GET /v2/shops/{missing-slug} returns HTTP 200
		// with shops:[], so a status-based error probe has no error path.
		// Normalized venues get still maps the empty listing to NotFound.
		c.Annotations["pp:no-error-path-probe"] = "true"
	}
	for _, child := range c.Commands() {
		hidePlanningSource(child)
	}
}

func planningGroup(group *cobra.Command, flags *rootFlags) {
	group.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}
	group.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return usageErr(fmt.Errorf("unknown subcommand %q for %s", args[0], cmd.CommandPath()))
		}
		return cmd.Help()
	}
}

func planningLeaf(c *cobra.Command, happy string, flags *rootFlags) *planningFlags {
	o := &planningFlags{}
	c.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:requires-input": "true", "pp:happy-args": happy}
	c.SilenceUsage = true
	c.Flags().BoolVar(&o.refresh, "refresh", false, "Bypass local cache reads; upstream freshness remains unknown")
	c.Flags().StringVar(&o.cacheDir, "cache-dir", "", "Local response cache directory")
	c.Flags().IntVar(&o.maxRequests, "max-requests", 20, "Maximum total HTTP attempts, including retries (1–20)")
	c.Flags().IntVar(&o.concurrency, "concurrency", 2, "Maximum concurrent HTTP requests (1–2)")
	c.Flags().IntVar(&o.retries, "retries", 1, "Maximum transient retry per request (0–1)")
	c.Flags().StringVar(&o.timeoutText, "timeout", "10s", "Request and command deadline (positive, at most 30s)")
	return o
}

func (p *planningFlags) validate() error {
	parsed, e := cliutil.ParseDurationLoose(p.timeoutText)
	if e != nil {
		return &planner.ValidationError{Message: fmt.Sprintf("invalid timeout: %v", e)}
	}
	p.timeout = parsed
	if p.maxRequests < 1 || p.maxRequests > 20 {
		return &planner.ValidationError{Message: "max-requests must be between 1 and 20"}
	}
	if p.concurrency < 1 || p.concurrency > 2 {
		return &planner.ValidationError{Message: "concurrency must be between 1 and 2"}
	}
	if p.retries < 0 || p.retries > 1 {
		return &planner.ValidationError{Message: "retries must be 0 or 1"}
	}
	if p.timeout <= 0 || p.timeout > 30*time.Second {
		return &planner.ValidationError{Message: "timeout must be positive and at most 30s"}
	}
	return nil
}

func planningError(e error) error {
	if e == nil {
		return nil
	}
	var invalid *planner.ValidationError
	var absent *planner.NotFoundError
	var throttle *cliutil.RateLimitError
	switch {
	case errors.As(e, &invalid):
		return usageErr(e)
	case errors.As(e, &absent):
		return notFoundErr(e)
	case errors.As(e, &throttle):
		return rateLimitErr(e)
	default:
		return apiErr(e)
	}
}

func planningPrint(cmd *cobra.Command, flags *rootFlags, result planner.Result) error {
	// Native --select is authoritative. Generic compact pruning would discard
	// inventory evidence and supported nulls, so formatting and pruning differ.
	copyFlags := *flags
	copyFlags.compact = false
	copyFlags.agent = false
	// A trailing wildcard selects every field beneath that path. Native
	// selection of the parent object/array preserves that exact meaning.
	selectedPaths := strings.Split(copyFlags.selectFields, ",")
	for i, path := range selectedPaths {
		selectedPaths[i] = strings.TrimSuffix(strings.TrimSpace(path), ".*")
	}
	copyFlags.selectFields = strings.Join(selectedPaths, ",")
	if copyFlags.csv || copyFlags.plain || copyFlags.quiet {
		nativeResult := result
		primary := planningPrimaryKey(result)
		if !copyFlags.quiet && (copyFlags.csv || copyFlags.plain) {
			var err error
			nativeResult, err = planningTable(result, primary, copyFlags.selectFields)
			if err != nil {
				return usageErr(err)
			}
			// Selection already ran against the full envelope. Dotted context
			// columns are table keys now, not paths to filter a second time.
			copyFlags.selectFields = ""
		} else if checks, ok := result["checks"]; ok {
			// Availability envelopes also carry venue and failure arrays. Native
			// formats deliberately expose one row per check, including failures.
			if copyFlags.quiet && copyFlags.selectFields != "" {
				retainSlug := false
				for _, path := range selectedPaths {
					if strings.EqualFold(path, "checks.slug") || strings.EqualFold(path, "checks") {
						retainSlug = true
						break
					}
				}
				if !retainSlug {
					return usageErr(&planner.ValidationError{Message: "availability --quiet selection must retain checks.slug or checks"})
				}
			}
			// Retain the checks parent so dotted --select paths remain valid.
			nativeResult = planner.Result{"checks": checks}
		} else if copyFlags.quiet && (primary == "course" || primary == "venue") {
			if copyFlags.selectFields != "" {
				identitySelected := false
				for _, path := range selectedPaths {
					if strings.EqualFold(path, primary) || strings.EqualFold(path, primary+".id") {
						identitySelected = true
					}
				}
				if !identitySelected {
					return usageErr(&planner.ValidationError{Message: primary + " --quiet selection must retain " + primary + ".id or " + primary})
				}
			}
			selected, err := planningSelectedEnvelope(result, copyFlags.selectFields)
			if err != nil {
				return usageErr(err)
			}
			detail, ok := selected[primary].(map[string]any)
			if !ok {
				return usageErr(&planner.ValidationError{Message: "quiet output requires a " + primary + " identity"})
			}
			nativeResult = planner.Result{"id": detail["id"]}
			copyFlags.selectFields = ""
		}
		// Delegate these modes to the native format renderer. Keep a final
		// checked write: native CSV/plain helpers do not propagate writer errors.
		var formatted bytes.Buffer
		formatCmd := &cobra.Command{}
		formatCmd.SetOut(&formatted)
		if err := copyFlags.printJSON(formatCmd, nativeResult); err != nil {
			return usageErr(err)
		}
		_, err := cmd.OutOrStdout().Write(formatted.Bytes())
		return err
	}
	copyFlags.asJSON = true
	var selected bytes.Buffer
	outCmd := &cobra.Command{}
	outCmd.SetOut(&selected)
	if e := copyFlags.printJSON(outCmd, result); e != nil {
		return usageErr(e)
	}
	raw := selected.Bytes()
	// Result already contains provenance. --agent keeps the same items/checks
	// shape and freshness/transport metadata rather than wrapping it again.
	var compact bytes.Buffer
	if e := json.Compact(&compact, raw); e != nil {
		return e
	}
	compact.WriteByte('\n')
	_, e := cmd.OutOrStdout().Write(compact.Bytes())
	return e
}

func planningPrimaryKey(result planner.Result) string {
	for _, key := range []string{"checks", "items", "course", "venue"} {
		if _, ok := result[key]; ok {
			return key
		}
	}
	return ""
}

// Use the shared selector once before changing the envelope into table rows.
func planningSelectedEnvelope(result planner.Result, fields string) (map[string]any, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if fields != "" {
		raw, err = filterFieldsChecked(raw, fields)
		if err != nil {
			return nil, err
		}
	}
	var selected map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&selected); err != nil {
		return nil, err
	}
	return selected, nil
}

func planningTable(result planner.Result, primary, fields string) (planner.Result, error) {
	selected, err := planningSelectedEnvelope(result, fields)
	if err != nil {
		return nil, err
	}
	if fields == "" && primary != "" {
		selected = map[string]any{primary: selected[primary]}
	}
	rows := make([]map[string]any, 0)
	contextColumns := map[string]any{}
	for key, value := range selected {
		if key == primary {
			switch records := value.(type) {
			case []any:
				if len(records) == 0 && fields != "" {
					if err := planningContextColumns(contextColumns, key, records); err != nil {
						return nil, err
					}
				}
				for _, record := range records {
					if row, ok := record.(map[string]any); ok {
						rows = append(rows, row)
					} else {
						return nil, fmt.Errorf("selected %s record is not an object", primary)
					}
				}
			case map[string]any:
				if len(records) > 0 {
					rows = append(rows, records)
				} else if err := planningContextColumns(contextColumns, key, value); err != nil {
					return nil, err
				}
			default:
				if err := planningContextColumns(contextColumns, key, value); err != nil {
					return nil, err
				}
			}
		} else if err := planningContextColumns(contextColumns, key, value); err != nil {
			return nil, err
		}
	}
	// Context-only selections, including an empty primary with selected
	// context, are one summary row rather than disappearing as an empty list.
	if len(rows) == 0 && len(contextColumns) > 0 {
		rows = append(rows, map[string]any{})
	}
	usedColumns := map[string]bool{}
	for _, row := range rows {
		for key := range row {
			usedColumns[key] = true
		}
	}
	keys := make([]string, 0, len(contextColumns))
	for key := range contextColumns {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	contextNames := map[string]string{}
	for _, key := range keys {
		name := key
		for usedColumns[name] {
			name = "envelope." + name
		}
		usedColumns[name] = true
		contextNames[key] = name
	}
	projected := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out := make(map[string]any, len(usedColumns))
		for key := range usedColumns {
			out[key] = "null"
		}
		for key, value := range row {
			cell, err := planningCell(value)
			if err != nil {
				return nil, err
			}
			out[key] = cell
		}
		for _, key := range keys {
			out[contextNames[key]] = contextColumns[key]
		}
		projected = append(projected, out)
	}
	return planner.Result{"items": projected}, nil
}

// Context is namespaced by its envelope path. Structured/null values use
// explicit JSON cell text instead of Go map formatting or blank null cells.
func planningContextColumns(out map[string]any, path string, value any) error {
	if object, ok := value.(map[string]any); ok && len(object) > 0 {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := planningContextColumns(out, path+"."+key, object[key]); err != nil {
				return err
			}
		}
		return nil
	}
	cell, err := planningCell(value)
	if err != nil {
		return err
	}
	out[path] = cell
	return nil
}

func planningCell(value any) (any, error) {
	switch typed := value.(type) {
	case nil, []any, map[string]any:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return string(raw), nil
	case json.Number:
		return typed.String(), nil
	default:
		return value, nil
	}
}

func planningRun(cmd *cobra.Command, flags *rootFlags, p *planningFlags, input any, validate func() error, op func(context.Context, planner.PlanningClient) (planner.Result, error)) error {
	if e := p.validate(); e != nil {
		return planningError(e)
	}
	for _, key := range []string{"limit", "party"} {
		if cmd.Flags().Changed(key) {
			value, e := cmd.Flags().GetInt(key)
			if e == nil && value == 0 {
				return usageErr(&planner.ValidationError{Message: "explicit --" + key + " must be positive"})
			}
		}
	}
	if e := validateDataSourceStrategy(flags, "live"); e != nil {
		return usageErr(e)
	}
	dry := dryRunOK(flags) || cliutil.IsVerifyEnv()
	if dry && !planningHasDomainInput(cmd) && cmd.Annotations["pp:requires-input"] == "true" {
		required := strings.Split(cmd.Annotations["pp:required-inputs"], ";")
		r := planner.Result{"dry_run": true, "command": cmd.CommandPath(), "action": cmd.CommandPath(), "would": "Read TableCheck planning data without creating a reservation", "request_plan": input, "required_inputs": required, "pending_inputs": required, "meta": map[string]any{"source": "dry-run", "requests": 0}}
		return planningPrint(cmd, flags, r)
	}
	if e := validate(); e != nil {
		return planningError(e)
	}
	if dry {
		r, e := planner.DryRunPlan(cmd.CommandPath(), input)
		if e != nil {
			return planningError(e)
		}
		return planningPrint(cmd, flags, r)
	}
	c, e := newPlanningClient(planner.Options{CacheDir: p.cacheDir, Refresh: p.refresh || flags.noCache, Timeout: p.timeout, MaxRequests: p.maxRequests, Concurrency: p.concurrency, Retries: p.retries})
	if e != nil {
		return planningError(e)
	}
	bound := *flags
	bound.timeout = p.timeout
	ctx, cancel := boundCtx(cmd.Context(), &bound)
	defer cancel()
	r, e := op(ctx, c)
	if r != nil {
		if printErr := planningPrint(cmd, flags, r); printErr != nil {
			return printErr
		}
	}
	return planningError(e)
}

func planningHasDomainInput(cmd *cobra.Command) bool {
	if len(cmd.Flags().Args()) > 0 {
		return true
	}
	shared := map[string]bool{"refresh": true, "cache-dir": true, "max-requests": true, "concurrency": true, "retries": true, "timeout": true}
	changed := false
	cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Changed && !shared[f.Name] {
			changed = true
		}
	})
	return changed
}

func planningArity(args []string, min, max int) error {
	if len(args) < min || len(args) > max {
		return &planner.ValidationError{Message: fmt.Sprintf("expected %d–%d positional arguments, got %d", min, max, len(args))}
	}
	return nil
}

// pp:data-source live
func newPlanningVenuesSearchCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "search", Short: "Search Japan venues using coordinates and discovery preferences.", Example: "  tablecheck-pp-cli venues search --lat 35.681236 --lon 139.767125 --radius 3000 --cuisine sushi --budget-max 20000 --date 2026-09-30 --party 2"}
	p := planningLeaf(c, "--lat=35.681236;--lon=139.767125;--radius=3000;--cuisine=sushi;--limit=2", flags)
	c.Annotations["pp:required-inputs"] = "--lat;--lon;--radius"
	var o planner.SearchOptions
	c.Flags().Float64Var(&o.Latitude, "lat", 0, "Latitude, explicitly required")
	c.Flags().Float64Var(&o.Longitude, "lon", 0, "Longitude, explicitly required")
	c.Flags().Float64Var(&o.Radius, "radius", 0, "Radius in metres, explicitly required (at most 50000)")
	c.Flags().StringVar(&o.Cuisine, "cuisine", "", "Stable cuisine key from cuisines list")
	c.Flags().StringVar(&o.BudgetMin, "budget-min", "", "Minimum quoted venue dinner average as decimal text")
	c.Flags().StringVar(&o.BudgetMax, "budget-max", "", "Maximum quoted venue dinner average as decimal text")
	c.Flags().StringVar(&o.Date, "date", "", "Discovery date (YYYY-MM-DD)")
	c.Flags().StringVar(&o.Time, "time", "", "Discovery meal-time preference (HH:MM)")
	c.Flags().IntVar(&o.Party, "party", 0, "Discovery party size (1–20)")
	c.Flags().IntVar(&o.Limit, "limit", 10, "Maximum venues (1–50)")
	c.Flags().StringVar(&o.Cursor, "cursor", "", "Opaque next-page cursor")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return planningRun(cmd, flags, p, o, func() error {
			if e := planningArity(args, 0, 0); e != nil {
				return e
			}
			for _, k := range []string{"lat", "lon", "radius"} {
				if !cmd.Flags().Changed(k) {
					return &planner.ValidationError{Message: "explicit --" + k + " is required"}
				}
			}
			return planner.ValidateSearch(o)
		}, func(ctx context.Context, client planner.PlanningClient) (planner.Result, error) {
			return client.Search(ctx, o)
		})
	}
	return c
}

// pp:data-source live
func newPlanningVenuesGetCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "get <slug>", Short: "Read venue identity, location and static policies.", Example: "  tablecheck-pp-cli venues get sushi-tokyo81 --select venue.id,venue.name,venue.time_zone"}
	p := planningLeaf(c, "slug=sushi-tokyo81", flags)
	c.Annotations["pp:required-inputs"] = "SLUG"
	c.RunE = func(cmd *cobra.Command, args []string) error {
		slug := ""
		if len(args) > 0 {
			slug = args[0]
		}
		return planningRun(cmd, flags, p, slug, func() error {
			if e := planningArity(args, 1, 1); e != nil {
				return e
			}
			return planner.ValidateVenue(slug)
		}, func(ctx context.Context, client planner.PlanningClient) (planner.Result, error) {
			return client.Venue(ctx, slug)
		})
	}
	return c
}

// pp:data-source live
func newPlanningCuisinesListCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "list", Short: "List or find stable cuisine keys.", Example: "  tablecheck-pp-cli cuisines list --query sushi --limit 5"}
	p := planningLeaf(c, "--limit=2;--query=sushi", flags)
	delete(c.Annotations, "pp:requires-input")
	var query string
	var limit, offset int
	c.Flags().StringVar(&query, "query", "", "Match cuisine key or translated label")
	c.Flags().IntVar(&limit, "limit", 10, "Maximum cuisines (1–50)")
	c.Flags().IntVar(&offset, "offset", 0, "Local list offset")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		input := map[string]any{"query": query, "limit": limit, "offset": offset}
		return planningRun(cmd, flags, p, input, func() error {
			if e := planningArity(args, 0, 0); e != nil {
				return e
			}
			if len(query) > 200 || limit < 1 || limit > 50 || offset < 0 || offset > 100000 {
				return &planner.ValidationError{Message: "query must be at most 200 characters, limit 1–50 and offset 0–100000"}
			}
			return nil
		}, func(ctx context.Context, client planner.PlanningClient) (planner.Result, error) {
			return client.Cuisines(ctx, query, limit, offset)
		})
	}
	return c
}

// pp:data-source live
func newPlanningCoursesListCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "list <slug>", Short: "List quoted courses; prices are not booking guarantees.", Example: "  tablecheck-pp-cli courses list sushi-tokyo81 --limit 5"}
	p := planningLeaf(c, "slug=sushi-tokyo81;--limit=2", flags)
	c.Annotations["pp:required-inputs"] = "SLUG"
	var o planner.CourseOptions
	c.Flags().StringVar(&o.From, "from", "", "Menu window start (YYYY-MM-DD)")
	c.Flags().StringVar(&o.To, "to", "", "Menu window end (YYYY-MM-DD)")
	c.Flags().IntVar(&o.Limit, "limit", 10, "Maximum courses (1–50)")
	c.Flags().IntVar(&o.Offset, "offset", 0, "Local list offset")
	planningCourseRun(c, p, flags, &o, false)
	return c
}

// pp:data-source live
func newPlanningCoursesGetCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "get <slug> <course_id>", Short: "Read course price, eligibility and exact source conditions.", Example: "  tablecheck-pp-cli courses get sushi-tokyo81 68da546fcde865308c33e7f9"}
	p := planningLeaf(c, "slug=sushi-tokyo81;course_id=68da546fcde865308c33e7f9", flags)
	c.Annotations["pp:required-inputs"] = "SLUG;COURSE_ID"
	var o planner.CourseOptions
	c.Flags().StringVar(&o.From, "from", "", "Menu window start (YYYY-MM-DD)")
	c.Flags().StringVar(&o.To, "to", "", "Menu window end (YYYY-MM-DD)")
	planningCourseRun(c, p, flags, &o, true)
	return c
}

func planningCourseRun(c *cobra.Command, p *planningFlags, flags *rootFlags, o *planner.CourseOptions, detail bool) {
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			o.Venue = args[0]
		}
		if detail && len(args) > 1 {
			o.CourseID = args[1]
		}
		n := 1
		if detail {
			n = 2
		}
		return planningRun(cmd, flags, p, *o, func() error {
			if e := planningArity(args, n, n); e != nil {
				return e
			}
			return planner.ValidateCourses(*o)
		}, func(ctx context.Context, client planner.PlanningClient) (planner.Result, error) {
			return client.Courses(ctx, *o)
		})
	}
}

// pp:data-source live
func newPlanningAvailabilityCheckCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "check <slug>", Short: "Check venue slots for an explicit party within the source time window.", Long: "The source returns a limited time window around the query anchor. --time anchors that window and checks the exact requested time. Omitting --time uses an 18:00 anchor and reports any available slots inside that window, with partial day coverage.", Example: "  tablecheck-pp-cli availability check sushi-tokyo81 --date 2026-09-30 --party 2 --time 18:00 --include-unavailable"}
	p := planningLeaf(c, "slug=sushi-tokyo81;--date=2026-09-30;--party=2;--limit=2", flags)
	c.Annotations["pp:required-inputs"] = "SLUG;--date;--party"
	var o planner.CheckOptions
	c.Flags().StringVar(&o.Date, "date", "", "Required local date (YYYY-MM-DD)")
	c.Flags().StringVar(&o.Time, "time", "", "Local time to anchor the source window and check exactly (HH:MM); omitted uses 18:00 anchor")
	c.Flags().IntVar(&o.Party, "party", 0, "Required party size (1–20)")
	c.Flags().IntVar(&o.Limit, "limit", 10, "Maximum slots (1–50)")
	c.Flags().BoolVar(&o.IncludeUnavailable, "include-unavailable", false, "Include explicitly unavailable and unknown slots")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			o.Venue = args[0]
		}
		return planningRun(cmd, flags, p, o, func() error {
			if e := planningArity(args, 1, 1); e != nil {
				return e
			}
			return planner.ValidateCheck(o)
		}, func(ctx context.Context, client planner.PlanningClient) (planner.Result, error) {
			return client.Check(ctx, o)
		})
	}
	return c
}

// pp:data-source live
func newPlanningAvailabilityScanCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "scan <slug>...", Short: "Scan source time windows for at most five venues over fourteen local dates.", Long: "Each local-date row covers only the returned source time window. --time anchors that window and checks the exact requested time. Omitting --time uses an 18:00 anchor and reports any available slots inside that window, with partial day coverage. Failed rows remain visible.", Example: "  tablecheck-pp-cli availability scan sushi-tokyo81 --from 2026-09-30 --to 2026-10-01 --party 2 --limit 5"}
	p := planningLeaf(c, "slug=sushi-tokyo81;--from=2026-09-30;--to=2026-10-01;--party=2;--limit=2", flags)
	c.Annotations["pp:required-inputs"] = "SLUG...;--from;--to;--party"
	var o planner.ScanOptions
	c.Flags().StringVar(&o.From, "from", "", "Required first local date (YYYY-MM-DD)")
	c.Flags().StringVar(&o.To, "to", "", "Required last local date (YYYY-MM-DD)")
	c.Flags().StringVar(&o.Time, "time", "", "Local time to anchor the source window and check exactly (HH:MM); omitted uses 18:00 anchor")
	c.Flags().IntVar(&o.Party, "party", 0, "Required party size (1–20)")
	c.Flags().IntVar(&o.Limit, "limit", 10, "Maximum slots per check (1–50)")
	c.Flags().BoolVar(&o.IncludeUnavailable, "include-unavailable", false, "Include explicitly unavailable and unknown slots")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		o.Venues = append([]string(nil), args...)
		return planningRun(cmd, flags, p, o, func() error { return planner.ValidateScan(o) }, func(ctx context.Context, client planner.PlanningClient) (planner.Result, error) {
			return client.Scan(ctx, o)
		})
	}
	return c
}

// pp:data-source live
func newPlanningBookingURLCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "booking-url <slug>", Short: "Build an official booking handoff without creating a reservation.", Example: "  tablecheck-pp-cli booking-url sushi-tokyo81 --date 2026-09-30 --time 18:00 --party 2"}
	p := planningLeaf(c, "slug=sushi-tokyo81;--date=2026-09-30;--time=18:00;--party=2", flags)
	c.Annotations["pp:required-inputs"] = "SLUG"
	var o planner.HandoffOptions
	c.Flags().StringVar(&o.Date, "date", "", "Optional prefilled local date (YYYY-MM-DD)")
	c.Flags().StringVar(&o.Time, "time", "", "Optional prefilled local time (HH:MM)")
	c.Flags().IntVar(&o.Party, "party", 0, "Optional prefilled party size (1–20)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			o.Venue = args[0]
		}
		return planningRun(cmd, flags, p, o, func() error {
			if e := planningArity(args, 1, 1); e != nil {
				return e
			}
			return planner.ValidateHandoff(o)
		}, func(ctx context.Context, client planner.PlanningClient) (planner.Result, error) {
			return client.BookingURL(ctx, o)
		})
	}
	return c
}
