// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// fires: trigger<->tag<->variable graph walk for incident debugging.

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type firesResult struct {
	Mode      string   `json:"mode"`
	Target    string   `json:"target"`
	Tags      []string `json:"tags,omitempty"`
	Triggers  []string `json:"triggers,omitempty"`
	Variables []string `json:"variables,omitempty"`
}

func newNovelFiresCmd(flags *rootFlags) *cobra.Command {
	var dbPath, container, tag, trigger string

	cmd := &cobra.Command{
		Use:         "fires",
		Short:       "Walk the trigger/tag/variable graph for a tag or trigger",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Graph walk for incident debugging. With --trigger, list every tag that fires
on the trigger. With --tag, list the triggers it fires on and the variables it
references. Identify the tag or trigger by name or id. Run 'pull' first.`,
		Example: `  # Which tags fire on a trigger?
  google-tag-manager-pp-cli fires --trigger 42

  # What does a tag depend on?
  google-tag-manager-pp-cli fires --tag "GA4 - Purchase" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would walk the tag/trigger graph")
				return nil
			}
			if (tag == "") == (trigger == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("pass exactly one of --tag or --trigger"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, snaps, ok, err := gtmReadSnapshots(ctx, cmd, flags, gtmDBPath(dbPath))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			snap, err := resolveSnapshot(snaps, container)
			if err != nil {
				return usageErr(err)
			}
			entities, err := snapshotEntities(ctx, s.DB(), snap.ID, "")
			if err != nil {
				return err
			}
			idx := buildRefIndex(entities)

			var res firesResult
			if trigger != "" {
				trigID, name := resolveTrigger(idx, trigger)
				if trigID == "" {
					return notFoundErr(fmt.Errorf("no trigger %q in %s", trigger, snap.Label))
				}
				res = firesResult{Mode: "trigger", Target: fmt.Sprintf("%s (id %s)", name, trigID), Tags: idx.trigUsedBy[trigID]}
				if res.Tags == nil {
					res.Tags = []string{}
				}
			} else {
				te, found := findTag(idx, tag)
				if !found {
					return notFoundErr(fmt.Errorf("no tag %q in %s", tag, snap.Label))
				}
				res = firesResult{Mode: "tag", Target: te.Name, Triggers: []string{}, Variables: []string{}}
				for _, id := range idx.tagTriggers[te.EntityID] {
					if tr, ok := idx.trigByID[id]; ok {
						res.Triggers = append(res.Triggers, fmt.Sprintf("%s (id %s)", tr.Name, id))
					} else if id == allPagesTriggerID {
						res.Triggers = append(res.Triggers, "All Pages (built-in)")
					} else {
						res.Triggers = append(res.Triggers, "id "+id)
					}
				}
				res.Variables = variableRefsIn(te.Data)
			}

			return gtmEmit(cmd, flags, res, func(w io.Writer) {
				if res.Mode == "trigger" {
					fmt.Fprintf(w, "%d tag(s) fire on %s:\n", len(res.Tags), res.Target)
					for _, t := range res.Tags {
						fmt.Fprintf(w, "  %s\n", t)
					}
					return
				}
				fmt.Fprintf(w, "tag %q fires on %d trigger(s), references %d variable(s):\n", res.Target, len(res.Triggers), len(res.Variables))
				for _, t := range res.Triggers {
					fmt.Fprintf(w, "  trigger: %s\n", t)
				}
				for _, v := range res.Variables {
					fmt.Fprintf(w, "  variable: {{%s}}\n", v)
				}
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&container, "snapshot", "", "Snapshot ref: live, workspace:<id>, container:<id>, version:<id>, or a snapshot id (default: most recent)")
	cmd.Flags().StringVar(&tag, "tag", "", "Tag name or id to inspect")
	cmd.Flags().StringVar(&trigger, "trigger", "", "Trigger name or id to inspect")
	return cmd
}

func resolveTrigger(idx *refIndex, input string) (id, name string) {
	if t, ok := idx.trigByID[input]; ok {
		return input, t.Name
	}
	for _, t := range idx.triggers {
		if t.Name == input {
			return t.EntityID, t.Name
		}
	}
	return "", ""
}

func findTag(idx *refIndex, input string) (gtmEntity, bool) {
	for _, t := range idx.tags {
		if t.Name == input || t.EntityID == input {
			return t, true
		}
	}
	return gtmEntity{}, false
}
