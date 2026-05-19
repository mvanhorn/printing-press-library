// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newMetadataFetchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch [nasa_id]",
		Short: "Fetch and clean the metadata sidecar (not just the URL)",
		Long: `Follow the /metadata/{nasa_id} indirection: fetch the location URL,
GET the metadata.json sidecar, flatten the AVAIL:* and EXIF/JFIF/File fields,
and drop the leak fields the upstream sidecar exposes (SourceFile,
File:Directory, File:DirectoryName, AVAIL:Owner — the curator's login name).`,
		Example:     "  nasa-images-pp-cli metadata fetch PIA24439 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			nasaID := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch metadata for %q\n", nasaID)
				return nil
			}
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/metadata/"+nasaID, nil)
			if err != nil {
				return fmt.Errorf("calling /metadata/%s: %w", nasaID, err)
			}
			var loc struct {
				Location string `json:"location"`
			}
			if err := json.Unmarshal(raw, &loc); err != nil {
				return fmt.Errorf("parsing metadata indirection: %w", err)
			}
			if loc.Location == "" {
				return fmt.Errorf("metadata response has no location URL")
			}
			body, err := httpGetBody(ctx, flags, loc.Location)
			if err != nil {
				return fmt.Errorf("fetching metadata sidecar: %w", err)
			}
			var sidecar map[string]any
			if err := json.Unmarshal(body, &sidecar); err != nil {
				return fmt.Errorf("parsing sidecar JSON: %w", err)
			}
			cleaned := cleanSidecar(sidecar)
			cleaned["nasa_id"] = nasaID
			cleaned["sidecar_url"] = upgradeToHTTPS(loc.Location)
			return flags.printJSON(cmd, cleaned)
		},
	}
	return cmd
}

// leakFields are paths in the upstream sidecar JSON that reveal NASA's ingest-
// server filesystem and curator account name. They are noise to every consumer
// (journalists, agents, editors) and are stripped before we return the sidecar.
var leakFields = map[string]bool{
	"SourceFile":         true,
	"File:Directory":     true,
	"File:DirectoryName": true,
	"AVAIL:Owner":        true,
}

// cleanSidecar drops leakFields and trims empty AVAIL:* fields.
// Returns a new map; the input is not modified.
func cleanSidecar(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if leakFields[k] {
			continue
		}
		if strings.HasPrefix(k, "AVAIL:") {
			// Empty strings and nil aren't useful editorial fields; drop them.
			if v == nil {
				continue
			}
			if s, ok := v.(string); ok && s == "" {
				continue
			}
		}
		out[k] = v
	}
	return out
}
