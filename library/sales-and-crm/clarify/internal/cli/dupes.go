// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: dupes — duplicate people/companies with ready-to-run merges.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type dupeGroupView struct {
	MatchedOn    string   `json:"matched_on"`
	Value        string   `json:"value"`
	RecordIDs    []string `json:"record_ids"`
	Names        []string `json:"names,omitempty"`
	MergeCommand string   `json:"merge_command"`
}

type dupesView struct {
	Type           string          `json:"type"`
	Groups         []dupeGroupView `json:"groups"`
	ScannedRecords int             `json:"scanned_records"`
	Note           string          `json:"note,omitempty"`
}

func newNovelDupesCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dupes",
		Short: "Finds likely duplicate people or companies by shared email, domain, or normalized name",
		Long: `Groups mirrored records that share an email address (people), a domain
(companies), or a normalized name, and prints a ready-to-run merge command for
each group. Review each group before running the merge — merges cannot be
undone.

Reads the local mirror: sync person or company objects first.`,
		Example: `  clarify-pp-cli dupes --type person --json
  clarify-pp-cli dupes --type company`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan the local mirror for duplicate records and print merge commands")
				return nil
			}
			objType := strings.ToLower(strings.TrimSpace(flagType))
			if objType != "person" && objType != "company" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--type must be person or company"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, ok, err := clarifyMirrorGuard(cmd, flags, ctx, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "resources") {
				hintIfStale(cmd, db, "resources", flags.maxAge)
			}

			records, err := loadClarifyObjects(ctx, db, objType)
			if err != nil {
				return err
			}

			byKey := map[string][]clarifyObj{}
			keyKind := map[string]string{}
			addKey := func(kind, value string, obj clarifyObj) {
				if value == "" {
					return
				}
				k := kind + "\x00" + value
				byKey[k] = append(byKey[k], obj)
				keyKind[k] = kind
			}
			for _, r := range records {
				if objType == "person" {
					for _, email := range attrItems(r.Attrs, clarifyEmailKeys...) {
						addKey("email", strings.ToLower(strings.TrimSpace(email)), r)
					}
				} else {
					for _, domain := range attrItems(r.Attrs, clarifyDomainKeys...) {
						addKey("domain", strings.ToLower(strings.TrimSpace(domain)), r)
					}
				}
				addKey("name", normalizeClarifyName(attrString(r.Attrs, clarifyNameKeys...)), r)
			}

			view := dupesView{Type: objType, Groups: []dupeGroupView{}, ScannedRecords: len(records)}
			seenGroups := map[string]bool{}
			for k, members := range byKey {
				// De-duplicate members that entered via multiple values.
				uniq := map[string]clarifyObj{}
				for _, m := range members {
					uniq[m.ID] = m
				}
				if len(uniq) < 2 {
					continue
				}
				ids := make([]string, 0, len(uniq))
				names := make([]string, 0, len(uniq))
				for id, m := range uniq {
					ids = append(ids, id)
					if n := attrString(m.Attrs, clarifyNameKeys...); n != "" {
						names = append(names, n)
					}
				}
				sort.Strings(ids)
				groupKey := strings.Join(ids, ",")
				if seenGroups[groupKey] {
					continue
				}
				seenGroups[groupKey] = true
				value := strings.SplitN(k, "\x00", 2)[1]
				target := ids[0]
				sourcesJSON, jerr := json.Marshal(ids[1:])
				if jerr != nil {
					continue
				}
				view.Groups = append(view.Groups, dupeGroupView{
					MatchedOn: keyKind[k],
					Value:     value,
					RecordIDs: ids,
					Names:     names,
					MergeCommand: fmt.Sprintf(
						"clarify-pp-cli objects records merge <workspace> %s %s --sources '%s'",
						objType, target, string(sourcesJSON)),
				})
			}
			sort.Slice(view.Groups, func(i, j int) bool {
				if len(view.Groups[i].RecordIDs) != len(view.Groups[j].RecordIDs) {
					return len(view.Groups[i].RecordIDs) > len(view.Groups[j].RecordIDs)
				}
				return view.Groups[i].Value < view.Groups[j].Value
			})

			if len(records) == 0 {
				view.Note = fmt.Sprintf("no %s records in the local mirror; run: clarify-pp-cli sync --resources resources --path-context object=%s", objType, objType)
			} else if len(view.Groups) == 0 {
				view.Note = fmt.Sprintf("no duplicate %s records detected across %d scanned", objType, len(records))
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(view.Groups) == 0 {
				fmt.Fprintln(out, view.Note)
				return nil
			}
			fmt.Fprintf(out, "%d likely duplicate groups (%d %s records scanned):\n\n", len(view.Groups), view.ScannedRecords, objType)
			for _, g := range view.Groups {
				fmt.Fprintf(out, "  shared %s %q: %v\n", g.MatchedOn, g.Value, g.Names)
				fmt.Fprintf(out, "    %s\n\n", g.MergeCommand)
			}
			fmt.Fprintln(out, "Review each group, then run its merge command with your workspace slug. Merges cannot be undone.")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "person", "Object type to scan for duplicates: person or company")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local mirror)")
	return cmd
}
