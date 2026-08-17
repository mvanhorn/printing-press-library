// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/v0/internal/store"
	"github.com/spf13/cobra"
)

func newNovelChatsStreamCmd(flags *rootFlags) *cobra.Command {
	var flagModel string
	var flagPrivacy string
	var flagTitle string
	var flagSystemPrompt string
	var flagMcpServerIDs string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "stream <message>",
		Short: "Create a chat and stream the SSE response live",
		Long:  `Create a chat and stream the model response live as Server-Sent Events. Renders each SSE event (chat, chat.title, message.parts.chunk, message.usage, error) as it arrives. With --json, emits one JSON object per event for agent-native consumption. Records model attribution locally so 'spend --by model' can attribute the resulting chat.`,
		Example: `  v0-pp-cli chats stream "Create a project management dashboard with a kanban board"
  v0-pp-cli chats stream "A landing page" --model v0-pro --privacy private --title "Landing"
  v0-pp-cli chats stream "A todo app" --json`,
		Annotations: map[string]string{"mcp:read-only": "false", "pp:no-error-path-probe": "true", "pp:happy-args": "<message>=Say hello"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "chats stream")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("message is required"))
			}
			message := strings.TrimSpace(args[0])
			if message == "" {
				return usageErr(fmt.Errorf("message must not be empty"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			body := map[string]any{"message": message}
			if flagSystemPrompt != "" {
				body["systemPrompt"] = flagSystemPrompt
			}
			if flagModel != "" {
				body["modelConfiguration"] = map[string]any{"modelId": flagModel}
			}
			if flagPrivacy != "" {
				body["privacy"] = flagPrivacy
			}
			if flagTitle != "" {
				body["title"] = flagTitle
			}
			if flagMcpServerIDs != "" {
				var ids []string
				if err := json.Unmarshal([]byte(flagMcpServerIDs), &ids); err != nil {
					return usageErr(fmt.Errorf("--mcp-server-ids must be a JSON array of strings"))
				}
				body["mcpServerIds"] = ids
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			streamBody, err := c.PostStream(ctx, "/chats/stream", body, map[string]string{"X-Printing-Press-Binary-Response": "true"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			defer streamBody.Close()

			var chatID string
			var sawError bool
			events := 0
			sc := bufio.NewScanner(streamBody)
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			// evtType is carried across lines within one SSE event block
			// (event: <type> is followed by data: <payload> on the next line).
			evtType := ""
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					evtType = ""
					continue
				}
				if strings.HasPrefix(line, "event:") {
					evtType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
					continue
				}
				var payload string
				if strings.HasPrefix(line, "data:") {
					payload = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				} else {
					payload = line
				}
				if payload == "" {
					continue
				}
				events++
				if evtType == "" {
					evtType = "data"
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{"event": evtType, "data": json.RawMessage(payload)}, flags)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), payload)
				}
				if evtType == "chat" {
					var chat struct {
						ID     string `json:"id"`
						ChatID string `json:"chatId"`
					}
					if json.Unmarshal([]byte(payload), &chat) == nil {
						if chat.ID != "" {
							chatID = chat.ID
						} else if chat.ChatID != "" {
							chatID = chat.ChatID
						}
					}
				}
				if evtType == "error" {
					sawError = true
					var errObj struct {
						Message string `json:"message"`
					}
					if json.Unmarshal([]byte(payload), &errObj) == nil && errObj.Message != "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "v0 error: %s\n", errObj.Message)
					}
				}
			}
			if sc.Err() != nil {
				return fmt.Errorf("reading SSE stream: %w", sc.Err())
			}
			if chatID != "" && flagModel != "" {
				recordV0ModelAttribution(cmd, flagDB, chatID, flagModel)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && chatID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "chat: %s (%d SSE events)\n", chatID, events)
			}
			if sawError {
				return fmt.Errorf("v0 returned an error event during generation")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagModel, "model", "", "Model to use for the generation (v0-mini, v0-pro, v0-max, v0-max-fast)")
	cmd.Flags().StringVar(&flagPrivacy, "privacy", "", "Visibility: public, private, team, team-edit, unlisted")
	cmd.Flags().StringVar(&flagTitle, "title", "", "Title for the new chat")
	cmd.Flags().StringVar(&flagSystemPrompt, "system-prompt", "", "System-level context for the chat")
	cmd.Flags().StringVar(&flagMcpServerIDs, "mcp-server-ids", "", "JSON array of MCP server IDs to enable")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path for model attribution recording")
	return cmd
}

func recordV0ModelAttribution(cmd *cobra.Command, dbPath, chatID, model string) {
	if dbPath == "" {
		dbPath = defaultDBPath("v0-pp-cli")
	}
	ctx, cancel := boundCtx(cmd.Context(), &rootFlags{timeout: 10 * time.Second})
	defer cancel()
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	_, _ = db.DB().ExecContext(ctx, `INSERT INTO model_usage (chat_id, model) VALUES (?, ?) ON CONFLICT(chat_id) DO UPDATE SET model = excluded.model`, chatID, model)
}
