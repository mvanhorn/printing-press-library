package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
	"github.com/spf13/cobra"
)

func articleIDFlag(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", usageErr(fmt.Errorf("--id is required"))
	}
	return id, nil
}

func articleDeleteRequestBody(articleID string) (map[string]any, error) {
	articleID, err := articleIDFlag(articleID)
	if err != nil {
		return nil, err
	}
	return articleOpRequestBody("ArticleEntityDelete", map[string]any{
		"articleEntityId": articleID,
	}), nil
}

func articleUpdateTitleRequestBody(articleID string, title string) (map[string]any, error) {
	articleID, err := articleIDFlag(articleID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(title) == "" {
		return nil, usageErr(fmt.Errorf("--title is required"))
	}
	return articleOpRequestBody("ArticleEntityUpdateTitle", map[string]any{
		"articleEntityId": articleID,
		"title":           title,
	}), nil
}

func articleUpdateCoverRequestBody(articleID string, mediaID string) (map[string]any, error) {
	articleID, err := articleIDFlag(articleID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mediaID) == "" {
		return nil, usageErr(fmt.Errorf("--media-id is required unless --cover is supplied"))
	}
	return articleOpRequestBody("ArticleEntityUpdateCoverMedia", map[string]any{
		"articleEntityId": articleID,
		"coverMedia": map[string]any{
			"media_id":       strings.TrimSpace(mediaID),
			"media_category": "DraftTweetImage",
		},
	}), nil
}

func articleUpdateContentRequestBody(articleID string, contentState draftContentState) (map[string]any, error) {
	articleID, err := articleIDFlag(articleID)
	if err != nil {
		return nil, err
	}
	return articleOpRequestBody("ArticleEntityUpdateContent", map[string]any{
		"content_state":  articleContentStateRequest(contentState),
		"article_entity": articleID,
	}), nil
}

func readArticleContentStateFile(path string) (draftContentState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return draftContentState{}, fmt.Errorf("read %s: %w", path, err)
	}
	return parseArticleContentStateJSON(data)
}

func parseArticleContentStateJSON(data []byte) (draftContentState, error) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(data, &outer); err == nil {
		if nested, ok := outer["content_state"]; ok {
			data = nested
		}
	}
	var raw struct {
		Blocks         []draftBlock  `json:"blocks"`
		EntityMap      []draftEntity `json:"entityMap"`
		EntityMapSnake []draftEntity `json:"entity_map"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return draftContentState{}, fmt.Errorf("parse content_state JSON: %w", err)
	}
	entityMap := raw.EntityMap
	if entityMap == nil {
		entityMap = raw.EntityMapSnake
	}
	return draftContentState{
		Blocks:    raw.Blocks,
		EntityMap: entityMap,
	}, nil
}

func printArticleMutationResponse(cmd *cobra.Command, flags *rootFlags, path string, data json.RawMessage, statusCode int) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var items []map[string]any
		if json.Unmarshal(data, &items) == nil && len(items) > 0 {
			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
			} else {
				return nil
			}
		} else {
			var wrapped struct {
				Data []map[string]any `json:"data"`
			}
			if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
				if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
					fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
				} else {
					return nil
				}
			}
		}
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		if flags.quiet {
			return nil
		}
		filtered := data
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		} else if flags.compact {
			filtered = compactFields(filtered)
		}
		envelope := map[string]any{
			"action":   "post",
			"resource": "articles",
			"path":     path,
			"status":   statusCode,
			"success":  statusCode >= 200 && statusCode < 300,
		}
		if flags.dryRun {
			envelope["dry_run"] = true
			envelope["status"] = 0
			envelope["success"] = false
		}
		if len(filtered) > 0 {
			var parsed any
			if err := json.Unmarshal(filtered, &parsed); err == nil {
				envelope["data"] = parsed
			}
		}
		envelopeJSON, err := json.Marshal(envelope)
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func printArticleMutationDryRun(cmd *cobra.Command, flags *rootFlags, path string, body map[string]any, extra map[string]any) error {
	selectedProfile := flags.profileName
	if selectedProfile == "" {
		selectedProfile = "default"
	}
	previewValue := map[string]any{
		"dry_run": true,
		"meta": map[string]any{
			"auth_lane":        "x_articles_cookie",
			"selected_profile": selectedProfile,
		},
		"mutation": true,
		"request": map[string]any{
			"body":   body,
			"method": "POST",
			"path":   path,
		},
		"sent": false,
	}
	for key, value := range extra {
		previewValue[key] = value
	}
	preview, err := json.Marshal(previewValue)
	if err != nil {
		return err
	}
	return printArticleMutationResponse(cmd, flags, path, preview, 0)
}

func runArticleGraphQLMutation(cmd *cobra.Command, flags *rootFlags, opName string, body map[string]any) error {
	path := client.ArticleOpURL(opName)
	if flags.dryRun {
		return printArticleMutationDryRun(cmd, flags, path, body, nil)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, statusCode, err := c.Post(cmd.Context(), path, body)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return printArticleMutationResponse(cmd, flags, path, data, statusCode)
}
