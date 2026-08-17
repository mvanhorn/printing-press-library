// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command: extension routing trace. For one extension number, list every
// ring group, queue, inbound rule, and DID that routes to it, plus whether the
// extension itself exists. Pure local joins over the synced mirror.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type traceRuleHit struct {
	Rule string `json:"rule"`
	When string `json:"when"` // office-hours | out-of-office | holidays
}

type traceResult struct {
	Extension   string         `json:"extension"`
	Exists      bool           `json:"exists"`
	Name        string         `json:"name,omitempty"`
	RingGroups  []string       `json:"ring_groups"`
	Queues      []string       `json:"queues"`
	InboundRule []traceRuleHit `json:"inbound_rules"`
	Dids        []string       `json:"dids"`
	PathCount   int            `json:"path_count"`
	Note        string         `json:"note,omitempty"`
}

// findRoutesToExtension is the pure trace core: scan the decoded config
// objects for every path that reaches ext. Exported field shapes mirror the
// 3CX OData model (Members[].Number, Agents[].Number, *Destination.Number,
// DidNumber.RoutingRule.*Destination).
func findRoutesToExtension(ext string, users, ringGroups, queues, inboundRules, didNumbers []map[string]json.RawMessage) traceResult {
	res := traceResult{
		Extension:   ext,
		RingGroups:  []string{},
		Queues:      []string{},
		InboundRule: []traceRuleHit{},
		Dids:        []string{},
	}
	for _, u := range users {
		if jsonString(u, "Number") == ext {
			res.Exists = true
			if n := firstString(u, "FirstName"); n != "" {
				res.Name = n + " " + jsonString(u, "LastName")
			}
			break
		}
	}
	for _, rg := range ringGroups {
		for _, m := range memberNumbers(rg, "Members", "Agents") {
			if m == ext {
				res.RingGroups = append(res.RingGroups, firstString(rg, "Name", "Number"))
				break
			}
		}
	}
	for _, q := range queues {
		for _, a := range memberNumbers(q, "Agents", "Members") {
			if a == ext {
				res.Queues = append(res.Queues, firstString(q, "Name", "Number"))
				break
			}
		}
	}
	whenLabels := map[string]string{
		"OfficeHoursDestination":      "office-hours",
		"OutOfOfficeHoursDestination": "out-of-office",
		"HolidaysDestination":         "holidays",
	}
	for _, r := range inboundRules {
		name := firstString(r, "RuleName", "Id")
		for key, when := range whenLabels {
			if destinationNumber(r, key) == ext {
				res.InboundRule = append(res.InboundRule, traceRuleHit{Rule: name, When: when})
			}
		}
	}
	for _, d := range didNumbers {
		// A DID routes via its nested RoutingRule (an InboundRule).
		var routed bool
		if raw, ok := d["RoutingRule"]; ok {
			var rr map[string]json.RawMessage
			if json.Unmarshal(raw, &rr) == nil {
				for key := range whenLabels {
					if destinationNumber(rr, key) == ext {
						routed = true
						break
					}
				}
			}
		}
		if routed {
			res.Dids = append(res.Dids, jsonString(d, "Number"))
		}
	}
	res.PathCount = len(res.RingGroups) + len(res.Queues) + len(res.InboundRule) + len(res.Dids)
	return res
}

func newNovelTraceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "trace <extension>",
		Short: "Show every ring group, queue, inbound rule, and DID that routes to an extension",
		Long: "For one extension number, list every ring group membership, queue agency, inbound-rule\n" +
			"destination, and DID route that reaches it, plus whether the extension exists.\n" +
			"Reads the local mirror only.\n\n" +
			"Use this command for all routing paths into one extension. For graph-wide broken\n" +
			"references use 'audit'; for free-text lookup use 'search'.",
		Example:     "  3cx-xapi-pp-cli trace 214 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would trace routing paths for the given extension")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("extension number is required, e.g. 'trace 214'"))
			}
			ext := args[0]
			// 3CX extensions/DNs are numeric. Reject obviously-invalid input with
			// a usage error rather than silently returning an empty trace.
			if !isAllDigits(ext) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("extension must be numeric, got %q", ext))
			}

			db, ok, err := openLocalMirror(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, rtUsers) {
				hintIfStale(cmd, db, rtUsers, flags.maxAge)
			}

			users, err := listObjects(db, rtUsers)
			if err != nil {
				return err
			}
			ringGroups, err := listObjects(db, rtRingGroups)
			if err != nil {
				return err
			}
			queues, err := listObjects(db, rtQueues)
			if err != nil {
				return err
			}
			inbound, err := listObjects(db, rtInboundRules)
			if err != nil {
				return err
			}
			dids, err := listObjects(db, rtDidNumbers)
			if err != nil {
				return err
			}

			hintMembersNotExpanded(cmd, ringGroups, queues)
			res := findRoutesToExtension(ext, users, ringGroups, queues, inbound, dids)
			if !res.Exists && res.PathCount == 0 {
				res.Note = fmt.Sprintf("extension %s not found in the local mirror and no routes reference it (sync first, or the extension may not exist)", ext)
			}

			if machineOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			w := cmd.OutOrStdout()
			status := red("not found")
			if res.Exists {
				status = green("exists")
			}
			fmt.Fprintf(w, "Extension %s (%s)%s\n", bold(ext), status, func() string {
				if res.Name != "" {
					return " — " + res.Name
				}
				return ""
			}())
			fmt.Fprintf(w, "  %d routing path(s)\n", res.PathCount)
			if len(res.RingGroups) > 0 {
				fmt.Fprintf(w, "  ring groups: %v\n", res.RingGroups)
			}
			if len(res.Queues) > 0 {
				fmt.Fprintf(w, "  queues: %v\n", res.Queues)
			}
			for _, h := range res.InboundRule {
				fmt.Fprintf(w, "  inbound rule: %s (%s)\n", h.Rule, h.When)
			}
			if len(res.Dids) > 0 {
				fmt.Fprintf(w, "  DIDs: %v\n", res.Dids)
			}
			if res.Note != "" {
				fmt.Fprintln(w, res.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
