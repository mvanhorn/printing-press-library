// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) location listing for WeWork-owned buildings: an
// agent-native way to resolve a fuzzy place ("barton springs") to a concrete
// bookable location plus every identifier the booking flow needs. Chains
// get-locations-by-geo (buildings) -> inventory-details (per-building desk
// inventory) so each result carries LocationID + WeWorkSpaceID + SpaceID +
// LocationType + price + availability, headless.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/client"
	"github.com/spf13/cobra"
)

// resolveCityGeo resolves a city query (e.g. "Austin, TX") to its market
// coordinates via get-affiliate-cities, returning the canonical matched name.
func resolveCityGeo(ctx context.Context, c *client.Client, cityQuery string) (lat, lng float64, matchedName string, err error) {
	raw, err := c.Get(ctx, "/wework-yardi/location/get-affiliate-cities", nil)
	if err != nil {
		return 0, 0, "", classifyAPIError(err, nil)
	}
	var cities []deskCity
	if err := json.Unmarshal(raw, &cities); err != nil {
		return 0, 0, "", fmt.Errorf("parsing cities: %w", err)
	}
	m, ok := matchCity(cities, cityQuery)
	if !ok {
		return 0, 0, "", usageErr(fmt.Errorf("no bookable WeWork city matched %q — try 'wework-pp-cli cities'", cityQuery))
	}
	if m.Marketgeo.Latitude == 0 && m.Marketgeo.Longitude == 0 {
		return 0, 0, "", fmt.Errorf("city %q has no coordinates", m.Name)
	}
	return m.Marketgeo.Latitude, m.Marketgeo.Longitude, m.Name, nil
}

// cityNameOnly strips a ", ST" state/region suffix (get-locations-by-geo wants
// the bare city name).
func cityNameOnly(name string) string {
	if i := strings.Index(name, ","); i >= 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}

func todayLocalDate() string {
	return time.Now().Format("2006-01-02")
}

// weworkBuilding is a WeWork-owned location enriched with its desk-inventory ids.
type weworkBuilding struct {
	LocationID    string  `json:"locationId"` // building uuid (get-locations-by-geo uuid == inventory-details propertyId)
	Name          string  `json:"name"`
	Line1         string  `json:"address"`
	City          string  `json:"city"`
	State         string  `json:"state"`
	Zip           string  `json:"zip"`
	Country       string  `json:"country"`
	TimeZone      string  `json:"timeZone"`     // IANA, e.g. America/Chicago
	LocationType  int     `json:"locationType"` // accountType
	Available     int     `json:"available"`
	WeWorkSpaceID string  `json:"weWorkSpaceId"` // inventory-details inventoryUuid — booking input
	SpaceID       string  `json:"spaceId"`       // inventory-details kubeSpaceId — booking input
	SpaceTypeID   int     `json:"spaceTypeId"`
	PriceAmount   float64 `json:"price"`
	Currency      string  `json:"currency"`
	Credits       float64 `json:"credits"`
	Bookable      bool    `json:"bookable"` // true when a desk inventory (WeWorkSpaceID) was resolved
}

type geoLocationsResp struct {
	LocationsByGeo []struct {
		UUID    string `json:"uuid"`
		Name    string `json:"name"`
		Address struct {
			Line1   string `json:"line1"`
			City    string `json:"city"`
			State   string `json:"state"`
			Zip     string `json:"zip"`
			Country string `json:"country"`
		} `json:"address"`
		TimeZone               string `json:"timeZone"`
		AccountType            int    `json:"accountType"`
		SpaceAvailabilityCount int    `json:"spaceAvailabilityCount"`
	} `json:"locationsByGeo"`
}

