// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/config"
)

// compactWorkout drops verbose fields when --compact is set. The kept fields
// are the ones actually consumed by agents looking at workout history; HR
// and calories are situational, output_kj + duration + title carry the signal.
// id + ride_id are kept because they're chain inputs — `workouts list
// --compact | jq` straight into `workouts show` / `ride show` is the
// canonical compositional recipe; without those keys the projection is
// terminal-only data, not pipeable.
type compactWorkout struct {
	ID              string  `json:"id"`
	RideID          string  `json:"ride_id,omitempty"`
	WorkoutDate     string  `json:"workout_date"`
	Title           string  `json:"title"`
	DurationSeconds int     `json:"duration_seconds"`
	TotalOutputKJ   float64 `json:"total_output_kj"`
}

func newWorkoutsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workouts",
		Short: "Workout history (list, show)",
	}
	cmd.AddCommand(newWorkoutsListCmd(flags))
	cmd.AddCommand(newWorkoutsShowCmd(flags))
	return cmd
}

func newWorkoutsShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <workout-id>",
		Short: "Show a single workout by id",
		Long: `Fetches one workout from /api/workout/{id} with the same shape as
` + "`workouts list`" + ` entries. Useful for piping into ` + "`jq`" + ` after grabbing an id
from list output.`,
		Example: `  peloton-pp-cli workouts show abc123
  peloton-pp-cli workouts show abc123 --compact | jq .total_output_kj`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return Errf(CodeAPI, "loading config: %w", err)
			}
			if cfg.Token == "" {
				return Errf(CodeAuth, "no token saved — run `peloton-pp-cli auth login` first")
			}
			c := client.New(cfg.Token)
			w, err := c.GetWorkout(args[0])
			if err != nil {
				return classify(err)
			}
			return emitOne(cmd, flags, w)
		},
	}
	return cmd
}

func newWorkoutsListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent workouts (newest first)",
		Long:  `Pulls the authenticated user's workout history from api.onepeloton.com. Defaults to 50; cap with --limit.`,
		Example: `  peloton-pp-cli workouts list
  peloton-pp-cli workouts list --limit 10
  peloton-pp-cli workouts list --compact | jq '.[] | .title'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return Errf(CodeAPI, "loading config: %w", err)
			}
			if cfg.Token == "" {
				return Errf(CodeAuth, "no token saved — run `peloton-pp-cli auth login` first")
			}

			c := client.New(cfg.Token)

			// Migrate any old cached UserID that still carries the Auth0 "auth0|" prefix —
			// Peloton's /api/user/{id}/workouts wants the bare ID. Re-fetch via /api/me.
			if cfg.UserID == "" || strings.Contains(cfg.UserID, "|") {
				id, username, err := c.Me()
				if err != nil {
					return classify(err)
				}
				cfg.UserID = id
				cfg.Username = username
				_ = cfg.Save()
			}

			workouts, err := c.ListWorkouts(cfg.UserID, limit, nil)
			if err != nil {
				return classify(err)
			}

			return emit(cmd, flags, workouts)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max workouts to return")
	return cmd
}

// classify maps client error types to typed CLI exit codes so callers can
// branch on $? without parsing strings.
func classify(err error) error {
	var ae *client.AuthError
	if errors.As(err, &ae) {
		return Errf(CodeAuth, "%s", err.Error())
	}
	var re *client.RateLimitError
	if errors.As(err, &re) {
		return Errf(CodeRateLimited, "%s", err.Error())
	}
	var ne *client.NotFoundError
	if errors.As(err, &ne) {
		return Errf(CodeNotFound, "%s", err.Error())
	}
	return Errf(CodeAPI, "%s", err.Error())
}

// emit handles the JSON-when-piped + --compact + pretty-fallback contract.
// Mirrors the convention from the lifecoach scripts/_agent_cli.py helpers
// the contributor built this CLI from.
func emit(cmd *cobra.Command, flags *rootFlags, workouts []client.Workout) error {
	wantJSON := flags.asJSON || flags.compact || !isStdoutTTY()

	if flags.compact {
		out := make([]compactWorkout, len(workouts))
		for i, w := range workouts {
			out[i] = compactWorkout{
				ID:              w.ID,
				RideID:          w.RideID,
				WorkoutDate:     w.WorkoutDate,
				Title:           w.Title,
				DurationSeconds: w.DurationSeconds,
				TotalOutputKJ:   w.TotalOutputKJ,
			}
		}
		return writeJSON(cmd, out, true)
	}
	if wantJSON {
		return writeJSON(cmd, workouts, false)
	}

	// Pretty mode (TTY without --json/--compact)
	fmt.Fprintf(cmd.OutOrStdout(), "Workouts (%d):\n", len(workouts))
	for _, w := range workouts {
		mins := w.DurationSeconds / 60
		out := ""
		if w.TotalOutputKJ > 0 {
			out = " (" + strconv.Itoa(int(w.TotalOutputKJ)) + " kJ)"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %dmin %s%s\n", w.WorkoutDate, mins, w.Title, out)
	}
	return nil
}

// emitOne is the single-workout sibling of emit. Emits a JSON object (not a
// one-element array) when piped/--json, the compactWorkout projection under
// --compact, or a one-line pretty summary otherwise.
func emitOne(cmd *cobra.Command, flags *rootFlags, w client.Workout) error {
	wantJSON := flags.asJSON || flags.compact || !isStdoutTTY()
	if flags.compact {
		return writeJSON(cmd, compactWorkout{
			ID:              w.ID,
			RideID:          w.RideID,
			WorkoutDate:     w.WorkoutDate,
			Title:           w.Title,
			DurationSeconds: w.DurationSeconds,
			TotalOutputKJ:   w.TotalOutputKJ,
		}, true)
	}
	if wantJSON {
		return writeJSON(cmd, w, false)
	}
	mins := w.DurationSeconds / 60
	out := ""
	if w.TotalOutputKJ > 0 {
		out = " (" + strconv.Itoa(int(w.TotalOutputKJ)) + " kJ)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s %dmin %s%s\n", w.ID, w.WorkoutDate, mins, w.Title, out)
	return nil
}

func writeJSON(cmd *cobra.Command, v any, compact bool) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	if !compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

func isStdoutTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
