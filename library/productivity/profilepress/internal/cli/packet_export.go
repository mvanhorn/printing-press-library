package cli

import "github.com/spf13/cobra"

func newPacketexportCmd() *cobra.Command {
	var dbPath, packetID string
	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export current profile, target opportunity, proposed changes, risk notes, and status for CEE review.",
		Example: "profilepress packet export --packet pkt_123",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			p, err := packetByIDOrLatest(db, packetID)
			if err != nil {
				return err
			}
			snap, err := db.GetSnapshot(p.SnapshotID)
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"packet": p, "snapshot": snap})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&packetID, "packet", "", "packet ID")
	return cmd
}
