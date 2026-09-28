// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

// stayService is the read-only source boundary. CLI tests substitute observable
// results and failures without networking or coupling to HTML extraction.
type stayService interface {
	Search(context.Context, jalan.Query) (jalan.Response, error)
	Property(context.Context, string) (jalan.Response, error)
	Offers(context.Context, string, jalan.Query) (jalan.Response, error)
	Plan(context.Context, string, string, string, jalan.Query) (jalan.Response, error)
	Compare(context.Context, string, jalan.Query, []string, []jalan.PlanRef) (jalan.Response, error)
}

var stayClientFactory = func(options jalan.Options) stayService { return jalan.NewClient(options) }

type stayFlags struct {
	query    jalan.Query
	maxAge   time.Duration
	refresh  bool
	cacheDir string
}

func newNovelStayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "stay", Short: "Search Japan stays and inspect exact rooms and plans",
		Long:          "Read anonymous public Jalan accommodation pages. Outputs one compact JSON envelope with source, observation time, party, coverage, and failures. --select projects result fields while retaining metadata. Inventory is fresh unless --max-age explicitly permits reuse (up to 5m). Each search/offers call fetches at most two source pages; compare accepts at most five alternatives. No booking or account actions.",
		Example:       "  jalan-pp-cli stay capabilities\n  jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --adults 2 --limit 5",
		Annotations:   stayAnnotations("live", "<subcommand>=capabilities"),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, jalan.Query{})
			}
			if len(args) != 0 {
				return stayReportError(cmd, &jalan.Error{Code: "usage", Message: fmt.Sprintf("unknown stay subcommand %q", args[0]), Hint: "Run 'jalan-pp-cli stay --help' for available commands."})
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(newStaySearchCmd(flags), newNovelStayLocationsCmd(flags), newNovelStayPropertyCmd(flags), newStayOffersCmd(flags), newNovelStayPlanCmd(flags), newNovelStayCompareCmd(flags), newStayCapabilitiesCmd(flags))
	return cmd
}

func stayAnnotations(source, happy string) map[string]string {
	annotations := map[string]string{"mcp:read-only": "true", "pp:data-source": source}
	// Printing Press consumes semicolon-separated --flag=value and
	// <positional-name>=value fixtures, rather than shell argument text.
	if happy != "" {
		annotations["pp:happy-args"] = happy
	}
	return annotations
}

func addStayCacheFlags(cmd *cobra.Command, f *stayFlags) {
	cmd.Flags().DurationVar(&f.maxAge, "max-age", 0, "Reuse matching observations up to this age (0=fresh; max 5m)")
	cmd.Flags().BoolVar(&f.refresh, "refresh", false, "Fetch fresh observations even when an exact cached response exists")
	cmd.Flags().StringVar(&f.cacheDir, "cache-dir", "", "Private observation cache directory (default: platform cache; --home is honored)")
}

func addStayPartyFlags(cmd *cobra.Command, f *stayFlags) {
	cmd.Flags().StringVar(&f.query.CheckIn, "check-in", "", "Check-in date YYYY-MM-DD in Asia/Tokyo; required for inventory")
	cmd.Flags().IntVar(&f.query.Nights, "nights", 1, "Nights (1..9)")
	cmd.Flags().IntVar(&f.query.Rooms, "rooms", 1, "Rooms (1..10); the same occupancy applies to every room")
	cmd.Flags().IntVar(&f.query.Adults, "adults", 2, "Adults per room (1..8); the source nine-plus bucket is unsupported")
	cmd.Flags().IntVar(&f.query.Children[0], "children-elementary", 0, "Elementary-school children per room (0..5)")
	cmd.Flags().IntVar(&f.query.Children[1], "children-meals-bed", 0, "Infants with meals and a bed per room (0..5)")
	cmd.Flags().IntVar(&f.query.Children[2], "children-meals", 0, "Infants with meals and no bed per room (0..5)")
	cmd.Flags().IntVar(&f.query.Children[3], "children-bed", 0, "Infants with a bed and no meals per room (0..5)")
	cmd.Flags().IntVar(&f.query.Children[4], "children-neither", 0, "Infants with neither meals nor a bed per room (0..5)")
}

