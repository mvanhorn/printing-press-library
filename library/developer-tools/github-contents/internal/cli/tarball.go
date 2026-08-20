// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: tarball.

package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newTarballCmd(flags *rootFlags) *cobra.Command {
	var (
		flagRef string
		flagOut string
	)

	cmd := &cobra.Command{
		Use:   "tarball <owner/repo[#ref]>",
		Short: "Download a full repository snapshot as a .tar.gz — no subdirectory filtering",
		Long:  "Download a full repository snapshot as a .tar.gz via GitHub's codeload archive endpoint. Snapshots the whole repo; use 'fetch' for a subdirectory.",
		Example: strings.Trim(`
  github-contents-pp-cli tarball octocat/Hello-World --out ./hello-world.tar.gz
  github-contents-pp-cli tarball octocat/Hello-World#main --agent
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "target=octocat/Hello-World;--out=/tmp/pp-ghc-dogfood/hw.tar.gz",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would download a full-repository tarball snapshot to --out")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				return usageErr(fmt.Errorf("target is required\nUsage: %s <owner/repo[#ref]>", cmd.CommandPath()))
			}
			target := args[0]
			addr, err := resolveGHAddress(target, flagRef)
			if err != nil {
				return usageErr(fmt.Errorf("%w\nUsage: %s <owner/repo[#ref]>", err, cmd.CommandPath()))
			}
			if addr.Path != "" {
				return usageErr(fmt.Errorf("tarball snapshots the whole repo; use fetch for a subdirectory (got path %q)", addr.Path))
			}
			ref := addr.Ref
			if ref == "" {
				ref = "HEAD"
			}
			outPath := flagOut
			if outPath == "" {
				outPath = fmt.Sprintf("%s-%s.tar.gz", addr.Repo, ref)
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would download tarball for %s@%s to %s\n", target, ref, outPath)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The single GET here IS the download: a whole-repo archive can
			// be multi-GB, so it runs on the download-phase context (exempt
			// from the DEFAULT --timeout; an explicit --timeout is honored)
			// and a streaming client whose ResponseHeaderTimeout — not a
			// whole-body Timeout — guards against a dead server. The 4xx
			// classification below needs only headers, which that timeout
			// covers.
			ctx, cancel := downloadPhaseCtx(cmd, flags)
			defer cancel()

			rawURL := c.RequestBaseURL() + fmt.Sprintf("/repos/%s/%s/tarball/%s", url.PathEscape(addr.Owner), url.PathEscape(addr.Repo), url.PathEscape(ref))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
			if err != nil {
				return apiErr(err)
			}
			if token := c.Config.AuthHeader(); token != "" {
				req.Header.Set("Authorization", token)
			}
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

			httpClient := ghfetch.NewStreamingHTTPClient()
			resp, err := httpClient.Do(req)
			if err != nil {
				return apiErr(fmt.Errorf("downloading tarball: %w", err))
			}
			defer resp.Body.Close()

			switch {
			case resp.StatusCode == http.StatusTooManyRequests:
				return rateLimitErr(&cliutil.RateLimitError{URL: rawURL, RetryAfter: cliutil.RetryAfter(resp)})
			case resp.StatusCode == http.StatusNotFound:
				return notFoundErr(fmt.Errorf("repository %s not found (or ref %q does not exist)\nhint: private repos return 404 without auth — set GITHUB_TOKEN", target, ref))
			case resp.StatusCode >= 400:
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				return apiErr(fmt.Errorf("downloading tarball: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
			}

			written, err := ghfetch.StreamToFile(resp.Body, outPath, -1)
			if err != nil {
				return apiErr(err)
			}

			envelope := map[string]any{
				"out":   outPath,
				"bytes": written,
				"ref":   ref,
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}

	cmd.Flags().StringVar(&flagRef, "ref", "", "Branch, tag, or commit SHA (overrides #ref in the target)")
	cmd.Flags().StringVar(&flagOut, "out", "", "Output path for the tarball (default: {repo}-{ref}.tar.gz)")

	return cmd
}
