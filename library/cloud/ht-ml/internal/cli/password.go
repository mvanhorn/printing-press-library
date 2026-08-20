// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built: per-site password management. ht-ml.app's PUT replaces the whole
// HTML, so changing only the password re-sends the latest stored HTML with the
// new password field. Survives generate --force.
// pp:data-source live

package cli

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

func newPasswordCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "password",
		Short: "Manage a site's shared-secret password (set, clear)",
		Long:  trimNL("Set, rotate, or clear a site's read password. The password is a SHARED secret: anyone you give it to can read the site. Readers pass it as the ht_ml_pwd cookie."),
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPasswordSetCmd(flags))
	cmd.AddCommand(newPasswordClearCmd(flags))
	return cmd
}

func newPasswordSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <site_id> [password]",
		Short: "Set or rotate a site's password (generates a non-personal one if omitted)",
		Long:  trimNL("Set or rotate a site's read password. If you omit the password, a unique non-personal one is generated and printed. Give the password to everyone who should read the site."),
		Example: trimNL(`
  ht-ml-pp-cli password set e5051f46
  ht-ml-pp-cli password set e5051f46 my-shared-secret`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site_id is required"))
			}
			siteID := args[0]
			password := ""
			generated := false
			if len(args) >= 2 {
				password = args[1]
			} else {
				password = generatePassword()
				generated = true
			}
			if htmlxWriteGuard(cmd, flags, "set password on site "+siteID) {
				return nil
			}
			return applyPassword(cmd, flags, siteID, password, generated)
		},
	}
}

func newPasswordClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "clear <site_id>",
		Short:   "Remove a site's password (make it public again)",
		Example: "  ht-ml-pp-cli password clear e5051f46",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site_id is required"))
			}
			siteID := args[0]
			if htmlxWriteGuard(cmd, flags, "clear password on site "+siteID) {
				return nil
			}
			return applyPassword(cmd, flags, siteID, "", false)
		},
	}
}

// applyPassword re-PUTs the latest stored HTML with the new password value
// ("" clears it), then records the change locally.
func applyPassword(cmd *cobra.Command, flags *rootFlags, siteID, password string, generated bool) error {
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
		return notFoundErr(fmt.Errorf("no update_key for %q in the local store; run 'ht-ml-pp-cli list' or 'keys import'", siteID))
	}
	versions, err := db.ListVersions(siteID)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return apiErr(fmt.Errorf("no stored HTML for %q; ht-ml.app replaces HTML on every write, so update the site once with 'ht-ml-pp-cli update %s <file>' before changing its password", siteID, siteID))
	}
	latestHTML := versions[0].HTML

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	body := map[string]any{"html_content": latestHTML, "password": password}
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
		Title:     site.Title,
		Alias:     site.Alias,
		UpdateKey: firstNonEmpty(resp.UpdateKey, site.UpdateKey),
		Password:  password,
		CreatedAt: site.CreatedAt,
	}
	// No new version: the HTML is unchanged, only the password changed.
	if err := db.SaveSite(rec, "", false); err != nil {
		return fmt.Errorf("saving to local store: %w", err)
	}

	out := map[string]any{
		"site_id":   siteID,
		"url":       rec.URL,
		"protected": password != "",
	}
	if password == "" {
		out["note"] = "password removed; the site is public again"
	} else {
		out["password"] = password
		out["note"] = "share this password with anyone who should read the site; readers pass it as the ht_ml_pwd cookie"
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.OutOrStdout()
		if password == "" {
			fmt.Fprintf(w, "%s password removed for %s\n", green("ok:"), siteID)
		} else {
			label := "password set"
			if generated {
				label = "password generated"
			}
			fmt.Fprintf(w, "%s %s for %s\n", green("ok:"), label, siteID)
			fmt.Fprintf(w, "password: %s\n", bold(password))
			fmt.Fprintln(w, "share this with readers; they pass it as the ht_ml_pwd cookie")
		}
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}

// generatePassword returns a URL-safe, non-personal shared secret.
func generatePassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "htmlpw-" + nowRFC3339()
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
