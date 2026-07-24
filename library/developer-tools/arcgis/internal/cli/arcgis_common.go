// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/arcgis"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/config"

	"github.com/spf13/cobra"
)

// newArcClient builds an ArcGIS protocol client from root flags and config.
// The optional token comes from ARCGIS_TOKEN or the config file; public
// endpoints need none.
func newArcClient(flags *rootFlags) (*arcgis.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 0 // client applies its own default
	}
	return arcgis.New(timeout, cfg.ArcgisToken, flags.rateLimit), nil
}

// openOut returns a writer for --out <path> or stdout, plus a close func.
func openOut(cmd *cobra.Command, path string) (io.Writer, func(), error) {
	if path == "" || path == "-" {
		return cmd.OutOrStdout(), func() {}, nil
	}
	f, err := os.Create(path) // #nosec G304 -- path is the user-supplied --out destination; writing to it is the flag's purpose.
	if err != nil {
		return nil, nil, fmt.Errorf("creating %s: %w", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}

// writeFeatureCollection streams features as a GeoJSON FeatureCollection.
func writeFeatureCollection(w io.Writer, feats []arcgis.Feature, geomType string) error {
	fc := map[string]any{"type": "FeatureCollection"}
	arr := make([]map[string]any, 0, len(feats))
	for _, f := range feats {
		gj, err := arcgis.FeatureToGeoJSON(f, geomType)
		if err != nil {
			return err
		}
		arr = append(arr, gj)
	}
	fc["features"] = arr
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fc)
}

// writeJSONL streams features as newline-delimited GeoJSON.
func writeJSONL(w io.Writer, feats []arcgis.Feature, geomType string) error {
	enc := json.NewEncoder(w)
	for _, f := range feats {
		gj, err := arcgis.FeatureToGeoJSON(f, geomType)
		if err != nil {
			return err
		}
		if err := enc.Encode(gj); err != nil {
			return err
		}
	}
	return nil
}

// writeCSV writes feature attributes as CSV using the given column order.
func writeCSV(w io.Writer, feats []arcgis.Feature, cols []string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(cols); err != nil {
		return err
	}
	for _, f := range feats {
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = arcgis.AttrToString(f.Attributes[c])
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
