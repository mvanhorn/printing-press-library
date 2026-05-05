package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newUploadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload media bytes and create upload tokens.",
	}
	cmd.AddCommand(newUploadFileCmd(flags))
	return cmd
}

func newUploadFileCmd(flags *rootFlags) *cobra.Command {
	var mimeType string

	cmd := &cobra.Command{
		Use:   "file <path>",
		Short: "Upload raw photo or video bytes and print the upload token.",
		Example: "  google-photos-pp-cli upload file ./photo.jpg\n" +
			"  google-photos-pp-cli upload file ./clip.mov --mime-type video/quicktime --json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			if mimeType == "" {
				mimeType = detectUploadMIME(path, data)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			authHeader := ""
			if c.Config != nil {
				authHeader = c.Config.AuthHeader()
			}
			if authHeader == "" && !flags.dryRun {
				return configErr(fmt.Errorf("authentication required: run 'google-photos-pp-cli auth login' or set GOOGLE_PHOTOS_TOKEN"))
			}

			if flags.dryRun {
				preview := map[string]any{
					"method": "POST",
					"url":    "https://photoslibrary.googleapis.com/v1/uploads",
					"headers": map[string]string{
						"Authorization":              "Bearer <redacted>",
						"Content-Type":               "application/octet-stream",
						"X-Goog-Upload-Content-Type": mimeType,
						"X-Goog-Upload-Protocol":     "raw",
					},
					"body_bytes": len(data),
				}
				out, err := json.Marshal(preview)
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
			}

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, "https://photoslibrary.googleapis.com/v1/uploads", bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("creating upload request: %w", err)
			}
			req.Header.Set("Authorization", authHeader)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("X-Goog-Upload-Content-Type", mimeType)
			req.Header.Set("X-Goog-Upload-Protocol", "raw")
			req.Header.Set("User-Agent", "google-photos-pp-cli/1.0.0")

			resp, err := c.HTTPClient.Do(req)
			if err != nil {
				return fmt.Errorf("uploading %s: %w", path, err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading upload response: %w", err)
			}
			if resp.StatusCode >= 400 {
				return classifyAPIError(fmt.Errorf("POST /v1/uploads returned HTTP %d: %s", resp.StatusCode, string(body)))
			}

			token := strings.TrimSpace(string(body))
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				result := map[string]any{
					"uploadToken": token,
					"filename":    filepath.Base(path),
					"mimeType":    mimeType,
					"size":        len(data),
				}
				out, err := json.Marshal(result)
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
			}
			if flags.quiet {
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), token)
			return nil
		},
	}

	cmd.Flags().StringVar(&mimeType, "mime-type", "", "MIME type of the media bytes, for example image/jpeg")
	return cmd
}

func detectUploadMIME(path string, data []byte) string {
	if byExt := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); byExt != "" {
		return byExt
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "application/octet-stream"
}
