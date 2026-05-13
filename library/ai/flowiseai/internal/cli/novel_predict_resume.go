// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newPredictResumeCmd(flags *rootFlags) *cobra.Command {
	var humanInputJSON string

	cmd := &cobra.Command{
		Use:   "resume [chatId]",
		Short: "Resume a suspended AgentFlow V2 run by chatId with a structured humanInput payload",
		Long: `When an AgentFlow V2 chatflow pauses at a human-in-the-loop checkpoint, the
runtime returns the chatId for the suspended run. Resume it by sending another
prediction with the humanInput payload populated.

This command resolves the chatId → chatflowId mapping from the local cache,
so an agent only needs to remember the chatId from the prior turn.

The --input value is a JSON string passed verbatim into the humanInput body
field. Common shapes:
  {"type":"proceed"}
  {"type":"reject","feedback":"Skip this section"}
  {"type":"proceed","feedback":"Approved by Sam"}`,
		Example: "  flowiseai-pp-cli predict resume abc-chat-id --input '{\"type\":\"proceed\",\"feedback\":\"Approved\"}'",
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
			if humanInputJSON == "" {
				return usageErr(fmt.Errorf("--input is required (JSON string for humanInput payload)"))
			}
			var humanInput any
			if err := json.Unmarshal([]byte(humanInputJSON), &humanInput); err != nil {
				return usageErr(fmt.Errorf("parsing --input JSON: %w", err))
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Resolve chatflowId from chatId via prediction or chatmessage.
			var chatflowID, sessionID string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(json_extract(data, '$.chatflowId'), ''),
					COALESCE(session_id,'') FROM prediction WHERE chat_id = ? ORDER BY synced_at DESC LIMIT 1`,
				chatID).Scan(&chatflowID, &sessionID)
			if err != nil || chatflowID == "" {
				err = db.DB().QueryRowContext(cmd.Context(),
					`SELECT COALESCE(chatflowid,''), COALESCE(session_id,'') FROM chatmessage WHERE chat_id = ? ORDER BY created_date DESC LIMIT 1`,
					chatID).Scan(&chatflowID, &sessionID)
				if err != nil || chatflowID == "" {
					return notFoundErr(fmt.Errorf("no prior record for chatId %s in local cache (try `sync` first)", chatID))
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"question":   "",
				"humanInput": humanInput,
			}
			overrideConfig := map[string]any{}
			if sessionID != "" {
				overrideConfig["sessionId"] = sessionID
			}
			overrideConfig["chatId"] = chatID
			body["overrideConfig"] = overrideConfig

			resp, statusCode, postErr := c.Post("/prediction/"+chatflowID, body)
			if postErr != nil {
				return classifyAPIError(postErr, flags)
			}
			if statusCode >= 400 {
				return apiErr(fmt.Errorf("resume returned HTTP %d", statusCode))
			}

			var blob map[string]any
			_ = json.Unmarshal(resp, &blob)

			result := struct {
				OriginalChatID string         `json:"originalChatId"`
				ChatflowID     string         `json:"chatflowId"`
				HumanInput     any            `json:"humanInput"`
				Response       map[string]any `json:"response"`
			}{
				OriginalChatID: chatID,
				ChatflowID:     chatflowID,
				HumanInput:     humanInput,
				Response:       blob,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resumed chatId %s on chatflow %s\n", chatID, chatflowID)
			if txt, ok := blob["text"].(string); ok && txt != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n%s\n", bold("Response:"), txt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&humanInputJSON, "input", "", "JSON string for the humanInput payload (e.g. '{\"type\":\"proceed\"}')")
	return cmd
}
