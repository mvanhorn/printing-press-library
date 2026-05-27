// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// stateObservation is one entry in GET /api/chargers/{id}/state.
type stateObservation struct {
	StateID       int    `json:"StateId"`
	ValueAsString string `json:"ValueAsString"`
	Timestamp     string `json:"Timestamp"`
}

type decodedObservation struct {
	StateID int    `json:"state_id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Unit    string `json:"unit,omitempty"`
}

// newStateCmd is a friendly, decoded view of GET /api/chargers/{id}/state. The
// raw API returns numeric observation IDs; this resolves them to plain-English
// names via the baked constants table.
func newStateCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var watch bool
	var interval time.Duration

	cmd := &cobra.Command{
		Use:         "state <charger-id>",
		Short:       "Show a charger's live state, decoded into plain English",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fetch a charger's current observations and decode the numeric StateIds into
readable names (operation mode, power, per-phase current/voltage, energy, etc.).

By default only the most useful readings are shown; pass --all for every
observation. Use --watch to poll on an interval until interrupted.`,
		Example: "  zaptec-pp-cli state " + exampleChargerID + "\n  zaptec-pp-cli state " + exampleChargerID + " --all --json\n  zaptec-pp-cli state " + exampleChargerID + " --watch --interval 10s",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			chargerID := args[0]

			render := func() error {
				decoded, err := fetchDecodedState(flags, chargerID, all)
				if err != nil {
					return err
				}
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printJSONFiltered(cmd.OutOrStdout(), decoded, flags)
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
				fmt.Fprintln(tw, "OBSERVATION\tVALUE")
				for _, d := range decoded {
					val := d.Value
					if d.Unit != "" {
						val = d.Value + " " + d.Unit
					}
					fmt.Fprintf(tw, "%s\t%s\n", d.Name, val)
				}
				return tw.Flush()
			}

			if !watch {
				return render()
			}
			if interval <= 0 {
				interval = 10 * time.Second
			}
			ctx := cmd.Context()
			for {
				fmt.Fprintf(cmd.OutOrStdout(), "--- %s ---\n", time.Now().Format("15:04:05"))
				if err := render(); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Show every observation, not just the common ones")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll repeatedly until interrupted")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Polling interval when --watch is set")
	return cmd
}

func fetchDecodedState(flags *rootFlags, chargerID string, all bool) ([]decodedObservation, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := replacePathParam("/api/chargers/{id}/state", "id", chargerID)
	data, err := c.Get(path, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}

	var obs []stateObservation
	if err := json.Unmarshal(data, &obs); err != nil {
		// Some deployments wrap the array; try {"Data": [...]}.
		var wrapped struct {
			Data []stateObservation `json:"Data"`
		}
		if json.Unmarshal(data, &wrapped) == nil {
			obs = wrapped.Data
		}
	}

	byID := make(map[int]stateObservation, len(obs))
	for _, o := range obs {
		byID[o.StateID] = o
	}

	out := []decodedObservation{}
	emit := func(id int, o stateObservation) {
		name := observationName(id)
		if name == "" {
			name = "State " + strconv.Itoa(id)
		}
		value := o.ValueAsString
		if id == 710 { // operation mode → decode
			if mode, err := strconv.Atoi(o.ValueAsString); err == nil {
				value = chargerOperationModeName(mode)
			}
		}
		out = append(out, decodedObservation{StateID: id, Name: name, Value: value, Unit: observationUnit(id)})
	}

	if all {
		ids := make([]int, 0, len(obs))
		for _, o := range obs {
			ids = append(ids, o.StateID)
		}
		sort.Ints(ids)
		for _, id := range ids {
			emit(id, byID[id])
		}
		return out, nil
	}

	for _, id := range observationsOfInterest {
		if o, ok := byID[id]; ok {
			emit(id, o)
		}
	}
	return out, nil
}

type liveCharger struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	OperationMode string `json:"operation_mode"`
	Online        bool   `json:"online"`
	Installation  string `json:"installation"`
}

// newLiveCmd shows a one-shot status snapshot of every charger: name, decoded
// operation mode, online state, installation. It reads the chargers list (which
// already carries OperationMode + IsOnline), so it costs one API call, not one
// per charger.
func newLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "live",
		Aliases:     []string{"fleet"},
		Short:       "Snapshot of what every charger is doing right now",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long:        "Print one row per charger with its decoded operation mode (charging / connected / finished / disconnected) and online status — the single-pane fleet view the portal app lacks.",
		Example:     "  zaptec-pp-cli live\n  zaptec-pp-cli live --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/api/chargers", map[string]string{"PageSize": "100"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var wrapped struct {
				Data []map[string]json.RawMessage `json:"Data"`
			}
			if err := json.Unmarshal(data, &wrapped); err != nil {
				return fmt.Errorf("parsing chargers: %w", err)
			}

			rows := make([]liveCharger, 0, len(wrapped.Data))
			for _, ch := range wrapped.Data {
				lc := liveCharger{}
				_ = json.Unmarshal(ch["Id"], &lc.ID)
				_ = json.Unmarshal(ch["Name"], &lc.Name)
				_ = json.Unmarshal(ch["InstallationName"], &lc.Installation)
				var online bool
				if json.Unmarshal(ch["IsOnline"], &online) == nil {
					lc.Online = online
				}
				// Zaptec's chargers-list payload uses "OperatingMode"; fall back to
				// "OperationMode" defensively in case a deployment differs.
				var mode int
				if raw, ok := ch["OperatingMode"]; ok && json.Unmarshal(raw, &mode) == nil {
					lc.OperationMode = chargerOperationModeName(mode)
				} else if raw, ok := ch["OperationMode"]; ok && json.Unmarshal(raw, &mode) == nil {
					lc.OperationMode = chargerOperationModeName(mode)
				} else {
					lc.OperationMode = "Unknown"
				}
				rows = append(rows, lc)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "CHARGER\tMODE\tONLINE\tINSTALLATION")
			for _, r := range rows {
				online := "no"
				if r.Online {
					online = "yes"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Name, r.OperationMode, online, r.Installation)
			}
			return tw.Flush()
		},
	}
	return cmd
}
