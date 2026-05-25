// PATCH: novel `db receipt` — snapshot affected object definitions + auto-derive the mechanical inverse before mutating. Not in the Management API.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/dbchange"
)

func newDBReceiptCmd(flags *rootFlags) *cobra.Command {
	var inlineSQL string

	cmd := &cobra.Command{
		Use:   "receipt <ref> [file.sql]",
		Short: "Snapshot affected objects + derive the inverse for a plan, without applying",
		Long: `Capture a reversal receipt for a migration without applying it. Snapshots the
current definition of every object the plan touches (function bodies via
pg_get_functiondef, policy bodies via pg_policies, grants via
information_schema) and assembles the mechanical inverse where derivable
(CREATE->DROP, ADD COLUMN->DROP COLUMN, REVOKE->GRANT). The receipt is what
'db revert' consumes.

'db apply' captures a receipt automatically; this command is for capturing one
ahead of time or independently.`,
		Example:     `  supabase-pp-cli db receipt abcdefgh migrations/008_rls.sql --json`,
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/v1/projects/{ref}/database/query"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref := args[0]
			sql, src, err := loadSQL(args[1:], inlineSQL)
			if err != nil {
				return err
			}
			m := dbchange.Parse(sql, src)
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rcpt := buildReceipt(cmd, c, ref, m)
			rpath, werr := writeReceipt(rcpt)
			if werr != nil {
				return werr
			}

			out := cmd.OutOrStdout()
			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(out, rcpt, flags)
			}
			fmt.Fprintf(out, "Receipt %s written to %s\n", rcpt.Receipt, rpath)
			fmt.Fprintf(out, "plan_hash: %s\n", rcpt.PlanHash)
			fmt.Fprintf(out, "snapshots captured: %d\n", len(rcpt.Snapshots))
			if m.HasIrreversible {
				fmt.Fprintln(out, "NOTE: plan has irreversible statement(s) — revert will be partial.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&inlineSQL, "sql", "", "Inline SQL (instead of a file argument)")
	return cmd
}

// buildReceipt snapshots prior object definitions for RevSnapshot statements and
// assembles the inverse script (mechanical inverses in reverse statement order).
// Snapshot capture is best-effort: an object the introspection query cannot
// resolve is skipped and its statement remains snapshot-reversible only insofar
// as the snapshot exists.
func buildReceipt(cmd *cobra.Command, c *client.Client, ref string, m *dbchange.Manifest) *receipt {
	r := &receipt{
		Receipt:   newReceiptID(ref),
		CreatedAt: nowRFC3339(),
		Ref:       ref,
		PlanHash:  m.PlanHash,
		Source:    m.SourceLabel,
		Manifest:  m,
		Snapshots: map[string]string{},
	}

	// Snapshot prior definitions for objects whose reversal needs the old body.
	for _, s := range m.Statements {
		if s.Reversibility != dbchange.RevSnapshot || len(s.Objects) == 0 {
			continue
		}
		obj := s.Objects[0]
		if snap := snapshotObject(c, ref, s.OpClass, obj); snap != "" {
			r.Snapshots[string(s.OpClass)+":"+obj] = snap
		}
	}

	// Assemble inverse: mechanical inverses in reverse order. Snapshot-only and
	// irreversible statements contribute a comment marker so the operator sees
	// the gap.
	var inv []string
	for i := len(m.Statements) - 1; i >= 0; i-- {
		s := m.Statements[i]
		switch {
		case s.Inverse != "":
			inv = append(inv, s.Inverse)
		case s.Reversibility == dbchange.RevSnapshot:
			inv = append(inv, fmt.Sprintf("-- statement %d (%s on %s): restore from snapshot (see receipt.snapshots); no mechanical inverse", s.Index, s.OpClass, firstOrEmpty(s.Objects)))
		case s.Reversibility == dbchange.RevNone:
			inv = append(inv, fmt.Sprintf("-- statement %d (%s): IRREVERSIBLE — no inverse for data %s", s.Index, s.OpClass, s.OpClass))
		}
	}
	r.InverseSQL = strings.Join(inv, "\n")
	return r
}

// snapshotObject pulls the prior definition of an object via the Management API
// database/query endpoint, returning the captured SQL/text or "" on failure.
func snapshotObject(c *client.Client, ref string, op dbchange.OpClass, obj string) string {
	var q string
	switch op {
	case dbchange.OpDDL:
		// Function bodies and table column defs. Try function def first.
		q = fmt.Sprintf("select pg_get_functiondef('%s'::regprocedure) as def", sqlEscape(obj))
	case dbchange.OpRLS:
		schema, tbl := splitSchemaTable(obj)
		q = fmt.Sprintf("select policyname, cmd, qual, with_check, roles from pg_policies where schemaname = '%s' and tablename = '%s'", sqlEscape(schema), sqlEscape(tbl))
	case dbchange.OpGrant:
		schema, tbl := splitSchemaTable(obj)
		q = fmt.Sprintf("select grantee, privilege_type from information_schema.role_table_grants where table_schema = '%s' and table_name = '%s'", sqlEscape(schema), sqlEscape(tbl))
	default:
		return ""
	}
	path := replacePathParam("/v1/projects/{ref}/database/query", "ref", ref)
	data, _, err := c.Post(path, map[string]any{"query": q, "read_only": true})
	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

func splitSchemaTable(obj string) (string, string) {
	parts := strings.SplitN(obj, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "public", obj
}

func sqlEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func firstOrEmpty(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

// jsonUnmarshal is a thin wrapper so callers in this package don't all import
// encoding/json directly for one call site.
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
