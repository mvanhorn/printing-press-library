// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// n8nRecipes maps a name to a ready-to-paste n8n workflow JSON snippet.
// Each recipe is a single n8n workflow with one or more nodes wired to
// the CLI via the Execute Command node, or to a Webhook trigger.
var n8nRecipes = map[string]struct {
	Title       string
	Description string
	JSON        string
}{
	"held-comment-mod": {
		Title:       "Daily held-comment moderation digest",
		Description: "Schedule Trigger (daily) → Execute Command (`youtube-creator-pp-cli mod queue --since 24h --json`) → Slack/Email node.",
		JSON: `{
  "name": "YT Held Comment Mod Digest",
  "nodes": [
    {"parameters": {"rule": {"interval": [{"field": "days"}]}}, "name": "Schedule", "type": "n8n-nodes-base.scheduleTrigger", "typeVersion": 1, "position": [240, 300]},
    {"parameters": {"command": "youtube-creator-pp-cli mod queue --since 24h --json"}, "name": "Held Queue", "type": "n8n-nodes-base.executeCommand", "typeVersion": 1, "position": [460, 300]},
    {"parameters": {"channel": "#yt-mod", "text": "={{$json.stdout}}"}, "name": "Slack", "type": "n8n-nodes-base.slack", "typeVersion": 2, "position": [680, 300]}
  ],
  "connections": {"Schedule": {"main": [[{"node": "Held Queue", "type": "main", "index": 0}]]}, "Held Queue": {"main": [[{"node": "Slack", "type": "main", "index": 0}]]}}
}`,
	},
	"daily-digest": {
		Title:       "Weekly analytics digest to email",
		Description: "Schedule Trigger (weekly Mondays) → Execute Command (`digest analytics --since 7d --markdown`) → Send Email.",
		JSON: `{
  "name": "YT Weekly Analytics Digest",
  "nodes": [
    {"parameters": {"rule": {"interval": [{"field": "weeks", "weekday": "monday"}]}}, "name": "Weekly", "type": "n8n-nodes-base.scheduleTrigger", "typeVersion": 1, "position": [240, 300]},
    {"parameters": {"command": "youtube-creator-pp-cli digest analytics --since 7d --markdown"}, "name": "Digest", "type": "n8n-nodes-base.executeCommand", "typeVersion": 1, "position": [460, 300]},
    {"parameters": {"to": "you@example.com", "subject": "YT Weekly Digest", "text": "={{$json.stdout}}"}, "name": "Email", "type": "n8n-nodes-base.emailSend", "typeVersion": 2, "position": [680, 300]}
  ],
  "connections": {"Weekly": {"main": [[{"node": "Digest", "type": "main", "index": 0}]]}, "Digest": {"main": [[{"node": "Email", "type": "main", "index": 0}]]}}
}`,
	},
	"upload-trigger": {
		Title:       "PubSubHubbub upload-trigger (push, not poll)",
		Description: "Webhook node receives the WebSub callback; CLI is used once to subscribe (in setup) and then on each ping to enrich.",
		JSON: `{
  "name": "YT Upload Trigger",
  "nodes": [
    {"parameters": {"httpMethod": "POST", "path": "yt-upload", "options": {"rawBody": true}}, "name": "Webhook", "type": "n8n-nodes-base.webhook", "typeVersion": 2, "position": [240, 300]},
    {"parameters": {"command": "echo \"{{$binary.data.toString('utf8')}}\" | grep -oP '<yt:videoId>\\K[^<]+'"}, "name": "Extract VideoID", "type": "n8n-nodes-base.executeCommand", "typeVersion": 1, "position": [460, 300]},
    {"parameters": {"command": "youtube-creator-pp-cli youtube videos-list --part snippet,statistics --id={{$json.stdout.trim()}} --json"}, "name": "Fetch Metadata", "type": "n8n-nodes-base.executeCommand", "typeVersion": 1, "position": [680, 300]}
  ],
  "connections": {"Webhook": {"main": [[{"node": "Extract VideoID", "type": "main", "index": 0}]]}, "Extract VideoID": {"main": [[{"node": "Fetch Metadata", "type": "main", "index": 0}]]}}
}`,
	},
	"bulk-metadata-monthly": {
		Title:       "Monthly bulk-metadata footer refresh",
		Description: "Schedule Trigger (monthly) → Execute Command (`bulk metadata --append-description ./footer.md --apply`).",
		JSON: `{
  "name": "YT Monthly Metadata Footer",
  "nodes": [
    {"parameters": {"rule": {"interval": [{"field": "months"}]}}, "name": "Monthly", "type": "n8n-nodes-base.scheduleTrigger", "typeVersion": 1, "position": [240, 300]},
    {"parameters": {"command": "youtube-creator-pp-cli bulk metadata --append-description /data/footer.md --apply --json"}, "name": "Bulk Update", "type": "n8n-nodes-base.executeCommand", "typeVersion": 1, "position": [460, 300]}
  ],
  "connections": {"Monthly": {"main": [[{"node": "Bulk Update", "type": "main", "index": 0}]]}}
}`,
	},
	"ab-thumbnail-loop": {
		Title:       "A/B thumbnail rotation loop",
		Description: "Schedule Trigger (every 24h) → Execute Command (`ab thumbnails rotate <video>`).",
		JSON: `{
  "name": "YT AB Thumbnail Rotate",
  "nodes": [
    {"parameters": {"rule": {"interval": [{"field": "hours", "hoursInterval": 24}]}}, "name": "24h", "type": "n8n-nodes-base.scheduleTrigger", "typeVersion": 1, "position": [240, 300]},
    {"parameters": {"command": "youtube-creator-pp-cli ab thumbnails rotate dQw4w9WgXcQ --json"}, "name": "Rotate", "type": "n8n-nodes-base.executeCommand", "typeVersion": 1, "position": [460, 300]}
  ],
  "connections": {"24h": {"main": [[{"node": "Rotate", "type": "main", "index": 0}]]}}
}`,
	},
	"backup-weekly": {
		Title:       "Weekly channel backup via yt-dlp",
		Description: "Schedule Trigger (weekly) → Execute Command (`backup --since 7d --captions --thumbnails --out /backup/`).",
		JSON: `{
  "name": "YT Weekly Backup",
  "nodes": [
    {"parameters": {"rule": {"interval": [{"field": "weeks"}]}}, "name": "Weekly", "type": "n8n-nodes-base.scheduleTrigger", "typeVersion": 1, "position": [240, 300]},
    {"parameters": {"command": "youtube-creator-pp-cli backup --since 7d --captions --thumbnails --info-json --out /backup/yt"}, "name": "Backup", "type": "n8n-nodes-base.executeCommand", "typeVersion": 1, "position": [460, 300]}
  ],
  "connections": {"Weekly": {"main": [[{"node": "Backup", "type": "main", "index": 0}]]}}
}`,
	},
}

func newRecipesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "recipes",
		Short:       "Print ready-to-paste workflow snippets (currently: n8n)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRecipesN8nCmd(flags))
	return cmd
}

func newRecipesN8nCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "n8n",
		Short:       "n8n workflow snippets you can paste directly into n8n's import dialog",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRecipesN8nListCmd(flags))
	cmd.AddCommand(newRecipesN8nPrintCmd(flags))
	return cmd
}

func newRecipesN8nListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List available n8n recipe names",
		Example:     "  youtube-creator-pp-cli recipes n8n list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			type row struct {
				Name        string `json:"name"`
				Title       string `json:"title"`
				Description string `json:"description"`
			}
			var rows []row
			for k, r := range n8nRecipes {
				rows = append(rows, row{Name: k, Title: r.Title, Description: r.Description})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
			return flags.printJSON(cmd, rows)
		},
	}
	return cmd
}

func newRecipesN8nPrintCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print [recipe-name]",
		Short: "Print the workflow JSON for a recipe",
		Example: "  youtube-creator-pp-cli recipes n8n print held-comment-mod\n" +
			"  youtube-creator-pp-cli recipes n8n print daily-digest > /tmp/digest.json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			r, ok := n8nRecipes[name]
			if !ok {
				var names []string
				for k := range n8nRecipes {
					names = append(names, k)
				}
				sort.Strings(names)
				return notFoundErr(fmt.Errorf("unknown recipe %q (try: %s)", name, strings.Join(names, ", ")))
			}
			// Print raw JSON to stdout
			fmt.Fprintln(cmd.OutOrStdout(), r.JSON)
			return nil
		},
	}
	return cmd
}