// invDetailsResp is the common-booking/inventory-details response for a
// WeWork-owned building. It carries every id the booking flow needs, so it —
// not the affiliate /spaces/get-spaces endpoint — is how owned inventory is
// resolved. (get-spaces keys on numeric affiliate ids and 500s when handed an
// owned building's uuid.)
type invDetailsResp struct {
	KubeSpaceID   json.Number `json:"kubeSpaceId"`   // SpaceID
	InventoryUUID string      `json:"inventoryUuid"` // WeWorkSpaceID
	Price         struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"price"`
	Inventory struct {
		AvailableSeats int `json:"availableSeats"`
		Capacity       int `json:"capacity"`
		SpaceType      int `json:"spaceType"`
	} `json:"inventory"`
}

// tzOffset returns the UTC offset (e.g. "-05:00") for a date in an IANA zone,
// falling back to "+00:00" for an unknown/blank zone.
func tzOffset(date, tz string) string {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	t, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return "+00:00"
	}
	_, off := t.Zone()
	sign := "+"
	if off < 0 {
		sign, off = "-", -off
	}
	return fmt.Sprintf("%s%02d:%02d", sign, off/3600, (off%3600)/60)
}

// fetchInventoryDetails resolves a single owned building's desk inventory
// (WeWorkSpaceID, SpaceID, price, availability) via inventory-details.
func fetchInventoryDetails(ctx context.Context, c *client.Client, b *weworkBuilding, date string) error {
	start, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("bad date %q: %w", date, err)
	}
	params := map[string]string{
		"propertyType": formatCLIParamValue(b.LocationType), "propertyId": b.LocationID, "spaceType": "0",
		"startDate": start.Format("01/02/2006"), "endDate": "", "duration": "0", "roomTypeFilter": "",
		"locationOffset": tzOffset(date, b.TimeZone), "capacity": "0", "limit": "0", "offset": "0", "floorId": "0",
		"spaceId": "", "useInventoryUuid": "false", "platFormType": "1", "applicationType": "WorkplaceOne",
	}
	raw, err := c.Get(ctx, "/common-booking/inventory-details", params)
	if err != nil {
		return err
	}
	var inv invDetailsResp
	if err := json.Unmarshal(raw, &inv); err != nil {
		return fmt.Errorf("parsing inventory-details: %w", err)
	}
	b.WeWorkSpaceID = inv.InventoryUUID
	b.SpaceID = inv.KubeSpaceID.String()
	b.SpaceTypeID = inv.Inventory.SpaceType
	if inv.Price.Amount > 0 {
		b.PriceAmount = inv.Price.Amount
	}
	if inv.Price.Currency != "" {
		b.Currency = inv.Price.Currency
	}
	if inv.Inventory.AvailableSeats > 0 {
		b.Available = inv.Inventory.AvailableSeats
	}
	b.Bookable = inv.InventoryUUID != "" && b.SpaceID != "" && b.SpaceID != "0"
	return nil
}

// resolveWeworkBuildings lists WeWork-owned buildings in a bounding box around
// (lat,lng) and enriches each with its desk inventory (WeWorkSpaceID, SpaceID,
// price, availability) so callers have everything needed to book — all headless.
func resolveWeworkBuildings(ctx context.Context, c *client.Client, lat, lng float64, city, date string) ([]weworkBuilding, error) {
	const dLat, dLng = 0.18, 0.22
	bounds := map[string]string{
		"boundnwLat":    formatCLIParamValue(lat + dLat),
		"boundnwLng":    formatCLIParamValue(lng - dLng),
		"boundseLat":    formatCLIParamValue(lat - dLat),
		"boundseLng":    formatCLIParamValue(lng + dLng),
		"userLatitude":  formatCLIParamValue(lat),
		"userLongitude": formatCLIParamValue(lng),
	}
	// Step 1: WeWork-owned buildings near the city.
	geoParams := map[string]string{"city": city, "isAuthenticated": "true", "isOnDemandUser": "true"}
	if c.Config != nil && c.Config.WeworkUuid != "" {
		geoParams["accountUUID"] = c.Config.WeworkUuid
	}
	for k, v := range bounds {
		geoParams[k] = v
	}
	geoRaw, err := c.Get(ctx, "/wework-yardi/ondemand/get-locations-by-geo", geoParams)
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	var geo geoLocationsResp
	if err := json.Unmarshal(geoRaw, &geo); err != nil {
		return nil, fmt.Errorf("parsing locations: %w", err)
	}
	buildings := make([]weworkBuilding, 0, len(geo.LocationsByGeo))
	for _, l := range geo.LocationsByGeo {
		buildings = append(buildings, weworkBuilding{
			LocationID: l.UUID, Name: l.Name, Line1: l.Address.Line1, City: l.Address.City,
			State: l.Address.State, Zip: l.Address.Zip, Country: l.Address.Country,
			TimeZone: l.TimeZone, LocationType: l.AccountType, Available: l.SpaceAvailabilityCount,
		})
	}
	// Step 2: enrich each building's desk inventory via inventory-details,
	// concurrently (bounded) so a city of N buildings costs ~1 round-trip.
	const workers = 6
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := range buildings {
		if buildings[i].LocationID == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(b *weworkBuilding) {
			defer wg.Done()
			defer func() { <-sem }()
			// Best-effort: a building that fails enrichment stays listed
			// (just not marked bookable).
			_ = fetchInventoryDetails(ctx, c, b, date)
		}(&buildings[i])
	}
	wg.Wait()
	return buildings, nil
}

func newLocationsCmd(flags *rootFlags) *cobra.Command {
	var flagCity, flagDate, flagFilter string
	var flagAvailableOnly, flagBookableOnly bool
	cmd := &cobra.Command{
		Use:   "locations",
		Short: "List WeWork-owned locations in a city with their booking identifiers",
		Long: "Lists WeWork-owned buildings in a city, each with the identifiers the booking flow\n" +
			"needs (locationId, weWorkSpaceId, locationType), plus price and availability. This is\n" +
			"the agent-native way to resolve a place name to a concrete, bookable location: list,\n" +
			"pick the match, then `book --location-id <locationId>`.",
		Example: strings.Trim(`
  wework-pp-cli locations --city "Austin, TX" --json
  wework-pp-cli locations --city "Austin, TX" --filter "barton" --bookable-only
  wework-pp-cli locations --city "New York, NY" --date 2026-08-18 --available-only`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateWeworkLiveDataSource(flags); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagCity) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--city is required (e.g. --city \"Austin, TX\")"))
			}
			date, err := normalizeWeworkDate(flagDate)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			lat, lng, matchedCity, err := resolveCityGeo(ctx, c, flagCity)
			if err != nil {
				return err
			}
			buildings, err := resolveWeworkBuildings(ctx, c, lat, lng, cityNameOnly(matchedCity), date)
			if err != nil {
				return err
			}
			// Filters.
			out := buildings[:0:0]
			needle := strings.ToLower(flagFilter)
			for _, b := range buildings {
				if flagFilter != "" && !strings.Contains(strings.ToLower(b.Name+" "+b.Line1+" "+b.City), needle) {
					continue
				}
				if flagAvailableOnly && b.Available <= 0 {
					continue
				}
				if flagBookableOnly && !b.Bookable {
					continue
				}
				out = append(out, b)
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })

			if flags.csv || flags.plain || flags.quiet {
				return printWeworkLiveJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printWeworkLiveJSONFiltered(cmd.OutOrStdout(), map[string]any{"city": matchedCity, "date": date, "count": len(out), "locations": out}, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No WeWork-owned locations matched in %s.\n", matchedCity)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-34s  %-24s  %6s  %5s  %s\n", "NAME", "LOCATION ID", "PRICE", "AVAIL", "BOOKABLE")
			for _, b := range out {
				name := b.Name
				if len(name) > 34 {
					name = name[:33] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-34s  %-24s  %6.0f  %5d  %v\n", name, b.LocationID, b.PriceAmount, b.Available, b.Bookable)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City to list locations in, e.g. \"Austin, TX\" (required)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Availability date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&flagFilter, "filter", "", "Case-insensitive substring filter on name/address")
	cmd.Flags().BoolVar(&flagAvailableOnly, "available-only", false, "Only locations with open desks")
	cmd.Flags().BoolVar(&flagBookableOnly, "bookable-only", false, "Only locations with a resolved bookable desk")
	return cmd
}
