// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTBThreadsCmd(flags))
	})
}

func newTBThreadsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "threads",
		Short:       "Follow a conversation across folders, including your sent replies",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTBThreadsShowCmd(flags))
	return cmd
}

func newTBThreadsShowCmd(flags *rootFlags) *cobra.Command {
	var last int
	cmd := &cobra.Command{
		Use:   "show <message-id|thread-id>",
		Short: "Show every message of a thread in chronological order with direction in/out",
		Long: `Show all messages of a conversation, oldest first, across every folder and
account (inbox, archives and sent). Pass a message id from messages list, an
RFC Message-ID, or a thread_id. Threads follow References/In-Reply-To, never
the subject. direction=out marks messages stored in one of your sent folders.
--last N keeps only the N most recent messages (still oldest first); every row
carries total_messages, the size of the whole thread. Rows have no body: read
one with messages show <id> --no-quotes.`,
		Example: strings.Trim(`
  thunderbird-pp-cli threads show 3f9a1c2b7d4e
  thunderbird-pp-cli threads show 3f9a1c2b7d4e --last 5 --json --select id,date,direction,from_addr,total_messages`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "id=0123456789ab"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "threads show")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("expected exactly one message or thread id\nUsage: %s <id>", cmd.CommandPath()))
			}
			if last < 0 {
				return usageErr(fmt.Errorf("--last must be 0 (all) or a positive number"))
			}
			db, err := tbStoreFor(cmd, flags, "messages")
			if err != nil || db == nil {
				return err
			}
			defer db.Close()
			threadID := strings.TrimSpace(args[0])
			d, err := tbGetMessage(db, threadID)
			var ce *cliError
			switch {
			case err == nil:
				threadID = d.ThreadID
			case errors.As(err, &ce) && ce.code == 3:
			default:
				return err
			}
			docs, err := tbQueryMessages(db, `json_extract(data,'$.thread_id') = ?`, []any{threadID}, `json_extract(data,'$.date'), id`, 0)
			if err != nil {
				return err
			}
			if len(docs) == 0 {
				return notFoundErr(fmt.Errorf("no message or thread %q in the local store", args[0]))
			}
			mb, err := tbLoadMailbox(db)
			if err != nil {
				return err
			}
			total := len(docs)
			if last > 0 && last < total {
				docs = docs[total-last:]
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.ErrOrStderr(), "showing the last %d of %d messages\n", last, total)
				}
			}
			rows := make([]tbMessageRow, 0, len(docs))
			for _, doc := range docs {
				r := tbRowFromDoc(doc)
				r.Direction = mb.directionLabel(doc)
				r.TotalMessages = total
				rows = append(rows, r)
			}
			return tbPrintMessageRows(cmd, flags, rows, true)
		},
	}
	cmd.Flags().IntVar(&last, "last", 0, "Only the N most recent messages, still oldest first (0 = all)")
	return cmd
}
