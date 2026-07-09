// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/store"
	"github.com/spf13/cobra"
)

func newArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Offline archive of your conversations, searchable full-text",
		Long: `Keep a local, offline-searchable archive of your Airbnb conversations.
'archive index' pulls your inbox and every thread's messages into a local
SQLite store; 'archive search' runs full-text search across them — something
neither the website nor any other tool offers.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newArchiveIndexCmd(flags))
	cmd.AddCommand(newArchiveSearchCmd(flags))
	return cmd
}

func newArchiveIndexCmd(flags *rootFlags) *cobra.Command {
	var maxThreads int
	cmd := &cobra.Command{
		Use:     "index",
		Short:   "Pull inbox threads and messages into the local archive",
		Example: "  airbnb-outreach-pp-cli archive index --max-threads 100",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := newAirbnbClient(flags)
			inbox, err := c.Inbox(maxThreads, "")
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			threadIDs := extractThreadIDs(inbox)
			indexed := 0
			for _, tid := range threadIDs {
				thread, err := c.Thread(tid, 100)
				if err != nil {
					continue
				}
				id := tid
				if err := db.Upsert("thread", id, thread); err == nil {
					indexed++
				}
			}
			result := map[string]any{"status": "indexed", "threads": indexed}
			if flags.asJSON {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s Indexed %d threads into the local archive.\n", green("✓"), indexed)
			fmt.Fprintln(cmd.OutOrStdout(), "  Search them with: airbnb-outreach-pp-cli archive search \"<query>\"")
			return nil
		},
	}
	cmd.Flags().IntVar(&maxThreads, "max-threads", 50, "Maximum threads to index")
	return cmd
}

func newArchiveSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Full-text search across your archived conversations",
		Example:     "  airbnb-outreach-pp-cli archive search \"early check-in\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			hits, err := db.Search(args[0], limit)
			if err != nil {
				return err
			}
			out := make([]json.RawMessage, 0, len(hits))
			out = append(out, hits...)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matches. Have you run 'airbnb-outreach-pp-cli archive index'?")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d matches\n", len(out))
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matches to return")
	return cmd
}

// extractThreadIDs pulls thread node IDs out of a ViaductInboxData inboxItems tree.
func extractThreadIDs(inbox json.RawMessage) []string {
	var tree struct {
		Edges []struct {
			Node map[string]json.RawMessage `json:"node"`
		} `json:"edges"`
	}
	if json.Unmarshal(inbox, &tree) != nil {
		return nil
	}
	var ids []string
	for _, e := range tree.Edges {
		// Thread ID may appear under a few keys depending on the inbox item shape.
		for _, key := range []string{"threadId", "id", "globalThreadId"} {
			if raw, ok := e.Node[key]; ok {
				var s string
				if json.Unmarshal(raw, &s) == nil && s != "" {
					ids = append(ids, s)
					break
				}
			}
		}
	}
	return ids
}
