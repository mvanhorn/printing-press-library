// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newWorkflowSnapshotSaveCmd(flags *rootFlags) *cobra.Command {
	o := &sendfoxEvidenceOptions{}
	cmd := &cobra.Command{
		Use:     "snapshot-save",
		Short:   "Persist a private, dated and checksummed local snapshot history artifact.",
		Example: "  sendfox-pp-cli workflow snapshot-save --input examples/snapshot.json --out ./history --agent",
	}
	name := "snapshot-save"

	cmd.Flags().StringVar(&o.input, "input", "", "Version 1 local JSON/YAML evidence snapshot, or Markdown YAML frontmatter")
	cmd.Flags().StringVar(&o.dbPath, "db", "", "Local SQLite mirror created by sync; no network refresh")
	cmd.Flags().StringVar(&o.outputDir, "out", "", "Required durable snapshot history directory (writes a dated, checksummed 0600 JSON file)")
	cmd.Flags().IntVar(&o.limit, "limit", 100, "Maximum findings printed; counts remain complete")
	cmd = configureSendfoxEvidenceCmd(cmd, flags, name, o)
	cmd.Annotations["mcp:local-write"] = "true"
	delete(cmd.Annotations, "mcp:read-only")
	cmd.Annotations["pp:happy-args"] += ";--out=/tmp/sendfox-snapshot-history"
	cmd.Example = "  sendfox-pp-cli workflow snapshot-save --input examples/snapshot.json --out ./history --agent"
	return cmd
}
