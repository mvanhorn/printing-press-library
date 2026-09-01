// Hand-authored transcendence command. generate --force preserves this file.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/cliutil"
)

func newNovelOversellWatchCmd(flags *rootFlags) *cobra.Command {
	var flagDays string
	var flagProperty string
	var flagMaxProps string

	cmd := &cobra.Command{
		Use:   "oversell-watch",
		Short: "Flag dates a channel still shows bookable on a property blocked/booked on the master calendar.",
		Long: "Queries Hostex live: compares each property's master availability calendar\n" +
			"against its per-channel listing inventory and flags dates where a channel still\n" +
			"shows inventory while the master calendar is blocked — double-sell risk.\n\n" +
			"Live command (needs a valid token). Pass --property to scope; otherwise it scans\n" +
			"properties up to --max-properties. For price drift use `price-parity` instead.",
		Example:     "  hostex-pp-cli oversell-watch --days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compare availabilities vs listing inventory over %s days\n", flagDays)
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			days, err := strconv.Atoi(strings.TrimSpace(flagDays))
			if err != nil || days <= 0 {
				return usageErr(fmt.Errorf("--days must be a positive integer"))
			}
			maxProps, err := strconv.Atoi(strings.TrimSpace(flagMaxProps))
			if err != nil || maxProps <= 0 {
				return usageErr(fmt.Errorf("--max-properties must be a positive integer"))
			}
			if cliutil.IsDogfoodEnv() {
				maxProps = 1
				if days > 7 {
					days = 7
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := nowUTC()
			start := now.Format("2006-01-02")
			end := now.AddDate(0, 0, days).Format("2006-01-02")

			// Resolve property ids.
			var propIDs []string
			if strings.TrimSpace(flagProperty) != "" {
				for _, p := range strings.Split(flagProperty, ",") {
					if p = strings.TrimSpace(p); p != "" {
						propIDs = append(propIDs, p)
					}
				}
			} else {
				raw, gerr := c.Get(ctx, "/properties", map[string]string{})
				if gerr != nil {
					return fmt.Errorf("listing properties: %w", gerr)
				}
				var pr struct {
					Properties []struct {
						ID any `json:"id"`
					} `json:"properties"`
				}
				_ = json.Unmarshal(novUnwrapData(raw), &pr)
				for _, p := range pr.Properties {
					if id := idString(p.ID); id != "" {
						propIDs = append(propIDs, id)
					}
				}
			}
			if len(propIDs) > maxProps {
				propIDs = propIDs[:maxProps]
			}
			if len(propIDs) == 0 {
				return novEmit(cmd, flags, map[string]any{"oversells": []any{}, "note": "no properties to scan"})
			}

			type osRow struct {
				PropertyID  string  `json:"property_id"`
				Date        string  `json:"date"`
				ChannelType string  `json:"channel_type"`
				Inventory   float64 `json:"channel_inventory"`
			}
			rows := make([]osRow, 0)
			scannedProps := 0
			fetchFailures := 0

			for _, pid := range propIDs {
				scannedProps++
				// Master availability for this property.
				availRaw, aerr := c.Get(ctx, "/availabilities", map[string]string{
					"property_ids": pid,
					"start_date":   start,
					"end_date":     end,
				})
				if aerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: availabilities for property %s failed: %v\n", pid, aerr)
					continue
				}
				blocked := blockedDates(availRaw)
				if len(blocked) == 0 {
					continue
				}
				// Per-channel listings + calendar inventory.
				listings, lerr := propertyListings(ctx, c, pid)
				if lerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: pricing ratios for property %s failed: %v\n", pid, lerr)
					fetchFailures++
					continue
				}
				if len(listings) == 0 {
					continue
				}
				calRows, cerr := listingInventory(ctx, c, start, end, listings)
				if cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: listing calendar for property %s failed: %v\n", pid, cerr)
					fetchFailures++
					continue
				}
				for _, cr := range calRows {
					if cr.inventory > 0 && blocked[cr.date] {
						rows = append(rows, osRow{PropertyID: pid, Date: cr.date, ChannelType: cr.channel, Inventory: cr.inventory})
					}
				}
			}

			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].PropertyID != rows[j].PropertyID {
					return rows[i].PropertyID < rows[j].PropertyID
				}
				return rows[i].Date < rows[j].Date
			})

			view := struct {
				Range         string  `json:"range"`
				ScannedProps  int     `json:"scanned_properties"`
				OversellDays  int     `json:"oversell_dates"`
				Oversells     []osRow `json:"oversells"`
				FetchFailures int     `json:"fetch_failures"`
			}{
				Range:         start + ".." + end,
				ScannedProps:  scannedProps,
				OversellDays:  len(rows),
				Oversells:     rows,
				FetchFailures: fetchFailures,
			}
			return novEmit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagDays, "days", "30", "Number of days forward to check")
	cmd.Flags().StringVar(&flagProperty, "property", "", "Property ID(s), comma-separated; omit to scan up to --max-properties")
	cmd.Flags().StringVar(&flagMaxProps, "max-properties", "25", "Max properties to scan when --property is omitted")
	return cmd
}

// blockedDates returns the set of dates where the master calendar is unavailable.
func blockedDates(raw json.RawMessage) map[string]bool {
	var av struct {
		Properties []struct {
			Availabilities []struct {
				Date      string `json:"date"`
				Available bool   `json:"available"`
			} `json:"availabilities"`
		} `json:"properties"`
	}
	_ = json.Unmarshal(novUnwrapData(raw), &av)
	out := map[string]bool{}
	for _, p := range av.Properties {
		for _, a := range p.Availabilities {
			if !a.Available {
				out[a.Date] = true
			}
		}
	}
	return out
}

type pListing struct {
	ChannelType string `json:"channel_type"`
	ListingID   any    `json:"listing_id"`
}

func propertyListings(ctx context.Context, c *client.Client, propID string) ([]pListing, error) {
	raw, err := c.Get(ctx, "/pricing_ratios", map[string]string{"property_id": propID})
	if err != nil {
		return nil, err
	}
	var rat struct {
		Channels []pListing `json:"channels"`
	}
	if err := json.Unmarshal(novUnwrapData(raw), &rat); err != nil {
		return nil, err
	}
	return rat.Channels, nil
}

type invCell struct {
	date      string
	channel   string
	inventory float64
}

func listingInventory(ctx context.Context, c *client.Client, start, end string, listings []pListing) ([]invCell, error) {
	body := map[string]any{"start_date": start, "end_date": end, "listings": listings}
	raw, _, err := c.PostQueryWithParams(ctx, "/listings/calendar", nil, body)
	if err != nil {
		return nil, err
	}
	var cal struct {
		Listings []struct {
			ChannelType string `json:"channel_type"`
			Calendar    []struct {
				Date      string `json:"date"`
				Inventory any    `json:"inventory"`
			} `json:"calendar"`
		} `json:"listings"`
	}
	if err := json.Unmarshal(novUnwrapData(raw), &cal); err != nil {
		return nil, err
	}
	out := make([]invCell, 0)
	for _, l := range cal.Listings {
		for _, d := range l.Calendar {
			inv, _ := novToFloat(d.Inventory)
			out = append(out, invCell{date: d.Date, channel: l.ChannelType, inventory: inv})
		}
	}
	return out, nil
}
