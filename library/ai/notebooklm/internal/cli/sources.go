// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newSourceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Aliases: []string{"sources"},
		Short:   "Manage notebook sources",
	}
	cmd.AddCommand(newSourceListCmd(flags))
	cmd.AddCommand(newSourceAddCmd(flags))
	cmd.AddCommand(newSourceDeleteCmd(flags))
	return cmd
}

func newSourceListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list <notebook>",
		Short: "List URL and document sources attached to a notebook",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli source list "Q3 Research" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON([]any{})
				}
				dryRunMessage("list sources")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			nb, err := client.ResolveNotebook(context.Background(), args[0])
			if err != nil {
				return err
			}
			sources, err := client.ListSources(context.Background(), nb.ID)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(sources)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tURL")
			for _, s := range sources {
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.ID, s.Title, s.URL)
			}
			return w.Flush()
		},
	}
}

func newSourceAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "add <notebook> <url>",
		Short:   "Add a public URL source to a notebook",
		Args:    cobra.ExactArgs(2),
		Example: `  notebooklm-pp-cli source add "Q3 Research" https://en.wikipedia.org/wiki/NotebookLM --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]string{"id": "dry-run-source", "title": args[1]})
				}
				dryRunMessage("add URL source")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			nb, err := client.ResolveNotebook(context.Background(), args[0])
			if err != nil {
				return err
			}
			src, err := client.AddURLSource(context.Background(), nb.ID, args[1])
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(src)
			}
			fmt.Printf("added %s (%s)\n", src.ID, src.Title)
			return nil
		},
	}
}

func newSourceDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <notebook> <source-id>",
		Short:   "Remove a source from a notebook by source id",
		Args:    cobra.ExactArgs(2),
		Example: `  notebooklm-pp-cli source delete "Q3 Research" src_abc123 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]string{"deleted": args[1]})
				}
				dryRunMessage("delete source")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			nb, err := client.ResolveNotebook(context.Background(), args[0])
			if err != nil {
				return err
			}
			if err := client.DeleteSource(context.Background(), nb.ID, args[1]); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(map[string]string{"deleted": args[1]})
			}
			fmt.Printf("deleted source %s\n", args[1])
			return nil
		},
	}
}
