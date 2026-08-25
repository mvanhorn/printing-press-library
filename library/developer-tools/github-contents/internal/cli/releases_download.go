// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: releases download. Wired as a child of the
// generated `releases` parent command in root.go (local-variable capture
// pattern) rather than by editing the generated releases.go.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	goPath "path"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/spf13/cobra"
)

// safeAssetName validates a remote-supplied release-asset name before it
// is joined onto the local --out directory. Asset names come from the API
// response, not the user, so a malicious release could carry names like
// "../../.bashrc", "/etc/cron.d/x", or "C:evil". Only a plain single-level
// file name passes: SafeRelPath screens the traversal/absolute/drive-and-
// stream classes, and the Base==name check rejects anything with a
// separator or that Clean() had to rewrite.
func safeAssetName(name string) error {
	if name == "" {
		return errors.New("empty asset name")
	}
	safe, err := ghfetch.SafeRelPath(name)
	if err != nil {
		return err
	}
	if safe != name || goPath.Base(name) != name {
		return fmt.Errorf("asset name %q is not a plain file name", name)
	}
	return nil
}

// pp:data-source live
func newReleasesDownloadCmd(flags *rootFlags) *cobra.Command {
	var (
		flagTag      string
		flagPattern  string
		flagOut      string
		flagListOnly bool
	)

	cmd := &cobra.Command{
		Use:   "download <owner> <repo>",
		Short: "Download release assets matching a glob pattern",
		Long:  "Resolve a release (latest, or --tag) and download its assets, optionally filtered by --pattern. Use --list-only to preview matching assets without downloading.",
		Example: strings.Trim(`
  github-contents-pp-cli releases download cli cli --pattern "*.deb" --out ./assets
  github-contents-pp-cli releases download mjwoon AI-readings --list-only --json
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "owner=mjwoon;repo=AI-readings;--list-only",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both <owner> and <repo> are required\nUsage: %s <owner> <repo>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve a release and download matching assets to --out")
				return nil
			}
			owner, repo := args[0], args[1]
			if !flagListOnly && cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would download assets for %s/%s\n", owner, repo)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			releasePath := fmt.Sprintf("/repos/%s/%s/releases/latest", url.PathEscape(owner), url.PathEscape(repo))
			if flagTag != "" {
				releasePath = fmt.Sprintf("/repos/%s/%s/releases/tags/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(flagTag))
			}
			data, err := c.Get(ctx, releasePath, nil)
			if err != nil {
				var apiErrTyped *client.APIError
				if errors.As(err, &apiErrTyped) && apiErrTyped.StatusCode == http.StatusNotFound {
					// GitHub 404s both for "release/tag doesn't exist" and
					// "repo doesn't exist (or is private without auth)" —
					// probe the repo itself so a bad owner/repo isn't masked
					// as a benign exit-0 empty result.
					if _, repoErr := c.Get(ctx, fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)), nil); repoErr != nil {
						return classifyGHTreeError(repoErr, flags, owner+"/"+repo)
					}
					// Repo exists. An explicit --tag that resolved nowhere is
					// a caller mistake (typo'd tag) — surface it as not-found
					// (exit 3), like a missing repo path. Only the no-flag
					// "latest" case is a benign exit-0 empty result ("this
					// repo just has no releases").
					if flagTag != "" {
						return notFoundErr(fmt.Errorf("tag %q not found (repo exists) — check the tag name with 'releases list %s %s'", flagTag, owner, repo))
					}
					envelope := map[string]any{
						"owner":  owner,
						"repo":   repo,
						"assets": []any{},
						"note":   "repo has no releases",
					}
					return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
				}
				return classifyAPIError(err, flags)
			}

			var release struct {
				TagName string `json:"tag_name"`
				Assets  []struct {
					ID   int64  `json:"id"`
					Name string `json:"name"`
					Size int64  `json:"size"`
				} `json:"assets"`
			}
			if err := json.Unmarshal(data, &release); err != nil {
				return apiErr(fmt.Errorf("parsing release response: %w", err))
			}

			type matchedAsset struct {
				ID   int64
				Name string
				Size int64
			}
			var matched []matchedAsset
			for _, a := range release.Assets {
				if flagPattern != "" {
					ok, matchErr := goPath.Match(flagPattern, a.Name)
					if matchErr != nil {
						return usageErr(fmt.Errorf("invalid --pattern %q: %w", flagPattern, matchErr))
					}
					if !ok {
						continue
					}
				}
				matched = append(matched, matchedAsset{ID: a.ID, Name: a.Name, Size: a.Size})
			}

			type assetResult struct {
				Name       string `json:"name"`
				Size       int64  `json:"size"`
				Downloaded bool   `json:"downloaded"`
				Path       string `json:"path,omitempty"`
				Error      string `json:"error,omitempty"`
			}

			if len(matched) == 0 {
				envelope := map[string]any{
					"owner":  owner,
					"repo":   repo,
					"tag":    release.TagName,
					"assets": []any{},
					"note":   "no assets matched the given pattern",
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			if flagListOnly {
				results := make([]assetResult, 0, len(matched))
				for _, m := range matched {
					results = append(results, assetResult{Name: m.Name, Size: m.Size})
				}
				envelope := map[string]any{
					"owner":  owner,
					"repo":   repo,
					"tag":    release.TagName,
					"assets": results,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			if err := os.MkdirAll(flagOut, 0o755); err != nil { // #nosec G301 -- user-requested download output dir; conventional mkdir mode, narrowed by the process umask
				return apiErr(fmt.Errorf("creating output directory: %w", err))
			}

			// Asset transfer phase: release assets can be huge, so it runs
			// on the download-phase context (exempt from the DEFAULT
			// --timeout; explicit --timeout honored) and a streaming client
			// whose ResponseHeaderTimeout — not a whole-body Timeout —
			// guards the handshake. The release-JSON API calls above stay
			// on the --timeout-bounded ctx.
			dlCtx, cancelDl := downloadPhaseCtx(cmd, flags)
			defer cancelDl()
			httpClient := ghfetch.NewStreamingHTTPClient()
			token := c.Config.AuthHeader()
			apiBase := c.RequestBaseURL()
			results := make([]assetResult, 0, len(matched))
			failures := 0
			for _, m := range matched {
				if nameErr := safeAssetName(m.Name); nameErr != nil {
					failures++
					results = append(results, assetResult{Name: m.Name, Size: m.Size, Error: "unsafe asset name rejected: " + nameErr.Error()})
					continue
				}
				destPath := filepath.Join(flagOut, m.Name)
				if err := downloadAssetByID(dlCtx, httpClient, token, apiBase, owner, repo, m.ID, destPath, m.Size); err != nil {
					failures++
					results = append(results, assetResult{Name: m.Name, Size: m.Size, Error: err.Error()})
					continue
				}
				results = append(results, assetResult{Name: m.Name, Size: m.Size, Downloaded: true, Path: destPath})
			}

			envelope := map[string]any{
				"owner":  owner,
				"repo":   repo,
				"tag":    release.TagName,
				"assets": results,
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), envelope, flags); err != nil {
				return err
			}
			if failures > 0 {
				return apiErr(fmt.Errorf("%d of %d asset downloads failed", failures, len(matched)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagTag, "tag", "", "Release tag (default: latest published release)")
	cmd.Flags().StringVar(&flagPattern, "pattern", "", "Glob pattern to filter asset names (default: all assets)")
	cmd.Flags().StringVar(&flagOut, "out", ".", "Local destination directory for downloaded assets")
	cmd.Flags().BoolVar(&flagListOnly, "list-only", false, "List matching assets without downloading")

	return cmd
}

// downloadAssetByID streams one release asset's bytes to destPath via the
// asset API endpoint (GET {apiBase}/repos/{o}/{r}/releases/assets/{id}
// with Accept: application/octet-stream) instead of the release JSON's
// browser_download_url. Two reasons: (a) the API endpoint works for
// private-repo assets, which browser_download_url does not; (b) the URL in
// the response body is remote-supplied — attaching the bearer token to a
// request for an arbitrary host would leak it. Here the token goes only to
// the configured API base; GitHub answers with a 302 to a short-lived
// signed storage URL, and Go's http.Client strips the Authorization header
// on that cross-host redirect automatically (no CheckRedirect override —
// deliberately, so the strip stays in force).
// expectedSize is the asset's size from the release JSON; values <= 0 are
// treated as unknown and skip the byte-count verification.
func downloadAssetByID(ctx context.Context, httpClient *http.Client, token, apiBase, owner, repo string, assetID int64, destPath string, expectedSize int64) error {
	assetURL := fmt.Sprintf("%s/repos/%s/%s/releases/assets/%d", apiBase, url.PathEscape(owner), url.PathEscape(repo), assetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return &cliutil.RateLimitError{URL: assetURL, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d downloading asset", resp.StatusCode)
	}
	if expectedSize <= 0 {
		expectedSize = -1
	}
	_, err = ghfetch.StreamToFile(resp.Body, destPath, expectedSize)
	return err
}
