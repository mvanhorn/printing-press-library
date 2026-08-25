// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

// chatFrozenReason is the second half of the staleness disclosure for chat.
// Granola's internal API exposes no chat endpoint (seven namings probed, all
// 404), so chat_threads/chat_messages only ever advance when a decryptable
// desktop cache is synced. On a migrated install that never happens again:
// the thread set is frozen wherever the last cache sync left it, however far
// behind the meetings it discusses that now is.
const chatFrozenReason = "These threads cannot be refreshed: Granola's internal API exposes no chat endpoint, so the set is frozen wherever the last desktop-cache sync left it."

func newChatCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Read Granola's AI chat threads from the local store or cache",
	}
	cmd.AddCommand(newChatListCmd(flags))
	cmd.AddCommand(newChatGetCmd(flags))
	return cmd
}

func newChatListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [<meeting-id>]",
		Short: "List chat threads (all, or anchored to a meeting)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// PATCH(dual-path-store-read): chat_threads/chat_messages are
			// hydrated into the store by the cache sync, so the 225 threads on
			// a migrated install remain readable long after the cache stops
			// decrypting. The read seam is the only way to reach them.
			v, err := openGranolaRead(cmd.Context())
			if err != nil {
				return err
			}
			defer v.Close()
			meetingFilter := ""
			if len(args) > 0 {
				meetingFilter = args[0]
			}
			// Index messages per thread to extract a first-message preview.
			byThread := map[string][]granola.ChatMessage{}
			for _, m := range v.ChatMessages() {
				byThread[m.Data.ThreadID] = append(byThread[m.Data.ThreadID], m)
			}
			for tid := range byThread {
				sort.Slice(byThread[tid], func(i, j int) bool {
					return byThread[tid][i].Data.TurnIndex < byThread[tid][j].Data.TurnIndex
				})
			}
			out := []map[string]any{}
			for tid, t := range v.ChatThreads() {
				if meetingFilter != "" && t.Data.DocumentID != meetingFilter {
					continue
				}
				preview := ""
				if msgs := byThread[tid]; len(msgs) > 0 {
					preview = truncatePreview(msgs[0].Data.RawText, 120)
				}
				out = append(out, map[string]any{
					"id":            tid,
					"title":         t.Data.Title,
					"meeting_id":    t.Data.DocumentID,
					"workspace_id":  t.WorkspaceID,
					"created_at":    t.CreatedAt,
					"updated_at":    t.UpdatedAt,
					"preview":       preview,
					"message_count": len(byThread[tid]),
				})
			}
			sort.Slice(out, func(i, j int) bool {
				return fmt.Sprint(out[i]["created_at"]) > fmt.Sprint(out[j]["created_at"])
			})
			return emitWithStaleness(cmd, flags, "threads", out, v.staleness(chatFrozenReason))
		},
	}
	return cmd
}

func newChatGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <thread-id>",
		Short: "Dump every message in a chat thread",
		Example: `  # Print one chat thread, oldest -> newest
  granola-pp-cli chat get 196037d9-7d28-4d4d-9c4f-c0e7e95b1aaa

  # JSON for downstream pipelines
  granola-pp-cli chat get 196037d9-7d28-4d4d-9c4f-c0e7e95b1aaa --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			tid := args[0]
			v, err := openGranolaRead(cmd.Context())
			if err != nil {
				return err
			}
			defer v.Close()
			t, ok := v.ChatThreads()[tid]
			if !ok {
				return notFoundErr(fmt.Errorf("thread %s not found", tid))
			}
			// ChatThreadMessages already returns turn order from either path.
			msgs := v.ChatThreadMessages(tid)
			out := map[string]any{
				"id":         tid,
				"title":      t.Data.Title,
				"meeting_id": t.Data.DocumentID,
				"messages":   msgs,
			}
			return emitJSON(cmd, flags, out)
		},
	}
	return cmd
}

func truncatePreview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
