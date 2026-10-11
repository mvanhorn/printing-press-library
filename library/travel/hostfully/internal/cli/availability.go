package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/config"
	"github.com/spf13/cobra"
)

type availDay struct {
	Date   string `json:"date"`
	Status string `json:"status"`
	Guest  string `json:"guest,omitempty"`
}

type unitAvail struct {
	Unit int        `json:"unit"`
	Days []availDay `json:"days"`
}

type availResult struct {
	Period string      `json:"period"`
	Units  []unitAvail `json:"units"`
}

type availabilityLoader func(flags *rootFlags, units []int) ([]reservation, error)

func newAvailabilityCmd(flags *rootFlags) *cobra.Command {
	return newAvailabilityCmdWithLoader(flags, loadLiveAvailabilityReservations)
}

func newAvailabilityCmdWithLoader(flags *rootFlags, load availabilityLoader) *cobra.Command {
	var unit int
	var days int

	cmd := &cobra.Command{
		Use:   "availability",
		Short: "Show availability grid across all units",
		Long: `Show availability using a fresh Hostfully read.

Availability never reads from or falls back to the local SQLite mirror. If
Hostfully cannot be reached or any requested unit cannot be checked, the
command fails instead of presenting cached or partial results as current.`,
		Example: strings.Trim(`
  hostfully-pp-cli availability
  hostfully-pp-cli availability --unit 3
  hostfully-pp-cli availability --days 14 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "availability")
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf(
					"availability requires live Hostfully data; --data-source local is not supported",
				))
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			if unit > 0 {
				if _, err := requireUnits(); err != nil {
					return err
				}
				if _, ok := unitUID(unit); !ok {
					return usageErr(fmt.Errorf("unknown unit %d", unit))
				}
			}

			targetUnits := unitNumbers()
			if unit > 0 {
				targetUnits = []int{unit}
			}
			var reservations []reservation
			var err error
			if mockMode() {
				reservations = mockHostfullyReservations(time.Now())
			} else {
				reservations, err = load(flags, targetUnits)
			}
			if err != nil {
				return fmt.Errorf("loading live Hostfully availability: %w", err)
			}

			location, err := propertyLocation()
			if err != nil {
				return err
			}
			now := time.Now().In(location)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
			result := buildAvailabilityResult(reservations, today, targetUnits, days)

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Grid view
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 1, ' ', 0)

			// Header row
			fmt.Fprintf(w, "## Hostfully — %s\n\n", result.Period)
			fmt.Fprint(w, "Unit")
			for i := 0; i < days && i < 14; i++ { // cap display at 14 cols
				d := today.Add(time.Duration(i) * 24 * time.Hour)
				fmt.Fprintf(w, "\t%s", d.Format("Mon 2"))
			}
			fmt.Fprintln(w)

			for _, ua := range result.Units {
				fmt.Fprintf(w, "#%d", ua.Unit)
				for i, d := range ua.Days {
					if i >= 14 {
						break
					}
					if d.Guest != "" {
						fmt.Fprintf(w, "\t■ %s", d.Guest)
					} else {
						fmt.Fprint(w, "\t—")
					}
				}
				fmt.Fprintln(w)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&unit, "unit", 0, "Show availability for a specific unit only")
	cmd.Flags().IntVar(&days, "days", 7, "Number of days to show (default 7)")

	return cmd
}

func loadLiveAvailabilityReservations(flags *rootFlags, units []int) ([]reservation, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	if cfg.AgencyUID() == "" {
		return nil, fmt.Errorf("HOSTFULLY_AGENCY_ID environment variable is required")
	}
	if cfg.HostfullyApiKey == "" {
		return nil, fmt.Errorf("HOSTFULLY_API_KEY environment variable is required")
	}

	graphQLCfg := *cfg
	graphQLCfg.BaseURL = hostfullyGraphQLBaseURL(cfg.BaseURL)
	graphql := client.New(&graphQLCfg, flags.timeout, liveFanoutRateLimit(flags))
	graphql.NoCache = true

	properties := make(map[int]string, len(units))
	for _, unit := range units {
		uid, ok := unitUID(unit)
		if !ok {
			return nil, fmt.Errorf("unknown unit %d", unit)
		}
		properties[unit] = uid
	}
	ctx, cancel := boundCtx(context.Background(), flags)
	defer cancel()
	return fetchWhoisReservationsAcrossUnits(bindLeadAPI(ctx, graphql), properties, liveWhoisConcurrency)
}

func buildAvailabilityResult(
	reservations []reservation,
	today time.Time,
	targetUnits []int,
	days int,
) availResult {
	endDate := today.AddDate(0, 0, days-1)
	occ := make(map[int]map[string]string, len(targetUnits))
	for _, unit := range targetUnits {
		occ[unit] = make(map[string]string)
	}
	for _, r := range reservations {
		if !operationalReservation(r) {
			continue
		}
		if _, ok := occ[r.Unit]; !ok {
			continue
		}
		guest := r.GuestLast
		if guest == "" {
			guest = r.GuestFirst
		}
		checkIn := r.CheckIn.Format("2006-01-02")
		checkOut := r.CheckOut.Format("2006-01-02")
		for i := 0; i < days; i++ {
			day := today.AddDate(0, 0, i)
			date := day.Format("2006-01-02")
			if checkIn <= date && checkOut > date {
				occ[r.Unit][date] = guest
			}
		}
	}

	result := availResult{
		Period: fmt.Sprintf("%s – %s", today.Format("Jan 2"), endDate.Format("Jan 2, 2006")),
	}
	units := append([]int(nil), targetUnits...)
	sort.Ints(units)
	for _, unit := range units {
		ua := unitAvail{Unit: unit}
		for i := 0; i < days; i++ {
			day := today.AddDate(0, 0, i)
			guest := occ[unit][day.Format("2006-01-02")]
			status := "VACANT"
			if guest != "" {
				status = "OCCUPIED"
			}
			ua.Days = append(ua.Days, availDay{
				Date: day.Format("Mon Jan 2"), Status: status, Guest: guest,
			})
		}
		result.Units = append(result.Units, ua)
	}
	return result
}
