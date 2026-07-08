// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: the CSV export button self-hosted Umami never shipped.
// Wraps GET /api/websites/{id}/export and writes a real .zip file.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newExportCmd(flags *rootFlags) *cobra.Command {
	var period string
	var outPath string
	cmd := &cobra.Command{
		Use:   "export [site]",
		Short: "Export a website's data as a ZIP of CSVs (events, pages, referrers, browsers, os, devices, countries)",
		Long: `Download a website's collected data as a ZIP archive of CSV files.
This is the export button that self-hosted Umami's UI does not have.

The site argument accepts a website UUID or its domain.`,
		Example:     "  umami-pp-cli export restodom.fr --period 90d --out restodom-q2.zip",
		Annotations: map[string]string{"pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=7d;--out=/tmp/umami-export-test.zip"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export website data to a zip file")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			websiteID, err := resolveWebsiteID(ctx, c, args[0])
			if err != nil {
				return err
			}
			w, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			params := map[string]string{
				"startAt": strconv.FormatInt(w.StartMs, 10),
				"endAt":   strconv.FormatInt(w.EndMs, 10),
			}
			data, err := c.Get(ctx, "/api/websites/"+websiteID+"/export", params)
			if err != nil {
				return apiErr(fmt.Errorf("export failed: %w", err))
			}
			zipBytes, err := decodeExportPayload(data)
			if err != nil {
				return apiErr(err)
			}
			if outPath == "" {
				outPath = fmt.Sprintf("umami-export-%s-%s.zip", strings.ReplaceAll(args[0], "/", "_"), period)
			}
			// 0o600: exports contain per-visitor analytics rows (IPs, geo,
			// referrers) — keep them owner-only; users can chmod to share.
			if err := os.WriteFile(outPath, zipBytes, 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}
			out := map[string]any{"ok": true, "file": outPath, "bytes": len(zipBytes), "period": period}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "90d", "Period to export (7d, 30d, 90d, this-month, ...)")
	cmd.Flags().StringVar(&outPath, "out", "", "Output .zip path (default umami-export-<site>-<period>.zip)")
	return cmd
}

// decodeExportPayload handles every shape the transport can hand us: the
// client's binary envelope ({"_pp_binary":true,"data":"<b64>"}) emitted for
// application/zip responses, a {"zip":"<b64>"} JSON field, a bare base64
// string, or raw ZIP bytes.
func decodeExportPayload(data []byte) ([]byte, error) {
	if len(data) >= 4 && string(data[:2]) == "PK" {
		return data, nil
	}
	var binEnvelope struct {
		PPBinary bool   `json:"_pp_binary"`
		Data     string `json:"data"`
	}
	if err := json.Unmarshal(data, &binEnvelope); err == nil && binEnvelope.PPBinary && binEnvelope.Data != "" {
		decoded, err := base64.StdEncoding.DecodeString(binEnvelope.Data)
		if err != nil {
			return nil, fmt.Errorf("decoding binary export envelope: %w", err)
		}
		if len(decoded) < 2 || string(decoded[:2]) != "PK" {
			return nil, fmt.Errorf("binary export envelope did not contain a ZIP archive")
		}
		return decoded, nil
	}
	var envelope struct {
		Zip string `json:"zip"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Zip != "" {
		decoded, err := base64.StdEncoding.DecodeString(envelope.Zip)
		if err != nil {
			return nil, fmt.Errorf("decoding base64 export payload: %w", err)
		}
		return decoded, nil
	}
	// Some builds return the base64 string bare.
	trimmed := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) >= 2 && string(decoded[:2]) == "PK" {
		return decoded, nil
	}
	return nil, fmt.Errorf("unrecognized export payload (%d bytes); expected ZIP or base64 envelope", len(data))
}
