// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

// earliestSlot returns the first slot that starts after now+minLead.
func earliestSlot(slots []rewe.Slot, now time.Time, minLead time.Duration) (rewe.Slot, bool) {
	sort.Slice(slots, func(i, j int) bool { return slots[i].StartTime.Before(slots[j].StartTime) })
	for _, s := range slots {
		if s.StartTime.After(now.Add(minLead)) {
			return s, true
		}
	}
	return rewe.Slot{}, false
}

func newNovelSlotsEarliestCmd(flags *rootFlags) *cobra.Command {
	var reserve bool
	var after string
	cmd := &cobra.Command{
		Use:   "earliest",
		Short: "The earliest free pickup slot with its fee, optionally reserved right away.",
		Long: "Lists the pickup slots of the selected market and returns the earliest one (optionally not\n" +
			"before --after, e.g. 2026-11-18T17:00). --reserve reserves it for the basket (the shop keeps\n" +
			"a reservation for about an hour unless the order is placed).",
		Example:     "  rewe-pp-cli slots earliest --json\n  rewe-pp-cli slots earliest --after 2026-11-18T17:00 --reserve",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "--json", "pp:preview-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyDryRun(cmd, flags, "slots earliest"); done {
				return err
			}
			sh, c, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			slots, err := sh.Slots(ctx)
			if err != nil {
				return reweErr(err)
			}
			from := time.Now()
			if after != "" {
				t, perr := time.ParseInLocation("2006-01-02T15:04", after, time.Local)
				if perr != nil {
					return usageErr(fmt.Errorf("--after must look like 2026-11-18T17:00: %w", perr))
				}
				from = t
			}
			s, ok := earliestSlot(slots, from, 0)
			if !ok {
				return notFoundErr(errors.New("no pickup slot available after the requested time"))
			}
			out := map[string]any{"slot": s, "start_local": s.StartTime.Local().Format("Mon 02.01. 15:04"), "end_local": s.EndTime.Local().Format("15:04"), "reserved": false, "available_slots": len(slots)}
			if reserve {
				if flags.dryRun {
					out["would_reserve"] = true
				} else {
					id, err := sh.BasketID(ctx)
					if err != nil {
						return reweErr(err)
					}
					b, err := sh.GetBasket(ctx, id)
					if err != nil {
						return reweErr(err)
					}
					r, err := sh.Reserve(ctx, s.ID, b.CustomerUUID, c.SelectedMarket, c.CustomerZipCode)
					if err != nil {
						return reweErr(err)
					}
					out["reserved"] = true
					out["reservation"] = r
				}
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Earliest pickup: %s - %s", out["start_local"], out["end_local"])
				if out["reserved"] == true {
					fmt.Fprint(w, " (reserved)")
				}
				fmt.Fprintln(w)
			})
		},
	}
	cmd.Flags().BoolVar(&reserve, "reserve", false, "Reserve the slot for the basket")
	cmd.Flags().StringVar(&after, "after", "", "Earliest acceptable start, local time, e.g. 2026-11-18T17:00")
	return cmd
}
