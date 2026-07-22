// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: publish a recurring document under a stable local
// alias. ht-ml.app mints a new random site_id (and a fresh public URL) on every
// create, which breaks recurring docs like a weekly status report. Binding an
// alias to a site_id lets you keep one URL and update it in place. Survives
// generate --force.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

func newNovelRepublishCmd(flags *rootFlags) *cobra.Command {
	var flagAs, htmlFlag, passwordFlag, assetRoot string
	var withAssets bool

	cmd := &cobra.Command{
		Use:   "republish --as <alias> [file|-]",
		Short: "Publish a recurring document under a stable local alias: update in place if the alias exists, else create and bind it.",
		Long: trimNL(`
ht-ml.app gives every create a new random site_id and URL, so re-publishing a
recurring document (a weekly status report, a changelog) would scatter it across
throwaway URLs. republish binds a memorable local alias to one site_id: the
first run creates the site and records the alias; later runs find the alias and
update that same site in place, keeping the URL stable.`),
		Example: trimNL(`
  ht-ml-pp-cli republish --as weekly-status ./status.html
  ht-ml-pp-cli republish --as weekly-status ./status.html --assets --root ./public`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if flagAs == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--as <alias> is required"))
			}
			html, srcLabel, filePath, err := readHTMLInput(cmd, args, htmlFlag)
			if err != nil {
				return usageErr(err)
			}

			publicContentWarn(cmd)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			existingID, err := db.ResolveAlias(flagAs)
			if err != nil {
				return err
			}

			action, verb := "created", "create"
			if existingID != "" {
				action, verb = "updated", "update"
			}
			if htmlxWriteGuard(cmd, flags, fmt.Sprintf("%s alias %q from %s", verb, flagAs, srcLabel)) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var rec store.SiteRecord
			if existingID != "" {
				site, gerr := db.GetSite(existingID)
				if gerr != nil {
					return gerr
				}
				if site == nil || site.UpdateKey == "" {
					return notFoundErr(fmt.Errorf("alias %q points at site %q but its update_key is not in the store; restore it with 'keys import'", flagAs, existingID))
				}
				body := map[string]any{"html_content": html}
				if passwordFlag != "" {
					body["password"] = passwordFlag
				}
				raw, _, perr := c.PutWithHeaders(ctx, "/sites/"+existingID, body, bearerHeaders(site.UpdateKey))
				if perr != nil {
					var ae *client.APIError
					if As(perr, &ae) {
						return mapSiteHTTPError(ae.StatusCode, []byte(ae.Body), flags)
					}
					return apiErr(perr)
				}
				resp, _ := parseSiteAPIResponse(raw)
				rec = store.SiteRecord{
					SiteID:    existingID,
					URL:       firstNonEmpty(resp.URL, site.URL, siteLiveURL(existingID)),
					Status:    firstNonEmpty(resp.Status, site.Status),
					Title:     extractTitle(html),
					Alias:     flagAs,
					UpdateKey: site.UpdateKey,
					Password:  firstNonEmpty(passwordFlag, site.Password),
					CreatedAt: site.CreatedAt,
				}
			} else {
				body := map[string]any{"html_content": html}
				if passwordFlag != "" {
					body["password"] = passwordFlag
				}
				raw, _, perr := c.Post(ctx, "/sites", body)
				if perr != nil {
					var ae *client.APIError
					if As(perr, &ae) {
						return mapSiteHTTPError(ae.StatusCode, []byte(ae.Body), flags)
					}
					return apiErr(perr)
				}
				resp, rerr := parseSiteAPIResponse(raw)
				if rerr != nil {
					return apiErr(rerr)
				}
				if resp.SiteID == "" {
					return apiErr(fmt.Errorf("ht-ml.app returned no site_id: %s", string(raw)))
				}
				rec = store.SiteRecord{
					SiteID:    resp.SiteID,
					URL:       firstNonEmpty(resp.URL, siteLiveURL(resp.SiteID)),
					Status:    resp.Status,
					Title:     extractTitle(html),
					Alias:     flagAs,
					UpdateKey: resp.UpdateKey,
					Password:  passwordFlag,
					CreatedAt: nowRFC3339(),
				}
			}

			if err := db.SaveSite(rec, html, true); err != nil {
				return fmt.Errorf("saving to local store: %w", err)
			}

			uploaded := 0
			if assets, aerr := fetchSiteAssets(ctx, flags, rec.SiteID); aerr == nil {
				refs := make([]store.AssetRef, 0, len(assets.Assets))
				for _, a := range assets.Assets {
					refs = append(refs, store.AssetRef{SiteID: rec.SiteID, RelativePath: a.RelativePath, AssetType: a.AssetType})
				}
				_ = db.SaveAssets(rec.SiteID, refs)
				if withAssets && len(refs) > 0 {
					root := assetRoot
					if root == "" {
						root = assetRootFromFile(filePath)
					}
					uploaded, _ = uploadReferencedAssets(ctx, cmd, c.RequestBaseURL(), rec.SiteID, rec.UpdateKey, refs, root)
				}
			}

			out := map[string]any{
				"alias":      flagAs,
				"site_id":    rec.SiteID,
				"url":        rec.URL,
				"action":     action,
				"key_stored": rec.UpdateKey != "",
			}
			if uploaded > 0 {
				out["assets_uploaded"] = uploaded
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s alias %s -> %s\n", green(action), bold(flagAs), bold(rec.URL))
				fmt.Fprintf(cmd.OutOrStdout(), "site_id: %s\n", rec.SiteID)
				if uploaded > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "assets:  %d uploaded\n", uploaded)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagAs, "as", "", "Stable local alias to bind this document to (required)")
	cmd.Flags().StringVar(&htmlFlag, "html", "", "HTML content as a literal string (instead of a file)")
	cmd.Flags().StringVar(&passwordFlag, "password", "", "Set a shared-secret password to make the site private")
	cmd.Flags().BoolVar(&withAssets, "assets", false, "Upload every referenced local image/video after publishing")
	cmd.Flags().StringVar(&assetRoot, "root", "", "Directory to resolve referenced asset paths from (default: the HTML file's directory)")
	return cmd
}
