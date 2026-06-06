package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
)

func newArticlesSetCoverCmd(flags *rootFlags) *cobra.Command {
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
			body := map[string]any{
				"variables": map[string]any{
					"articleEntityId": args[0],
					"coverMedia": map[string]any{
						"media_id":       mediaID,
						"media_category": "DraftTweetImage",
					},
				},
				"features": articleGraphQLFeatures(),
				"queryId":  "Es8InPh7mEkK9PxclxFAVQ",
			}
			data, _, err := c.Post(cmd.Context(), client.ArticleOpURL("ArticleEntityUpdateCoverMedia"), body)
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
	return cmd
}
