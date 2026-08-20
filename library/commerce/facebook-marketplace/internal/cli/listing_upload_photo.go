package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/facebook-marketplace/internal/client"
	"github.com/spf13/cobra"
)

func newListingUploadPhotoCmd(flags *rootFlags) *cobra.Command {
	var photoPath string
	var uploadTargetID string
	var uploadID int

	cmd := &cobra.Command{
		Use:   "upload-photo",
		Short: "Upload a local photo for a Marketplace listing draft.",
		Example: "  facebook-marketplace-pp-cli listing upload-photo --photo chair.jpg --write --json\n" +
			"  facebook-marketplace-pp-cli listing create --photo chair.jpg --variables '{...}' --write",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if photoPath == "" {
				return usageErr(fmt.Errorf("--photo is required"))
			}
			writeKey, err := requireWriteCheckpoint(flags)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				_ = recordWriteState(writeKey, "failed", err.Error())
				return err
			}
			upload, statusCode, err := c.UploadMarketplacePhoto(photoPath, uploadTargetID, uploadID)
			if err != nil {
				_ = recordWriteState(writeKey, "unknown_outcome", err.Error())
				return classifyAPIError(err, flags)
			}
			_ = recordWriteState(writeKey, "submitted", "photo upload submitted")
			envelope := map[string]any{
				"action":   "upload-photo",
				"resource": "listing",
				"path":     "/ajax/react_composer/attachments/photo/upload",
				"status":   statusCode,
				"success":  statusCode >= 200 && statusCode < 300,
				"data": map[string]any{
					"photo_id": upload.PhotoID,
					"width":    upload.Width,
					"height":   upload.Height,
				},
			}
			data, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			if flags.quiet {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), upload.PhotoID)
				return err
			}
			return printOutput(cmd.OutOrStdout(), json.RawMessage(data), true)
		},
	}
	cmd.Flags().StringVar(&photoPath, "photo", "", "Local photo path to upload.")
	cmd.Flags().StringVar(&uploadTargetID, "upload-target-id", client.DefaultMarketplaceUploadTargetID, "Marketplace composer upload target id captured from the photo-upload HAR.")
	cmd.Flags().IntVar(&uploadID, "upload-id", 1024, "upload_id value for Marketplace photo upload.")
	return cmd
}
