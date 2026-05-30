// Hand-authored — NOT generated. `encounters network <vesselId>`: builds the
// at-sea meeting graph for a vessel from GFW encounter events — the counterpart
// vessels it met, when, and where. Wired from root.go via newEncountersParentCmd.
//
// pp:data-source live
// Fetches encounter events from the GFW events endpoint and assembles the graph
// in-process; no local cache table for the graph itself.
package cli

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
)

func newEncountersParentCmd(flags *rootFlags) *cobra.Command {
	cmd := newNovelEncountersCmd(flags)
	cmd.Short = "At-sea encounter analysis for a vessel."
	cmd.Long = "Build the encounter (at-sea meeting) graph for a vessel from GFW encounter events — useful for surfacing transshipment partners."
	cmd.ResetCommands()
	cmd.AddCommand(newEncountersNetworkCmd(flags))
	return cmd
}

type encounterEdge struct {
	EventID         string          `json:"event_id,omitempty"`
	Start           string          `json:"start,omitempty"`
	CounterpartID   string          `json:"counterpart_id,omitempty"`
	CounterpartName string          `json:"counterpart_name,omitempty"`
	CounterpartFlag string          `json:"counterpart_flag,omitempty"`
	Encounter       json.RawMessage `json:"encounter,omitempty"`
}

type encounterNetwork struct {
	VesselID     string          `json:"vessel_id"`
	EncounterN   int             `json:"encounter_count"`
	PartnerCount int             `json:"distinct_partners"`
	Encounters   []encounterEdge `json:"encounters"`
}

// encounterCounterpart defensively pulls the counterpart vessel from an
// encounter event's nested `encounter` object (GFW nests it under one of a few
// keys depending on the dataset version).
func encounterCounterpart(event map[string]any) (id, name, flag string, encRaw json.RawMessage) {
	enc, _ := event["encounter"].(map[string]any)
	if enc == nil {
		return "", "", "", nil
	}
	encRaw, _ = json.Marshal(enc)
	for _, key := range []string{"vessel", "encounteredVessel", "mainVessel", "encounteredVesselInfo"} {
		if v, ok := enc[key].(map[string]any); ok {
			name = mapStr(v, "name")
			if name == "" {
				name = mapStr(v, "shipname")
			}
			return mapStr(v, "id"), name, mapStr(v, "flag"), encRaw
		}
	}
	return "", "", "", encRaw
}

func newEncountersNetworkCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "network <vesselId>",
		Short:       "Map a vessel's at-sea encounters (counterpart vessels, when, where).",
		Long:        "Fetches a vessel's encounter events and extracts the counterpart vessel for each — the at-sea meeting graph the API never returns directly. Use to surface transshipment partners. For raw events use 'events list --datasets-0 public-global-encounters-events:latest'.",
		Example:     "  gfw-pp-cli encounters network 8c7304226-6c71-edbe-0b63-c246734b3c01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := fetchEvents(cmd.Context(), c, id, []string{dsEncounters}, limit, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			entries := entriesOf(raw)
			edges := make([]encounterEdge, 0, len(entries))
			partners := map[string]bool{}
			for _, e := range entries {
				var ev map[string]any
				if json.Unmarshal(e, &ev) != nil {
					continue
				}
				cid, cname, cflag, encRaw := encounterCounterpart(ev)
				edge := encounterEdge{
					EventID:         mapStr(ev, "id"),
					Start:           mapStr(ev, "start"),
					CounterpartID:   cid,
					CounterpartName: cname,
					CounterpartFlag: cflag,
					Encounter:       encRaw,
				}
				edges = append(edges, edge)
				key := cid
				if key == "" {
					key = strings.ToLower(cname)
				}
				if key != "" {
					partners[key] = true
				}
			}
			net := encounterNetwork{VesselID: id, EncounterN: len(edges), PartnerCount: len(partners), Encounters: edges}
			return printJSONFiltered(cmd.OutOrStdout(), net, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max encounter events to analyze")
	return cmd
}
