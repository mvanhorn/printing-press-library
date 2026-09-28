package cli

// pp:data-source auto

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type tenkiSource interface {
	Search(context.Context, string, string, int, int) (tenki.SearchResult, error)
	Resolve(context.Context, string) (tenki.PlaceResult, error)
	Daily(context.Context, string) (tenki.ForecastResult, error)
	Hourly(context.Context, string) (tenki.ForecastResult, error)
	SeasonalList(context.Context, string, string, int, int, int) (tenki.SeasonalListResult, error)
	Seasonal(context.Context, string, string, int) (tenki.SeasonalResult, error)
	Mountain(context.Context, string) (tenki.MountainResult, error)
	Metrics() tenki.Metrics
}

type tenkiFactory func(tenki.Config) tenkiSource

type tenkiReadFlags struct {
	refresh    bool
	allowStale bool
	cacheDir   string
	directory  string
}

func (o *tenkiReadFlags) attach(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.refresh, "refresh", false, "Fetch again instead of using a fresh cached page")
	cmd.Flags().BoolVar(&o.allowStale, "allow-stale", false, "Permit explicitly marked stale cache on unavailable live/local data")
	cmd.Flags().StringVar(&o.cacheDir, "cache-dir", "", "Page cache directory (default: resolved tenki cache directory)")
}

func (o tenkiReadFlags) source(flags *rootFlags, factory tenkiFactory) (tenkiSource, error) {
	if flags.dataSource == "local" && (o.refresh || flags.noCache) {
		return nil, usageErr(fmt.Errorf("--refresh/--no-cache cannot be combined with --data-source local"))
	}
	cacheDir := o.cacheDir
	if cacheDir == "" {
		var err error
		cacheDir, err = cliutil.CacheDir()
		if err != nil {
			return nil, configErr(err)
		}
	}
	return factory(tenki.Config{CacheDir: cacheDir, Refresh: o.refresh || flags.noCache || flags.dataSource == "live", AllowStale: o.allowStale, Local: flags.dataSource == "local", Timeout: flags.timeout, RateLimit: flags.rateLimit, SearchDirectory: o.directory}), nil
}

func tenkiAnnotations(happy string) map[string]string {
	return map[string]string{"pp:data-source": "auto", "mcp:read-only": "true", "pp:happy-args": happy}
}

func tenkiPrelude(cmd *cobra.Command, args []string, flags *rootFlags) (bool, error) {
	if dryRunOK(flags) {
		return true, writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
	}
	if len(args) == 0 && !hasChangedLocalFlags(cmd) && !hasChangedPersistentFlags(cmd) {
		return true, cmd.Help()
	}
	if len(args) != 0 {
		return true, usageErr(fmt.Errorf("%s accepts flags only; see --help", cmd.CommandPath()))
	}
	return false, nil
}

func hasChangedPersistentFlags(cmd *cobra.Command) bool {
	changed := false
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			changed = true
		}
	})
	return changed
}

var tenkiPlacePath = regexp.MustCompile(`^/(forecast/[0-9]+/[0-9]+/[0-9]+/[0-9]+/(?:1hour\.html|10days\.html)?|leisure/(?:[0-9]+/){4}|(?:mountain/(?:famous100|normal)|sakura|kouyou)/(?:[0-9]+/){2}[0-9]+\.html)$`)

func validateTenkiPlace(value string) error {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || value != strings.TrimSpace(value) || u.Scheme != "https" || u.Host != "tenki.jp" || u.User != nil || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" || !tenkiPlacePath.MatchString(u.Path) {
		return usageErr(fmt.Errorf("--place must be a canonical https://tenki.jp/ forecast, leisure, mountain, sakura or kouyou URL; resolve it with places search"))
	}
	return nil
}

func tenkiBound(name string, value, max int) error {
	if value < 1 || value > max {
		return usageErr(fmt.Errorf("--%s must be between 1 and %d", name, max))
	}
	return nil
}

func tenkiDate(name, value string) (time.Time, error) {
	location := time.FixedZone("JST", 9*60*60)
	date, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil || date.Format("2006-01-02") != value {
		return time.Time{}, usageErr(fmt.Errorf("--%s must use YYYY-MM-DD", name))
	}
	return date, nil
}

