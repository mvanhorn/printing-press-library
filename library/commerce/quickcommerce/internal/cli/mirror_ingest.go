// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelMirrorIngestCmd(flags *rootFlags) *cobra.Command {
	var useStdin bool
	var dbPath string
	var location, query, platform string
	cmd := &cobra.Command{
		Use: "ingest", Short: "Persist real QuickCommerce command responses and metadata into the local SQLite mirror.",
		Example:     "  quickcommerce-pp-cli mirror ingest --stdin --location 12.9021,77.6639 --query milk --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mirror ingest")
			}
			if !useStdin {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--stdin is required; pipe JSON from a QuickCommerce command"))
			}
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return usageErr(fmt.Errorf("reading stdin: %w", err))
			}
			if strings.TrimSpace(string(raw)) == "" {
				return qcPrint(cmd.OutOrStdout(), flags, map[string]any{"ingested": 0, "resources": map[string]int{}, "note": "stdin was empty; nothing recorded"}, nil)
			}
			if !json.Valid(raw) {
				return usageErr(fmt.Errorf("stdin must contain one valid JSON response"))
			}
			observations := qcExtractObservations(raw)
			if len(observations) == 0 {
				return usageErr(fmt.Errorf("stdin JSON contained no recognized product, item, ETA, credit, or platform observations"))
			}
			if location != "" {
				_, _, canonical, err := parseQCLocation(location)
				if err != nil {
					return usageErr(err)
				}
				location = canonical
			}
			for i := range observations {
				if location != "" {
					observations[i].Location = location
				}
				if query != "" {
					observations[i].Query = query
				}
				if platform != "" {
					observations[i].Platform = platform
				}
			}
			path := qcDBPath(dbPath)
			if _, err := os.Stat(path); err != nil && !os.IsNotExist(err) {
				return apiErr(err)
			}
			db, err := openQCCreatable(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer db.Close()
			resources := map[string]int{}
			for _, in := range observations {
				if err := qcSaveInput(cmd.Context(), db, in); err != nil {
					return apiErr(fmt.Errorf("saving %s observation: %w", in.Resource, err))
				}
				resources[in.Resource]++
			}
			names := make([]string, 0, len(resources))
			for name := range resources {
				names = append(names, name)
			}
			sort.Strings(names)
			view := map[string]any{"ingested": len(observations), "resources": resources, "resource_order": names, "db": path}
			return qcPrint(cmd.OutOrStdout(), flags, view, []map[string]any{view})
		},
	}
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read a real QuickCommerce JSON response from stdin")
	cmd.Flags().StringVar(&location, "location", "", "Override observation coordinates as latitude,longitude")
	cmd.Flags().StringVar(&query, "query", "", "Override the saved product query")
	cmd.Flags().StringVar(&platform, "platform", "", "Override the saved platform name")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path")
	return cmd
}
