// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built: publish an HTML document to ht-ml.app and capture the once-only
// update_key into the local store. This is the foundation the registry, key
// recovery, and version history are built on. Survives generate --force.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"

	"github.com/spf13/cobra"
)

type publishResult struct {
	SiteID        string `json:"site_id"`
	URL           string `json:"url"`
	Status        string `json:"status"`
	Title         string `json:"title,omitempty"`
	KeyStored     bool   `json:"key_stored"`
	AssetsUpload  int    `json:"assets_uploaded,omitempty"`
	AssetsMissing int    `json:"assets_missing,omitempty"`
	Note          string `json:"note,omitempty"`
}

func newPublishCmd(flags *rootFlags) *cobra.Command {
	var htmlFlag, passwordFlag, assetRoot string
	var withAssets bool

	cmd := &cobra.Command{
		Use:   "publish [file|-]",
		Short: "Publish an HTML document to ht-ml.app and store its site + update_key locally",
		Long: trimNL(`
Publish a single HTML document to ht-ml.app and get a public URL.

Unlike the raw API, publish captures the once-only update_key into a local
store, so you can update, version, and recover the site later. The HTML can
come from a file, '-' for stdin, or the --html string flag.

Everything you publish is PUBLIC and permanent (there is no delete endpoint).
Use --password to gate reads behind a shared secret, and --assets to upload
every referenced local image/video in one pass.`),
		Example: trimNL(`
  ht-ml-pp-cli publish ./deck.html
  ht-ml-pp-cli publish ./report.html --assets --root ./public
  cat page.html | ht-ml-pp-cli publish - --agent --select url,site_id`),
		// happy-args drives the live-dogfood happy path with a literal HTML string
		// so the gate never depends on an example file existing on disk.
		Annotations: map[string]string{"pp:happy-args": "--html=<p>smoke</p>"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			html, srcLabel, filePath, err := readHTMLInput(cmd, args, htmlFlag)
			if err != nil {
				return usageErr(err)
			}

			publicContentWarn(cmd)
			if htmlxWriteGuard(cmd, flags, "publish HTML from "+srcLabel) {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"html_content": html}
			if passwordFlag != "" {
				body["password"] = passwordFlag
			}
			raw, _, err := c.Post(ctx, "/sites", body)
			if err != nil {
				var ae *client.APIError
				if As(err, &ae) {
					return mapSiteHTTPError(ae.StatusCode, []byte(ae.Body), flags)
				}
				return apiErr(err)
			}
			resp, err := parseSiteAPIResponse(raw)
			if err != nil {
				return apiErr(err)
			}
			if resp.SiteID == "" {
				return apiErr(fmt.Errorf("ht-ml.app returned no site_id: %s", string(raw)))
			}

			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			rec := store.SiteRecord{
				SiteID:    resp.SiteID,
				URL:       resp.URL,
				Status:    resp.Status,
				Title:     extractTitle(html),
				UpdateKey: resp.UpdateKey,
				Password:  passwordFlag,
				CreatedAt: nowRFC3339(),
			}
			if err := db.SaveSite(rec, html, true); err != nil {
				return fmt.Errorf("saving to local store: %w", err)
			}

			result := publishResult{
				SiteID:    resp.SiteID,
				URL:       resp.URL,
				Status:    resp.Status,
				Title:     rec.Title,
				KeyStored: resp.UpdateKey != "",
			}

			// Cache the referenced-asset list and optionally upload them.
			if assets, aerr := fetchSiteAssets(ctx, flags, resp.SiteID); aerr == nil {
				refs := make([]store.AssetRef, 0, len(assets.Assets))
				for _, a := range assets.Assets {
					refs = append(refs, store.AssetRef{SiteID: resp.SiteID, RelativePath: a.RelativePath, AssetType: a.AssetType})
				}
				_ = db.SaveAssets(resp.SiteID, refs)
				if withAssets && len(refs) > 0 {
					root := assetRoot
					if root == "" {
						root = assetRootFromFile(filePath)
					}
					uploaded, missing := uploadReferencedAssets(ctx, cmd, c.RequestBaseURL(), resp.SiteID, resp.UpdateKey, refs, root)
					result.AssetsUpload = uploaded
					result.AssetsMissing = missing
				} else if len(refs) > 0 {
					result.AssetsMissing = len(refs)
					result.Note = fmt.Sprintf("%d referenced asset(s) not yet uploaded; re-run with --assets or 'assets sync %s'", len(refs), resp.SiteID)
				}
			}

			return renderPublish(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&htmlFlag, "html", "", "HTML content as a literal string (instead of a file)")
	cmd.Flags().StringVar(&passwordFlag, "password", "", "Set a shared-secret password to make the site private")
	cmd.Flags().BoolVar(&withAssets, "assets", false, "Upload every referenced local image/video after publishing")
	cmd.Flags().StringVar(&assetRoot, "root", "", "Directory to resolve referenced asset paths from (default: the HTML file's directory)")
	return cmd
}

func renderPublish(cmd *cobra.Command, flags *rootFlags, result publishResult) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "%s  %s\n", green("published"), bold(result.URL))
		fmt.Fprintf(w, "site_id: %s\n", result.SiteID)
		if result.Title != "" {
			fmt.Fprintf(w, "title:   %s\n", result.Title)
		}
		if result.KeyStored {
			fmt.Fprintln(w, "update_key stored locally (back up with 'ht-ml-pp-cli keys export')")
		}
		if result.AssetsUpload > 0 {
			fmt.Fprintf(w, "assets:  %d uploaded\n", result.AssetsUpload)
		}
		if result.Note != "" {
			fmt.Fprintf(w, "note:    %s\n", result.Note)
		}
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}

