package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

func newFactsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts",
		Short: "Operate on the three-layer-memory facts table (inline + TOML).",
	}
	cmd.AddCommand(newFactListCmd(flags))
	cmd.AddCommand(newFactAddCmd(flags))
	cmd.AddCommand(newFactSupersedeCmd(flags))
	cmd.AddCommand(newFactsDecisionTraceCmd(flags))
	cmd.AddCommand(newFactsGraduationCmd(flags))
	return cmd
}

func newFactListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list [path]",
		Short:       "List every fact on a note (inline + TOML facts file merged).",
		Example:     "  obsidian-pp-cli facts list 'People/Jeff Smith.md'\n  obsidian-pp-cli facts list 'People/Jeff Smith.md' --json",
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
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT id, fact, COALESCE(category,''), COALESCE(timestamp,''), COALESCE(status,''), COALESCE(source,''), COALESCE(decision_trace_id,''), storage
				 FROM facts WHERE parent_note_path = ? ORDER BY timestamp DESC, id`,
				args[0])
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				ID              string `json:"id"`
				Fact            string `json:"fact"`
				Category        string `json:"category,omitempty"`
				Timestamp       string `json:"timestamp,omitempty"`
				Status          string `json:"status,omitempty"`
				Source          string `json:"source,omitempty"`
				DecisionTraceID string `json:"decision_trace_id,omitempty"`
				Storage         string `json:"storage"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.ID, &e.Fact, &e.Category, &e.Timestamp, &e.Status, &e.Source, &e.DecisionTraceID, &e.Storage); err != nil {
					return apiErr(err)
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", e.Storage, e.ID, e.Fact)
				if e.Category != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  category=%s timestamp=%s status=%s\n", e.Category, e.Timestamp, e.Status)
				}
			}
			return nil
		},
	}
	return cmd
}

