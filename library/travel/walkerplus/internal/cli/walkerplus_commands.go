// Public Walkerplus workflows; source facts stay separate from trip matches.
// pp:data-source live
package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
	"github.com/spf13/cobra"
)

type walkerService interface {
	Search(context.Context, walkerplus.Query) (walkerplus.Result, error)
	Shortlist(context.Context, walkerplus.Query) (walkerplus.Result, error)
	Event(context.Context, string) (walkerplus.Event, error)
	Areas(context.Context, string) (walkerplus.CatalogResult, error)
	Categories() walkerplus.CatalogResult
	Stats() walkerplus.Coverage
}

var makeWalkerClient = func(options walkerplus.Options) (walkerService, error) {
	return walkerplus.NewClient(options)
}

type walkerRuntime struct {
	cacheDir, cacheTTL, requestTimeout string
	refresh                            bool
	concurrency, retries               int
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		runtime := &walkerRuntime{}
		flags.walkerRuntime = runtime
		root.PersistentFlags().StringVar(&runtime.cacheDir, "cache-dir", "", "Directory for the bounded source HTML cache")
		root.PersistentFlags().StringVar(&runtime.cacheTTL, "cache-ttl", "1h", "Freshness lifetime for cached HTML, greater than zero and at most 1d")
		root.PersistentFlags().StringVar(&runtime.requestTimeout, "request-timeout", "15s", "Timeout per HTTP request, greater than zero and at most 15s")
		root.PersistentFlags().BoolVar(&runtime.refresh, "refresh", false, "Fetch fresh source HTML and replace cached responses")
		root.PersistentFlags().IntVar(&runtime.concurrency, "concurrency", 2, "Maximum concurrent HTTP fetches, from 1 to 4")
		root.PersistentFlags().IntVar(&runtime.retries, "retries", 2, "Retry transient requests this many times, from 0 to 3")
		root.PersistentFlags().StringVar(&flags.selectFields, "fields", "", "Alias for --select; project event fields and retain metadata")
		flags.asJSON = true
		root.PersistentFlags().Lookup("json").DefValue = "true"
		root.PersistentFlags().Lookup("json").Usage = "Emit JSON (the default for Walkerplus commands)"
		root.PersistentFlags().Lookup("select").Usage = "Comma-separated event fields; query and coverage stay present"
		root.PersistentFlags().Lookup("timeout").Usage = "Overall command timeout, greater than zero and at most 60s"
		root.PersistentFlags().Lookup("compact").Usage = "Compact JSON output (already the Walkerplus default)"
		root.Long = "Discover Japan events from public Walkerplus pages.\n\n  search       Browse bounded listing candidates without detail requests.\n  shortlist    Match an exact trip window using bounded source detail.\n  event        Read source facts for a Walkerplus event ID or URL.\n  areas        Discover prefecture aliases and current city routes.\n  categories   Discover Japanese event categories and accepted aliases.\n\nJSON is compact by default. --select projects event fields and keeps coverage.\nDates use Asia/Tokyo. Listing ranges are envelopes, not attendance guarantees.\nRun doctor for connectivity or doctor --dry-run for an offline check."
		for _, child := range root.Commands() {
			switch child.Name() {
			case "event":
			case "shortlist", "doctor", "context", "agent-context", "version":
			default:
				hideWalkerFramework(child)
			}
		}
		for _, name := range []string{"csv", "plain", "quiet", "config", "home", "receipt", "receipt-file", "audit-dir", "yes", "human-friendly", "data-source", "max-age", "profile", "client-profile", "deliver", "rate-limit"} {
			if flag := root.PersistentFlags().Lookup(name); flag != nil {
				flag.Hidden = true
			}
		}
		root.AddCommand(newWalkerSearchCmd(flags), newWalkerAreasCmd(flags), newWalkerCategoriesCmd(flags), newWalkerSchemaCmd(flags))
	})
}

func hideWalkerFramework(cmd *cobra.Command) {
	cmd.Hidden = true
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["mcp:hidden"] = "true"
	for _, child := range cmd.Commands() {
		hideWalkerFramework(child)
	}
}

func walkerAnnotations(happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": happy}
}

func newWalkerSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "search"}
	return configureWalkerListCmd(cmd, flags, false)
}

func newWalkerShortlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "shortlist"}
	return configureWalkerListCmd(cmd, flags, true)
}

