// Copyright 2026 jason-holt. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drivethrurpg/internal/client"
	"github.com/spf13/cobra"
)

type preparedDownload struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	var fileIndex int
	var outputDir string
	var outputName string
	var siteID string
	var getChecksums int
	var pollInterval time.Duration
	var maxWait time.Duration
	var urlOnly bool
	var force bool

	cmd := &cobra.Command{
		Use:   "download <libraryProductId> [fileIndex]",
		Short: "Download a purchased DriveThruRPG file",
		Long: `Download a file from your authenticated DriveThruRPG library.

Authentication uses DRIVETHRURPG_APPLICATION_KEY, a saved token from
'auth login', or DRIVETHRURPG_DTRPG_TOKEN.`,
		Example: `  drivethrurpg-pp-cli library --page-size 10
  drivethrurpg-pp-cli download 123456 --index 0 --output-dir ~/Downloads
  drivethrurpg-pp-cli download 123456 0 --url-only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return usageErr(fmt.Errorf("expected at most 2 arguments, got %d", len(args)))
			}
			orderProductID := "LIBRARY_PRODUCT_ID"
			if len(args) == 0 {
				if !flags.dryRun {
					return cmd.Help()
				}
			} else {
				orderProductID = args[0]
			}
			index := fileIndex
			if len(args) == 2 {
				parsed, err := strconv.Atoi(args[1])
				if err != nil {
					if flags.dryRun {
						parsed = fileIndex
					} else {
						return fmt.Errorf("invalid fileIndex %q: %w", args[1], err)
					}
				}
				index = parsed
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"siteId":       siteID,
				"index":        strconv.Itoa(index),
				"getChecksums": strconv.Itoa(getChecksums),
			}
			preparePath := "/order_products/" + url.PathEscape(orderProductID) + "/prepare"
			if flags.dryRun {
				data, getErr := c.Get(preparePath, params)
				if getErr != nil {
					return classifyAPIError(getErr, flags)
				}
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			status, err := waitForDownload(cmd.Context(), c, orderProductID, params, pollInterval, maxWait)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status.URL == "" {
				return fmt.Errorf("download was not ready: status=%q and no URL returned", status.Status)
			}

			if urlOnly {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"library_product_id": orderProductID,
						"index":              index,
						"status":             status.Status,
						"url":                status.URL,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), status.URL)
				return nil
			}

			path, bytesWritten, err := downloadPreparedURL(cmd.Context(), c.HTTPClient, status.URL, outputDir, outputName, force)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"downloaded":         true,
					"library_product_id": orderProductID,
					"index":              index,
					"status":             status.Status,
					"path":               path,
					"bytes":              bytesWritten,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %d bytes to %s\n", bytesWritten, path)
			return nil
		},
	}
	cmd.Flags().IntVar(&fileIndex, "index", 0, "File index from the library product's files array")
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Directory to write the downloaded file")
	cmd.Flags().StringVar(&outputName, "filename", "", "Override the output filename")
	cmd.Flags().StringVar(&siteID, "site-id", "10", "DriveThruRPG site id. The public Library App uses 10.")
	cmd.Flags().IntVar(&getChecksums, "get-checksums", 0, "Request checksums while preparing the download")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 2*time.Second, "Delay between prepare/check polls")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 2*time.Minute, "Maximum time to wait for DriveThruRPG to prepare the file")
	cmd.Flags().BoolVar(&urlOnly, "url-only", false, "Print the prepared download URL without downloading it")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing output file")
	return cmd
}

func waitForDownload(ctx context.Context, c *client.Client, orderProductID string, params map[string]string, pollInterval, maxWait time.Duration) (*preparedDownload, error) {
	start := time.Now()
	pathBase := "/order_products/" + url.PathEscape(orderProductID)
	status, err := fetchDownloadStatus(c, pathBase+"/prepare", params)
	if err != nil {
		return nil, err
	}

	for strings.HasPrefix(strings.ToLower(status.Status), "preparing") || status.URL == "" {
		if status.URL != "" {
			break
		}
		if time.Since(start) > maxWait {
			return nil, fmt.Errorf("timed out waiting for download after %s; last status=%q", maxWait, status.Status)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
		status, err = fetchDownloadStatus(c, pathBase+"/check", params)
		if err != nil {
			return nil, err
		}
	}
	return status, nil
}

func fetchDownloadStatus(c *client.Client, requestPath string, params map[string]string) (*preparedDownload, error) {
	data, err := c.Get(requestPath, params)
	if err != nil {
		return nil, err
	}
	var status preparedDownload
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parsing download status: %w", err)
	}
	if status.URL == "" && status.Status == "" {
		var wrapped struct {
			Data struct {
				Attributes preparedDownload `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &wrapped); err != nil {
			return nil, fmt.Errorf("parsing wrapped download status: %w", err)
		}
		status = wrapped.Data.Attributes
	}
	return &status, nil
}

func downloadPreparedURL(ctx context.Context, httpClient *http.Client, rawURL, outputDir, outputName string, force bool) (string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("creating download request: %w", err)
	}
	req.Header.Set("User-Agent", "drivethrurpg-pp-cli/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("downloading file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return "", 0, fmt.Errorf("download URL returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	name := outputName
	if name == "" {
		name = filenameFromResponse(resp.Header.Get("Content-Disposition"), rawURL)
	}
	name = safeFilename(name)
	if name == "" {
		name = "drivethrurpg-download"
	}
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("creating output directory: %w", err)
	}

	target := filepath.Join(outputDir, name)
	if !force {
		if _, err := os.Stat(target); err == nil {
			return "", 0, fmt.Errorf("output file already exists: %s (pass --force to overwrite)", target)
		} else if !os.IsNotExist(err) {
			return "", 0, fmt.Errorf("checking output file: %w", err)
		}
	}

	tmp, err := os.CreateTemp(outputDir, ".drivethrurpg-*")
	if err != nil {
		return "", 0, fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	written, copyErr := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return "", written, fmt.Errorf("writing download: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", written, fmt.Errorf("closing download file: %w", closeErr)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return "", written, fmt.Errorf("moving download into place: %w", err)
	}
	return target, written, nil
}

func filenameFromResponse(contentDisposition, rawURL string) string {
	if contentDisposition != "" {
		if _, params, err := mime.ParseMediaType(contentDisposition); err == nil {
			if name := params["filename"]; name != "" {
				return name
			}
			if name := params["filename*"]; name != "" {
				return name
			}
		}
	}
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if base := path.Base(parsed.Path); base != "." && base != "/" {
			return base
		}
	}
	return ""
}

func safeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimSpace(name)
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}
