// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: audit every site in the local registry for
// publicly-visible broken assets. Because ht-ml.app sites are public and
// permanent, a missing image is visible to anyone with the URL; this finds them
// across all sites at once. Read-only (live HEAD checks, no writes). Survives
// generate --force.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

type auditAssetIssue struct {
	RelativePath string `json:"relative_path"`
	HTTPStatus   int    `json:"http_status"`
}

type auditSiteResult struct {
	SiteID     string            `json:"site_id"`
	URL        string            `json:"url"`
	Title      string            `json:"title,omitempty"`
	Referenced int               `json:"referenced"`
	BrokenN    int               `json:"broken_count"`
	Broken     []auditAssetIssue `json:"broken,omitempty"`
}

func newNovelAssetsAuditCmd(flags *rootFlags) *cobra.Command {
	var flagMissingOnly bool

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Across all your sites, list the ones with publicly-visible broken or missing images.",
		Long: trimNL(`
Every ht-ml.app site is public and permanent, so a referenced asset that was
never uploaded is a broken image anyone can see. audit walks every site in your
local registry, asks the API which assets each one references, and live-checks
each on the CDN, reporting the broken ones so you can 'assets sync' them.`),
		Example: trimNL(`
  ht-ml-pp-cli assets audit --agent
  ht-ml-pp-cli assets audit --missing-only`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			sites, err := db.ListSites()
			if err != nil {
				return err
			}
			if len(sites) == 0 {
				// Populated by publish/update, not `sync`; avoid the misleading
				// unsynced hint and point at publish instead.
				fmt.Fprintln(cmd.ErrOrStderr(), "no sites in the local store yet; publish one with 'ht-ml-pp-cli publish <file>'")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			results := make([]auditSiteResult, 0, len(sites))
			for _, s := range sites {
				refs, _ := db.ListAssets(s.SiteID)
				if len(refs) == 0 {
					// No cached list; ask the API once.
					if live, aerr := fetchSiteAssets(ctx, flags, s.SiteID); aerr == nil {
						for _, a := range live.Assets {
							refs = append(refs, store.AssetRef{SiteID: s.SiteID, RelativePath: a.RelativePath, AssetType: a.AssetType})
						}
						_ = db.SaveAssets(s.SiteID, refs)
					}
				}
				res := auditSiteResult{SiteID: s.SiteID, URL: firstNonEmpty(s.URL, siteLiveURL(s.SiteID)), Title: s.Title, Referenced: len(refs)}
				for _, a := range refs {
					if present, code := liveAssetPresent(ctx, s.SiteID, a.RelativePath); !present {
						res.Broken = append(res.Broken, auditAssetIssue{RelativePath: a.RelativePath, HTTPStatus: code})
					}
				}
				res.BrokenN = len(res.Broken)
				if flagMissingOnly && res.BrokenN == 0 {
					continue
				}
				results = append(results, res)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("SITE_ID")+"\t"+bold("REF")+"\t"+bold("BROKEN")+"\t"+bold("TITLE")+"\t"+bold("URL"))
				for _, r := range results {
					broken := green("0")
					if r.BrokenN > 0 {
						broken = red(fmt.Sprintf("%d", r.BrokenN))
					}
					fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", r.SiteID, r.Referenced, broken, truncate(r.Title, 28), r.URL)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				for _, r := range results {
					for _, b := range r.Broken {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s %s/%s (HTTP %d)\n", red("broken:"), r.SiteID, b.RelativePath, b.HTTPStatus)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().BoolVar(&flagMissingOnly, "missing-only", false, "Only show sites that have at least one broken asset")
	return cmd
}
