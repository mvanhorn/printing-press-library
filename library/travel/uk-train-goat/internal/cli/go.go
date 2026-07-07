// uk-train-goat hand-authored: saved-commute one-shot. Transcendence feature.
//
// `uk-train-goat go morning` resolves a saved route from the local store and
// runs a board call with the saved destination filter and time-window.
// Joins local saved_routes x live OpenLDBWS data — no competing UK rail
// tool remembers your route.
package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/store"

	"github.com/spf13/cobra"
)

func newGoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "go <saved-name>",
		Short:       "Run a saved commute as a one-shot live board (transcendence)",
		Long:        "Resolves a saved route by name from the local store and runs a `board` call with the saved destination filter and time-window. Daily-commuter shortcut: one keystroke instead of remembering the CRS codes every morning.",
		Example:     "  uk-train-goat-pp-cli go morning --json --select services.std,services.platform",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
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

			raw, err := s.Get("saved_route", args[0])
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("no saved route named %q (use `saved add` first)", args[0]))
				}
				return fmt.Errorf("reading saved route %q: %w", args[0], err)
			}
			var route savedRoute
			if err := json.Unmarshal(raw, &route); err != nil {
				return fmt.Errorf("parsing saved route: %w", err)
			}

			// pp:client-call — wraps openldbws.Departures.
			board, err := c.Departures(route.From, route.To, 5, 0, route.WindowMins)
			if err != nil {
				return apiErr(err)
			}
			payload := map[string]any{
				"saved":    route,
				"services": serializeJourneys(board),
				"messages": collectMessages(board),
			}
			out, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// collectMessages extracts NRCC operator alerts from a single station board.
func collectMessages(board *openldbws.StationBoard) []string {
	if board == nil {
		return nil
	}
	return board.NRCCMessages
}
