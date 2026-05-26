package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
)

func newSourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <notebook-id>",
		Short: "List sources in a notebook",
		Example: `notebooklm-pp-cli sources list <notebook-id>`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			var sources []nlm.Source
			if err := r.RunJSON(&sources, "source", "list", args[0]); err != nil {
				return err
			}
			if flags.json {
				return printJSON(sources)
			}
			for _, s := range sources {
				fmt.Printf("%s  [%s]  %s\n", s.ID, s.Type, s.Title)
			}
			return nil
		},
	}
}
