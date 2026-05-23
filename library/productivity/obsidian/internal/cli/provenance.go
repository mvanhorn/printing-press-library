package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

func newProvenanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provenance [path]",
		Short: "Walk the source chain for a note and every fact it carries.",
		Long: "Reports the `source` field on the note's frontmatter plus the `source`\n" +
			"and `decision_trace_id` columns on every fact (inline + TOML). Useful\n" +
			"when auditing where a datum came from or reconstructing a contested\n" +
			"fact's provenance for a stakeholder review.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			path := args[0]
			// Note-level source from frontmatter_fields.
			var noteSource, noteType, noteDate, noteDescription string
			_ = vc.S.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(value, '') FROM frontmatter_fields WHERE path = ? AND key = 'source'`, path).Scan(&noteSource)
			_ = vc.S.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(type,''), COALESCE(date,''), COALESCE(description,'') FROM notes WHERE path = ?`,
				path).Scan(&noteType, &noteDate, &noteDescription)
			if noteType == "" && noteDate == "" {
				return notFoundErr(fmt.Errorf("note not indexed (run sync): %s", path))
			}
			// Fact-level sources.
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT id, fact, COALESCE(source,'') as src, COALESCE(decision_trace_id,'') as trace, storage
				 FROM facts WHERE parent_note_path = ? ORDER BY timestamp`,
				path)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type factProv struct {
				ID              string `json:"id"`
				Fact            string `json:"fact"`
				Source          string `json:"source,omitempty"`
				DecisionTraceID string `json:"decision_trace_id,omitempty"`
				Storage         string `json:"storage"`
			}
			var facts []factProv
			for rows.Next() {
				var f factProv
				if err := rows.Scan(&f.ID, &f.Fact, &f.Source, &f.DecisionTraceID, &f.Storage); err != nil {
					return apiErr(err)
				}
				facts = append(facts, f)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"path":        path,
					"type":        noteType,
					"date":        noteDate,
					"description": noteDescription,
					"note_source": noteSource,
					"facts":       facts,
					"facts_count": len(facts),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "  type=%s date=%s\n", noteType, noteDate)
			if noteSource != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  source: %s\n", noteSource)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "  source: (not set in frontmatter)")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Facts (%d):\n", len(facts))
			for _, f := range facts {
				src := f.Source
				if src == "" {
					src = "(no source)"
				}
				trace := ""
				if f.DecisionTraceID != "" {
					trace = " trace=" + f.DecisionTraceID
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s\n    source: %s%s\n", f.Storage, f.ID, f.Fact, src, trace)
			}
			return nil
		},
	}
	return cmd
}
