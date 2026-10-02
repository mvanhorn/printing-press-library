// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelChatResumeCmd(flags *rootFlags) *cobra.Command {
	var flagModel string
	var flagMaxTokens int

	cmd := &cobra.Command{
		Use:         "resume <conversation-id> <message>",
		Short:       "Continue a past chat thread from local history with full context",
		Example:     "  sarvam-pp-cli chat resume 20260814_2d09e061 'what was our conclusion?'",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "id=20260814_2d09e061-f89b-400e-8d64-89cfbe4e8e7d;msg=what was our conclusion?", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "chat resume")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional arguments: <conversation-id> <message>"))
			}
			conversationID := args[0]
			userMessage := strings.Join(args[1:], " ")
			if flagModel == "" {
				flagModel = "sarvam-105b"
			}
			if flagMaxTokens == 0 {
				flagMaxTokens = 2048
			}

			// Load the stored chat response for this conversation from local history.
			dbPath := defaultDBPath("sarvam-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: sarvam-pp-cli sync --resources chat --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"conversation_id": conversationID,
						"error":           "no local chat history; run sync first",
						"messages":        []any{},
					}, flags)
				}
				return nil
			}
			db, err := openStoreForRead(cmd.Context(), "sarvam-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return apiErr(fmt.Errorf("no local chat history. Run 'sarvam-pp-cli sync --resources chat' first"))
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "chat") {
				hintIfStale(cmd, db, "chat", flags.maxAge)
			}

			raw, err := db.Get("chat", conversationID)
			if err != nil {
				return notFoundErr(fmt.Errorf("conversation %q not found in local history", conversationID))
			}
			record, err := decodeStoredChatConversation(raw)
			if err != nil {
				return apiErr(err)
			}
			messages, err := buildChatResumeMessages(record, userMessage)
			if err != nil {
				return apiErr(err)
			}
			if !cmd.Flags().Changed("model") && record.Model != "" {
				flagModel = record.Model
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"messages":   messages,
				"model":      flagModel,
				"max_tokens": flagMaxTokens,
			}
			data, _, err := c.PostWithParams(ctx, "/v1/chat/completions", nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				ID      string `json:"id"`
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing chat response: %w", err))
			}
			if resp.ID == "" {
				return apiErr(fmt.Errorf("chat response has no conversation id"))
			}
			if persistErr := persistChatConversation(cmd.Context(), data, messages, flagModel); persistErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: resumed chat succeeded but local history was not saved: %v\n", persistErr)
			}
			reply := ""
			if len(resp.Choices) > 0 {
				reply = resp.Choices[0].Message.Content
			}

			result := map[string]any{
				"conversation_id":     conversationID,
				"model":               flagModel,
				"reply":               reply,
				"new_conversation_id": resp.ID,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), reply)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagModel, "model", "sarvam-105b", "Chat model to use for the continuation")
	cmd.Flags().IntVar(&flagMaxTokens, "max-tokens", 2048, "Maximum tokens for the continuation reply")
	return cmd
}
