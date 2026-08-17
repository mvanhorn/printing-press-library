// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: rich DSpace-backed thesis item detail with full Dublin Core
// metadata (replaces the generated html_extract "page" baseline that only
// captured the title and nav links). Not generated — generate --force preserves
// this file.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/dspace"
)

func newThesisGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Get a thesis's full Dublin Core metadata by handle (305247 or 10603/305247)",
		Example: "  shodhganga-pp-cli thesis get 305247",
		Annotations: map[string]string{
			"pp:endpoint": "thesis.get", "pp:method": "GET", "pp:path": "/handle/10603/{id}",
			"mcp:read-only": "true", "pp:happy-args": "id=305247",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a thesis handle is required (e.g. 305247 or 10603/305247)"))
			}
			id, err := dspace.NormalizeID(args[0])
			if err != nil {
				return usageErr(err)
			}
			// verify runs in mock mode with no network; return a valid stub record.
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), &dspace.Thesis{
					Handle: dspace.HandleNamespace + "/" + id, ID: id,
				}, flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newDSpaceClient(flags)
			if err != nil {
				return err
			}
			th, err := c.Item(ctx, id)
			if err != nil {
				if err == dspace.ErrNotFound {
					return notFoundErr(fmt.Errorf("thesis %s not found", id))
				}
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), th, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Title:      %s\n", th.Title)
			fmt.Fprintf(w, "Researcher: %s\n", th.Researcher)
			if len(th.Guides) > 0 {
				fmt.Fprintf(w, "Guide(s):   %v\n", th.Guides)
			}
			if th.University != "" {
				fmt.Fprintf(w, "University: %s\n", th.University)
			}
			if th.Department != "" {
				fmt.Fprintf(w, "Department: %s\n", th.Department)
			}
			if th.CompletedDate != "" {
				fmt.Fprintf(w, "Completed:  %s\n", th.CompletedDate)
			}
			if len(th.Keywords) > 0 {
				fmt.Fprintf(w, "Keywords:   %v\n", th.Keywords)
			}
			fmt.Fprintf(w, "Handle:     %s\n", th.Handle)
			fmt.Fprintf(w, "URI:        %s\n", th.URI)
			if th.Abstract != "" {
				fmt.Fprintf(w, "\nAbstract:\n%s\n", th.Abstract)
			}
			return nil
		},
	}
	return cmd
}
