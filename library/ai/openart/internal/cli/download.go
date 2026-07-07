package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/cliutil"
)

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	var (
		outPath string
		quiet   bool
	)
	cmd := &cobra.Command{
		Use:   "download <resourceId>",
		Short: "Download a completed generation's MP4/PNG/audio to local disk",
		Long: `Stream the CDN URL of a finished OpenArt resource to a local file.

If --output is a directory, the file is named <resourceId>.<ext>.
If --output is omitted entirely, the file is named ./<resourceId>.<ext>
in the current directory.`,
		Example: `  # Save the MP4 to ./out/<resourceId>.mp4
  openart-pp-cli download 3dVHEhDjyq82gLwBudaG --output ./out/

  # Save to an explicit filename
  openart-pp-cli download 3dVHEhDjyq82gLwBudaG --output ./hero.mp4`,
		Annotations: map[string]string{
			// Writes to local filesystem; not annotated as read-only.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			resourceID := args[0]

			if cliutil.IsVerifyEnv() {
				out := map[string]any{
					"would_download": true,
					"resource_id":    resourceID,
					"output":         outPath,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if flags.dryRun {
				out := map[string]any{
					"action":      "download",
					"resource_id": resourceID,
					"endpoint":    "/resources/" + resourceID,
					"output":      outPath,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			snap, err := pollResource(c, resourceID)
			if err != nil {
				return err
			}
			if snap.URL == "" {
				return fmt.Errorf("resource %s has no URL (status=%s); generation may still be in progress", resourceID, snap.Status)
			}

			ext := extForResource(snap.URL)
			dst := outPath
			if dst == "" {
				dst = resourceID + ext
			} else if isDir(dst) {
				dst = filepath.Join(dst, resourceID+ext)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := streamToFile(ctx, snap.URL, dst); err != nil {
				return fmt.Errorf("download: %w", err)
			}
			result := map[string]any{
				"resource_id": resourceID,
				"status":      snap.Status,
				"url":         snap.URL,
				"path":        dst,
			}
			if !quiet {
				fmt.Fprintf(cmd.ErrOrStderr(), "downloaded %s -> %s\n", resourceID, dst)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "Output file or directory")
	cmd.Flags().BoolVarP(&quiet, "no-progress", "Q", false, "Suppress the stderr progress line")
	return cmd
}

func extForResource(u string) string {
	for _, ext := range []string{".mp4", ".webm", ".mov", ".png", ".jpg", ".jpeg", ".webp", ".mp3", ".wav"} {
		if hasSuffixCI(u, ext) {
			return ext
		}
	}
	return ".bin"
}

func hasSuffixCI(s, suf string) bool {
	if len(s) < len(suf) {
		return false
	}
	tail := s[len(s)-len(suf):]
	for i := 0; i < len(tail); i++ {
		a, b := tail[i], suf[i]
		if a >= 'A' && a <= 'Z' {
			a += 32
		}
		if b >= 'A' && b <= 'Z' {
			b += 32
		}
		if a != b {
			return false
		}
	}
	return true
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		// Treat trailing-slash paths as directory intent.
		return len(p) > 0 && (p[len(p)-1] == '/' || p[len(p)-1] == os.PathSeparator)
	}
	return info.IsDir()
}

// downloadCmdJSON is a small helper for tests that want the JSON output of
// download.
func downloadCmdJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
