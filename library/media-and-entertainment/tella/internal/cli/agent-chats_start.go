// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"strings"
	"unicode/utf8"
)

func newAgentChatsStartCmd(flags *rootFlags) *cobra.Command {
	var prompt, video, name string
	var newVideo, stdin bool
	cmd := &cobra.Command{Use: "start", Aliases: []string{"create"}, Short: "Start a paid editor chat after checking credits and explicit --yes", Args: cobra.NoArgs, Annotations: map[string]string{"pp:workflow-endpoint": "agent-chats.start", "mcp:read-only": "false", "pp:data-source": "live"}, Example: "  tella-pp-cli agent-chats start --video-id vid_example --prompt 'Review the layout' --yes"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if !hasChangedLocalFlags(cmd) && !flags.dryRun {
			return cmd.Help()
		}
		if stdin {
			var b struct {
				Prompt  string `json:"prompt"`
				VideoID string `json:"videoId"`
				Name    string `json:"name"`
			}
			d := json.NewDecoder(io.LimitReader(cmd.InOrStdin(), 1<<20))
			d.DisallowUnknownFields()
			if err := d.Decode(&b); err != nil {
				return usageErr(err)
			}
			var extra any
			if err := d.Decode(&extra); err != io.EOF {
				return usageErr(fmt.Errorf("expected exactly one JSON body"))
			}
			prompt, video, name = b.Prompt, b.VideoID, b.Name
		}
		if workflowOffline(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "agent-chats start")
		}
		if strings.TrimSpace(prompt) == "" || utf8.RuneCountInString(prompt) > 20000 {
			return usageErr(fmt.Errorf("prompt must contain 1–20000 characters"))
		}
		if (video == "") == !newVideo {
			return usageErr(fmt.Errorf("choose --video-id or explicit --new-video"))
		}
		if video != "" {
			if err := workflowArgs(1)(cmd, []string{video}); err != nil {
				return usageErr(err)
			}
			if name != "" {
				return usageErr(fmt.Errorf("--name only applies to --new-video"))
			}
		}
		if utf8.RuneCountInString(name) > 120 {
			return usageErr(fmt.Errorf("name exceeds 120 characters"))
		}
		if !flags.dryRun {
			if requiredInputWaived() {
				return usageErr(fmt.Errorf("agent chats are disabled under PRINTING_PRESS_VERIFY/DOGFOOD"))
			}
			if !cmd.Flags().Changed("yes") || !flags.yes {
				return usageErr(fmt.Errorf("paid agent chat requires explicit --yes; --agent does not grant consent"))
			}
		}
		c, err := workflowClient(flags)
		if err != nil {
			return err
		}
		if video != "" {
			if _, err = c.Get(cmd.Context(), videoPath(video), nil); err != nil {
				return err
			}
		}
		before, err := c.Get(cmd.Context(), "/v1/account", nil)
		if err != nil {
			return err
		}
		balance, err := accountBalance(before)
		if err != nil {
			return err
		}
		body := map[string]any{"prompt": prompt}
		if video != "" {
			body["videoId"] = video
		}
		if name != "" {
			body["name"] = name
		}
		if flags.dryRun {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "credits_before": balance, "body": body, "explicit_yes_required": true}, flags)
		}
		if balance <= 0 {
			return fmt.Errorf("no agent credits available (balance %g)", balance)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Agent credits before: %g\n", balance)
		data, _, postErr := c.Post(cmd.Context(), "/v1/agent-chats", body)
		after, afterErr := c.Get(cmd.Context(), "/v1/account", nil)
		report := map[string]any{"credits_before": balance, "chat": jsonRawToAny(data)}
		if afterErr == nil {
			var n float64
			n, afterErr = accountBalance(after)
			if afterErr == nil {
				report["credits_after"] = n
				fmt.Fprintf(cmd.ErrOrStderr(), "Agent credits after: %g\n", n)
			}
		}
		if err = printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
			return err
		}
		if postErr != nil {
			return fmt.Errorf("chat outcome may be unknown; inspect agent-chats list before retrying: %w", postErr)
		}
		if afterErr != nil {
			return fmt.Errorf("chat started; after-credit check failed: %w", afterErr)
		}
		return nil
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "Instructions for the agent (no generated publishing prompt)")
	cmd.Flags().StringVar(&video, "video-id", "", "Existing video to edit")
	cmd.Flags().StringVar(&name, "name", "", "Name for an intentionally created video")
	cmd.Flags().BoolVar(&newVideo, "new-video", false, "Intentionally create a new video instead of targeting an existing video")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read prompt/videoId/name JSON; consent still requires --yes")
	return cmd
}
func accountBalance(raw json.RawMessage) (float64, error) {
	var r struct {
		Credits *struct {
			Balance *float64 `json:"balance"`
		} `json:"credits"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return 0, err
	}
	if r.Credits == nil || r.Credits.Balance == nil {
		return 0, fmt.Errorf("account credits balance unavailable")
	}
	return *r.Credits.Balance, nil
}