func tenkiToday() string { return time.Now().In(time.FixedZone("JST", 9*60*60)).Format("2006-01-02") }

type tenkiHourWindow struct{ Start, End int }

func parseTenkiHours(value string) (tenkiHourWindow, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return tenkiHourWindow{}, usageErr(fmt.Errorf("--hours must use HH:00-HH:00, with increasing endpoints within 00:00-24:00"))
	}
	parse := func(s string) (int, bool) {
		if len(s) != 5 || s[2:] != ":00" || s[0] < '0' || s[0] > '2' || s[1] < '0' || s[1] > '9' {
			return 0, false
		}
		h := int(s[0]-'0')*10 + int(s[1]-'0')
		return h, h <= 24
	}
	start, ok1 := parse(parts[0])
	end, ok2 := parse(parts[1])
	if !ok1 || !ok2 || start >= end {
		return tenkiHourWindow{}, usageErr(fmt.Errorf("--hours must use HH:00-HH:00, with increasing endpoints within 00:00-24:00"))
	}
	return tenkiHourWindow{Start: start, End: end}, nil
}

func tenkiProductError(err error) error {
	var rateErr *cliutil.RateLimitError
	if errors.As(err, &rateErr) {
		return rateLimitErr(err)
	}
	return apiErr(err)
}

// Keep the generated field selector/agent envelope while emitting one compact
// JSON line. Product views choose their summary fields before this renderer.
func printTenki(cmd *cobra.Command, flags *rootFlags, value any, sources ...tenki.Source) error {
	for _, source := range sources {
		if source.URL == "" {
			continue
		}
		flags.agentSource = "live"
		if source.FromCache {
			flags.agentSource = "local"
		}
		break
	}
	format := *flags
	// This command already owns its provenance envelope. Projection is applied
	// once and must not introduce a second results wrapper in --agent mode.
	format.agent = false
	// Product controllers already select summary/detail evidence. Generic
	// structural compaction drops sparse temporal and confidence fields;
	// retain the evidence and compact JSON whitespace below instead.
	format.compact = false
	if flags.selectFields != "" {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		matched := false
		for _, field := range strings.Split(flags.selectFields, ",") {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			path := strings.Split(field, ".")
			for i := range path {
				path[i] = strings.ToLower(path[i])
			}
			_, state := filterFieldsRec(raw, [][]string{path}, true)
			// An unrelated empty list can only produce fallbackIndeterminate.
			if state.matched || state.anchoredIndeterminate {
				matched = true
				break
			}
		}
		if !matched {
			return usageErr(fmt.Errorf("--select %q matched no fields; inspect unprojected output or --help", flags.selectFields))
		}
	}
	if !format.csv && !format.plain && !format.quiet {
		format.asJSON = true
	}
	var buffer bytes.Buffer
	err := printJSONFiltered(&buffer, value, &format)
	if format.csv || format.plain || format.quiet {
		_, writeErr := cmd.OutOrStdout().Write(buffer.Bytes())
		if writeErr != nil {
			return writeErr
		}
		return err
	}
	var compact bytes.Buffer
	if compactErr := json.Compact(&compact, buffer.Bytes()); compactErr != nil {
		if err != nil {
			return err
		}
		return compactErr
	}
	compact.WriteByte('\n')
	_, writeErr := cmd.OutOrStdout().Write(compact.Bytes())
	if writeErr != nil {
		return writeErr
	}
	return err
}

func emitTenki(cmd *cobra.Command, flags *rootFlags, client tenkiSource, value any, sources ...tenki.Source) error {
	source := "live"
	for _, item := range sources {
		if item.URL == "" {
			continue
		}
		if item.FromCache {
			source = "local"
		}
		break
	}
	meta := map[string]any{"schema_version": "1", "provider": "tenki.jp", "source": source, "timezone": tenki.Timezone, "metrics": client.Metrics()}
	return printTenki(cmd, flags, map[string]any{"meta": meta, "results": value}, sources...)
}

func tenkiWarnings(cmd *cobra.Command, warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
	}
}
