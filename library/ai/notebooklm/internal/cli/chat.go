// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	"github.com/spf13/cobra"
)

func newChatCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Chat with a notebook",
	}
	cmd.AddCommand(newChatAskCmd(flags))
	cmd.AddCommand(newChatHistoryCmd(flags))
	return cmd
}

func newChatAskCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "ask <notebook> [question]",
		Short: "Ask a grounded question against notebook sources",
		Args:  cobra.MinimumNArgs(1),
		Example: `  notebooklm-pp-cli chat ask "Q3 Research" "Summarize the key risks" --json
  echo "What changed since last quarter?" | notebooklm-pp-cli chat ask "Q3 Research" --stdin --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]any{"answer": "dry-run answer", "citations": []any{}})
				}
				dryRunMessage("ask notebook")
				return nil
			}
			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			nb, err := client.ResolveNotebook(context.Background(), args[0])
			if err != nil {
				return wrapAPIError(err)
			}
			question := ""
			if flags.stdin {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return usageErr(err)
				}
				question = strings.TrimSpace(string(b))
			}
			if question == "" && len(args) >= 2 {
				question = args[1]
				if len(args) > 2 {
					question = joinArgs(args[1:])
				}
			}
			if question == "" {
				return usageErr(fmt.Errorf("question required as argument or via --stdin"))
			}
			result, err := client.Ask(context.Background(), nb.ID, question, nil)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(result)
			}
			fmt.Println(result.Answer)
			if len(result.Citations) > 0 {
				fmt.Fprintf(os.Stderr, "\n%d citations\n", len(result.Citations))
			}
			return nil
		},
	}
}

func newChatHistoryCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "history <notebook>",
		Short: "List prior chat turns for a notebook conversation",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli chat history "Q3 Research" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON([]nlm.ConversationTurn{})
				}
				dryRunMessage("list chat history")
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
			turns, err := client.ListConversationTurns(context.Background(), nb.ID)
			if err != nil {
				return err
			}
			if turns == nil {
				turns = []nlm.ConversationTurn{}
			}
			if flags.asJSON {
				return printJSON(turns)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for i, t := range turns {
				fmt.Fprintf(w, "#%d\tQ: %s\n", i+1, t.Question)
				fmt.Fprintf(w, "\tA: %s\n", truncateText(t.Answer, 200))
			}
			return w.Flush()
		},
	}
}

func joinArgs(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += " " + p
	}
	return out
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