func configureWalkerListCmd(cmd *cobra.Command, flags *rootFlags, enrich bool) *cobra.Command {
	name := cmd.Name()
	query := walkerplus.Query{}
	short := "Discover bounded listing candidates without fetching event details"
	long := "Search source listing cards only. Date filters use the exact published edition envelope; activity remains possible until schedule evidence is read. --limit bounds output and --max-pages independently bounds listing work."
	happy := "--prefecture=kyoto;--category=festival;--from=2026-10-01;--to=2026-10-31;--max-pages=1;--limit=3"
	if enrich {
		short = "Match a trip window using bounded event details and schedule evidence"
		long = "Enrich at most --max-details candidates, then apply exact trip dates and strict source-backed --free/--indoor constraints. Confirmed days require schedule evidence. Unknown, approximate and unresolved schedules remain possible. Canceled events are excluded. starts/ends match source envelope boundaries."
		happy += ";--max-details=3"
	}
	cmd.Short = short
	cmd.Long = long
	cmd.Annotations = walkerAnnotations(happy)
	cmd.Example = strings.Trim(fmt.Sprintf("\n  walkerplus-pp-cli %s --prefecture kyoto --category festival --from 2026-10-01 --to 2026-10-31 --limit 5\n  walkerplus-pp-cli %s --prefecture tokyo --from 2026-09-27 --to 2026-09-30 --select id,title_ja,start_date,end_date,location,match\n  walkerplus-pp-cli %s --dry-run --agent", name, name, name), "\n")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeWalkerDryRun(cmd, flags, name)
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("%s accepts flags only; use --prefecture, --city and --category", name))
		}
		if enrich && (query.From == "" || query.To == "") {
			if !hasChangedLocalFlags(cmd) && !cmd.Flags().Changed("json") && !flags.agent {
				return cmd.Help()
			}
			return usageErr(fmt.Errorf("shortlist requires --from YYYY-MM-DD and --to YYYY-MM-DD"))
		}
		if err := validateWalkerQueryBounds(query, enrich); err != nil {
			return usageErr(err)
		}
		normalized, err := walkerplus.NormalizeQuery(query)
		if err != nil {
			return usageErr(err)
		}
		if err := validateWalkerSelection(flags.selectFields); err != nil {
			return usageErr(err)
		}
		if cliutil.IsDogfoodEnv() {
			normalized.MaxPages = 1
			if normalized.MaxDetails > 3 {
				normalized.MaxDetails = 3
			}
			if normalized.Limit > 3 {
				normalized.Limit = 3
			}
		}
		client, err := newWalkerService(flags)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		var result walkerplus.Result
		if enrich {
			result, err = client.Shortlist(ctx, normalized)
		} else {
			result, err = client.Search(ctx, normalized)
		}
		if err != nil {
			return writeWalkerFetchError(cmd, flags, err)
		}
		if result.Events == nil {
			result.Events = []walkerplus.Event{}
		}
		normalizeWalkerCoverage(&result.Coverage)
		payload, err := walkerJSONMap(result)
		if err != nil {
			return err
		}
		if len(result.Events) == 0 {
			payload["note"] = "No matches in the sampled source pages. --max-pages widens the bounded scan; future editions may remain undisclosed."
		}
		payload["meta"] = walkerMeta("live")
		if result.Coverage.Incomplete {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: source coverage is incomplete: %s\n", strings.Join(result.Coverage.Reasons, "; "))
		}
		return printWalkerPayload(cmd, flags, payload, "events")
	}
	cmd.Flags().StringVar(&query.Prefecture, "prefecture", "", "Prefecture alias or source code; discover with areas")
	cmd.Flags().StringVar(&query.City, "city", "", "City code, source slug or Japanese name; discover with areas --prefecture")
	cmd.Flags().StringVar(&query.Category, "category", "", "Event category alias or source code; discover with categories")
	cmd.Flags().StringVar(&query.From, "from", "", "First trip date in Asia/Tokyo, as YYYY-MM-DD")
	cmd.Flags().StringVar(&query.To, "to", "", "Last trip date in Asia/Tokyo, as YYYY-MM-DD")
	cmd.Flags().StringVar(&query.Timing, "timing", "overlap", "Match source envelope overlap, starts, or ends")
	cmd.Flags().StringVar(&query.Sort, "sort", "relevance", "Stable result order: relevance, start, end, or source")
	cmd.Flags().IntVar(&query.Limit, "limit", 10, "Maximum returned events, from 1 to 100")
	cmd.Flags().IntVar(&query.Page, "page", 1, "First source listing page to sample, from 1 to 1000")
	cmd.Flags().IntVar(&query.MaxPages, "max-pages", 3, "Maximum sampled listing pages, from 1 to 20")
	query.MaxDetails = 10
	if enrich {
		cmd.Flags().IntVar(&query.MaxDetails, "max-details", 10, "Maximum candidates to enrich, from 1 to 30")
		cmd.Flags().BoolVar(&query.Free, "free", false, "Require explicit free event admission evidence")
		cmd.Flags().BoolVar(&query.Indoor, "indoor", false, "Require unconditional explicit indoor venue evidence")
	}
	return cmd
}

