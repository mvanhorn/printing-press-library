// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	"github.com/spf13/cobra"
)

func newNotebookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "notebook",
		Aliases: []string{"notebooks"},
		Short:   "Manage notebooks",
	}
	cmd.AddCommand(newNotebookListCmd(flags))
	cmd.AddCommand(newNotebookCreateCmd(flags))
	cmd.AddCommand(newNotebookGetCmd(flags))
	cmd.AddCommand(newNotebookRenameCmd(flags))
	cmd.AddCommand(newNotebookDeleteCmd(flags))
	return cmd
}

func newNotebookListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recently viewed notebooks from your Google account",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli notebook list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON([]nlm.Notebook{{ID: "dry-run-id", Title: "Example Notebook"}})
				}
				dryRunMessage("list notebooks")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			notebooks, err := client.ListNotebooks(context.Background())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(notebooks)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tSOURCES")
			for _, nb := range notebooks {
				title := nb.Title
				if nb.Emoji != "" {
					title = title + " " + nb.Emoji
				}
				fmt.Fprintf(w, "%s\t%s\t%d\n", nb.ID, title, nb.SourceCount)
			}
			return w.Flush()
		},
	}
}

func newNotebookCreateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "create [title]",
		Short:   "Create a new notebook with the given title",
		Args:    cobra.MinimumNArgs(1),
		Example: `  notebooklm-pp-cli notebook create "Q3 Research" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(nlm.Notebook{ID: "dry-run-id", Title: args[0]})
				}
				dryRunMessage("create notebook")
				return nil
			}
			title := args[0]
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			nb, err := client.CreateNotebook(context.Background(), title)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(nb)
			}
			fmt.Printf("created %s (%s)\n", nb.ID, nb.Title)
			return nil
		},
	}
}

func newNotebookGetCmd(flags *rootFlags) *cobra.Command {
	var notebookID string
	cmd := &cobra.Command{
		Use:   "get <id-or-title>",
		Short: "Get notebook metadata and source list by id or title",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli notebook get "Q3 Research" --json
  notebooklm-pp-cli notebook get --notebook-id abc123def456 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := notebookID
			if len(args) > 0 {
				target = args[0]
			}
			if target == "" {
				return usageErr(fmt.Errorf("notebook id or title required"))
			}
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]any{"id": "dry-run-id", "title": target, "sources": []any{}})
				}
				dryRunMessage("get notebook")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			nb, err := client.ResolveNotebook(context.Background(), target)
			if err != nil {
				return wrapAPIError(err)
			}
			detail, err := client.GetNotebook(context.Background(), nb.ID)
			if err != nil {
				return wrapAPIError(err)
			}
			if flags.asJSON {
				return printJSON(detail)
			}
			fmt.Printf("%s\t%s\n", detail.ID, detail.Title)
			for _, s := range detail.Sources {
				fmt.Printf("  - %s\t%s\n", s.ID, s.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&notebookID, "notebook-id", "", "Notebook id when not passed as a positional argument")
	return cmd
}

func newNotebookRenameCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "rename <id-or-title> <new-title>",
		Short:   "Rename an existing notebook to a new title",
		Args:    cobra.ExactArgs(2),
		Example: `  notebooklm-pp-cli notebook rename "Q3 Research" "Q3 Research (final)" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]string{"id": "dry-run-id", "title": args[1]})
				}
				dryRunMessage("rename notebook")
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
			if err := client.RenameNotebook(context.Background(), nb.ID, args[1]); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(map[string]string{"id": nb.ID, "title": args[1]})
			}
			fmt.Printf("renamed %s\n", nb.ID)
			return nil
		},
	}
}

func newNotebookDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id-or-title>",
		Short:   "Permanently delete a notebook and its sources",
		Args:    cobra.ExactArgs(1),
		Example: `  notebooklm-pp-cli notebook delete "Old Draft" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]string{"deleted": "dry-run-id"})
				}
				dryRunMessage("delete notebook")
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
			if err := client.DeleteNotebook(context.Background(), nb.ID); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(map[string]string{"deleted": nb.ID})
			}
			fmt.Printf("deleted %s\n", nb.ID)
			return nil
		},
	}
}

func newWhoamiCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show account tier and output language from user settings",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli whoami --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]string{"tier": "standard", "output_language": "en"})
				}
				dryRunMessage("fetch account settings")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			info, err := client.GetAccountInfo(context.Background())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(info)
			}
			fmt.Printf("tier: %s\nlanguage: %s\n", info.Tier, info.OutputLanguage)
			return nil
		},
	}
}

// used by studio wait flag default
const defaultArtifactWait = 120 * time.Second
