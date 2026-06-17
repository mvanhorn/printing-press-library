package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
)

func newArticlesSetCoverCmd(flags *rootFlags) *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:     "set-cover <article-id> <image-file>",
		Short:   "Upload an image and set it as an X Article cover",
		Example: "  x-twitter-pp-cli articles set-cover 1750000000000000000 ./cover.jpg",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN: would upload %s and set it as cover for article %s\n", args[1], args[0])
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			mediaID, err := c.UploadArticleImage(cmd.Context(), args[1])
			if err != nil {
				return err
			}
			// PATCH: resolve the queryId through the article-ops table instead
			// of a hardcoded literal that drifts when X redeploys.
			body := articleOpRequestBody("ArticleEntityUpdateCoverMedia", map[string]any{
				"articleEntityId": args[0],
				"coverMedia": map[string]any{
					"media_id":       mediaID,
					"media_category": "DraftTweetImage",
				},
			})
			titleResolver := func() (string, error) {
				if strings.TrimSpace(title) != "" {
					return title, nil
				}
				return existingArticleTitle(cmd.Context(), c, args[0])
			}
			data, _, err := postCoverMediaWithMetadataHeal(cmd.Context(), c, args[0], body, titleResolver)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{
				"article_id": args[0],
				"media_id":   mediaID,
				"response":   json.RawMessage(data),
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Article title to use if X needs ArticleMetadata before setting the cover; defaults to the existing title")
	return cmd
}

func postCoverMediaWithMetadataHeal(ctx context.Context, p articleGraphQLPoster, articleID string, body map[string]any, titleResolver func() (string, error)) (json.RawMessage, int, error) {
	path := client.ArticleOpURL("ArticleEntityUpdateCoverMedia")
	data, statusCode, err := p.Post(ctx, path, body)
	if err == nil || !isMissingArticleMetadataError(err) {
		return data, statusCode, err
	}
	title, titleErr := titleResolver()
	if titleErr != nil {
		return nil, statusCode, fmt.Errorf("cover update hit Missing ArticleMetadata (code 214), and the existing title could not be resolved; pass --title to self-heal manually: %w", titleErr)
	}
	if strings.TrimSpace(title) == "" {
		return nil, statusCode, fmt.Errorf("cover update hit Missing ArticleMetadata (code 214), and the article title is empty; pass --title to self-heal manually")
	}
	if err := updateArticleEntityTitle(ctx, p, articleID, title); err != nil {
		return nil, statusCode, fmt.Errorf("cover update hit Missing ArticleMetadata (code 214), but self-heal update-title failed: %w", err)
	}
	return p.Post(ctx, path, body)
}

func isMissingArticleMetadataError(err error) bool {
	var apiErr *client.APIError
	body := err.Error()
	if errors.As(err, &apiErr) {
		body = apiErr.Body
	}
	body = strings.ToLower(body)
	return strings.Contains(body, "articlemetadata") && strings.Contains(body, "214")
}

func existingArticleTitle(ctx context.Context, c *client.Client, articleID string) (string, error) {
	for _, lifecycle := range []string{"Draft", "Published"} {
		raw, err := fetchArticleSlice(ctx, c, lifecycle)
		if err != nil {
			return "", err
		}
		if title, found := articleTitleFromSlice(raw, articleID); found {
			return title, nil
		}
	}
	return "", fmt.Errorf("article %s not found in your Draft or Published articles", articleID)
}

func articleTitleFromSlice(raw json.RawMessage, articleID string) (string, bool) {
	var response struct {
		Data struct {
			User struct {
				Result struct {
					Slice struct {
						Items []struct {
							Results struct {
								Result map[string]any `json:"result"`
							} `json:"article_entity_results"`
						} `json:"items"`
					} `json:"articles_article_mixer_slice"`
				} `json:"result"`
			} `json:"user"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return "", false
	}
	for _, item := range response.Data.User.Result.Slice.Items {
		article := item.Results.Result
		if id, _ := article["rest_id"].(string); id == articleID {
			return articleTitleField(article), true
		}
	}
	return "", false
}

func articleTitleField(article map[string]any) string {
	if title, _ := article["title"].(string); strings.TrimSpace(title) != "" {
		return title
	}
	for _, value := range article {
		if nested, ok := value.(map[string]any); ok {
			if title := articleTitleField(nested); strings.TrimSpace(title) != "" {
				return title
			}
		}
	}
	return ""
}
