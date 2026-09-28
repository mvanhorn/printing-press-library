package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The adapter seam lets command tests exercise output and flag propagation
// without substituting fixtures for the provider's parsing tests or live E2E.
type navitimeService interface {
	Places(context.Context, string, string, int) (navitime.PlacesResult, error)
	Routes(context.Context, navitime.Query) (navitime.RouteResult, error)
	Passes(context.Context) (navitime.PassResult, error)
	Show(context.Context, string) (navitime.DetailResult, error)
	Metrics() navitime.Metrics
}

var newNavitimeClient = func(options navitime.Options) (navitimeService, error) {
	return navitime.NewClient(options)
}

func navitimeCacheDir(flags *rootFlags) (string, error) {
	path := strings.TrimSpace(flags.cacheDir)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("NAVITIME_CACHE_DIR"))
	}
	if path != "" {
		if clean, ok := cliutil.CleanPathOverride(path); ok {
			return clean, nil
		}
		return "", usageErr(fmt.Errorf("--cache-dir / NAVITIME_CACHE_DIR must be an absolute directory path"))
	}
	base, err := cliutil.CacheDir()
	if err != nil {
		return "", configErr(fmt.Errorf("resolve public response cache: %w", err))
	}
	return filepath.Join(base, "navitime"), nil
}

func navitimeError(err error) error {
	var argument *navitime.ArgumentError
	if errors.As(err, &argument) {
		return usageErr(err)
	}
	var missing *navitime.NotFoundError
	if errors.As(err, &missing) {
		return notFoundErr(err)
	}
	var throttled *cliutil.RateLimitError
	if errors.As(err, &throttled) {
		return rateLimitErr(err)
	}
	var source *navitime.SourceError
	if errors.As(err, &source) {
		if source.Status == 404 {
			return notFoundErr(err)
		}
	}
	return apiErr(err)
}

// Keep the generated projection/format pipeline, but retain all domain fields
// and serialize its validated JSON compactly unless the operator asks for it.
func printNavitime(cmd *cobra.Command, flags *rootFlags, value any) (int, error) {
	outputFlags := *flags
	outputFlags.asJSON = true
	outputFlags.compact = false
	// Domain responses already carry source/freshness metadata. Keep their
	// paths stable under --agent, including a projected single array.
	outputFlags.agent = false
	var rendered bytes.Buffer
	outputCmd := &cobra.Command{}
	outputCmd.SetOut(&rendered)
	err := outputFlags.printJSON(outputCmd, value)
	data := rendered.Bytes()
	if !flags.pretty && !flags.csv && !flags.plain && !flags.quiet && len(data) > 0 {
		var compact bytes.Buffer
		if compactErr := json.Compact(&compact, data); compactErr != nil {
			return 0, compactErr
		}
		compact.WriteByte('\n')
		data = compact.Bytes()
	}
	n, writeErr := cmd.OutOrStdout().Write(data)
	if writeErr != nil {
		return n, writeErr
	}
	return n, err
}

func emitNavitimeMetrics(w io.Writer, metrics navitime.Metrics, elapsed time.Duration, outputBytes int) {
	_ = json.NewEncoder(w).Encode(struct {
		RequestCount  int     `json:"request_count"`
		CacheHits     int     `json:"cache_hits"`
		CacheWrites   int     `json:"cache_writes"`
		Retries       int     `json:"retries"`
		BytesReceived int64   `json:"bytes_received"`
		LatencyMS     float64 `json:"latency_ms"`
		OutputBytes   int     `json:"output_bytes"`
	}{metrics.Requests, metrics.CacheHits, metrics.CacheWrites, metrics.Retries, metrics.BytesReceived, float64(elapsed.Microseconds()) / 1000, outputBytes})
}

func navitimeDryRun(cmd *cobra.Command, flags *rootFlags, action string) error {
	started := time.Now()
	previewFlags := *flags
	// The preview has its own schema; domain paths apply only to real results.
	previewFlags.selectFields = ""
	n, err := printNavitime(cmd, &previewFlags, dryRunResult{DryRun: true, Action: action, Would: "run " + action + "; no requests or cache changes made"})
	if flags.metrics {
		emitNavitimeMetrics(cmd.ErrOrStderr(), navitime.Metrics{}, time.Since(started), n)
	}
	return err
}

func runNavitime(cmd *cobra.Command, flags *rootFlags, refresh bool, run func(context.Context, navitimeService) (any, error)) error {
	if flags.timeout <= 0 {
		return usageErr(fmt.Errorf("--timeout must be greater than zero (for example --timeout 30s)"))
	}
	cacheDir, err := navitimeCacheDir(flags)
	if err != nil {
		return err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	started := time.Now()
	c, err := newNavitimeClient(navitime.Options{CacheDir: cacheDir, Timeout: flags.timeout, Refresh: refresh || flags.dataSource == "live", NoCache: flags.noCache})
	if err != nil {
		return configErr(err)
	}
	outputBytes := 0
	defer func() {
		if flags.metrics {
			emitNavitimeMetrics(cmd.ErrOrStderr(), c.Metrics(), time.Since(started), outputBytes)
		}
	}()
	value, err := run(ctx, c)
	if err != nil {
		return navitimeError(err)
	}
	outputBytes, err = printNavitime(cmd, flags, value)
	return err
}

func navitimeParentRunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usageErr(fmt.Errorf("unknown subcommand %q; run '%s --help'", args[0], cmd.CommandPath()))
	}
	return cmd.Help()
}

func boundedLimit(limit, maximum int) error {
	if limit < 1 || limit > maximum {
		return usageErr(fmt.Errorf("--limit must be between 1 and %d", maximum))
	}
	return nil
}

func navitimeHelpOnly(cmd *cobra.Command, args []string) bool {
	if len(args) != 0 {
		return false
	}
	changed := false
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			changed = true
		}
	})
	return !changed
}
