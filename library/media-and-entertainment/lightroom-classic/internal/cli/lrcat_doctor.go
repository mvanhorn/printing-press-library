// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: catalog health checks wired into the framework doctor.
package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/internal/lrcat"
)

// collectCatalogReport fills doctor report keys for the local catalog:
// "catalog" (status line) and "catalog_health" (hygiene sweep details).
// The sweep is strictly read-only.
func collectCatalogReport(ctx context.Context, flags *rootFlags, report map[string]any) {
	path, err := lrcat.Resolve(flags.catalog)
	if err != nil {
		report["catalog"] = fmt.Sprintf("not found (%s)", err)
		return
	}
	cat, err := lrcat.Open(ctx, path)
	if err != nil {
		report["catalog"] = fmt.Sprintf("error: %s", err)
		return
	}
	defer cat.Close()
	health, err := cat.Health(ctx, 50)
	if err != nil {
		report["catalog"] = fmt.Sprintf("opened but health sweep failed: %s", err)
		return
	}
	verdict := fmt.Sprintf("ok — %d photos at %s", health.TotalPhotos, path)
	if health.MissingCount > 0 {
		verdict = fmt.Sprintf("WARN %d masters missing on disk (of %d photos) at %s", health.MissingCount, health.TotalPhotos, path)
	}
	report["catalog"] = verdict
	report["catalog_health"] = map[string]any{
		"catalog_path":      health.CatalogPath,
		"total_photos":      health.TotalPhotos,
		"missing_count":     health.MissingCount,
		"missing_masters":   health.MissingMasters,
		"no_capture_time":   health.NoCaptureTime,
		"orphan_keywords":   health.OrphanKeywords,
		"empty_collections": health.EmptyCollections,
		"scanned_files":     health.ScannedFiles,
	}
}

// renderCatalogHealth prints the hygiene sweep for human doctor output.
func renderCatalogHealth(w io.Writer, rep map[string]any) {
	fmt.Fprintln(w, "  Catalog health:")
	fmt.Fprintf(w, "    photos: %v   missing on disk: %v   no capture time: %v\n",
		rep["total_photos"], rep["missing_count"], rep["no_capture_time"])
	if orphans, ok := rep["orphan_keywords"].([]string); ok && len(orphans) > 0 {
		fmt.Fprintf(w, "    orphan keywords (%d): first few %s\n", len(orphans), quoteJoin(orphans[:min(3, len(orphans))]))
	}
	if empties, ok := rep["empty_collections"].([]string); ok && len(empties) > 0 {
		fmt.Fprintf(w, "    empty collections (%d): first few %s\n", len(empties), quoteJoin(empties[:min(3, len(empties))]))
	}
	if missing, ok := rep["missing_masters"].([]string); ok && len(missing) > 0 {
		fmt.Fprintf(w, "    first missing: %s\n", missing[0])
	}
}

// quoteJoin renders a preview list as quoted, comma-separated items so
// multi-word names stay unambiguous in human output.
func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = strconv.Quote(s)
	}
	return strings.Join(quoted, ", ")
}
