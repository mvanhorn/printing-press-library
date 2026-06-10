package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/spf13/cobra"
)

func newCmsCollectionsLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collections",
		Short: "List CMS collections directly from the Framer API (live)",
		Long: `Connect to the Framer Server API via the Node.js bridge and list all CMS
collections with their fields and item counts.

Requires FRAMER_PROJECT_URL and FRAMER_API_KEY environment variables.`,
		Example: `  # List collections as table
  framer-pp-cli collections

  # List collections as JSON
  framer-pp-cli collections --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would call collections-list via bridge")
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			raw, err := bc.Call("collections-list")
			if err != nil {
				return fmt.Errorf("listing collections: %w", err)
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			// Parse and display as table
			var collections []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				FieldCount int    `json:"fieldCount"`
				ItemCount  int    `json:"itemCount"`
			}
			if err := json.Unmarshal(raw, &collections); err != nil {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			headers := []string{"ID", "NAME", "FIELDS", "ITEMS"}
			rows := make([][]string, 0, len(collections))
			for _, c := range collections {
				rows = append(rows, []string{
					c.ID,
					c.Name,
					fmt.Sprintf("%d", c.FieldCount),
					fmt.Sprintf("%d", c.ItemCount),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	return cmd
}

func newCmsItemsLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "items <collection-id>",
		Short: "List CMS items in a collection directly from the Framer API (live)",
		Long: `Connect to the Framer Server API via the Node.js bridge and list all items
in the specified collection.

Requires FRAMER_PROJECT_URL and FRAMER_API_KEY environment variables.`,
		Example: `  # List items in a collection
  framer-pp-cli items abc123

  # List items as JSON
  framer-pp-cli items abc123 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would call items-list via bridge")
				return nil
			}

			collectionID := args[0]

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			raw, err := bc.Call("items-list", collectionID)
			if err != nil {
				return fmt.Errorf("listing items: %w", err)
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			// Parse and display as table
			var items []struct {
				ID    string `json:"id"`
				Slug  string `json:"slug"`
				Draft bool   `json:"draft"`
			}
			if err := json.Unmarshal(raw, &items); err != nil {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			headers := []string{"ID", "SLUG", "DRAFT"}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.ID,
					item.Slug,
					fmt.Sprintf("%t", item.Draft),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	return cmd
}
