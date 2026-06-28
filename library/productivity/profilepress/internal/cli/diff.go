package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var dbPath, packetID string
	cmd := &cobra.Command{
		Use:     "diff",
		Short:   "Show a human-readable before/after diff for a change packet.",
		Example: "profilepress diff --packet pkt_123",
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
			for _, ch := range p.Changes {
				fmt.Fprintf(cmd.OutOrStdout(), "## %s\n", ch.Section)
				writeDiffBlock(cmd.OutOrStdout(), "- ", ch.Before)
				writeDiffBlock(cmd.OutOrStdout(), "+ ", ch.After)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&packetID, "packet", "", "packet ID")
	return cmd
}

func writeDiffBlock(w io.Writer, prefix, body string) {
	if body == "" {
		fmt.Fprintln(w, prefix)
		return
	}
	for _, line := range strings.Split(body, "\n") {
		fmt.Fprintf(w, "%s%s\n", prefix, line)
	}
}
