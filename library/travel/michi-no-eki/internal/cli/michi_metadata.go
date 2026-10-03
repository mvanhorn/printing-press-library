// Copyright 2026 zjsng. Licensed under Apache-2.0.
// Source-authored adapter for the approved low-level metadata/handoff commands.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, kind := range []string{"stations", "bulletins"} {
			leaf, _, err := root.Find([]string{kind, "get"})
			if err != nil || leaf == root || leaf.RunE == nil {
				continue
			}
			original := leaf.RunE
			leaf.RunE = func(cmd *cobra.Command, args []string) error {
				// Keep generated bare/help, missing-input and dry-run semantics.
				if dryRunOK(flags) || !cmd.Flags().Changed("id") {
					return original(cmd, args)
				}
				if len(args) > 0 {
					return usageErr(fmt.Errorf("use --id rather than positional arguments"))
				}
				id, err := cmd.Flags().GetString("id")
				if err != nil {
					return usageErr(err)
				}
				if strings.Contains(id, ",") {
					return usageErr(fmt.Errorf("one numeric station/notice ID is required"))
				}
				if err := michi.ValidateIDs(id); err != nil {
					return usageErr(err)
				}
				if strings.TrimSpace(id) != id {
					return usageErr(fmt.Errorf("station/notice ID must contain digits only"))
				}
				if flags.dataSource == "local" {
					return usageErr(fmt.Errorf("no local source for HTML page metadata"))
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				path := "/stations/views/" + id
				if kind == "bulletins" {
					path = "/notices/views/" + id
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				raw, err := c.GetWithHeadersNoCache(ctx, path, nil, map[string]string{client.HTMLResponseHeader: "true"})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				sourceURL := strings.TrimRight(c.RequestBaseURL(), "/") + path
				at := time.Now().In(time.FixedZone("JST", 9*3600)).Format(time.RFC3339)
				data, err := michiPageMetadata(raw, kind, id, sourceURL, at)
				if err != nil {
					return err
				}
				flags.agentSource = "live"
				return printJSONFiltered(cmd.OutOrStdout(), data, flags)
			}
		}
	})
}

func michiPageMetadata(raw []byte, kind, id, sourceURL, at string) (map[string]any, error) {
	if len(raw) > 5*1024*1024 {
		return nil, fmt.Errorf("provider metadata HTML exceeds 5 MiB bound after download")
	}
	data, err := extractHTMLResponse(raw, htmlExtractionOptions{Mode: "page", BaseURL: sourceURL})
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	// Some plain 200 denial pages lack the generator's vendor-specific markers.
	pageTitle, _ := out["title"].(string)
	denialTitle := strings.ToLower(strings.TrimSpace(pageTitle))
	if strings.Contains(denialTitle, "access denied") || denialTitle == "forbidden" || denialTitle == "403 forbidden" {
		return nil, fmt.Errorf("provider returned an access-denial page, not station/notice metadata")
	}

	// Generic link-image fallback is unreliable; detail metadata doesn't emit links.
	delete(out, "links")
	out["id"] = id
	out["source_url"] = sourceURL
	out["observed_at"] = at
	out["date_timezone"] = "Asia/Tokyo"
	out["entity_fields_status"] = "unknown"
	path := "/stations/views/" + id
	if kind == "bulletins" {
		path = "/notices/views/" + id
	}
	out["url"] = michi.Origin + path
	out["canonical_url"] = michi.Origin + path
	out["page_title"] = out["title"]
	if kind == "stations" {
		out["name"] = "unknown"
		if st, err := michi.ParseStation(raw, id, sourceURL, at); err == nil {
			out["name"] = st.Name
			out["entity_fields_status"] = "observed"
		}
	} else {
		out["notice_title"] = "unknown"
		out["published_date"] = "unknown"
		if n, err := michi.ParseNotice(raw, id, sourceURL, at); err == nil {
			out["notice_title"] = n.Title
			out["published_date"] = n.PublishedDate
			out["station_ids"] = n.StationIDs
			out["entity_fields_status"] = "observed"
		}
	}
	out["evidence_scope"] = "Requested page metadata and canonical handoff; entity fields are observed only when the source layout is recognized. This does not establish registration, overnight permission or current service availability."
	return out, nil
}
