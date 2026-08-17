// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type v0TailMessageView struct {
	ChatID       string   `json:"chat_id"`
	MessageID    string   `json:"message_id"`
	Role         string   `json:"role"`
	Status       string   `json:"status"`
	FinishReason string   `json:"finish_reason,omitempty"`
	Content      string   `json:"content,omitempty"`
	PartTypes    []string `json:"part_types,omitempty"`
	Polled       int      `json:"polled"`
}

func newNovelMessagesTailCmd(flags *rootFlags) *cobra.Command {
	var flagInterval string
	var flagTimeout string
	var flagFollow bool

	cmd := &cobra.Command{
		Use:   "tail <chatId>",
		Short: "Poll a chat until the newest assistant message finishes, with --follow for continuous watching.",
		Long:  `Poll the messages endpoint until the newest assistant message has a non-null finishReason (stop, length, error, ...) or the timeout is hit. With --follow, keeps tailing and re-arms whenever a new generation starts.`,
		Example: `  v0-pp-cli messages tail 8RbjHxB3FyL
  v0-pp-cli messages tail 8RbjHxB3FyL --interval 3s --timeout 10m
  v0-pp-cli messages tail 8RbjHxB3FyL --json`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<chatId>=ft7dqhYEX8n"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "messages tail")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("chatId is required"))
			}
			chatID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			interval := 3 * time.Second
			if flagInterval != "" {
				d, err := time.ParseDuration(flagInterval)
				if err != nil || d <= 0 {
					return usageErr(fmt.Errorf("--interval must be a positive duration like 3s (got %q)", flagInterval))
				}
				interval = d
			}
			timeout := 30 * time.Minute
			if flagTimeout != "" {
				d, err := time.ParseDuration(flagTimeout)
				if err != nil || d <= 0 {
					return usageErr(fmt.Errorf("--timeout must be a positive duration like 10m (got %q)", flagTimeout))
				}
				timeout = d
			}
			deadline := time.Now().Add(timeout)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/chats/{chatId}/messages", "chatId", chatID)

			polled := 0
			lastEmittedID := ""
			lastEmittedStatus := ""
			for {
				if time.Now().After(deadline) {
					timeoutErr := fmt.Errorf("timed out after %s; generation may still be running", timeout)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						_ = printJSONFiltered(cmd.OutOrStdout(), v0TailMessageView{ChatID: chatID, Status: "timeout", Polled: polled}, flags)
						return apiErr(timeoutErr)
					}
					fmt.Fprintln(cmd.ErrOrStderr(), timeoutErr.Error())
					return apiErr(timeoutErr)
				}
				data, err := c.GetNoCache(ctx, path, map[string]string{"limit": "10"})
				if err != nil {
					if ctxErr := ctx.Err(); ctxErr != nil {
						return nil
					}
					return classifyAPIError(err, flags)
				}
				var page struct {
					Messages []struct {
						ID           string            `json:"id"`
						Role         string            `json:"role"`
						FinishReason string            `json:"finishReason"`
						Content      string            `json:"content"`
						Parts        []json.RawMessage `json:"parts"`
					} `json:"messages"`
				}
				if err := json.Unmarshal(data, &page); err != nil {
					return fmt.Errorf("parsing messages page: %w", err)
				}
				polled++
				var view *v0TailMessageView
				for i := range page.Messages {
					m := &page.Messages[i]
					if m.Role != "assistant" {
						continue
					}
					v := &v0TailMessageView{
						ChatID:       chatID,
						MessageID:    m.ID,
						Role:         m.Role,
						Status:       "running",
						FinishReason: m.FinishReason,
						Content:      m.Content,
						Polled:       polled,
					}
					if m.FinishReason != "" && m.FinishReason != "null" {
						v.Status = "finished"
					}
					for _, p := range m.Parts {
						var part struct {
							Type string `json:"type"`
						}
						if json.Unmarshal(p, &part) == nil && part.Type != "" {
							v.PartTypes = append(v.PartTypes, part.Type)
						}
					}
					view = v
					break
				}
				if view != nil {
					// Deduplicate: with --follow, an unchanged finished (or
					// still-running) newest message must not be re-rendered on
					// every poll. Emit only when the observed message or its
					// terminal state actually changed.
					if view.MessageID == lastEmittedID && view.Status == lastEmittedStatus {
						if view.Status == "finished" && !flagFollow {
							return nil
						}
						select {
						case <-ctx.Done():
							return nil
						case <-time.After(interval):
						}
						continue
					}
					lastEmittedID = view.MessageID
					lastEmittedStatus = view.Status
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						_ = printJSONFiltered(cmd.OutOrStdout(), *view, flags)
					} else {
						status := view.Status
						if view.FinishReason != "" && view.FinishReason != "null" {
							status = "finished (" + view.FinishReason + ")"
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s  [%s]  parts=%v\n", view.MessageID, status, view.PartTypes)
					}
					if view.Status == "finished" && !flagFollow {
						return nil
					}
				} else if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no assistant message yet (poll %d)\n", polled)
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().StringVar(&flagInterval, "interval", "3s", "Poll interval between checks")
	cmd.Flags().StringVar(&flagTimeout, "timeout", "30m", "Give up after this long and exit 5 (generation still unfinished)")
	cmd.Flags().BoolVar(&flagFollow, "follow", false, "Keep watching after a generation finishes")
	return cmd
}
