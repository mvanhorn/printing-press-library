// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: license conflict audit, either for one model or the whole
// local download library.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

type licenseAuditEntry struct {
	Source   string `json:"source"`
	ModelID  string `json:"model_id"`
	Name     string `json:"name,omitempty"`
	License  string `json:"license"`
	Conflict bool   `json:"conflict"`
	Reason   string `json:"reason,omitempty"`
	Error    string `json:"error,omitempty"`
}

// licenseConflicts reports whether a license string looks incompatible with
// a commercial-use intent. Deliberately simple substring matching per the
// spec: any "nc" token or "non-commercial" phrase flags a conflict. License
// naming across these three sites is inconsistent free text, so this is a
// heuristic, not a legal determination.
func licenseConflicts(license, intent string) (bool, string) {
	if intent != "commercial" {
		return false, ""
	}
	lower := strings.ToLower(license)
	if lower == "" || lower == "unknown" {
		return false, ""
	}
	if strings.Contains(lower, "non-commercial") || strings.Contains(lower, "noncommercial") {
		return true, "license text mentions non-commercial use"
	}
	for _, tok := range strings.FieldsFunc(lower, func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	}) {
		if tok == "nc" {
			return true, "license code includes an NC (non-commercial) restriction"
		}
	}
	return false, ""
}

// pp:data-source computed
func newNovelLicenseAuditCmd(flags *rootFlags) *cobra.Command {
	var flagLibrary bool
	var flagIntent string

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Flag license conflicts across your entire local library against how you actually intend to use the files.",
		Example:     "  printgoat-pp-cli license audit --library --intent commercial --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if !flagLibrary && len(args) == 0 {
				return usageErr(fmt.Errorf("license audit requires either --library or a <model-key> argument\nUsage: %s [--library] [<model-key>]", cmd.CommandPath()))
			}
			if flagIntent != "" && flagIntent != "personal" && flagIntent != "commercial" {
				return usageErr(fmt.Errorf("invalid --intent %q: must be \"personal\" or \"commercial\"", flagIntent))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var entries []licenseAuditEntry

			if flagLibrary {
				entries, err = auditLibraryLicenses(ctx, c, flagIntent)
				if err != nil {
					return err
				}
			} else {
				source, id, perr := parseModelRef(args[0])
				if perr != nil {
					return usageErr(perr)
				}
				entries = append(entries, auditOneModel(ctx, c, source, id, flagIntent))
			}

			conflicts := 0
			for _, e := range entries {
				if e.Conflict {
					conflicts++
				}
			}
			out := map[string]any{
				"intent":    flagIntent,
				"audited":   len(entries),
				"conflicts": conflicts,
				"entries":   entries,
			}
			if flagLibrary && len(entries) == 0 {
				out["message"] = "no downloads recorded yet; run a download first, or audit a single model with a <model-key> argument"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&flagLibrary, "library", false, "Audit every model recorded in the local downloads table instead of a single model key")
	cmd.Flags().StringVar(&flagIntent, "intent", "", "How you intend to use the files: personal or commercial (omit to just report licenses)")
	return cmd
}

// auditOneModel re-fetches a single model's current license from its source
// API and evaluates it against intent. Never returns an error: fetch
// failures are folded into the entry's Error field so a single bad model
// (deleted, rate-limited, source down) doesn't abort the whole audit.
func auditOneModel(ctx context.Context, c *client.Client, source, id, intent string) licenseAuditEntry {
	entry := licenseAuditEntry{Source: source, ModelID: id, License: "unknown"}
	detail, err := fetchModelDetail(ctx, c, source, id)
	if err != nil {
		entry.Error = err.Error()
		return entry
	}
	if !detail.Found {
		entry.Error = "model not found (delisted or deleted)"
		return entry
	}
	entry.Name = detail.Name
	entry.License = detail.License
	entry.Conflict, entry.Reason = licenseConflicts(detail.License, intent)
	return entry
}

// auditLibraryLicenses reads every distinct model recorded in the parallel
// download command's printgoat_downloads table and re-fetches each one's
// current license from its source API. The table is read-only here and may
// not exist yet if the download command hasn't run — that is reported as a
// friendly message, not a crash.
func auditLibraryLicenses(ctx context.Context, c *client.Client, intent string) ([]licenseAuditEntry, error) {
	dbPath := defaultDBPath("printgoat-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	defer s.Close()

	rows, err := s.DB().QueryContext(ctx, `SELECT DISTINCT source, model_id FROM printgoat_downloads`)
	if err != nil {
		if isNoSuchTable(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading local downloads: %w", err)
	}
	defer rows.Close()

	var pairs [][2]string
	for rows.Next() {
		var source, modelID string
		if err := rows.Scan(&source, &modelID); err != nil {
			return nil, fmt.Errorf("scanning local downloads: %w", err)
		}
		pairs = append(pairs, [2]string{source, modelID})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading local downloads: %w", err)
	}

	entries := make([]licenseAuditEntry, 0, len(pairs))
	for _, p := range pairs {
		entries = append(entries, auditOneModel(ctx, c, p[0], p[1], intent))
	}
	return entries, nil
}