func addStayFilterFlags(cmd *cobra.Command, f *stayFlags) {
	cmd.Flags().StringVar(&f.query.Meals, "meals", "", "Meals: none, breakfast, dinner, breakfast_dinner")
	cmd.Flags().StringVar(&f.query.LodgingType, "lodging-type", "", "Lodging: hotel, ryokan, pension, vacation_rental, public_lodging")
	cmd.Flags().BoolVar(&f.query.Onsen, "onsen", false, "Require a property hot spring; this does not establish room hot-spring water")
	cmd.Flags().BoolVar(&f.query.OutdoorBath, "outdoor-bath", false, "Require a property outdoor bath")
	cmd.Flags().BoolVar(&f.query.PrivateBath, "private-bath", false, "Require a private reservable bath")
	cmd.Flags().BoolVar(&f.query.RoomOutdoorBath, "room-outdoor-bath", false, "Require a room with an outdoor bath")
	cmd.Flags().BoolVar(&f.query.NonSmoking, "non-smoking", false, "Require a non-smoking room")
}

func addStayPageFlags(cmd *cobra.Command, f *stayFlags) {
	cmd.Flags().IntVar(&f.query.Page, "page", 1, "Logical result page (source pages sliced without skipping results)")
	cmd.Flags().IntVar(&f.query.Limit, "limit", 5, "Returned results (1..30); at most two source pages fetched")
}

func addStayOfferFilterFlags(cmd *cobra.Command, f *stayFlags) {
	cmd.Flags().StringVar(&f.query.Meals, "meals", "", "Meals: none, breakfast, dinner, breakfast_dinner")
	cmd.Flags().BoolVar(&f.query.RoomOutdoorBath, "room-outdoor-bath", false, "Require a room with an outdoor bath")
	cmd.Flags().BoolVar(&f.query.NonSmoking, "non-smoking", false, "Require a non-smoking room")
}

func stayValidateOptions(flags *rootFlags, f *stayFlags) error {
	if flags.timeout <= 0 || flags.timeout > 60*time.Second {
		return &jalan.Error{Code: "usage", Message: "--timeout must be greater than 0 and at most 60s", Hint: "Use --timeout 30s; individual HTTP requests are bounded to 20s."}
	}
	if f.maxAge < 0 || f.maxAge > 5*time.Minute {
		return &jalan.Error{Code: "usage", Message: "--max-age must be between 0 and 5m", Hint: "Use --max-age 0 for a fresh source observation."}
	}
	if flags.dataSource == "local" {
		return &jalan.Error{Code: "unsupported", Message: "stay inventory is a public live observation; --data-source local is unsupported", Hint: "Use --data-source live. To reuse a recent exact observation, specify --max-age up to 5m."}
	}
	if flags.csv && (flags.plain || flags.quiet) || flags.plain && flags.quiet {
		return &jalan.Error{Code: "usage", Message: "choose only one of --csv, --plain, or --quiet", Hint: "Omit these flags for the complete JSON envelope."}
	}
	return nil
}

func stayOptions(flags *rootFlags, f *stayFlags) (jalan.Options, error) {
	cacheDir := f.cacheDir
	if cacheDir == "" {
		base, err := cliutil.CacheDir()
		if err != nil {
			return jalan.Options{}, &jalan.Error{Code: "config", Message: "cannot resolve private observation cache", Hint: "Pass an absolute --cache-dir or --home directory.", Cause: err}
		}
		cacheDir = filepath.Join(base, "observations")
	}
	maxAge := f.maxAge
	if f.refresh || flags.noCache {
		maxAge = 0
	}
	return jalan.Options{Timeout: flags.timeout, MaxAge: maxAge, Refresh: f.refresh || flags.noCache, DisableCache: flags.noCache, CacheDir: cacheDir}, nil
}

