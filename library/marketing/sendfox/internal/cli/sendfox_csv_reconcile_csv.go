// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newContactsReconcileCSVCmd(f *rootFlags) *cobra.Command {
	o := &sendfoxCSVOptions{}
	cmd := &cobra.Command{Use: "reconcile-csv", Short: "Audit CSV identities and preview guarded, suppression-aware contact creation"}
	cmd.Flags().StringVar(&o.file, "file", "", "CSV file containing email and optional profile/suppression columns")
	cmd.Flags().StringVar(&o.input, "input", "", "Local snapshot with complete contacts and unsubscribe evidence")
	cmd.Flags().StringVar(&o.lists, "lists", "", "Comma-separated existing list IDs for new contacts")

	return configureSendfoxCSV(cmd, f, "reconcile-csv", o)
}
