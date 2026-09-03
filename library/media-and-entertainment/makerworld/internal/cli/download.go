// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored absorbed command: resolve a design to its default print instance
// and download the 3MF file. Requires a Bambu Cloud JWT in MAKERWORLD_TOKEN.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source live

const downloadUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

func newNovelDownloadCmd(flags *rootFlags) *cobra.Command {
	var flagInstance string
	var flagOutput string

	cmd := &cobra.Command{
		Use:   "download <design_id>",
		Short: "Download a design's 3MF print file (requires MAKERWORLD_TOKEN)",
		Long: "Resolves a design to its default print instance (or use --instance) and downloads " +
			"the 3MF file. Requires a Bambu Cloud JWT in MAKERWORLD_TOKEN; MakerWorld blocks " +
			"anonymous downloads. The token is sent only to api.bambulab.com, never to the CDN.",
		Example:     strings.Trim("\n  makerworld-pp-cli download 2865269\n  makerworld-pp-cli download 2865269 -o dragon-egg.3mf", "\n"),
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would download the design's 3MF file")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a design ID is required: makerworld-pp-cli download <design_id>"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("download fetches from the live API and has no local mode"))
			}
			designID := strings.TrimSpace(args[0])
			tok, ok := requireToken(cmd, flags)
			if !ok {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			instanceID := strings.TrimSpace(flagInstance)
			if instanceID == "" {
				data, derr := c.GetWithHeaders(ctx, "/design-service/design/"+designID, nil,
					map[string]string{"Authorization": "Bearer " + tok})
				if derr != nil {
					return classifyAPIError(derr, flags)
				}
				var d struct {
					DefaultInstanceID json.Number `json:"defaultInstanceId"`
					Instances         []struct {
						ID        json.Number `json:"id"`
						IsDefault bool        `json:"isDefault"`
					} `json:"instances"`
				}
				if json.Unmarshal(data, &d) != nil {
					return fmt.Errorf("could not parse design %s", designID)
				}
				if s := d.DefaultInstanceID.String(); s != "" && s != "0" {
					instanceID = s
				} else {
					for _, in := range d.Instances {
						if in.IsDefault {
							instanceID = in.ID.String()
							break
						}
					}
					if instanceID == "" && len(d.Instances) > 0 {
						instanceID = d.Instances[0].ID.String()
					}
				}
				if instanceID == "" || instanceID == "0" {
					return fmt.Errorf("could not resolve a printable instance for design %s; pass --instance", designID)
				}
			}

			outPath := strings.TrimSpace(flagOutput)
			if outPath == "" {
				outPath = designID + ".3mf"
			}
			fileURL := strings.TrimRight(c.RequestBaseURL(), "/") + "/design-service/instance/" + instanceID + "/f3mf?type=download"

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would download instance %s of design %s to %s\n", instanceID, designID, outPath)
				return nil
			}

			n, err := downloadF3MF(ctx, fileURL, tok, outPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "downloaded %d bytes to %s (design %s, instance %s)\n", n, outPath, designID, instanceID)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagInstance, "instance", "", "Instance ID to download (default: the design's default plate)")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path (default: <design_id>.3mf)")
	return cmd
}

// downloadF3MF streams the 3MF to disk. Go's http client strips the Authorization
// header on the cross-host redirect to the presigned CDN URL, matching the
// upstream contract that the token never reaches the CDN.
func downloadF3MF(ctx context.Context, url, tok, outPath string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("User-Agent", downloadUA)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("download failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// #nosec G304 -- outPath is the user's own --output flag (or a default in the
	// current directory); writing the download to a caller-chosen path under the
	// caller's own permissions is the command's purpose, not a traversal sink.
	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		_ = os.Remove(outPath)
		return 0, fmt.Errorf("writing %s: %w", outPath, err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		_ = os.Remove(outPath)
		return 0, fmt.Errorf("incomplete download: wrote %d of %d bytes", n, resp.ContentLength)
	}
	return n, nil
}
