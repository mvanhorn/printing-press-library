// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/vehicle-safety/internal/store"
	"github.com/spf13/cobra"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var flagGarage string

	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Report newly observed recall campaigns and remedy changes for saved vehicles.",
		Example:     "  vehicle-safety-pp-cli watch --garage examples/vehicles.csv --agent",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if strings.TrimSpace(flagGarage) == "" {
				return errors.New("--garage is required")
			}
			vehicles, err := readGarage(flagGarage)
			if err != nil {
				return err
			}
			if len(vehicles) > 50 {
				return fmt.Errorf("garage contains %d vehicles; the safety cap is 50 per run", len(vehicles))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, defaultDBPath("vehicle-safety-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			garageKey, err := filepath.Abs(flagGarage)
			if err != nil {
				return err
			}
			priorSnapshots := garageSnapshots{}
			raw, getErr := db.Get("vehicle-recall-garage", garageKey)
			if getErr != nil && !errors.Is(getErr, sql.ErrNoRows) {
				return getErr
			}
			if getErr == nil {
				if err := json.Unmarshal(raw, &priorSnapshots); err != nil {
					legacy := map[string]map[string]string{}
					if legacyErr := json.Unmarshal(raw, &legacy); legacyErr != nil {
						return fmt.Errorf("decode prior garage snapshot: %w", err)
					}
					for vehicleKey, campaigns := range legacy {
						priorSnapshots[vehicleKey] = map[string]campaignSnapshot{}
						for campaign, remedy := range campaigns {
							priorSnapshots[vehicleKey][campaign] = campaignSnapshot{Remedy: remedy, Active: true}
						}
					}
				}
			}
			nextSnapshots := garageSnapshots{}
			results := make([]map[string]any, 0, len(vehicles))
			for _, vehicle := range vehicles {
				response, fetchErr := nhtsaGet(ctx, flags, nhtsaBaseURL, "/recalls/recallsByVehicle", vehicleParams(vehicle))
				if fetchErr != nil {
					return fetchErr
				}
				current := map[string]string{}
				unidentified := 0
				for _, item := range response.Results {
					campaign := strings.TrimSpace(stringValue(item, "NHTSACampaignNumber"))
					if campaign == "" {
						unidentified++
						continue
					}
					current[campaign] = stringValue(item, "Remedy")
				}
				key := fmt.Sprintf("%d|%s|%s", vehicle.Year, strings.ToUpper(vehicle.Make), strings.ToUpper(vehicle.Model))
				previous, existed := priorSnapshots[key]
				next, delta := reconcileCampaignSnapshot(previous, current, existed)
				nextSnapshots[key] = next
				results = append(results, map[string]any{"vehicle": vehicle, "baseline_created": !existed, "campaign_count": len(current), "new_campaigns": delta.Added, "restored_campaigns": delta.Restored, "removed_campaigns": delta.Removed, "remedy_changes": delta.Changed, "unidentified_campaign_count": unidentified, "snapshot_advanced": delta.Advanced})
			}
			next, err := json.Marshal(nextSnapshots)
			if err != nil {
				return err
			}
			if err := db.Upsert("vehicle-recall-garage", garageKey, next); err != nil {
				return err
			}
			return emitNHTSA(cmd, flags, "mixed", map[string]any{"garage": flagGarage, "vehicles": results, "note": "A first run creates one atomic garage baseline; later runs report newly observed model-level campaigns and changed remedy text."})
		},
	}
	cmd.Flags().StringVar(&flagGarage, "garage", "", "CSV file with year, make, and model columns (maximum 50 rows)")
	return cmd
}

type campaignSnapshot struct {
	Remedy string `json:"remedy"`
	Active bool   `json:"active"`
}

type garageSnapshots map[string]map[string]campaignSnapshot

type campaignDelta struct {
	Added, Restored, Removed, Changed []string
	Advanced                          bool
}

func reconcileCampaignSnapshot(previous map[string]campaignSnapshot, current map[string]string, hadPrior bool) (map[string]campaignSnapshot, campaignDelta) {
	delta := campaignDelta{Advanced: true}
	activePrevious := 0
	for _, prior := range previous {
		if prior.Active {
			activePrevious++
		}
	}
	if hadPrior && activePrevious > 0 && len(current) == 0 {
		delta.Advanced = false
		return previous, delta
	}

	next := make(map[string]campaignSnapshot, len(previous)+len(current))
	for campaign, remedy := range current {
		old, existed := previous[campaign]
		switch {
		case !existed:
			delta.Added = append(delta.Added, campaign)
		case !old.Active:
			delta.Restored = append(delta.Restored, campaign)
		case old.Remedy != remedy:
			delta.Changed = append(delta.Changed, campaign)
		}
		next[campaign] = campaignSnapshot{Remedy: remedy, Active: true}
	}
	for campaign, old := range previous {
		if _, present := current[campaign]; present {
			continue
		}
		if old.Active {
			delta.Removed = append(delta.Removed, campaign)
		}
		old.Active = false
		next[campaign] = old
	}
	for _, values := range []*[]string{&delta.Added, &delta.Restored, &delta.Removed, &delta.Changed} {
		sort.Strings(*values)
	}
	return next, delta
}

func readGarage(path string) ([]vehicleQuery, error) {
	// #nosec G304 -- path is an explicit operator-supplied CLI input.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	indices := map[string]int{}
	for i, name := range header {
		indices[strings.ToLower(strings.TrimSpace(name))] = i
	}
	for _, required := range []string{"year", "make", "model"} {
		if _, ok := indices[required]; !ok {
			return nil, fmt.Errorf("garage CSV is missing %q column", required)
		}
	}
	var vehicles []vehicleQuery
	for rowNumber := 2; ; rowNumber++ {
		row, readErr := r.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("garage row %d: %w", rowNumber, readErr)
		}
		year, parseErr := strconv.Atoi(strings.TrimSpace(row[indices["year"]]))
		if parseErr != nil {
			return nil, fmt.Errorf("garage row %d: invalid year", rowNumber)
		}
		vehicle, validateErr := validateVehicle(year, row[indices["make"]], row[indices["model"]])
		if validateErr != nil {
			return nil, fmt.Errorf("garage row %d: %w", rowNumber, validateErr)
		}
		vehicles = append(vehicles, vehicle)
	}
	return vehicles, nil
}
