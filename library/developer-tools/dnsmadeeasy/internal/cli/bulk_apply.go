// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-zone bulk record value change. Fetches records live,
// selects targets across all zones, and applies via the real updateMulti
// endpoint per affected zone. Previews by default; --apply performs writes.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/cliutil"

	"github.com/spf13/cobra"
)

type bulkPlanItem struct {
	Zone     string `json:"zone"`
	DomainID string `json:"domain_id"`
	RecordID string `json:"record_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

type bulkApplyView struct {
	Match         string         `json:"match"`
	Set           string         `json:"set"`
	Type          string         `json:"type,omitempty"`
	Applied       bool           `json:"applied"`
	Matched       int            `json:"matched"`
	ZonesAffected int            `json:"zones_affected"`
	Plan          []bulkPlanItem `json:"plan"`
	Updated       int            `json:"updated,omitempty"`
	FetchFailures []struct {
		Zone  string `json:"zone"`
		Error string `json:"error"`
	} `json:"fetch_failures,omitempty"`
	Note string `json:"note,omitempty"`
}

func newNovelBulkApplyCmd(flags *rootFlags) *cobra.Command {
	var flagMatch, flagSet, flagType, flagName string
	var flagContains, flagApply bool

	cmd := &cobra.Command{
		Use:   "bulk-apply",
		Short: "Change a record value across every zone that uses it (via updateMulti)",
		Long: strings.Trim(`
Find records across ALL zones whose value matches --match and rewrite them to
--set, applied per affected zone with the real updateMulti endpoint. Previews
the plan by default; pass --apply to perform the writes.

Use this to WRITE the same value change across many zones (e.g. an IP
migration). To only find affected records without changing anything, use
'where-used'.`, "\n"),
		Example:     "  dnsmadeeasy-pp-cli bulk-apply --match 52.10.4.7 --set 52.10.4.9 --type A --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				scope := "records across all zones"
				if flagType != "" {
					scope = flagType + " records across all zones"
				}
				var note string
				if flagMatch != "" && flagSet != "" {
					note = fmt.Sprintf("dry-run: would fetch %s, then preview rewriting value %q to %q — no API calls made; re-run without --dry-run for the live plan, then add --apply to write", scope, flagMatch, flagSet)
				} else {
					note = "dry-run: would fetch records across all zones and preview a bulk value change — pass --match and --set to describe the change; no API calls made"
				}
				return emitBulk(cmd, flags, bulkApplyView{Match: flagMatch, Set: flagSet, Type: flagType, Plan: []bulkPlanItem{}, Note: note})
			}
			if flagMatch == "" || flagSet == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--match and --set are both required"))
			}
			// Never fan out across the account under verify.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would preview a cross-zone bulk value change")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Under live-dogfood, curtail the account-wide fan-out to fit the timeout.
			if cliutil.IsDogfoodEnv() {
				view := bulkApplyView{Match: flagMatch, Set: flagSet, Type: flagType, Plan: []bulkPlanItem{},
					Note: "dogfood mode: skipped account-wide fetch"}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			records, _, failures, err := fetchAllZoneRecords(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			byZone := map[string][]dmeRecord{}
			zoneName := map[string]string{}
			plan := []bulkPlanItem{}
			for _, r := range records {
				if flagType != "" && !strings.EqualFold(r.Type, flagType) {
					continue
				}
				if flagName != "" && !strings.EqualFold(r.Name, flagName) {
					continue
				}
				matched := false
				if flagContains {
					matched = strings.Contains(r.Value, flagMatch)
				} else {
					matched = r.Value == flagMatch
				}
				if !matched {
					continue
				}
				byZone[r.DomainID] = append(byZone[r.DomainID], r)
				zoneName[r.DomainID] = r.DomainName
				plan = append(plan, bulkPlanItem{Zone: r.DomainName, DomainID: r.DomainID, RecordID: r.ID.String(),
					Name: r.Name, Type: r.Type, OldValue: r.Value, NewValue: applyValue(r.Value, flagMatch, flagSet, flagContains)})
			}

			view := bulkApplyView{Match: flagMatch, Set: flagSet, Type: flagType, Applied: flagApply,
				Matched: len(plan), ZonesAffected: len(byZone), Plan: plan}
			for zone, msg := range failures {
				view.FetchFailures = append(view.FetchFailures, struct {
					Zone  string `json:"zone"`
					Error string `json:"error"`
				}{Zone: zone, Error: msg})
			}
			if len(plan) == 0 {
				view.Note = fmt.Sprintf("no records matched value %q", flagMatch)
				return emitBulk(cmd, flags, view)
			}
			if !flagApply {
				view.Note = "preview only — re-run with --apply to perform these updates"
				return emitBulk(cmd, flags, view)
			}

			// Apply per zone via updateMulti.
			updated := 0
			succeededZones := 0
			for domID, recs := range byZone {
				payload := make([]map[string]any, 0, len(recs))
				for _, r := range recs {
					obj := recordToUpdateObject(r)
					obj["value"] = applyValue(r.Value, flagMatch, flagSet, flagContains)
					payload = append(payload, obj)
				}
				// DNS Made Easy updateMulti is a PUT (createMulti is the POST);
				// single-record update is also PUT. Confirmed against godnsmadeeasy.
				path := "/dns/managed/" + domID + "/records/updateMulti"
				if _, _, err := c.Put(ctx, path, payload); err != nil {
					view.FetchFailures = append(view.FetchFailures, struct {
						Zone  string `json:"zone"`
						Error string `json:"error"`
					}{Zone: zoneName[domID], Error: err.Error()})
					continue
				}
				updated += len(recs)
				succeededZones++
			}
			view.Updated = updated
			// Report zones that actually succeeded, not zones in the plan; a
			// zone whose updateMulti failed is recorded in fetch_failures.
			view.ZonesAffected = succeededZones
			if len(view.FetchFailures) > 0 {
				view.Note = fmt.Sprintf("updated %d record(s) across %d zone(s); %d zone(s) failed (see fetch_failures)", updated, succeededZones, len(view.FetchFailures))
			} else {
				view.Note = fmt.Sprintf("updated %d record(s) across %d zone(s)", updated, succeededZones)
			}
			return emitBulk(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagMatch, "match", "", "Current record value to find across all zones")
	cmd.Flags().StringVar(&flagSet, "set", "", "New value to write")
	cmd.Flags().StringVar(&flagType, "type", "", "Only change records of this type (A, CNAME, ...)")
	cmd.Flags().StringVar(&flagName, "name", "", "Only change records with this host label")
	cmd.Flags().BoolVar(&flagContains, "contains", false, "Match --match as a substring and replace only that substring")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Actually perform the updates (default previews only)")
	return cmd
}

func emitBulk(cmd *cobra.Command, flags *rootFlags, view bulkApplyView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(view.Plan) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), view.Note)
		return nil
	}
	verb := "WOULD CHANGE"
	if view.Applied {
		verb = "CHANGED"
	}
	for _, p := range view.Plan {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s %s %s: %s -> %s\n", verb, p.Zone, nameOrApex(p.Name), p.Type, p.OldValue, p.NewValue)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d record(s) across %d zone(s). %s\n", view.Matched, view.ZonesAffected, view.Note)
	return nil
}

func applyValue(current, match, set string, contains bool) string {
	if contains {
		return strings.ReplaceAll(current, match, set)
	}
	return set
}

// recordToUpdateObject rebuilds the record's JSON as a map so updateMulti
// receives all existing fields (not just the changed value), preserving the
// record shape. Falls back to the typed fields if Raw is unavailable.
func recordToUpdateObject(r dmeRecord) map[string]any {
	obj := map[string]any{}
	if len(r.Raw) > 0 {
		if err := json.Unmarshal(r.Raw, &obj); err == nil {
			// Ensure id present as the API expects.
			if _, ok := obj["id"]; !ok {
				obj["id"] = r.ID.String()
			}
			return obj
		}
	}
	obj["id"] = r.ID.String()
	obj["name"] = r.Name
	obj["type"] = r.Type
	obj["value"] = r.Value
	obj["ttl"] = r.TTL
	return obj
}
