// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built: update a site's HTML by site_id. The per-site update_key is
// resolved from the local store so the user never handles it. Survives
// generate --force.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

func newUpdateCmd(flags *rootFlags) *cobra.Command {
	var htmlFlag, passwordFlag, assetRoot string
	var clearPassword, withAssets bool

	cmd := &cobra.Command{
		Use:   "update <site_id> [file|-]",
		Short: "Replace a site's HTML by site_id (the update_key is resolved from your local store)",
		Long: trimNL(`
Replace a published site's HTML. The once-only update_key is looked up from the
local store by site_id, so you never have to handle it.

Each update is saved as a new local version, so you can 'rollback' later. Use
--password to set a shared-secret password, --clear-password to remove it, or
neither to leave it unchanged.`),
		Example: trimNL(`
  ht-ml-pp-cli update e5051f46 ./deck.html
  ht-ml-pp-cli update e5051f46 --html "<h1>updated</h1>"
  ht-ml-pp-cli update e5051f46 ./deck.html --assets`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site_id is required"))
			}
			siteID := args[0]
			html, srcLabel, filePath, err := readHTMLInput(cmd, args[1:], htmlFlag)
			if err != nil {
				return usageErr(err)
			}

			publicContentWarn(cmd)
			if htmlxWriteGuard(cmd, flags, "update site "+siteID+" from "+srcLabel) {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			site, err := db.GetSite(siteID)
			if err != nil {
				return err
			}
			if site == nil || site.UpdateKey == "" {
				return notFoundErr(fmt.Errorf("no update_key for %q in the local store; it was not published from here. Run 'ht-ml-pp-cli list', or 'ht-ml-pp-cli keys import <vault>' to restore keys", siteID))
			}

			body := map[string]any{"html_content": html}
			newPassword := site.Password
			if clearPassword {
				body["password"] = ""
				newPassword = ""
			} else if passwordFlag != "" {
				body["password"] = passwordFlag
				newPassword = passwordFlag
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, _, err := c.PutWithHeaders(ctx, "/sites/"+siteID, body, bearerHeaders(site.UpdateKey))
			if err != nil {
				var ae *client.APIError
				if As(err, &ae) {
					return mapSiteHTTPError(ae.StatusCode, []byte(ae.Body), flags)
				}
				return apiErr(err)
			}
			resp, _ := parseSiteAPIResponse(raw)

			rec := store.SiteRecord{
				SiteID:    siteID,
				URL:       firstNonEmpty(resp.URL, site.URL, siteLiveURL(siteID)),
				Status:    firstNonEmpty(resp.Status, site.Status),
				Title:     extractTitle(html),
				Alias:     site.Alias,
				UpdateKey: firstNonEmpty(resp.UpdateKey, site.UpdateKey),
				Password:  newPassword,
				CreatedAt: site.CreatedAt,
			}
			if err := db.SaveSite(rec, html, true); err != nil {
				return fmt.Errorf("saving to local store: %w", err)
			}

			result := publishResult{
				SiteID:    siteID,
				URL:       rec.URL,
				Status:    rec.Status,
				Title:     rec.Title,
				KeyStored: true,
			}
			if assets, aerr := fetchSiteAssets(ctx, flags, siteID); aerr == nil {
				refs := make([]store.AssetRef, 0, len(assets.Assets))
				for _, a := range assets.Assets {
					refs = append(refs, store.AssetRef{SiteID: siteID, RelativePath: a.RelativePath, AssetType: a.AssetType})
				}
				_ = db.SaveAssets(siteID, refs)
				if withAssets && len(refs) > 0 {
					root := assetRoot
					if root == "" {
						root = assetRootFromFile(filePath)
					}
					uploaded, missing := uploadReferencedAssets(ctx, cmd, c.RequestBaseURL(), siteID, site.UpdateKey, refs, root)
					result.AssetsUpload = uploaded
					result.AssetsMissing = missing
				}
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", green("updated"), bold(result.URL))
				if result.AssetsUpload > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "assets:  %d uploaded\n", result.AssetsUpload)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&htmlFlag, "html", "", "HTML content as a literal string (instead of a file)")
	cmd.Flags().StringVar(&passwordFlag, "password", "", "Set a shared-secret password (omit to leave unchanged)")
	cmd.Flags().BoolVar(&clearPassword, "clear-password", false, "Remove the site's password")
	cmd.Flags().BoolVar(&withAssets, "assets", false, "Upload every referenced local image/video after updating")
	cmd.Flags().StringVar(&assetRoot, "root", "", "Directory to resolve referenced asset paths from (default: the HTML file's directory)")
	return cmd
}