func newFactAddCmd(flags *rootFlags) *cobra.Command {
	var factText, category, source, traceID, idFlag string
	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "Add a protocol-compliant fact to a note's inline facts list.",
		Long: "Add a fact under the note's inline `facts:` list. When the inline count\n" +
			"reaches the graduation threshold (default 20), prints a hint to run\n" +
			"`facts graduation-candidates` to migrate the list into a TOML sidecar.",
		Example:     "  obsidian-pp-cli facts add 'People/Jeff Smith.md' --fact 'Prefers async' --category preference --source 'manual'",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || factText == "" {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			abs, err := v.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			n, err := v.LoadNote(abs)
			if err != nil {
				return apiErr(err)
			}
			if !n.HasFM {
				return usageErr(fmt.Errorf("note has no frontmatter — facts require a protocol-compliant note"))
			}
			id := idFlag
			if id == "" {
				stem := slugify(strings.TrimSuffix(args[0], ".md"))
				id = fmt.Sprintf("%s-%03d", strings.ToLower(stem), len(n.Frontmatter.Facts)+1)
			}
			f := vault.Fact{
				ID:              id,
				Fact:            factText,
				Category:        category,
				Timestamp:       time.Now().Format("2006-01-02"),
				Status:          "active",
				Source:          source,
				DecisionTraceID: traceID,
			}
			n.Frontmatter.Facts = append(n.Frontmatter.Facts, f)
			data, err := vault.AssembleNote(n.Frontmatter, []byte(n.Body))
			if err != nil {
				return apiErr(err)
			}
			if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"path":            n.Path,
					"id":              id,
					"status":          "added",
					"inline_count":    len(n.Frontmatter.Facts),
					"graduation_hint": len(n.Frontmatter.Facts) >= 15,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added: %s on %s (inline count: %d)\n", id, n.Path, len(n.Frontmatter.Facts))
			if len(n.Frontmatter.Facts) >= 15 {
				fmt.Fprintf(cmd.OutOrStdout(), "hint: %d inline facts — run `facts graduation-candidates` to consider TOML migration\n", len(n.Frontmatter.Facts))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&factText, "fact", "", "Fact text (required)")
	_ = cmd.MarkFlagRequired("fact")
	cmd.Flags().StringVar(&category, "category", "", "Fact category (relationship|milestone|status|preference|decision|role)")
	cmd.Flags().StringVar(&source, "source", "", "Source citation (e.g. '2026-05-15 Meeting Notes')")
	cmd.Flags().StringVar(&traceID, "decision-trace", "", "Decision trace ID (DT-YYYY-NNNN) if this fact is a decision")
	cmd.Flags().StringVar(&idFlag, "id", "", "Explicit fact id (defaults to <slug>-NNN)")
	return cmd
}

func newFactSupersedeCmd(flags *rootFlags) *cobra.Command {
	var idFlag string
	cmd := &cobra.Command{
		Use:         "supersede [path]",
		Short:       "Mark an inline fact as superseded (preserves history).",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || idFlag == "" {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			abs, err := v.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			n, err := v.LoadNote(abs)
			if err != nil {
				return apiErr(err)
			}
			updated := false
			for i, f := range n.Frontmatter.Facts {
				if f.ID == idFlag {
					n.Frontmatter.Facts[i].Status = "superseded"
					updated = true
					break
				}
			}
			if !updated {
				return notFoundErr(fmt.Errorf("fact id %s not found in inline facts (TOML facts must be edited in the sidecar)", idFlag))
			}
			data, err := vault.AssembleNote(n.Frontmatter, []byte(n.Body))
			if err != nil {
				return apiErr(err)
			}
			if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "id": idFlag, "status": "superseded"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "superseded: %s on %s\n", idFlag, n.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&idFlag, "id", "", "Fact id (required)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newFactsDecisionTraceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "decision-trace [trace-id]",
		Short:       "Walk every fact across the vault that cites a decision_trace_id.",
		Example:     "  obsidian-pp-cli facts decision-trace DT-2026-0142 --json",
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
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT parent_note_path, id, fact, COALESCE(category,''), COALESCE(timestamp,''), COALESCE(source,''), storage
				 FROM facts WHERE decision_trace_id = ? ORDER BY timestamp`,
				args[0])
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				ParentNotePath string `json:"parent_note_path"`
				ID             string `json:"id"`
				Fact           string `json:"fact"`
				Category       string `json:"category,omitempty"`
				Timestamp      string `json:"timestamp,omitempty"`
				Source         string `json:"source,omitempty"`
				Storage        string `json:"storage"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.ParentNotePath, &e.ID, &e.Fact, &e.Category, &e.Timestamp, &e.Source, &e.Storage); err != nil {
					return apiErr(err)
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no facts cite trace id %s\n", args[0])
				return nil
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s [%s] %s\n  fact: %s\n  source: %s\n\n", e.Timestamp, e.ParentNotePath, e.ID, e.Fact, e.Source)
			}
			return nil
		},
	}
	return cmd
}

func newFactsGraduationCmd(flags *rootFlags) *cobra.Command {
	var threshold int
	cmd := &cobra.Command{
		Use:   "graduation-candidates",
		Short: "List entities whose inline fact count is approaching the TOML graduation threshold.",
		Long: "Per the three-layer-memory protocol, entities with 20+ facts should\n" +
			"migrate from inline `facts:` to a `_facts/<name>.toml` sidecar file.\n" +
			"This reports entities at or near the threshold so you can graduate\n" +
			"them before the frontmatter gets unwieldy.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			if threshold <= 0 {
				threshold = 20
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT parent_note_path, COUNT(*) as c FROM facts
				 WHERE storage = 'inline'
				 GROUP BY parent_note_path
				 HAVING c >= ?
				 ORDER BY c DESC`, threshold-5)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				ParentNotePath string `json:"parent_note_path"`
				InlineCount    int    `json:"inline_count"`
				Threshold      int    `json:"threshold"`
				Overdue        bool   `json:"overdue"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.ParentNotePath, &e.InlineCount); err != nil {
					return apiErr(err)
				}
				e.Threshold = threshold
				e.Overdue = e.InlineCount >= threshold
				out = append(out, e)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no graduation candidates")
				return nil
			}
			for _, e := range out {
				marker := ""
				if e.Overdue {
					marker = " [overdue]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d facts%s\n", e.ParentNotePath, e.InlineCount, marker)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&threshold, "threshold", 20, "Inline fact count threshold for graduation")
	return cmd
}
