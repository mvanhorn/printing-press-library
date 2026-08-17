// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `export` writes routes/trips to GPX/TCX/CSV/KML files. Synthesized formats are
// built from the live track-point detail; --native streams Ride with GPS's own
// file render (incl. .fit). Hand-authored transcendence command.
package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/store"
	"github.com/spf13/cobra"
)

type exportedFile struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	Format string `json:"format"`
	Bytes  int    `json:"bytes"`
	Error  string `json:"error,omitempty"`
}

type exportView struct {
	Type     string         `json:"type"`
	Format   string         `json:"format"`
	Native   bool           `json:"native"`
	OutDir   string         `json:"out_dir"`
	Exported []exportedFile `json:"exported"`
	Count    int            `json:"count"`
	Note     string         `json:"note,omitempty"`
}

var synthFormats = map[string]bool{"gpx": true, "tcx": true, "csv": true}
var nativeFormats = map[string]bool{"gpx": true, "tcx": true, "fit": true, "kml": true}

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var assetType string
	var format string
	var out string
	var native bool
	var ids []string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export many routes or trips to GPX, TCX, CSV, or KML files in one command — with cue sheets included",
		Long: `Export routes or trips to files ready for a bike computer (Garmin, Wahoo, Hammerhead).

Synthesized formats (gpx, tcx, csv) are built from the live track-point detail and
include cue sheets where present. --native streams Ride with GPS's own file render
and additionally supports fit and kml.

Targets come from --id (repeatable) or, when omitted, the locally synced library
(bounded by --limit). Run 'ridewithgps-pp-cli sync --resources routes,trips' first
to export the whole library without listing ids.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli export --type routes --format gpx --out ./gpx
  ridewithgps-pp-cli export --type trips --format csv --id 12345678
  ridewithgps-pp-cli export --type routes --native --format fit --out ./fit
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--type=routes;--limit=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export routes/trips to files")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			assetType = strings.ToLower(strings.TrimSpace(assetType))
			if assetType == "" {
				assetType = "routes"
			}
			if assetType != "routes" && assetType != "trips" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--type must be 'routes' or 'trips'"))
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "gpx"
			}
			if native {
				if !nativeFormats[format] {
					return usageErr(fmt.Errorf("--native supports gpx, tcx, fit, kml (got %q)", format))
				}
			} else if !synthFormats[format] {
				return usageErr(fmt.Errorf("format %q requires --native (synthesized formats: gpx, tcx, csv)", format))
			}
			if out == "" {
				out = "."
			}
			if limit <= 0 {
				limit = 50
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := exportView{Type: assetType, Format: format, Native: native, OutDir: out, Exported: make([]exportedFile, 0)}

			// Verify (mock) mode: report the plan without touching DB/API/disk.
			if cliutil.IsVerifyEnv() {
				view.Note = "verify mode: no files written"
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			// Resolve target ids: explicit --id, else local mirror enumeration.
			// "No targets" is an empty result (exit 0 with a hint), not a usage error.
			if dbPath == "" {
				dbPath = defaultDBPath("ridewithgps-pp-cli")
			}
			targetIDs := append([]string{}, ids...)
			if len(targetIDs) == 0 {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no --id given and no local mirror at %s\nrun: ridewithgps-pp-cli sync --resources %s --db %s, or pass --id\n", dbPath, assetType, dbPath)
					view.Note = "no targets: provide --id or sync the local mirror"
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				db, err := store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return fmt.Errorf("opening local database: %w", err)
				}
				rows, err := db.DB().QueryContext(ctx, fmt.Sprintf("SELECT id FROM %s ORDER BY updated_at DESC LIMIT ?", assetType), limit)
				if err != nil {
					_ = db.Close()
					return fmt.Errorf("listing %s: %w", assetType, err)
				}
				for rows.Next() {
					var id string
					if err := rows.Scan(&id); err == nil && id != "" {
						targetIDs = append(targetIDs, id)
					}
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					_ = db.Close()
					return fmt.Errorf("iterating %s: %w", assetType, err)
				}
				_ = rows.Close()
				_ = db.Close()
				if len(targetIDs) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "no %s in the local mirror; run 'ridewithgps-pp-cli sync --resources %s' or pass --id\n", assetType, assetType)
					view.Note = "no targets in local mirror"
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
			}

			// Curtail under live-dogfood to fit the per-command timeout.
			if cliutil.IsDogfoodEnv() && len(targetIDs) > 1 {
				targetIDs = targetIDs[:1]
			}

			if err := os.MkdirAll(out, 0o750); err != nil {
				return fmt.Errorf("creating output dir %q: %w", out, err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			for _, id := range targetIDs {
				ef := exportAsset(ctx, c, assetType, id, format, native, out)
				view.Exported = append(view.Exported, ef)
			}
			for _, ef := range view.Exported {
				if ef.Error == "" {
					view.Count++
				}
			}
			if view.Count == 0 {
				view.Note = "no files exported"
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, ef := range view.Exported {
				if ef.Error != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: FAILED — %s\n", ef.ID, ef.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s -> %s (%d bytes)\n", ef.ID, ef.File, ef.Bytes)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %d of %d %s to %s\n", view.Count, len(targetIDs), assetType, out)
			return nil
		},
	}
	cmd.Flags().StringVar(&assetType, "type", "", "What to export: routes or trips (required)")
	cmd.Flags().StringVar(&format, "format", "gpx", "Output format: gpx, tcx, csv (synthesized) or gpx, tcx, fit, kml (with --native)")
	cmd.Flags().StringVar(&out, "out", ".", "Output directory")
	cmd.Flags().BoolVar(&native, "native", false, "Stream Ride with GPS's own file render (enables fit/kml)")
	cmd.Flags().StringArrayVar(&ids, "id", nil, "Specific route/trip id to export (repeatable)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max assets to export when enumerating the local mirror")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local mirror)")
	return cmd
}

func singularType(t string) string {
	if strings.HasSuffix(t, "s") {
		return t[:len(t)-1]
	}
	return t
}

// exportAsset fetches a single route/trip and writes it to disk in the chosen
// format. Shared by `export` and `events routes`.
func exportAsset(ctx context.Context, c *client.Client, assetType, id, format string, native bool, outDir string) exportedFile {
	ef := exportedFile{ID: id, Format: format}
	// Guard against path traversal: --id is user-controlled and flows into both
	// the output filename and the request path. Ride with GPS ids are numeric;
	// reject anything that could escape --out or rewrite the request path.
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		ef.Error = "invalid id (must not contain path separators or '..')"
		return ef
	}
	ef.File = filepath.Join(outDir, fmt.Sprintf("%s-%s.%s", singularType(assetType), id, format))

	var data []byte
	if native {
		raw, err := c.Get(ctx, fmt.Sprintf("/%s/%s.%s", assetType, id, format), nil)
		if err != nil {
			ef.Error = err.Error()
			return ef
		}
		data, err = decodeNativeExport(raw)
		if err != nil {
			ef.Error = fmt.Sprintf("decoding native export: %v", err)
			return ef
		}
	} else {
		raw, err := c.Get(ctx, fmt.Sprintf("/api/v1/%s/%s.json", assetType, id), nil)
		if err != nil {
			ef.Error = err.Error()
			return ef
		}
		key := "route"
		if assetType == "trips" {
			key = "trip"
		}
		detail, err := unwrapAssetDetail(raw, key)
		if err != nil {
			ef.Error = fmt.Sprintf("parsing detail: %v", err)
			return ef
		}
		if len(detail.TrackPoints) == 0 {
			ef.Error = "no track points in detail response"
			return ef
		}
		name := detail.Name
		if name == "" {
			name = singularType(assetType) + " " + id
		}
		switch format {
		case "gpx":
			data = []byte(buildGPX(name, detail.TrackPoints, detail.CoursePoints))
		case "tcx":
			data = []byte(buildTCX(name, detail.TrackPoints, 0))
		case "csv":
			data = []byte(buildCSV(detail.TrackPoints))
		}
	}
	if err := os.WriteFile(ef.File, data, 0o600); err != nil {
		ef.Error = err.Error()
		return ef
	}
	ef.Bytes = len(data)
	return ef
}

func decodeNativeExport(raw json.RawMessage) ([]byte, error) {
	var envelope struct {
		PPBinary bool   `json:"_pp_binary"`
		Encoding string `json:"encoding"`
		Bytes    int    `json:"bytes"`
		Data     string `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || !envelope.PPBinary {
		return []byte(raw), nil
	}
	if envelope.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported binary response encoding %q", envelope.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(envelope.Data)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 payload: %w", err)
	}
	if len(data) != envelope.Bytes {
		return nil, fmt.Errorf("binary response size mismatch: got %d bytes, expected %d", len(data), envelope.Bytes)
	}
	return data, nil
}
