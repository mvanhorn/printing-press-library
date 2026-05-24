// tesla supercharger watch — poll Supercharger stall availability via the
// vehicle's nearby_charging_sites endpoint. Hand-coded out-of-tree.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
)

func newSuperchargerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "supercharger",
		Short: "Tesla Supercharger queue intelligence",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSuperchargerWatchCmd(flags))
	return cmd
}

func newSuperchargerWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		vehicleID  string
		freeStalls int
		watchMode  bool
		interval   time.Duration
		maxIters   int
	)
	cmd := &cobra.Command{
		Use:   "watch [site_id_or_name]",
		Short: "Poll Supercharger stall availability; --watch emits JSON-lines transitions",
		Long: `Calls /api/1/vehicles/{vehicle_id}/nearby_charging_sites and filters by site.
Site can be an integer site_id or a name substring matched against the cached
nearby_charging_sites response.`,
		Example:     "  tesla-pp-cli supercharger watch 1000 --vehicle 5YJ3E1EA6XXXXXXXX --json\n  tesla-pp-cli supercharger watch Issaquah --vehicle 5YJ3E1EA6XXXXXXXX --watch --free-stalls 2 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "args": args}, flags)
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if vehicleID == "" {
				return fmt.Errorf("--vehicle is required (vehicle_id from /api/1/products)")
			}

			// Live-dogfood: force single iteration regardless of --watch
			if cliutil.IsDogfoodEnv() {
				maxIters = 1
				watchMode = false
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			site := args[0]

			if !watchMode {
				snap, err := pollSuperchargerSite(cmd.Context(), c, vehicleID, site)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), snap, flags)
			}

			// Watch mode: emit JSON-lines on transitions
			if interval < 30*time.Second {
				interval = 30 * time.Second
			}
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			var lastAvail int = -1
			iter := 0
			ctx := cmd.Context()
			for {
				snap, err := pollSuperchargerSite(ctx, c, vehicleID, site)
				if err != nil {
					return err
				}
				if lastAvail == -1 || snap.AvailableStalls != lastAvail ||
					(freeStalls > 0 && snap.AvailableStalls >= freeStalls && lastAvail < freeStalls) {
					if err := emitJSONLine(cmd, snap); err != nil {
						return err
					}
					lastAvail = snap.AvailableStalls
				}
				iter++
				if maxIters > 0 && iter >= maxIters {
					return nil
				}
				select {
				case <-sigCh:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().StringVar(&vehicleID, "vehicle", "", "Vehicle id from /api/1/products (required for nearby_charging_sites)")
	cmd.Flags().IntVar(&freeStalls, "free-stalls", 0, "Only emit transitions that cross this free-stall threshold")
	cmd.Flags().BoolVar(&watchMode, "watch", false, "Poll continuously, emit JSON-lines on availability transitions")
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "Poll interval (min 30s)")
	cmd.Flags().IntVar(&maxIters, "max-iterations", 0, "Cap iterations in watch mode (0 = unlimited; 1 implied under PRINTING_PRESS_DOGFOOD)")
	return cmd
}

type superchargerSnapshot struct {
	SiteID          int     `json:"site_id"`
	Name            string  `json:"name"`
	Address         string  `json:"address,omitempty"`
	DistanceMi      float64 `json:"distance_mi,omitempty"`
	AvailableStalls int     `json:"available_stalls"`
	TotalStalls     int     `json:"total_stalls"`
	LastUpdated     string  `json:"last_updated"`
	Match           string  `json:"match,omitempty"`
}

func pollSuperchargerSite(ctx context.Context, c *client.Client, vehicleID, site string) (*superchargerSnapshot, error) {
	path := strings.ReplaceAll("/api/1/vehicles/{vehicle_id}/nearby_charging_sites", "{vehicle_id}", vehicleID)
	raw, err := c.Get(path, nil)
	if err != nil {
		return nil, fmt.Errorf("nearby_charging_sites: %w", err)
	}
	var envelope struct {
		Response struct {
			Superchargers []struct {
				LocationID      int     `json:"location_id"`
				Name            string  `json:"name"`
				Type            string  `json:"type"`
				DistanceMi      float64 `json:"distance_miles"`
				AvailableStalls int     `json:"available_stalls"`
				TotalStalls     int     `json:"total_stalls"`
				SiteClosed      bool    `json:"site_closed"`
			} `json:"superchargers"`
			Timestamp int64 `json:"timestamp"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse nearby_charging_sites: %w", err)
	}
	siteID, idOk := parseInt(site)
	var picked *superchargerSnapshot
	for _, sc := range envelope.Response.Superchargers {
		match := ""
		if idOk && sc.LocationID == siteID {
			match = "id"
		} else if !idOk && strings.Contains(strings.ToLower(sc.Name), strings.ToLower(site)) {
			match = "name"
		}
		if match == "" {
			continue
		}
		picked = &superchargerSnapshot{
			SiteID:          sc.LocationID,
			Name:            sc.Name,
			DistanceMi:      sc.DistanceMi,
			AvailableStalls: sc.AvailableStalls,
			TotalStalls:     sc.TotalStalls,
			LastUpdated:     time.Unix(envelope.Response.Timestamp/1000, 0).UTC().Format(time.RFC3339),
			Match:           match,
		}
		break
	}
	if picked == nil {
		return nil, fmt.Errorf("no Supercharger matched %q (returned %d nearby sites)", site, len(envelope.Response.Superchargers))
	}
	return picked, nil
}

func emitJSONLine(cmd *cobra.Command, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	if _, err := w.Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

func parseInt(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return n, true
}
