// uk-train-goat hand-authored: saved-route CRUD and saved-route status fan-out.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/store"

	"github.com/spf13/cobra"
)

// savedRoute is the JSON shape persisted in resource_type='saved_route'.
type savedRoute struct {
	Name       string `json:"name"`
	From       string `json:"from"`
	To         string `json:"to"`
	WindowMins int    `json:"window_mins"`
}

func newSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Manage saved commutes",
		Long:  "Add, list, remove, or check the live status of saved commutes. Saved routes power the `go <name>` shortcut and the `saved status` morning-status fan-out.",
	}
	cmd.AddCommand(newSavedAddCmd(flags))
	cmd.AddCommand(newSavedListCmd(flags))
	cmd.AddCommand(newSavedRmCmd(flags))
	cmd.AddCommand(newSavedStatusCmd(flags))
	return cmd
}

func newSavedAddCmd(flags *rootFlags) *cobra.Command {
	var (
		from   string
		to     string
		window int
	)
	cmd := &cobra.Command{
		Use:     "add <name> --from <crs> --to <crs>",
		Short:   "Save a named commute",
		Example: "  uk-train-goat-pp-cli saved add morning --from PAD --to RDG --window 30",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if from == "" || to == "" {
				return usageErr(fmt.Errorf("saved add requires --from <crs> and --to <crs>"))
			}
			route := savedRoute{
				Name:       args[0],
				From:       strings.ToUpper(from),
				To:         strings.ToUpper(to),
				WindowMins: window,
			}
			s, err := store.Open(defaultDBPath("uk-train-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			data, _ := json.Marshal(route)
			if err := s.Upsert("saved_route", route.Name, data); err != nil {
				return err
			}
			payload := map[string]any{"saved": route}
			out, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Origin CRS code")
	cmd.Flags().StringVar(&to, "to", "", "Destination CRS code")
	cmd.Flags().IntVar(&window, "window", 30, "Time-window in minutes the `go` and `saved status` commands use")
	return cmd
}

func newSavedListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved commutes",
		Example:     "  uk-train-goat-pp-cli saved list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.Open(defaultDBPath("uk-train-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.List("saved_route", 1000)
			if err != nil {
				return err
			}
			payload := map[string]any{"saved": rows}
			out, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newSavedRmCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm <name>",
		Short:   "Remove a saved commute",
		Example: "  uk-train-goat-pp-cli saved rm morning",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.Open(defaultDBPath("uk-train-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			// Inspect rows-affected on the DELETE to surface a clean
			// notFoundErr when the named saved route doesn't exist.
			// Store.Get returns (nil, nil) for missing rows, so checking
			// the err alone is not enough.
			res, err := s.DB().Exec(`DELETE FROM resources WHERE resource_type = 'saved_route' AND id = ?`, args[0])
			if err != nil {
				return err
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return notFoundErr(fmt.Errorf("no saved route named %q", args[0]))
			}
			payload := map[string]any{"removed": args[0]}
			out, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// newSavedStatusCmd is the transcendence feature — fan out across every
// saved route and return a single ranked status table.
func newSavedStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Live status across all saved commutes (transcendence)",
		Long:        "Parallel-fan-out: for each saved route, fetch the next departure via OpenLDBWS with the saved destination filter, then merge into one ranked status table. Cross-entity local query no single API call returns.",
		Example:     "  uk-train-goat-pp-cli saved status --json --select routes.name,routes.next.std",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c, err := openldbws.New(cfg.LdbwsApiToken)
			if err != nil {
				return authErr(err)
			}
			s, err := store.Open(defaultDBPath("uk-train-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.List("saved_route", 1000)
			if err != nil {
				return err
			}

			type result struct {
				Name string         `json:"name"`
				Err  string         `json:"error,omitempty"`
				Next map[string]any `json:"next,omitempty"`
			}
			results := make([]result, len(rows))
			var wg sync.WaitGroup
			for i, raw := range rows {
				var route savedRoute
				if err := json.Unmarshal(raw, &route); err != nil {
					results[i] = result{Name: "<malformed>", Err: err.Error()}
					continue
				}
				wg.Add(1)
				go func(i int, route savedRoute) {
					defer wg.Done()
					// pp:client-call — wraps openldbws.Departures.
					board, err := c.Departures(route.From, route.To, 1, 0, route.WindowMins)
					if err != nil {
						results[i] = result{Name: route.Name, Err: err.Error()}
						return
					}
					var next map[string]any
					if board != nil && len(board.TrainServices) > 0 {
						s := board.TrainServices[0]
						next = map[string]any{
							"std":      s.STD,
							"etd":      s.ETD,
							"platform": ptrOrEmpty(s.Platform),
						}
						if s.Destination != nil {
							next["destination"] = s.Destination.Name
						}
					}
					results[i] = result{Name: route.Name, Next: next}
				}(i, route)
			}
			wg.Wait()

			payload := map[string]any{"routes": results}
			out, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

var _ = fmt.Sprintf
