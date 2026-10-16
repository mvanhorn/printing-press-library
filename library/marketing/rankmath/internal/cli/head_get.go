// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/seohead"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if head, _, err := root.Find([]string{"head"}); err == nil {
			addNovelCommandIfAbsent(head, newHeadGetCmd(flags))
		}
	})
}

func newHeadGetCmd(flags *rootFlags) *cobra.Command {
	var save bool
	var dbPath string
	cmd := &cobra.Command{
		Use:         "get [url]",
		Short:       "Fetch SEO head data for a URL",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "url=https://example.com/"},
		Example: strings.Trim(`
  rankmath-pp-cli head get https://example.com/ --json
		`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "head get")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("head get requires a URL"))
			}
			target := args[0]
			parsedURL, err := url.ParseRequestURI(target)
			if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
				if err == nil {
					err = fmt.Errorf("URL must be absolute")
				}
				return usageErr(fmt.Errorf("invalid URL %q: %w", target, err))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			h, err := getRankMathHead(ctx, flags, target)
			source := "getHead"
			getHeadError := ""
			if err != nil {
				getHeadError = err.Error()
				fetch := seohead.New(20*time.Second, 1)
				h, err = fetch.Fetch(ctx, target)
				if err != nil {
					return fmt.Errorf("fetch SEO head: %w", err)
				}
				source = "html"
			}
			if h.FetchedAt.IsZero() {
				h.FetchedAt = time.Now().UTC()
			}
			if save {
				if dbPath == "" {
					dbPath = defaultDBPath("rankmath-pp-cli")
				}
				db, e := store.OpenWithContext(ctx, dbPath)
				if e != nil {
					return fmt.Errorf("open local database: %w", e)
				}
				defer db.Close()
				if e = db.EnsureSEOSnapshots(ctx); e == nil {
					e = db.InsertSEOSnapshot(ctx, h)
				}
				if e != nil {
					return fmt.Errorf("save SEO snapshot: %w", e)
				}
			}
			out := struct {
				Head         seohead.Head `json:"head"`
				Source       string       `json:"source"`
				GetHeadError string       `json:"get_head_error,omitempty"`
			}{Head: h, Source: source, GetHeadError: getHeadError}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		}}
	cmd.Flags().BoolVar(&save, "save", false, "Save this result to the local SEO snapshot store")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

func getRankMathHead(ctx context.Context, flags *rootFlags, target string) (seohead.Head, error) {
	c, err := flags.newClient()
	if err != nil {
		return seohead.Head{}, err
	}
	data, err := c.Get(ctx, "/rankmath/v1/getHead", map[string]string{"url": target})
	if err != nil {
		return seohead.Head{}, err
	}
	var response struct {
		Success bool   `json:"success"`
		Head    string `json:"head"`
		Message string `json:"message"`
	}
	if err = json.Unmarshal(data, &response); err != nil {
		return seohead.Head{}, fmt.Errorf("decode getHead response: %w", err)
	}
	if !response.Success {
		return seohead.Head{}, fmt.Errorf("getHead unsuccessful: %s", response.Message)
	}
	h := seohead.Parse(target, []byte(response.Head))
	h.FetchedAt = time.Now().UTC()
	return h, nil
}
