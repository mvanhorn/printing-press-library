// Hand-authored — NOT generated. Implements `gisis-pp-cli ship batch`: resolve
// a list of IMOs in one invocation, throttled for GISIS politeness, persisting
// each to the local cache. The throttled resolve loop (resolveIMOs) is shared
// with `ship refresh`.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
)

// batchItemResult is the per-IMO outcome of a batch/refresh run.
type batchItemResult struct {
	IMONumber string `json:"imo_number"`
	Status    string `json:"status"` // ok | not_found | auth_error | cache_error | error
	Name      string `json:"name,omitempty"`
	Flag      string `json:"flag,omitempty"`
	Error     string `json:"error,omitempty"`
}

// resolveIMOs fetches each IMO under a shared client (so the adaptive rate
// limiter applies across the whole run), sleeps `throttle` between requests for
// GISIS politeness, and upserts each success into the local cache. Per-item
// failures are recorded in the result rather than aborting the run. A cancelled
// context returns the partial results gathered so far plus ctx.Err().
func resolveIMOs(cmd *cobra.Command, flags *rootFlags, imos []string, throttle time.Duration, forceFresh bool) ([]batchItemResult, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	if forceFresh {
		c.NoCache = true
	}

	results := make([]batchItemResult, 0, len(imos))
	ctx := cmd.Context()
	for i, imo := range imos {
		if i > 0 && throttle > 0 {
			select {
			case <-time.After(throttle):
			case <-ctx.Done():
				return results, ctx.Err()
			}
		}

		ship, ferr := fetchShipParticulars(ctx, c, imo)
		if ferr != nil {
			results = append(results, batchItemResult{IMONumber: imo, Status: classifyBatchStatus(ferr), Error: ferr.Error()})
			continue
		}
		if cerr := cacheShipParticulars(ctx, flags, ship); cerr != nil {
			results = append(results, batchItemResult{IMONumber: ship.IMONumber, Status: "cache_error", Name: ship.Name, Flag: ship.Flag, Error: cerr.Error()})
			continue
		}
		results = append(results, batchItemResult{IMONumber: ship.IMONumber, Status: "ok", Name: ship.Name, Flag: ship.Flag})
	}
	return results, nil
}

func classifyBatchStatus(err error) string {
	switch {
	case errors.Is(err, errLoginWall):
		return "auth_error"
	case errors.Is(err, errNotFound):
		return "not_found"
	default:
		return "error"
	}
}

// parseIMOList splits a comma- or whitespace-separated list of IMO numbers.
func parseIMOList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}

func readIMOFile(path string) ([]string, error) {
	// #nosec G304 -- path is the user's own --file argument; reading it is the command's purpose
	b, err := os.ReadFile(path) //nolint:gosec // user-supplied --file path is expected
	if err != nil {
		return nil, err
	}
	return parseIMOList(string(b)), nil
}

func dedupeIMOs(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// emitBatchResults prints results and returns runErr; partial output is still
// emitted when the run was interrupted.
func emitBatchResults(cmd *cobra.Command, flags *rootFlags, results []batchItemResult, runErr error) error {
	if runErr != nil {
		if len(results) > 0 {
			_ = printJSONFiltered(cmd.OutOrStdout(), results, flags)
		}
		return runErr
	}
	return printJSONFiltered(cmd.OutOrStdout(), results, flags)
}

func newShipBatchCmd(flags *rootFlags) *cobra.Command {
	var flagImos, flagFile string
	var flagThrottle time.Duration

	cmd := &cobra.Command{
		Use:     "batch",
		Short:   "Resolve many IMOs in one run, throttled, persisting each to the local cache.",
		Long:    "Fetches each IMO's Ship Particulars in turn (default 2s between requests for GISIS politeness) and caches the result. Per-vessel failures are reported in the output without stopping the run, so a bad IMO never sinks the batch.",
		Example: "  gisis-pp-cli ship batch --imos 9866641,9966233 --json\n  gisis-pp-cli ship batch --file imos.txt --throttle 3s",
		// PATCH(pr-953 greptile): no mcp:read-only — batch fetches from GISIS and writes the cache.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			imos := parseIMOList(flagImos)
			if flagFile != "" {
				fileIMOs, err := readIMOFile(flagFile)
				if err != nil {
					return fmt.Errorf("reading --file %q: %w", flagFile, err)
				}
				imos = append(imos, fileIMOs...)
			}
			imos = dedupeIMOs(imos)
			if len(imos) == 0 {
				return usageErr(fmt.Errorf("no IMO numbers given: pass --imos 9866641,9966233 or --file path"))
			}
			// PATCH(pr-953 greptile): reject malformed IMOs up front (same 7-digit guard
			// as ship pin/unpin) so typos and runner sentinels exit 2 instead of each
			// firing a throttled GISIS request that just logs not_found.
			var badIMOs []string
			for _, imo := range imos {
				if !isValidIMOFormat(imo) {
					badIMOs = append(badIMOs, imo)
				}
			}
			if len(badIMOs) > 0 {
				return usageErr(fmt.Errorf("invalid IMO number(s): %s — each must be 7 digits", strings.Join(badIMOs, ", ")))
			}
			results, err := resolveIMOs(cmd, flags, imos, flagThrottle, false)
			return emitBatchResults(cmd, flags, results, err)
		},
	}
	cmd.Flags().StringVar(&flagImos, "imos", "", "Comma- or space-separated IMO numbers")
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to a file of IMO numbers (one per line or comma-separated)")
	cmd.Flags().DurationVar(&flagThrottle, "throttle", 2*time.Second, "Delay between requests for GISIS politeness")
	return cmd
}
