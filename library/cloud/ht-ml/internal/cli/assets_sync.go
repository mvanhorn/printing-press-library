// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: reconcile every referenced-but-missing asset for one
// site in a single pass. ht-ml.app parses the HTML and expects each referenced
// asset uploaded separately; this finds the gaps and fills them. Survives
// generate --force.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

type assetSyncResult struct {
	SiteID       string   `json:"site_id"`
	Root         string   `json:"root"`
	Referenced   int      `json:"referenced"`
	AlreadyLive  int      `json:"already_live"`
	Uploaded     int      `json:"uploaded"`
	Missing      int      `json:"missing"`
	MissingPaths []string `json:"missing_paths,omitempty"`
}

func newNovelAssetsSyncCmd(flags *rootFlags) *cobra.Command {
	var flagRoot string

	cmd := &cobra.Command{
		Use:   "sync <site_id>",
		Short: "Upload every referenced-but-missing image or video for one site in a single pass.",
		Long: trimNL(`
ht-ml.app parses your HTML for referenced assets but does not host them until
each is uploaded separately. sync asks the API which assets the site references,
checks which are already live on the CDN, and uploads the rest from a local
directory. The update_key is resolved from your store, so you never handle it.

Presence is verified by fetching the live asset URL, because the API's
assets[].status field can read "missing" even after a successful upload.`),
		Example: trimNL(`
  ht-ml-pp-cli assets sync e5051f46 --root ./public
  ht-ml-pp-cli assets sync e5051f46 --root ./public --agent`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site_id is required"))
			}
			siteID := args[0]
			root := flagRoot
			if root == "" {
				root = "."
			}
			if htmlxWriteGuard(cmd, flags, fmt.Sprintf("sync referenced assets for site %s from %s", siteID, root)) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			key, err := db.GetUpdateKey(siteID)
			if err != nil {
				return err
			}
			if key == "" {
				return notFoundErr(fmt.Errorf("no update_key for %q in the local store; run 'ht-ml-pp-cli list' or 'keys import'", siteID))
			}

			// Prefer the live referenced-asset list; fall back to the cache.
			var refs []store.AssetRef
			if live, aerr := fetchSiteAssets(ctx, flags, siteID); aerr == nil {
				for _, a := range live.Assets {
					refs = append(refs, store.AssetRef{SiteID: siteID, RelativePath: a.RelativePath, AssetType: a.AssetType})
				}
				_ = db.SaveAssets(siteID, refs)
			} else {
				refs, _ = db.ListAssets(siteID)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := assetSyncResult{SiteID: siteID, Root: root, Referenced: len(refs)}
			for _, a := range refs {
				if present, _ := liveAssetPresent(ctx, siteID, a.RelativePath); present {
					result.AlreadyLive++
					continue
				}
				data, name, rerr := resolveAssetFile(root, a.RelativePath)
				if rerr != nil {
					result.Missing++
					result.MissingPaths = append(result.MissingPaths, a.RelativePath)
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s referenced but not found under %s\n", a.RelativePath, root)
					continue
				}
				status, respBody, uerr := uploadAssetMultipart(ctx, c.RequestBaseURL(), siteID, key, a.RelativePath, data, name)
				if uerr != nil || status >= 400 {
					result.Missing++
					result.MissingPaths = append(result.MissingPaths, a.RelativePath)
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: upload failed for %s (HTTP %d): %s\n", a.RelativePath, status, firstNonEmpty(extractAPIMessage(respBody), "upload error"))
					continue
				}
				result.Uploaded++
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "%s site %s\n", green("synced"), siteID)
				fmt.Fprintf(w, "referenced: %d   already live: %d   uploaded: %d   missing: %d\n", result.Referenced, result.AlreadyLive, result.Uploaded, result.Missing)
				for _, p := range result.MissingPaths {
					fmt.Fprintf(w, "  %s %s\n", red("missing:"), p)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagRoot, "root", "", "Directory to resolve referenced asset paths from (default: current directory)")
	return cmd
}
