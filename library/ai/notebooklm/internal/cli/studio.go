// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newStudioCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "studio",
		Short: "Studio artifacts (quiz, audio, etc.)",
	}
	cmd.AddCommand(newStudioListCmd(flags))
	cmd.AddCommand(newStudioGenerateQuizCmd(flags))
	return cmd
}

func newStudioListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list <notebook>",
		Short: "List Studio artifacts generated for a notebook",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli studio list "Q3 Research" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON([]any{})
				}
				dryRunMessage("list studio artifacts")
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
			arts, err := client.ListArtifacts(context.Background(), nb.ID)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(arts)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tTITLE\tSTATUS")
			for _, a := range arts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Type, a.Title, a.Status)
			}
			return w.Flush()
		},
	}
}

func newStudioGenerateQuizCmd(flags *rootFlags) *cobra.Command {
	var wait bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:     "generate-quiz <notebook>",
		Short:   "Start Studio quiz generation for a notebook",
		Args:    cobra.ExactArgs(1),
		Example: `  notebooklm-pp-cli studio generate-quiz "Q3 Research" --wait --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]string{"id": "dry-run-artifact", "status": "pending"})
				}
				dryRunMessage("generate quiz artifact")
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
			art, err := client.GenerateQuiz(context.Background(), nb.ID, nil, "")
			if err != nil {
				return err
			}
			if wait && art.ID != "" {
				art, err = client.WaitForArtifact(context.Background(), nb.ID, art.ID, timeout)
				if err != nil {
					return err
				}
			}
			if flags.asJSON {
				return printJSON(art)
			}
			fmt.Printf("quiz %s status=%s\n", art.ID, art.Status)
			return nil
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for artifact completion")
	cmd.Flags().DurationVar(&timeout, "timeout", defaultArtifactWait, "Wait timeout")
	return cmd
}
