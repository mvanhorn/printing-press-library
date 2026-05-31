// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored novel command: `publish dir <path>`. Walks a local directory,
// classifies files into inline (small text) vs upload (everything else),
// publishes via /api/v1/publish, PUTs each upload target, finalizes, and
// records the result locally. Not generated — survives a regeneration merge.
package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPublishDirCmd(flags *rootFlags) *cobra.Command {
	var (
		flagAnon     bool
		flagSlug     string
		flagPassword string
		flagSPA      bool
		flagDB       string
	)

	cmd := &cobra.Command{
		Use:   "dir <path>",
		Short: "Publish a local directory to a live here.now URL in one command",
		Long: strings.Trim(`
Publish a whole local directory to a live URL. Small text files are inlined in
the request; binary and large files are uploaded to presigned targets. The
result (slug, version, and — for --anon — the claim token and 24h expiry) is
recorded in the local store so 'claims' and 'publish resume' can act on it.
`, "\n"),
		Example: strings.Trim(`
  here-now-pp-cli publish dir ./site --json
  here-now-pp-cli publish dir ./site --anon
  here-now-pp-cli publish dir ./spa --spa --slug my-site
  here-now-pp-cli publish dir ./site --dry-run
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			dir := args[0]
			opts := publishDirOptions{
				Dir:      dir,
				Anon:     flagAnon,
				Slug:     flagSlug,
				Password: flagPassword,
				SPA:      flagSPA,
			}

			if dryRunOK(flags) {
				preview := map[string]any{
					"dry_run":    true,
					"dir":        dir,
					"anonymous":  flagAnon,
					"spa":        flagSPA,
					"slug":       flagSlug,
					"would_post": "/api/v1/publish",
				}
				if flagSlug == "" {
					preview["slug"] = "(server-assigned)"
				}
				// A dry-run is a preview of intent: if the directory isn't on
				// disk yet, still show what would be published rather than
				// erroring. Only scan file stats when the directory exists.
				plan, err := classifyForPublish(dir)
				switch {
				case err == nil:
					preview["file_count"] = len(plan.Files)
					preview["small_text_files"] = plan.InlineEligible
					preview["binary_or_large_files"] = plan.BinaryOrLarge
					preview["total_bytes"] = plan.TotalSize
				case errors.Is(err, fs.ErrNotExist):
					preview["dir_exists"] = false
					preview["note"] = "directory does not exist yet; this is a preview of what would be published"
				default:
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openHereNowStore(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			resp, err := runPublishDir(cmd.Context(), c, db, opts, flags.timeout)
			if err != nil {
				if resp != nil {
					return err
				}
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Published %s\n  %s\n", resp.Slug, resp.SiteURL)
			if flagAnon && resp.ClaimToken != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  anonymous — claim before %s with 'here-now-pp-cli claims redeem %s'\n", resp.ExpiresAt, resp.Slug)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagAnon, "anon", false, "Publish anonymously (no auth); records a claim token locally")
	cmd.Flags().StringVar(&flagSlug, "slug", "", "Requested slug (default: server-assigned)")
	cmd.Flags().StringVar(&flagPassword, "password", "", "Password-protect the published site")
	cmd.Flags().BoolVar(&flagSPA, "spa", false, "Enable single-page-app fallback routing")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	return cmd
}
