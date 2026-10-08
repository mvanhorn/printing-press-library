// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/spf13/cobra"
)

type transferMetadata struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Path        string `json:"path_display"`
	Rev         string `json:"rev"`
	Size        int64  `json:"size"`
	ContentHash string `json:"content_hash,omitempty"`
}
type downloadResult struct {
	Path      string `json:"path"`
	Rev       string `json:"rev"`
	Size      int64  `json:"size"`
	WrittenTo string `json:"written_to"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, err := root.Find([]string{"files"})
		if err == nil && parent != nil {
			addNovelCommandIfAbsent(parent, newFilesDownloadCmd(flags))
		}
	})
}

func newFilesDownloadCmd(flags *rootFlags) *cobra.Command {
	var output string
	var force bool
	cmd := &cobra.Command{Use: "download <path>", Short: "Stream a Dropbox file to a local file or stdout", Long: "Download a Dropbox file using the content API. Specify --output with a local path or - for stdout.", Example: strings.Trim(`
  dropbox-pp-cli files download "/Documents/Taxes/2019.pdf" --output 2019.pdf --agent
	  dropbox-pp-cli files download "/Camera Uploads/photo.jpg" --output -`, "\n"), Annotations: map[string]string{"pp:data-source": "live", "pp:no-error-path-probe": "true", "mcp:write-flags": "output,force", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "files download")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("files download requires one Dropbox path"))
			}
			if err := liveSourceOnly(flags); err != nil {
				return err
			}
			remotePath := args[0]
			if !strings.HasPrefix(remotePath, "/") || remotePath == "/" {
				return usageErr(fmt.Errorf("Dropbox path must start with / and name a file"))
			}
			if output == "" {
				return usageErr(fmt.Errorf("files download requires --output <file> or --output -"))
			}
			if output != "-" && !force {
				if _, err := os.Stat(output); err == nil {
					return usageErr(fmt.Errorf("output %s already exists; use --force to overwrite", output))
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			ctx := cmd.Context()
			cancel := func() {}
			if cmd.Flags().Changed("timeout") {
				ctx, cancel = boundCtx(ctx, flags)
			}
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			info, err := fetchDropboxAccount(ctx, c)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			arg, err := dropbox.EncodeAPIArg(map[string]string{"path": remotePath})
			if err != nil {
				return err
			}
			resp, err := contentRequest(ctx, c, flags, http.MethodPost, "/files/download", nil, 0, map[string]string{"Dropbox-API-Arg": arg}, pathRootHeaders(info))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				return contentError(resp, "download")
			}
			var meta transferMetadata
			if err := json.Unmarshal([]byte(resp.Header.Get("Dropbox-API-Result")), &meta); err != nil {
				return fmt.Errorf("invalid Dropbox-API-Result: %w", err)
			}
			if output == "-" {
				_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
				return err
			}
			file, err := os.CreateTemp(filepath.Dir(output), ".dropbox-download-*")
			if err != nil {
				return err
			}
			tmp := file.Name()
			defer os.Remove(tmp)
			written, err := io.Copy(file, resp.Body)
			if err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			if force {
				if err := os.Rename(tmp, output); err != nil {
					return err
				}
			} else {
				if err := os.Link(tmp, output); err != nil {
					if os.IsExist(err) {
						return usageErr(fmt.Errorf("output %s already exists; use --force to overwrite", output))
					}
					return err
				}
			}
			if meta.Size == 0 {
				meta.Size = written
			}
			result := downloadResult{Path: meta.Path, Rev: meta.Rev, Size: meta.Size, WrittenTo: output}
			if result.Path == "" {
				result.Path = remotePath
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s (%d bytes) to %s\n", result.Path, result.Size, output)
			return err
		}}
	cmd.Flags().StringVar(&output, "output", "", "Local destination file, or - for stdout")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing destination")
	return cmd
}

func contentURL(c *client.Client, route string) string {
	base := strings.TrimRight(c.RequestBaseURL(), "/")
	if strings.Contains(base, "api.dropboxapi.com") {
		return "https://content.dropboxapi.com/2" + route
	}
	return base + route
}

func contentRequest(ctx context.Context, c *client.Client, flags *rootFlags, method, route string, body io.Reader, size int64, headers, rootHeaders map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, contentURL(c, route), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = size
	}
	auth, err := novelAuthHeader(flags)
	if err != nil {
		return nil, err
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	for key, value := range rootHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	httpClient := client.StreamingHTTPClient(c.HTTPClient, c.ConfiguredTimeout())
	if flags.timeoutExplicit {
		httpClient = c.HTTPClient
	}
	return httpClient.Do(req)
}

func contentError(resp *http.Response, verb string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	err := &client.APIError{Method: http.MethodPost, Path: resp.Request.URL.Path, StatusCode: resp.StatusCode, Body: string(body)}
	if resp.StatusCode == http.StatusConflict {
		if dropbox.HasSummaryPrefix(err, "path/not_found") {
			return notFoundErr(fmt.Errorf("Dropbox file not found: %w", err))
		}
		if dropbox.HasSummaryPrefix(err, "unsupported_file") || dropbox.HasSummaryPrefix(err, "path/unsupported_file") {
			return usageErr(fmt.Errorf("Dropbox file is not downloadable; try Dropbox export: %w", err))
		}
	}
	return classifyAPIErrorOnly(fmt.Errorf("Dropbox %s failed: %w", verb, err))
}
