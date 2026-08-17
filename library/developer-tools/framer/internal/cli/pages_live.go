package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"

	"github.com/spf13/cobra"
)

func newPagesLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pages",
		Short: "List pages from the live Framer project via the Server API bridge",
		Example: `  framer-pp-cli pages
  framer-pp-cli pages --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list pages from live Framer project")
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			raw, err := bc.Call("pages-list")
			if err != nil {
				return fmt.Errorf("pages-list failed: %w", err)
			}

			var pages []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
				Path string `json:"path"`
			}
			if err := json.Unmarshal(raw, &pages); err != nil {
				return fmt.Errorf("parsing pages response: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, pages)
			}

			headers := []string{"ID", "NAME", "TYPE", "PATH"}
			var rows [][]string
			for _, p := range pages {
				rows = append(rows, []string{p.ID, p.Name, p.Type, p.Path})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}
