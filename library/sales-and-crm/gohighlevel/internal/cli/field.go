// Copyright 2026 Jen Williams and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `field` command tree — custom field name <-> ID resolver. Reads the
// local SQLite cache's `custom_fields` table populated by sync.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newFieldCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field",
		Short: "Custom field name <-> ID lookup",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFieldIDCmd(flags))
	cmd.AddCommand(newFieldMapCmd(flags))
	return cmd
}

type fieldPair struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"fieldKey,omitempty"`
}

func readCustomFields(cmd *cobra.Command) ([]fieldPair, error) {
	ctx := cmd.Context()
	s, err := openGHLStore(ctx)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	rows, err := s.DB().QueryContext(ctx, `SELECT id, data FROM custom_fields`)
	if err != nil {
		return nil, fmt.Errorf("query custom_fields: %w", err)
	}
	defer rows.Close()
	var out []fieldPair
	for rows.Next() {
		var id sql.NullString
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		name, _ := obj["name"].(string)
		key, _ := obj["fieldKey"].(string)
		out = append(out, fieldPair{ID: nullStr(id), Name: name, Key: key})
	}
	return out, nil
}

func newFieldIDCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "id <name>",
		Short:       "Look up the custom-field ID for a given human name (with did-you-mean suggestions)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			target := args[0]
			fields, err := readCustomFields(cmd)
			if err != nil {
				return err
			}
			lcTarget := strings.ToLower(target)
			for _, f := range fields {
				if strings.EqualFold(f.Name, target) || strings.EqualFold(f.Key, target) || f.ID == target {
					return printJSONFiltered(cmd.OutOrStdout(), f, flags)
				}
			}
			// Did-you-mean by Levenshtein distance.
			type cand struct {
				p    fieldPair
				dist int
			}
			cands := make([]cand, 0, len(fields))
			for _, f := range fields {
				cands = append(cands, cand{p: f, dist: levenshteinDistance(lcTarget, strings.ToLower(f.Name))})
			}
			sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })
			top := cands
			if len(top) > 5 {
				top = top[:5]
			}
			suggestions := make([]fieldPair, 0, len(top))
			for _, c := range top {
				suggestions = append(suggestions, c.p)
			}
			result := map[string]any{
				"error":       fmt.Sprintf("no custom field with name %q", target),
				"suggestions": suggestions,
			}
			if flags.asJSON {
				_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "no custom field with name %q\n", target)
				fmt.Fprintln(cmd.OutOrStdout(), "did you mean:")
				for _, s := range suggestions {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\n", s.Name, s.ID)
				}
			}
			return notFoundErr(fmt.Errorf("field %q not found", target))
		},
	}
	return cmd
}

func newFieldMapCmd(flags *rootFlags) *cobra.Command {
	var tsv bool
	cmd := &cobra.Command{
		Use:         "map",
		Short:       "Dump all custom field name -> ID pairs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fields, err := readCustomFields(cmd)
			if err != nil {
				return err
			}
			sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
			if tsv && !flags.asJSON {
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, "name\tid\tkey")
				for _, f := range fields {
					fmt.Fprintf(w, "%s\t%s\t%s\n", f.Name, f.ID, f.Key)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), fields, flags)
		},
	}
	cmd.Flags().BoolVar(&tsv, "tsv", false, "Emit TSV instead of JSON")
	return cmd
}