// uploadReferencedAssets uploads each referenced asset that is not already live
// on the CDN. Returns (uploaded, stillMissing).
func uploadReferencedAssets(ctx context.Context, cmd *cobra.Command, baseURL, siteID, updateKey string, refs []store.AssetRef, root string) (int, int) {
	uploaded := 0
	missing := 0
	for _, a := range refs {
		if present, _ := liveAssetPresent(ctx, siteID, a.RelativePath); present {
			continue
		}
		data, name, rerr := resolveAssetFile(root, a.RelativePath)
		if rerr != nil {
			missing++
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s referenced but not found under %s: %v\n", a.RelativePath, root, rerr)
			continue
		}
		status, respBody, uerr := uploadAssetMultipart(ctx, baseURL, siteID, updateKey, a.RelativePath, data, name)
		if uerr != nil || status >= 400 {
			missing++
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: upload failed for %s (HTTP %d): %s %v\n", a.RelativePath, status, string(respBody), uerr)
			continue
		}
		uploaded++
	}
	return uploaded, missing
}

// assetRootFromFile returns the directory of the HTML source file, or "." when
// the HTML came from stdin or a literal string.
func assetRootFromFile(filePath string) string {
	if filePath == "" {
		return "."
	}
	return filepath.Dir(filePath)
}

// publishHTMLString creates a new site from an HTML string, stores it (capturing
// the once-only update_key), and renders the result. Callers MUST run
// htmlxWriteGuard before calling. Used by `new --publish`.
func publishHTMLString(cmd *cobra.Command, flags *rootFlags, html, title string) error {
	publicContentWarn(cmd)
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	raw, _, err := c.Post(ctx, "/sites", map[string]any{"html_content": html})
	if err != nil {
		var ae *client.APIError
		if As(err, &ae) {
			return mapSiteHTTPError(ae.StatusCode, []byte(ae.Body), flags)
		}
		return apiErr(err)
	}
	resp, err := parseSiteAPIResponse(raw)
	if err != nil {
		return apiErr(err)
	}
	if resp.SiteID == "" {
		return apiErr(fmt.Errorf("ht-ml.app returned no site_id: %s", string(raw)))
	}
	db, err := htmlxOpenStore(ctx, flags)
	if err != nil {
		return err
	}
	defer db.Close()
	rec := store.SiteRecord{
		SiteID:    resp.SiteID,
		URL:       resp.URL,
		Status:    resp.Status,
		Title:     firstNonEmpty(title, extractTitle(html)),
		UpdateKey: resp.UpdateKey,
		CreatedAt: nowRFC3339(),
	}
	if err := db.SaveSite(rec, html, true); err != nil {
		return fmt.Errorf("saving to local store: %w", err)
	}
	return renderPublish(cmd, flags, publishResult{
		SiteID:    resp.SiteID,
		URL:       resp.URL,
		Status:    resp.Status,
		Title:     rec.Title,
		KeyStored: resp.UpdateKey != "",
	})
}