func stayCall(cmd *cobra.Command, flags *rootFlags, f *stayFlags, call func(context.Context, stayService) (jalan.Response, error)) error {
	if err := stayValidateOptions(flags, f); err != nil {
		return stayReportError(cmd, err)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	options, err := stayOptions(flags, f)
	if err != nil {
		return stayReportError(cmd, err)
	}
	response, fetchErr := call(ctx, stayClientFactory(options))
	if fetchErr != nil && len(response.Results) == 0 {
		return stayReportError(cmd, fetchErr)
	}
	if err := stayPrintResponse(cmd, flags, response); err != nil {
		return stayReportError(cmd, err)
	}
	if fetchErr != nil || len(response.FetchFailures) != 0 {
		// Execute delivers successful commands after RunE. Partial observations
		// return an error, so deliver their completed buffer before reporting it.
		if flags.deliverBuf != nil && flags.deliverBuf.Len() > 0 && len(response.Results) > 0 && len(response.FetchFailures) > 0 {
			if err := Deliver(flags.deliverSink, flags.deliverBuf.Bytes(), flags.compact); err != nil {
				return stayReportError(cmd, &jalan.Error{
					Code:          "delivery_failure",
					Message:       fmt.Sprintf("could not deliver partial stay observations to %s:%s: %v", flags.deliverSink.Scheme, flags.deliverSink.Target, err),
					Hint:          "Check the --deliver destination and retry; the partial observation remains available on stdout.",
					Cause:         err,
					FetchFailures: response.FetchFailures,
				})
			}
			flags.deliverBuf = nil
		}
		// A failing alternative must never override the partial exit state.
		partial := &jalan.Error{Code: "partial", Message: fmt.Sprintf("%d source fetches failed; results cover successful fetches only", len(response.FetchFailures)), Hint: "Inspect fetch_failures and retry only the failed alternatives.", Cause: fetchErr}
		return stayReportError(cmd, partial)
	}
	return nil
}

func stayDryRun(cmd *cobra.Command, q jalan.Query) error {
	return stayEncode(cmd, map[string]any{"dry_run": true, "action": cmd.CommandPath(), "meta": map[string]any{"source": "dry-run", "dry_run": true, "action": cmd.CommandPath(), "query": q, "upstream_requests": 0, "timezone": "Asia/Tokyo"}, "results": []any{}, "pagination": map[string]any{}, "fetch_failures": []any{}})
}

func stayPrintResponse(cmd *cobra.Command, flags *rootFlags, response jalan.Response) error {
	if response.Meta == nil {
		response.Meta = map[string]any{}
	}
	if response.Results == nil {
		response.Results = []any{}
	}
	if response.Pagination == nil {
		response.Pagination = map[string]any{}
	}
	if response.FetchFailures == nil {
		response.FetchFailures = []map[string]any{}
	}
	var compactErr error
	response, compactErr = stayCompactEvidence(response)
	if compactErr != nil {
		return compactErr
	}
	if flags.selectFields != "" {
		selected, err := staySelect(response.Results, flags.selectFields)
		if err != nil {
			return err
		}
		response.Results = selected
	}
	// Stay JSON has a complete provenance envelope. Keep its correctness fields
	// under --agent instead of applying the generated compact allow-list.
	if flags.csv || flags.plain || flags.quiet {
		outputFlags := *flags
		outputFlags.asJSON, outputFlags.agent, outputFlags.compact = true, false, false
		outputFlags.selectFields = ""
		return printJSONFiltered(cmd.OutOrStdout(), response, &outputFlags)
	}
	return stayEncode(cmd, response)
}

func stayEncode(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(value)
}

func staySelect(items []any, fields string) ([]any, error) {
	originals := make([]map[string]any, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		var original map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&original); err != nil {
			return nil, err
		}
		originals = append(originals, original)
	}
	comparison := len(originals) > 0
	for _, original := range originals {
		if _, ok := original["alternative_index"]; !ok {
			comparison = false
			break
		}
		if _, ok := original["results"].([]any); !ok {
			comparison = false
			break
		}
	}
	comparisonHint := "For stay compare, use --select check_in,results.property_id,results.plan_id,results.room_id,results.price (or results.price.amount,results.price.basis); offers are inside each results array."
	paths := make([][]string, 0)
	for _, field := range strings.Split(fields, ",") {
		field = strings.TrimSpace(field)
		if !comparison {
			// A leading results. remains an optional envelope prefix for
			// ordinary property, offer, and plan items.
			field = strings.TrimPrefix(field, "results.")
		}
		parts := strings.Split(field, ".")
		for _, part := range parts {
			if part == "" || strings.ContainsAny(part, " []{}:/\\") {
				return nil, &jalan.Error{Code: "usage", Message: "--select requires comma-separated item dotted fields", Hint: "For example: --select id,name_ja,price.amount,baths.in_room"}
			}
		}
		if comparison {
			if parts[0] == "price" {
				return nil, &jalan.Error{Code: "usage", Message: "comparison prices belong to the nested results array", Hint: comparisonHint}
			}
		}
		paths = append(paths, parts)
	}
	selected := make([]any, 0, len(originals))
	matched := make([]bool, len(paths))
	for _, original := range originals {
		out := map[string]any{}
		if comparison {
			// These identify each alternative and document its individual
			// coverage, source and freshness even when only prices are selected.
			for _, key := range []string{"alternative_index", "check_in", "plan_id", "room_id", "query", "status", "pagination", "source_url", "observations"} {
				if value, ok := original[key]; ok {
					out[key] = value
				}
			}
		}
		for i, path := range paths {
			projection, ok := stayProjectPath(original, path)
			if !ok {
				continue
			}
			if comparison && len(path) > 1 && path[0] == "results" && len(original["results"].([]any)) == 0 {
				// Empty inventory has no item to inspect, but another cell
				// may still supply this field. Retain the empty array shape.
			} else {
				matched[i] = true
			}
			out = stayMergeProjection(out, projection).(map[string]any)
		}
		selected = append(selected, out)
	}
	anyMatched := false
	for i, ok := range matched {
		if ok {
			anyMatched = true
			continue
		}
		if comparison {
			// No fields can be verified when every alternative has empty
			// inventory. Keep syntactically valid nested paths and empty arrays.
			if len(paths[i]) > 1 && paths[i][0] == "results" {
				allEmpty := true
				for _, original := range originals {
					allEmpty = allEmpty && len(original["results"].([]any)) == 0
				}
				if allEmpty {
					continue
				}
			}
			return nil, &jalan.Error{Code: "usage", Message: fmt.Sprintf("unknown --select comparison field %q", strings.Join(paths[i], ".")), Hint: comparisonHint}
		}
	}
	if len(items) > 0 && !comparison && !anyMatched {
		return nil, &jalan.Error{Code: "usage", Message: "none of the --select fields exist in these results", Hint: "Inspect the command without --select, then choose item fields such as id,price.amount or baths.in_room."}
	}
	return selected, nil
}