func validateWalkerQueryBounds(q walkerplus.Query, enrich bool) error {
	if q.Limit < 1 || q.Limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100")
	}
	if q.Page < 1 || q.Page > 1000 {
		return fmt.Errorf("--page must be between 1 and 1000")
	}
	if q.MaxPages < 1 || q.MaxPages > 20 {
		return fmt.Errorf("--max-pages must be between 1 and 20")
	}
	if enrich && (q.MaxDetails < 1 || q.MaxDetails > 30) {
		return fmt.Errorf("--max-details must be between 1 and 30")
	}
	return nil
}

var walkerEventID = regexp.MustCompile("^ar[0-9]{4}e[0-9]+$")

func validateWalkerEventInput(input string) error {
	if walkerEventID.MatchString(input) {
		return nil
	}
	u, err := url.Parse(input)
	if err == nil && u.Scheme == "https" && (u.Host == "www.walkerplus.com" || u.Host == "walkerplus.com") && u.RawQuery == "" && u.Fragment == "" && u.User == nil {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 2 && parts[0] == "event" && walkerEventID.MatchString(parts[1]) {
			return nil
		}
	}
	return fmt.Errorf("event requires a Walkerplus ID such as ar0313e603640 or https://www.walkerplus.com/event/ar0313e603640/")
}

func newWalkerEventCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "event [ID|URL]",
		Short:       "Read one event edition with schedule, admission and source evidence",
		Long:        "Fetch the event, data and price pages for one stable Walkerplus ID. Preserve the original Japanese title, source schedule, exclusions, hours, access, weather, cancellation and reservation facts. Missing scalar facts are null. --select projects event fields and retains coverage.",
		Annotations: walkerAnnotations("event=ar0313e603640"),
		Example:     strings.Trim("\n  walkerplus-pp-cli event ar0313e603640\n  walkerplus-pp-cli event https://www.walkerplus.com/event/ar0727e612159/ --select id,title_ja,schedule,admission,indoor,sources\n  walkerplus-pp-cli event --dry-run --json\n  EVENT_ID=ar0313e603640\n  walkerplus-pp-cli event \"${EVENT_ID}\" --refresh --no-cache --select id,title_ja,source_url,start_date,end_date,location,schedule,admission,reservation_required,sources", "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeWalkerDryRun(cmd, flags, "event")
			}
			if len(args) == 0 && !cmd.Flags().Changed("json") && !flags.agent && !cmd.Flags().Changed("select") && !cmd.Flags().Changed("fields") {
				return cmd.Help()
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("event requires exactly one Walkerplus ID or URL"))
			}
			if err := validateWalkerEventInput(args[0]); err != nil {
				return usageErr(err)
			}
			if err := validateWalkerSelection(flags.selectFields); err != nil {
				return usageErr(err)
			}
			client, err := newWalkerService(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			event, err := client.Event(ctx, args[0])
			if err != nil {
				return writeWalkerFetchError(cmd, flags, err)
			}
			coverage := client.Stats()
			normalizeWalkerCoverage(&coverage)
			return printWalkerPayload(cmd, flags, map[string]any{"event": event, "coverage": coverage, "meta": walkerMeta("live")}, "event")
		},
	}
}

func newWalkerAreasCmd(flags *rootFlags) *cobra.Command {
	var prefecture string
	cmd := &cobra.Command{
		Use:         "areas",
		Short:       "Discover prefectures, or current cities within one prefecture",
		Annotations: walkerAnnotations("--prefecture=tokyo"),
		Example:     strings.Trim("\n  walkerplus-pp-cli areas\n  walkerplus-pp-cli areas --prefecture tokyo\n  walkerplus-pp-cli areas --dry-run", "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeWalkerDryRun(cmd, flags, "areas")
			}
			if len(args) != 0 {
				return usageErr(fmt.Errorf("areas accepts --prefecture, not positional arguments"))
			}
			if prefecture != "" {
				if _, err := walkerplus.NormalizeQuery(walkerplus.Query{Prefecture: prefecture}); err != nil {
					return usageErr(err)
				}
			}
			client, err := newWalkerService(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			result, err := client.Areas(ctx, prefecture)
			if err != nil {
				return writeWalkerFetchError(cmd, flags, err)
			}
			if result.Items == nil {
				result.Items = []walkerplus.CatalogItem{}
			}
			normalizeWalkerCoverage(&result.Coverage)
			payload, err := walkerJSONMap(result)
			if err != nil {
				return err
			}
			origin := "catalogue"
			if prefecture != "" {
				origin = "live"
			}
			payload["meta"] = walkerMeta(origin)
			return printWalkerPayload(cmd, flags, payload, "")
		},
	}
	cmd.Flags().StringVar(&prefecture, "prefecture", "", "Prefecture alias or source code to fetch its current city routes")
	return cmd
}

func newWalkerCategoriesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "categories",
		Short:       "List source-derived Japanese event categories and accepted aliases",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "catalogue"},
		Example:     "  walkerplus-pp-cli categories\n  walkerplus-pp-cli categories --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeWalkerDryRun(cmd, flags, "categories")
			}
			if len(args) != 0 {
				return usageErr(fmt.Errorf("categories accepts no positional arguments"))
			}
			client, err := newWalkerService(flags)
			if err != nil {
				return err
			}
			result := client.Categories()
			if result.Items == nil {
				result.Items = []walkerplus.CatalogItem{}
			}
			normalizeWalkerCoverage(&result.Coverage)
			payload, err := walkerJSONMap(result)
			if err != nil {
				return err
			}
			payload["meta"] = walkerMeta("catalogue")
			return printWalkerPayload(cmd, flags, payload, "")
		},
	}
}

func newWalkerService(flags *rootFlags) (walkerService, error) {
	runtime := flags.walkerRuntime
	if runtime == nil {
		runtime = &walkerRuntime{cacheTTL: "1h", requestTimeout: "15s", concurrency: 2, retries: 2}
	}
	if flags.timeout <= 0 || flags.timeout > 60*time.Second {
		return nil, usageErr(fmt.Errorf("--timeout must be greater than zero and at most 60s"))
	}
	if runtime.concurrency < 1 || runtime.concurrency > 4 {
		return nil, usageErr(fmt.Errorf("--concurrency must be between 1 and 4"))
	}
	if runtime.retries < 0 || runtime.retries > 3 {
		return nil, usageErr(fmt.Errorf("--retries must be between 0 and 3"))
	}
	ttl, err := cliutil.ParseDurationLoose(runtime.cacheTTL)
	if err != nil || ttl <= 0 || ttl > 24*time.Hour {
		return nil, usageErr(fmt.Errorf("--cache-ttl must be a positive duration at most 1d (for example 1h)"))
	}
	timeout, err := cliutil.ParseDurationLoose(runtime.requestTimeout)
	if err != nil || timeout <= 0 || timeout > 15*time.Second {
		return nil, usageErr(fmt.Errorf("--request-timeout must be a positive duration at most 15s"))
	}
	if flags.dataSource == "local" {
		return nil, usageErr(fmt.Errorf("Walkerplus reads source pages; use --no-cache for direct HTTP or --cache-dir for its HTML cache"))
	}
	client, err := makeWalkerClient(walkerplus.Options{CacheDir: runtime.cacheDir, CacheTTL: ttl, Timeout: timeout, Refresh: runtime.refresh || cliutil.IsDogfoodEnv(), NoCache: flags.noCache, Concurrency: runtime.concurrency, Retries: runtime.retries})
	if err != nil {
		return nil, apiErr(err)
	}
	return client, nil
}

func walkerFetchError(err error) error {
	if errors.Is(err, walkerplus.ErrInvalidQuery) {
		return usageErr(err)
	}
	var limited *cliutil.RateLimitError
	if errors.As(err, &limited) {
		return rateLimitErr(err)
	}
	if errors.Is(err, walkerplus.ErrNotFound) {
		return notFoundErr(err)
	}
	var status interface{ HTTPStatus() int }
	if errors.As(err, &status) && status.HTTPStatus() == 404 {
		return notFoundErr(err)
	}
	return apiErr(err)
}

func writeWalkerFetchError(cmd *cobra.Command, flags *rootFlags, err error) error {
	typed := walkerFetchError(err)
	payload := map[string]any{"error": map[string]any{"code": ExitCode(typed), "message": err.Error()}, "meta": walkerMeta("live")}
	copyFlags := *flags
	copyFlags.selectFields = ""
	if writeErr := printWalkerPayload(cmd, &copyFlags, payload, ""); writeErr != nil {
		return writeErr
	}
	return typed
}
