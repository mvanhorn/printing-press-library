// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/wpupload"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		mediaCmd, _, err := root.Find([]string{"media"})
		if err == nil && mediaCmd != nil {
			mediaCmd.AddCommand(newMediaUploadCmd(flags))
		}
	})
}

func newMediaUploadCmd(flags *rootFlags) *cobra.Command {
	var title string
	var altText string
	var caption string
	var description string
	var post int

	cmd := &cobra.Command{
		Use:     "upload <file>",
		Short:   "Upload a local file to the WordPress media library",
		Example: "  wordpress-pp-cli media upload ./hero.jpg --title \"Homepage hero\" --alt-text \"Team at work\"\n  wordpress-pp-cli media upload ./report.pdf --post 42 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				file := "the supplied file"
				if len(args) > 0 {
					file = args[0]
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "media_upload", "file": file, "dry_run": true}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would upload %s to the WordPress media library.\n", file)
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("upload file is required"))
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("media upload accepts exactly one file"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			generatedClient, err := flags.newClient()
			if err != nil {
				return err
			}
			uploader := wpupload.New(generatedClient.Config, generatedClient.HTTPClient)
			data, statusCode, err := uploader.UploadFile(ctx, args[0])
			if err != nil {
				return classifyWordPressUploadError(err)
			}

			var uploaded struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(data, &uploaded); err != nil || uploaded.ID <= 0 {
				return apiErr(fmt.Errorf("WordPress upload succeeded but returned no valid media id"))
			}

			metadata := make(map[string]any)
			if cmd.Flags().Changed("title") {
				metadata["title"] = title
			}
			if cmd.Flags().Changed("alt-text") {
				metadata["alt_text"] = altText
			}
			if cmd.Flags().Changed("caption") {
				metadata["caption"] = caption
			}
			if cmd.Flags().Changed("description") {
				metadata["description"] = description
			}
			if cmd.Flags().Changed("post") {
				metadata["post"] = post
			}
			if len(metadata) > 0 {
				path := "/wp/v2/media/" + strconv.Itoa(uploaded.ID)
				updated, updatedStatus, updateErr := generatedClient.Post(ctx, path, metadata)
				if updateErr != nil {
					return classifyAPIError(updateErr, flags)
				}
				data = updated
				statusCode = updatedStatus
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), data, flags)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				return apiErr(fmt.Errorf("decoding WordPress media response: %w", err))
			}
			result["http_status"] = statusCode
			return printAutoTable(cmd.OutOrStdout(), []map[string]any{result})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Media title")
	cmd.Flags().StringVar(&altText, "alt-text", "", "Alternative text for the media item")
	cmd.Flags().StringVar(&caption, "caption", "", "Media caption")
	cmd.Flags().StringVar(&description, "description", "", "Media description")
	cmd.Flags().IntVar(&post, "post", 0, "ID of the post this media item belongs to")
	return cmd
}

func classifyWordPressUploadError(err error) error {
	if errors.Is(err, wpupload.ErrEmptyFile) {
		return usageErr(fmt.Errorf("the file was empty"))
	}
	var uploadErr *wpupload.APIError
	if !errors.As(err, &uploadErr) {
		return apiErr(err)
	}
	switch uploadErr.Code {
	case "rest_upload_no_data":
		return usageErr(fmt.Errorf("the file was empty"))
	case "rest_upload_no_content_disposition", "rest_upload_invalid_disposition":
		return apiErr(fmt.Errorf("internal upload header bug (%s); report this to the wordpress-pp-cli maintainers", uploadErr.Code))
	case "rest_upload_hash_mismatch":
		return apiErr(fmt.Errorf("upload corrupted in transit; retry the upload"))
	case "rest_upload_file_too_big", "rest_upload_limited_space", "rest_upload_user_quota_exceeded":
		return apiErr(errors.New(uploadErr.Message))
	case "rest_cannot_create":
		return authErr(fmt.Errorf("the configured credential lacks the upload_files capability"))
	}
	switch uploadErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return authErr(uploadErr)
	case http.StatusNotFound:
		return notFoundErr(uploadErr)
	default:
		return apiErr(uploadErr)
	}
}
