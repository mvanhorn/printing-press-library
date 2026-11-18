// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call (calls the market shop through internal/edeka.Shop)

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func newNovelSlotsEarliestCmd(flags *rootFlags) *cobra.Command {
	var market, typ string
	cmd := &cobra.Command{
		Use:         "earliest",
		Short:       "The first free pickup slot of your market in one call.",
		Long:        "Reads the market's slot calendar and returns the first free slot (pickup by default). Works without login.\nUse 'slots reserve earliest' to book it for the order.",
		Example:     "  edeka-pp-cli slots earliest --market doerflinger-ulm --json\n  edeka-pp-cli slots earliest --type delivery",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--json;--market=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "slots earliest")
			}
			sh, err := openPublicShop(market)
			if err != nil {
				return err
			}
			cal, err := sh.Calendar(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			s, err := earliestSlot(cal, typ)
			if err != nil {
				return notFoundErr(err)
			}
			out := map[string]any{"market": sh.Host, "slot": s, "text": slotText(s)}
			if typ == "pickup" && cal.PickupAddress != "" {
				out["pickup_address"] = cal.PickupAddress
			}
			return emit(cmd, flags, out, func(w io.Writer) { fmt.Fprintf(w, "Earliest %s slot: %s\n", typ, slotText(s)) })
		},
	}
	cmd.Flags().StringVar(&typ, "type", "pickup", "Slot type: pickup or delivery")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

// earliestSlot returns the first available slot of the type.
func earliestSlot(cal *edeka.Calendar, typ string) (edeka.Slot, error) {
	list := cal.Pickup
	switch strings.ToLower(typ) {
	case "pickup", "":
	case "delivery":
		list = cal.Delivery
		if len(list) == 0 && len(cal.DeliveryAreas) > 0 {
			return edeka.Slot{}, errors.New("delivery slots show only after a delivery area is chosen in the shop; use pickup, or choose the area in the browser once")
		}
	default:
		return edeka.Slot{}, fmt.Errorf("unknown slot type %q (pickup or delivery)", typ)
	}
	for _, s := range list {
		if s.Available {
			return s, nil
		}
	}
	return edeka.Slot{}, fmt.Errorf("no free %s slot in the market's calendar", typ)
}

var weekdays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// slotText renders a slot as "Wed 2026-11-18 10:00-11:00 (pickup, free)".
func slotText(s edeka.Slot) string {
	day := s.Date
	if t, err := time.Parse("2006-01-02", s.Date); err == nil {
		day = weekdays[t.Weekday()][:3] + " " + s.Date
	}
	fee := "free"
	if s.FeeCents > 0 {
		fee = edeka.FormatCents(s.FeeCents)
	}
	return fmt.Sprintf("%s %s-%s (%s, %s)", day, s.From, s.Until, s.Type, fee)
}

// pickSlot resolves "earliest" or "YYYY-MM-DD HH:MM" against the calendar.
func pickSlot(ctx context.Context, sh *edeka.Shop, want, typ string) (edeka.Slot, error) {
	cal, err := sh.Calendar(ctx)
	if err != nil {
		return edeka.Slot{}, edekaErr(err)
	}
	want = strings.TrimSpace(want)
	if want == "" || strings.EqualFold(want, "earliest") {
		s, err := earliestSlot(cal, typ)
		if err != nil {
			return edeka.Slot{}, notFoundErr(err)
		}
		return s, nil
	}
	parts := strings.Fields(strings.ReplaceAll(want, "T", " "))
	if len(parts) != 2 {
		return edeka.Slot{}, usageErr(fmt.Errorf("slot %q: use 'earliest' or 'YYYY-MM-DD HH:MM'", want))
	}
	list := cal.Pickup
	if typ == "delivery" {
		list = cal.Delivery
	}
	for _, s := range list {
		if s.Date == parts[0] && (s.From == parts[1] || strings.HasPrefix(s.From, parts[1])) {
			if !s.Available {
				return edeka.Slot{}, notFoundErr(fmt.Errorf("slot %s is not free anymore", want))
			}
			return s, nil
		}
	}
	return edeka.Slot{}, notFoundErr(fmt.Errorf("no %s slot starts at %s (see 'slots list')", typ, want))
}
