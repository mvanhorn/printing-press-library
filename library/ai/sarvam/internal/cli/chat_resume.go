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
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "id=20260814_2d09e061-f89b-400e-8d64-89cfbe4e8e7d;msg=what was our conclusion?", "pp:typed-exit-codes": "0,3"},
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
			var stored struct {
				ID      string `json:"id"`
				Choices []struct {
					Message struct {
						Content string `json:"content"`
						Role    string `json:"role"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(raw, &stored); err != nil {
				return apiErr(fmt.Errorf("parsing stored conversation: %w", err))
			}
			var priorAssistant string
			for _, c := range stored.Choices {
				if c.Message.Content != "" {
					priorAssistant = c.Message.Content
				}
			}

			// Build the continuation request: prior assistant reply provides
			// context, the user's new message continues the thread.
			messages := []map[string]any{}
			if priorAssistant != "" {
				messages = append(messages,
					map[string]any{"role": "system", "content": "You are continuing a previous conversation. The assistant's last reply was: " + priorAssistant},
				)
			}
			messages = append(messages, map[string]any{"role": "user", "content": userMessage})

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
			reply := ""
			if len(resp.Choices) > 0 {
				reply = resp.Choices[0].Message.Content
			}

			result := map[string]any{
				"conversation_id": conversationID,
				"model":           flagModel,
				"reply":           reply,
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