func stayProjectPath(value any, path []string) (any, bool) {
	if len(path) == 0 {
		return value, true
	}
	switch current := value.(type) {
	case map[string]any:
		child, ok := current[path[0]]
		if !ok {
			return nil, false
		}
		projected, ok := stayProjectPath(child, path[1:])
		if !ok {
			return nil, false
		}
		return map[string]any{path[0]: projected}, true
	case []any:
		projected := make([]any, len(current))
		if len(current) == 0 {
			return projected, true
		}
		found := false
		for i, child := range current {
			if value, ok := stayProjectPath(child, path); ok {
				projected[i] = value
				found = true
			} else if child != nil {
				projected[i] = map[string]any{}
			}
		}
		return projected, found
	default:
		return nil, false
	}
}

func stayMergeProjection(left, right any) any {
	if leftMap, ok := left.(map[string]any); ok {
		if rightMap, ok := right.(map[string]any); ok {
			for key, value := range rightMap {
				if prior, exists := leftMap[key]; exists {
					leftMap[key] = stayMergeProjection(prior, value)
				} else {
					leftMap[key] = value
				}
			}
			return leftMap
		}
	}
	if leftArray, ok := left.([]any); ok {
		if rightArray, ok := right.([]any); ok {
			for i := range rightArray {
				leftArray[i] = stayMergeProjection(leftArray[i], rightArray[i])
			}
			return leftArray
		}
	}
	return right
}

type stayReportedError struct{ error }

func (e *stayReportedError) Unwrap() error { return e.error }

