package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/store"
)

func newSearchCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across synced notebook data",
		Long:  `Search locally synced notebook content. Run 'sync' first to populate the database.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()

			results, err := s.Search(args[0], limit)
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}
			if len(results) == 0 {
				fmt.Println("No results found. Run 'sync' to populate the database.")
				return nil
			}
			if flags.json {
				return printJSON(results)
			}
			for _, r := range results {
				fmt.Printf("%s  [%s]  %s\n", r.SourceID, r.NotebookID, r.Title)
				fmt.Printf("  %s\n\n", r.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")
	return cmd
}
