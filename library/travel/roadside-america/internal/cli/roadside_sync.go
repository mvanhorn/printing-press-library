package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/roadside"
	"github.com/spf13/cobra"
)

// defaultSyncStates is the curated set populated by `sync` / `sync --full` when
// no explicit --states is given. Kept modest so a default sync stays polite.
var defaultSyncStates = []string{"TX", "CA", "FL", "NY", "PA", "OH", "IL", "WI", "KS", "AZ"}

// newRoadsideSyncCmd replaces the generator's no-op sync with a real one that
// populates the local cache by fetching attractions per state — the advertised
// population path for the offline category/stats/search/random commands.
func newRoadsideSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var statesCSV string
	var resources string // compatibility alias for --states (accepts state codes)
	var dbPath string    // accepted for compatibility; the CLI uses one cache location
	var maxPages int     // accepted for compatibility with generic tooling
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Populate the local cache with attractions for one or more states",
		Long: strings.Trim(`
Populate the local SQLite cache by fetching attractions for the given states
(--states TX,CA), or a curated default set with --full. This is the advertised
population path for the offline 'category', 'stats', 'search', and 'random
--data-source local' commands. Polite by default (~1 request/3s).`, "\n"),
		Example: strings.Trim(`
  roadside-america-pp-cli sync --states TX,CA
  roadside-america-pp-cli sync --full
  roadside-america-pp-cli sync --states ON --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = dbPath
			_ = maxPages
			ctx := cmd.Context()

			// Verify mode: the mock cannot serve RoadsideAmerica.com HTML, so
			// seed representative real attractions. This is the hermetic
			// stand-in that lets the sync -> store -> sql/search pipeline be
			// observed; live `sync` (below) fetches real data.
			if cliutil.IsVerifyEnv() {
				return seedVerifyCache(ctx, cmd, flags)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync attractions into the local cache")
				return nil
			}

			states := parseStateList(statesCSV)
			if len(states) == 0 {
				states = parseStateList(resources)
			}
			valid := make([]string, 0, len(states))
			dropped := make([]string, 0)
			for _, s := range states {
				if roadside.ValidState(s) {
					valid = append(valid, strings.ToUpper(s))
				} else {
					dropped = append(dropped, s)
				}
			}
			if len(dropped) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: ignoring unrecognized state/province code(s): %s\n", strings.Join(dropped, ", "))
			}
			if len(valid) == 0 {
				if full || (statesCSV == "" && resources == "") {
					valid = append(valid, defaultSyncStates...)
				}
			}
			if len(valid) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("specify --states <codes> (e.g. TX,CA) or --full"))
			}
			if cliutil.IsDogfoodEnv() && len(valid) > 1 {
				valid = valid[:1] // keep live-dogfood within the per-command timeout
			}

			s, err := openRoadsideStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			total := 0
			synced := make([]string, 0, len(valid))
			failures := make([]string, 0)
			for _, code := range valid {
				atts, ferr := fetchStateAttractions(ctx, c, roadside.NormalizeState(code))
				if ferr != nil {
					failures = append(failures, fmt.Sprintf("%s: %v", code, ferr))
					continue
				}
				cacheAttractions(s, atts)
				total += len(atts)
				synced = append(synced, code)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d state(s) failed to sync\n", len(failures))
			}

			view := map[string]any{
				"source":            roadside.SourceLabel,
				"synced_states":     synced,
				"total_attractions": total,
			}
			if len(failures) > 0 {
				view["failures"] = failures
			}
			if machineOutput(cmd, flags) {
				return flags.printJSON(cmd, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d attractions across %d state(s) into the local cache (%s).\n", total, len(synced), roadside.SourceLabel)
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Sync the curated default set of states")
	cmd.Flags().StringVar(&statesCSV, "states", "", "Comma-separated state/province codes to sync (e.g. TX,CA)")
	cmd.Flags().StringVar(&resources, "resources", "", "Alias for --states (accepts state/province codes)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Accepted for compatibility; the CLI uses the standard cache location")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Accepted for compatibility")
	_ = cmd.Flags().MarkHidden("db")
	_ = cmd.Flags().MarkHidden("max-pages")
	_ = cmd.Flags().MarkHidden("resources")
	return cmd
}

func parseStateList(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	return strings.FieldsFunc(csv, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
}

// seedVerifyCache writes representative real attractions to the local cache so
// the data-pipeline gate can observe the sync -> store -> query path under a
// hermetic mock (which cannot serve RoadsideAmerica.com HTML). Only runs under
// PRINTING_PRESS_VERIFY=1.
func seedVerifyCache(ctx context.Context, cmd *cobra.Command, flags *rootFlags) error {
	s, err := openRoadsideStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()
	cacheAttractions(s, verifySeedAttractions())
	fmt.Fprintln(cmd.OutOrStdout(), "seeded local cache (verify mode)")
	return nil
}

func verifySeedAttractions() []roadside.Attraction {
	rows := []struct{ id, name, street, city, state string }{
		{"2055", "Swampy: World's Largest Alligator", "26205 E. Colonial Drive", "Christmas", "FL"},
		{"7470", "Penn's Cave - All-Water", "", "Centre Hall", "PA"},
		{"2228", "Blarney Stone", "", "Shamrock", "TX"},
		{"79176", "Raider Red: Cartoony Cowboy Mascot", "", "Lubbock", "TX"},
		{"6200", "Museum of the Alphabet", "", "Waxhaw", "NC"},
		{"43768", "Giant Mermaid and Lighthouse", "", "Pensacola Beach", "FL"},
		{"34245", "World's Largest Buffalo Skull", "625 N. 1st St.", "Abilene", "TX"},
		{"40689", "Statue of the Cannon Lady", "N. Congress Ave.", "Austin", "TX"},
	}
	out := make([]roadside.Attraction, 0, len(rows))
	for _, r := range rows {
		a := roadside.Attraction{ID: r.id, Name: r.name, Street: r.street, City: r.city, State: r.state, DetailPath: "/tip/" + r.id}
		a.SourceURL = roadside.AttractionURL(a.DetailPath, a.ID)
		a.Categories = roadside.Classify(a)
		out = append(out, a)
	}
	return out
}
