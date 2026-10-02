// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command — implemented from the absorb manifest transcendence row
// "Download resolver".
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/internal/cliutil"
	"github.com/spf13/cobra"
)

// logiDownloadRe matches Logitech's download01.logi.com file URLs found inside
// article bodies (firmware, software, PDF manuals).
var logiDownloadRe = regexp.MustCompile(`https://download[0-9]*\.logi\.com/[^\s"'<>\\]+`)

// logiDownloadHostRe is deliberately narrower than a general *.logi.com
// allowlist. Article links are sourced from Logitech's dedicated download
// hosts, so redirects must remain on that same host family and on HTTPS.
var logiDownloadHostRe = regexp.MustCompile(`(?i)^download[0-9]*\.logi\.com$`)

func validateLogitechDownloadURL(u *url.URL) error {
	if u == nil || !strings.EqualFold(u.Scheme, "https") || !logiDownloadHostRe.MatchString(u.Hostname()) {
		return fmt.Errorf("refusing download redirect outside HTTPS Logitech download hosts")
	}
	return nil
}

func newLogitechDownloadClient() *http.Client {
	return &http.Client{CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		return validateLogitechDownloadURL(req.URL)
	}}
}

// saveNewDownload claims the destination with O_EXCL. If writing fails, it
// leaves the partial file in place with an explicit error. Removing by path
// could delete a file another process swapped in after the open.
func saveNewDownload(dest string, src io.Reader) error {
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- dest is a validated basename joined under the caller-selected --save directory
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("refusing to overwrite existing download %s", dest)
		}
		return fmt.Errorf("creating download %s: %w", dest, err)
	}
	_, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("writing %s: %w; partial download may remain at this path", dest, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing %s: %w; partial download may remain at this path", dest, closeErr)
	}
	return nil
}

type downloadLink struct {
	ArticleID string `json:"article_id"`
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	SavedTo   string `json:"saved_to,omitempty"`
}

func newNovelDownloadCmd(flags *rootFlags) *cobra.Command {
	var saveDir string
	var limit int

	cmd := &cobra.Command{
		Use:   "download <article-id>",
		Short: "Resolve download01.logi.com file links in an article and fetch them (with --dry-run and local cache).",
		Long: "Reads an article's HTML body and extracts download01.logi.com file links (firmware, software, PDF manuals). " +
			"By default it only lists the resolved URLs. Pass --save <dir> to actually download the files there.",
		Example: "  logitech-docs-pp-cli download 360023302754\n" +
			"  logitech-docs-pp-cli download 360023302754 --save ./downloads\n" +
			"  logitech-docs-pp-cli download 360023302754 --dry-run",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "article-id=360023302754"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "download resolve")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("download requires an <article-id>"))
			}
			articleID := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, "/api/v2/help_center/en-us/articles/"+articleID+".json", nil)
			if err != nil {
				return apiErr(fmt.Errorf("fetching article %s: %w", articleID, err))
			}
			var article struct {
				Article struct {
					ID    int64  `json:"id"`
					Title string `json:"title"`
					Body  string `json:"body"`
				} `json:"article"`
			}
			if err := json.Unmarshal(data, &article); err != nil {
				return apiErr(fmt.Errorf("parsing article: %w", err))
			}

			matches := logiDownloadRe.FindAllString(article.Article.Body, -1)
			seen := map[string]bool{}
			links := make([]downloadLink, 0)
			for _, u := range matches {
				u = strings.TrimSuffix(u, ".")
				if seen[u] {
					continue
				}
				seen[u] = true
				filename := filepath.Base(strings.SplitN(u, "?", 2)[0])
				links = append(links, downloadLink{ArticleID: articleID, URL: u, Filename: filename})
				if limit > 0 && len(links) >= limit {
					break
				}
			}

			if saveDir == "" {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printNovelJSON(cmd.OutOrStdout(), links, flags, "live")
				}
				if len(links) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No download01.logi.com links found in article %s.\n", articleID)
					return nil
				}
				for _, l := range links {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n", l.Filename, l.URL)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d link(s). Re-run with --save <dir> to download.\n", len(links))
				return nil
			}

			// Download each link into saveDir.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would download %d file(s) to %s\n", len(links), saveDir)
				return nil
			}
			if err := os.MkdirAll(saveDir, 0o750); err != nil {
				return fmt.Errorf("creating directory %s: %w", saveDir, err)
			}
			client := newLogitechDownloadClient()
			for i := range links {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, links[i].URL, nil)
				if err != nil {
					return fmt.Errorf("building request for %s: %w", links[i].URL, err)
				}
				req.Header.Set("User-Agent", "logitech-docs-pp-cli")
				resp, err := client.Do(req)
				if err != nil {
					return fmt.Errorf("downloading %s: %w", links[i].URL, err)
				}
				if resp.StatusCode != http.StatusOK {
					_ = resp.Body.Close()
					return apiErr(fmt.Errorf("downloading %s: HTTP %d", links[i].URL, resp.StatusCode))
				}
				// The filename comes from a remote URL path, so reject anything
				// that is not a plain file name before joining it onto saveDir.
				name := links[i].Filename
				if name == "" || name == "." || name == ".." || strings.ContainsRune(name, os.PathSeparator) {
					_ = resp.Body.Close()
					return apiErr(fmt.Errorf("refusing unsafe download filename %q from %s", name, links[i].URL))
				}
				dest := filepath.Join(saveDir, name)
				err = saveNewDownload(dest, resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					return err
				}
				links[i].SavedTo = dest
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), links, flags, "live")
			}
			for _, l := range links {
				fmt.Fprintf(cmd.OutOrStdout(), "saved %s -> %s\n", l.Filename, l.SavedTo)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&saveDir, "save", "", "Directory to download files into (omit to only list resolved URLs)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum files to resolve (0 = all)")
	return cmd
}
