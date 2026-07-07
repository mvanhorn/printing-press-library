// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built: assets command group (upload, sync, audit). Survives generate --force.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelAssetsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "Manage a site's referenced assets: upload, sync, audit",
		Long:  trimNL("Upload referenced images/videos, reconcile all referenced assets for one site in a single pass, or audit every site for publicly-visible broken images."),
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAssetsUploadCmd(flags))
	cmd.AddCommand(newNovelAssetsAuditCmd(flags))
	cmd.AddCommand(newNovelAssetsSyncCmd(flags))
	return cmd
}

func newAssetsUploadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <site_id> <relative_path> <file>",
		Short: "Upload one referenced asset (the update_key is resolved from your store)",
		Long:  trimNL("Upload a single referenced asset. The relative_path must match how the asset is referenced in the site's HTML (e.g. images/logo.png), or ht-ml.app returns 403."),
		Example: trimNL(`
  ht-ml-pp-cli assets upload e5051f46 images/logo.png ./logo.png`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site_id, relative_path, and a local file are required"))
			}
			siteID, relPath, file := args[0], args[1], args[2]
			if htmlxWriteGuard(cmd, flags, fmt.Sprintf("upload %s to site %s", relPath, siteID)) {
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
			data, name, rerr := resolveAssetFile(".", file)
			if rerr != nil {
				return usageErr(fmt.Errorf("reading %s: %w", file, rerr))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			status, respBody, uerr := uploadAssetMultipart(ctx, c.RequestBaseURL(), siteID, key, relPath, data, name)
			if uerr != nil {
				return apiErr(uerr)
			}
			if status >= 400 {
				return mapSiteHTTPError(status, respBody, flags)
			}
			out := map[string]any{"site_id": siteID, "relative_path": relPath, "uploaded": true, "url": liveAssetURL(siteID, relPath)}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s uploaded %s -> %s\n", green("ok:"), relPath, liveAssetURL(siteID, relPath))
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
