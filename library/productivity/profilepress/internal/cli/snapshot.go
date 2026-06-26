package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"profilepress-pp-cli/internal/linkedin"
)

func newSnapshotCmd() *cobra.Command {
	var dbPath, fixture string
	cmd := &cobra.Command{
		Use:     "snapshot",
		Short:   "Capture current profile sections into the local mirror from a fixture or user-controlled browser session.",
		Example: "profilepress snapshot --fixture profile.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fixture == "" {
				return errors.New("snapshot requires --fixture until browser CDP capture is configured")
			}
			sections, source, err := linkedin.LoadSnapshotFixture(fixture)
			if err != nil {
				return err
			}
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			snap, err := db.CreateSnapshot(source, sections)
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"snapshot_id": snap.ID, "sections": len(snap.Sections), "source": snap.Source})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&fixture, "fixture", "", "JSON fixture with profile sections")
	return cmd
}
