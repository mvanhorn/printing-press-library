// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: revert a live site to any prior HTML version stored
// locally. ht-ml.app keeps no version history and PUT replaces the whole
// document, so the local snapshot taken on every publish/update is the only way
// to undo a bad change. Survives generate --force.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

func newNovelRollbackCmd(flags *rootFlags) *cobra.Command {
	var listOnly bool

	cmd := &cobra.Command{
		Use:   "rollback <site_id> [version]",
		Short: "Revert a live site to any prior HTML version stored locally, with the update_key resolved for you.",
		Long: trimNL(`
ht-ml.app has no server-side version history and PUT replaces the whole HTML, so
a bad update is normally unrecoverable. This CLI snapshots every publish/update
locally; rollback re-PUTs a chosen snapshot.

With no version, it reverts to the immediately previous snapshot. Pass an
explicit version (see --list) to go further back. The revert is itself saved as
a new version, so the history stays append-only and you can roll forward again.`),
		Example: trimNL(`
  ht-ml-pp-cli rollback e5051f46 --list
  ht-ml-pp-cli rollback e5051f46
  ht-ml-pp-cli rollback e5051f46 2`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site_id is required"))
			}
			siteID := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			versions, err := db.ListVersions(siteID)
			if err != nil {
				return err
			}
			if len(versions) == 0 {
				return notFoundErr(fmt.Errorf("no stored versions for %q; rollback needs at least one local snapshot (publish/update from this machine)", siteID))
			}

			if listOnly {
				return renderVersionList(cmd, flags, siteID, versions)
			}

			// Resolve the target version: explicit arg, else the previous snapshot.
			currentVersion := versions[0].Version
			target := 0
			if len(args) >= 2 {
				v, perr := strconv.Atoi(args[1])
				if perr != nil {
					return usageErr(fmt.Errorf("version must be an integer (see 'rollback %s --list'): %v", siteID, perr))
				}
				target = v
			} else {
				if len(versions) < 2 {
					return usageErr(fmt.Errorf("only one version exists for %q; nothing earlier to roll back to. Use --list to inspect", siteID))
				}
				target = versions[1].Version
			}

			snap, err := db.GetVersion(siteID, target)
			if err != nil {
				return err
			}
			if snap == nil {
				return notFoundErr(fmt.Errorf("version %d not found for %q; run 'rollback %s --list'", target, siteID, siteID))
			}

			key, err := db.GetUpdateKey(siteID)
			if err != nil {
				return err
			}
			if key == "" {
				return notFoundErr(fmt.Errorf("no update_key for %q in the local store; restore it with 'ht-ml-pp-cli keys import <vault>'", siteID))
			}

			if htmlxWriteGuard(cmd, flags, fmt.Sprintf("roll site %s back to version %d (currently v%d)", siteID, target, currentVersion)) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, _, err := c.PutWithHeaders(ctx, "/sites/"+siteID, map[string]any{"html_content": snap.HTML}, bearerHeaders(key))
			if err != nil {
				var ae *client.APIError
				if As(err, &ae) {
					return mapSiteHTTPError(ae.StatusCode, []byte(ae.Body), flags)
				}
				return apiErr(err)
			}
			resp, _ := parseSiteAPIResponse(raw)

			site, _ := db.GetSite(siteID)
			rec := store.SiteRecord{SiteID: siteID, UpdateKey: key, Title: extractTitle(snap.HTML)}
			if site != nil {
				rec.URL = firstNonEmpty(resp.URL, site.URL, siteLiveURL(siteID))
				rec.Status = firstNonEmpty(resp.Status, site.Status)
				rec.Alias = site.Alias
				rec.Password = site.Password
				rec.CreatedAt = site.CreatedAt
			} else {
				rec.URL = firstNonEmpty(resp.URL, siteLiveURL(siteID))
			}
			if err := db.SaveSite(rec, snap.HTML, true); err != nil {
				return fmt.Errorf("saving rollback snapshot: %w", err)
			}

			out := map[string]any{
				"site_id":          siteID,
				"rolled_back_to":   target,
				"previous_version": currentVersion,
				"url":              rec.URL,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s site %s reverted to version %d -> %s\n", green("rolled back"), siteID, target, bold(rec.URL))
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&listOnly, "list", false, "List stored versions instead of rolling back")
	return cmd
}

func renderVersionList(cmd *cobra.Command, flags *rootFlags, siteID string, versions []store.SiteVersion) error {
	type versionRow struct {
		Version   int    `json:"version"`
		Bytes     int    `json:"bytes"`
		CreatedAt string `json:"created_at"`
		Current   bool   `json:"current"`
	}
	rows := make([]versionRow, 0, len(versions))
	for i, v := range versions {
		rows = append(rows, versionRow{Version: v.Version, Bytes: len(v.HTML), CreatedAt: v.CreatedAt, Current: i == 0})
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		tw := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(tw, bold("VERSION")+"\t"+bold("BYTES")+"\t"+bold("CREATED")+"\t"+bold("CURRENT"))
		for _, r := range rows {
			cur := ""
			if r.Current {
				cur = green("← live")
			}
			fmt.Fprintf(tw, "%d\t%d\t%s\t%s\n", r.Version, r.Bytes, r.CreatedAt, cur)
		}
		return tw.Flush()
	}
	return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
}
