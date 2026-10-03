// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

type tripFailure struct {
	Reference string `json:"reference"`
	Error     string `json:"error"`
}
type tripComparison struct {
	Records          []trip.Assessment `json:"records"`
	FetchFailures    []tripFailure     `json:"fetch_failures"`
	RequestedRecords int               `json:"requested_records"`
	ComparedRecords  int               `json:"compared_records"`
	On               string            `json:"on"`
	AsOf             string            `json:"as_of"`
	Note             string            `json:"note"`
	Scope            string            `json:"scope"`
}

func newNovelTripCompareCmd(flags *rootFlags) *cobra.Command {
	var refs, on, asOf, amenities string
	var age int
	cmd := &cobra.Command{
		Use: "compare [references...]", Short: "Compare up to eight Iko-yo Trip references for visit date, age and amenities with qualified fees and application windows",
		Long: "Compare up to eight canonical Trip references. Age and amenities use supported/excluded/unknown evidence states, never a recommendation guarantee. A date inside a published multi-day span remains unknown for individual operation. Qualified child/adult fees stay separate; no family total is invented. Published application windows, capacities and lotteries do not establish available seats. Constraint inputs are per-invocation and not automatically learned. " + trip.ScopeNote,
		Example: strings.Trim(`
  iko-yo-pp-cli trip compare spots/8220 events/8412 --on 2026-11-15 --as-of 2026-10-03 --age-months 24 --amenities indoor,nursing --agent
  iko-yo-pp-cli trip compare --refs spots/8220,events/8412 --data-source local --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "reference=spots/8220;reference=events/8412;--on=2026-11-15;--as-of=2026-10-03;--age-months=24;--amenities=indoor,nursing"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trip compare")
			}
			if refs != "" && len(args) > 0 {
				return usageErr(fmt.Errorf("use positional references or --refs, not both"))
			}
			input := args
			if refs != "" {
				input = strings.Split(refs, ",")
			}
			if len(input) < 1 || len(input) > 8 {
				return usageErr(fmt.Errorf("trip compare requires one to eight spots/ID or events/ID references"))
			}
			canonical := make([]string, 0, len(input))
			seen := map[string]bool{}
			for _, ref := range input {
				k, id, e := trip.ParseReference(strings.TrimSpace(ref))
				if e != nil {
					return usageErr(e)
				}
				v := k + "/" + id
				if seen[v] {
					return usageErr(fmt.Errorf("duplicate Trip reference %s", v))
				}
				seen[v] = true
				canonical = append(canonical, v)
			}
			if on != "" && !trip.ValidDate(on) || !trip.ValidDate(asOf) {
				return usageErr(fmt.Errorf("--on and --as-of require valid YYYY-MM-DD dates"))
			}
			wanted := tripAmenities(amenities)
			if err := trip.ValidateQuery(trip.Query{Kind: "all", From: on, To: on, AgeMonths: age, Amenities: wanted, Limit: 8}); err != nil {
				return usageErr(err)
			}
			if flags.dataSource == "local" && flags.noCache {
				return usageErr(fmt.Errorf("--no-cache conflicts with --data-source local"))
			}
			if cliutil.IsDogfoodEnv() && len(canonical) > 2 {
				canonical = canonical[:2]
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ctx, capCancel := context.WithTimeout(ctx, time.Minute)
			defer capCancel()
			c := trip.New(flags.rateLimit)
			view := tripComparison{Records: make([]trip.Assessment, 0), FetchFailures: make([]tripFailure, 0), RequestedRecords: len(canonical), On: on, AsOf: asOf, Note: "Checks reflect only explicit published evidence. Fees retain qualifiers; application intervals are not inventory. Saved observations retain their original timestamps.", Scope: trip.ScopeNote}
			source := ""
			var firstErr error
			for _, ref := range canonical {
				r, err := tripRead(ctx, cmd, flags, c, ref)
				if err != nil {
					var rate *cliutil.RateLimitError
					if errors.As(err, &rate) {
						return tripError(err)
					}
					if ctx.Err() != nil {
						return tripError(err)
					}
					if firstErr == nil {
						firstErr = err
					}
					view.FetchFailures = append(view.FetchFailures, tripFailure{Reference: ref, Error: err.Error()})
					continue
				}
				r = trip.RefreshStatus(r, asOf)
				view.Records = append(view.Records, trip.Evaluate(r, on, asOf, age, wanted))
				if source == "" {
					source = r.DataSource
				} else if source != r.DataSource {
					source = "mixed"
				}
			}
			view.ComparedRecords = len(view.Records)
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d Trip reads failed; comparison includes only %d successful records\n", len(view.FetchFailures), view.RequestedRecords, view.ComparedRecords)
			}
			if len(view.Records) == 0 {
				return tripError(firstErr)
			}
			tripSource(cmd, flags, source)
			return tripPrintComparison(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&refs, "refs", "", "Comma-separated Trip references, alternative to positional arguments")
	cmd.Flags().StringVar(&on, "on", "", "YYYY-MM-DD visit date checked against explicit published schedules")
	cmd.Flags().StringVar(&asOf, "as-of", tripToday(), "YYYY-MM-DD evaluation date for event status and application windows")
	cmd.Flags().IntVar(&age, "age-months", -1, "Age in months, zero to 216; omit for no age check")
	cmd.Flags().StringVar(&amenities, "amenities", "", "Required evidence keys: indoor,nursing,changing,stroller, comma separated")
	return cmd
}