func stayReportError(cmd *cobra.Command, err error) error {
	var reported *stayReportedError
	if errors.As(err, &reported) {
		return err
	}
	code, hint, sourceURL, exit := "upstream", "Retry with --refresh; inspect the source URL if the failure persists.", "", 5
	var sourceErr *jalan.Error
	var rateErr *cliutil.RateLimitError
	var partialErr *jalan.PartialError
	if errors.As(err, &partialErr) {
		code = "partial"
		hint = "Inspect fetch_failures and retry only the failed alternatives."
	} else if errors.As(err, &sourceErr) {
		code, hint, sourceURL = sourceErr.Code, sourceErr.Hint, sourceErr.URL
	} else if errors.As(err, &rateErr) {
		code = "rate_limit"
		sourceURL = rateErr.URL
		hint = "Wait for the source rate limit to reset and retry."
	}
	if isCobraUsageError(err) || strings.HasPrefix(err.Error(), "invalid --data-source") {
		code = "usage"
		hint = "Run this command with --help to inspect valid arguments and flags."
	}
	if code != "partial" && code != "rate_limit" && errors.Is(err, context.DeadlineExceeded) {
		code = "timeout"
		hint = "Retry a smaller bounded query or use --timeout up to 60s."
	}
	switch code {
	case "usage", "unsupported", "invalid_query", "check_in_required", "destination_required", "unknown_destination", "ambiguous_destination":
		exit = 2
	case "not_found", "unavailable":
		exit = 3
	case "access", "access_denied", "access_failure", "blocked":
		exit = 4
	case "rate_limit", "rate_limited":
		exit = 7
	case "partial", "partial_failure":
		exit = 8
	case "parse", "parse_failure":
		exit = 9
	case "config":
		exit = 10
	}
	if hint == "" {
		hint = "Inspect the source and retry; run 'jalan-pp-cli stay capabilities' for supported inputs."
	}
	enc := json.NewEncoder(cmd.ErrOrStderr())
	enc.SetEscapeHTML(false)
	diagnostic := map[string]any{"error": map[string]any{"code": code, "message": err.Error(), "hint": hint, "url": sourceURL}, "exit_code": exit}
	if rateErr != nil {
		diagnostic["retry_after_ms"] = rateErr.RetryAfter.Milliseconds()
	}
	if sourceErr != nil && len(sourceErr.FetchFailures) != 0 {
		diagnostic["fetch_failures"] = sourceErr.FetchFailures
	}
	if partialErr != nil && len(partialErr.Failures) != 0 {
		diagnostic["fetch_failures"] = partialErr.Failures
	}
	_ = enc.Encode(diagnostic)
	return &stayReportedError{&cliError{code: exit, err: err}}
}

func isStayCommand(cmd *cobra.Command) bool {
	for cmd != nil {
		if cmd.Name() == "stay" {
			return true
		}
		cmd = cmd.Parent()
	}
	return false
}

func stayPropertyID(args []string) (string, error) {
	if len(args) != 1 {
		return "", &jalan.Error{Code: "usage", Message: "exactly one property ID is required", Hint: "For example: jalan-pp-cli stay property 385995"}
	}
	id := args[0]
	if len(id) != 6 || strings.Trim(id, "0123456789") != "" {
		return "", &jalan.Error{Code: "invalid_query", Message: "property ID must contain six digits", Hint: "Use the id from stay search, for example 385995."}
	}
	return id, nil
}

func stayValidateQuery(cmd *cobra.Command, q jalan.Query, requireDestination bool) (jalan.Query, error) {
	// The service accepts zero-valued programmatic defaults. A CLI flag with
	// an explicit zero is invalid rather than permission to change the party.
	for name, value := range map[string]int{"nights": q.Nights, "rooms": q.Rooms, "adults": q.Adults, "page": q.Page, "limit": q.Limit} {
		if cmd.Flags().Lookup(name) != nil && value == 0 {
			return q, &jalan.Error{Code: "invalid_query", Message: "--" + name + " must be greater than zero", Hint: "Inspect this command's --help for its supported numeric range."}
		}
	}
	_, err := jalan.ValidateQuery(q, requireDestination)
	// Keep caller input; service normalization resolves destination aliases.
	return q, err
}
