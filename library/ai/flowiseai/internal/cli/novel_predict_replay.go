// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newPredictReplayCmd(flags *rootFlags) *cobra.Command {
	var showDiff bool

	cmd := &cobra.Command{
		Use:   "replay [chatId]",
		Short: "Re-fire a prior prediction by chatId; optionally diff against the recorded response",
		Long: `Look up the original question + chatflowId for a chatId in the local cache,
then re-fire POST /prediction/{chatflowId} with the same question. With
--diff, compare the new response.text against the recorded one and emit both
side-by-side.

Useful for (1) recovering from a failed agent run, (2) comparing current
chatflow output against a known-good baseline, or (3) human-in-the-loop audit
when Sam needs to verify "is this still what we'd send today?"`,
		Example: "  flowiseai-pp-cli predict replay abc-chat-id --diff --json",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			chatID := args[0]

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// chatId can map to multiple rows (one per turn) — prefer the prediction
			// table's record if present, otherwise fall back to chatmessage.
			var question, originalText, chatflowID, sessionID string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(question,''), COALESCE(text,''), COALESCE(session_id,''),
					COALESCE(json_extract(data, '$.chatflowId'), '') AS cf
					FROM prediction WHERE chat_id = ? ORDER BY synced_at DESC LIMIT 1`,
				chatID).Scan(&question, &originalText, &sessionID, &chatflowID)
			if err != nil {
				// fall back to chatmessage
				err = db.DB().QueryRowContext(cmd.Context(),
					`SELECT COALESCE(content,''), COALESCE(chatflowid,''), COALESCE(session_id,'')
						FROM chatmessage WHERE chat_id = ? AND role = 'userMessage' ORDER BY created_date DESC LIMIT 1`,
					chatID).Scan(&question, &chatflowID, &sessionID)
				if err != nil {
					return notFoundErr(fmt.Errorf("no prior prediction or message recorded for chatId %s (try `sync` first)", chatID))
				}
			}
			if chatflowID == "" || question == "" {
				return apiErr(fmt.Errorf("cannot reconstruct replay: question=%q chatflowId=%q", question, chatflowID))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"question": question}
			if sessionID != "" {
				body["overrideConfig"] = map[string]any{"sessionId": sessionID}
			}
			resp, statusCode, postErr := c.Post("/prediction/"+chatflowID, body)
			if postErr != nil {
				return classifyAPIError(postErr, flags)
			}
			if statusCode >= 400 {
				return apiErr(fmt.Errorf("prediction returned HTTP %d", statusCode))
			}

			var newBlob map[string]any
			_ = json.Unmarshal(resp, &newBlob)
			newText, _ := newBlob["text"].(string)
			newChatID, _ := newBlob["chatId"].(string)

			result := struct {
				OriginalChatID string `json:"originalChatId"`
				NewChatID      string `json:"newChatId"`
				ChatflowID     string `json:"chatflowId"`
				Question       string `json:"question"`
				OriginalText   string `json:"originalText"`
				NewText        string `json:"newText"`
				TextsMatch     bool   `json:"textsMatch"`
				FullResponse   any    `json:"fullResponse,omitempty"`
			}{
				OriginalChatID: chatID,
				NewChatID:      newChatID,
				ChatflowID:     chatflowID,
				Question:       question,
				OriginalText:   originalText,
				NewText:        newText,
				TextsMatch:     originalText == newText,
				FullResponse:   newBlob,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Replayed chatId %s against chatflow %s\n", chatID, chatflowID)
			fmt.Fprintf(cmd.OutOrStdout(), "New chatId: %s\n\n", newChatID)
			fmt.Fprintf(cmd.OutOrStdout(), "Question: %s\n\n", question)
			if showDiff {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n\n", bold("Original:"), originalText)
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", bold("New:"), newText)
				if result.TextsMatch {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", green("Texts match exactly."))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", yellow("Texts differ."))
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", bold("New response:"), newText)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showDiff, "diff", false, "Show the recorded response alongside the new one")
	return cmd
}
