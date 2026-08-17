// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: `snapshot create` pins the exact local files (by hash)
// used for a print job, read from the parallel download command's
// printgoat_downloads table, for later proof via `snapshot verify`.

package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

type snapshotFileEntry struct {
	Source    string `json:"source"`
	ModelID   string `json:"model_id"`
	FileName  string `json:"file_name"`
	SHA256    string `json:"sha256"`
	LocalPath string `json:"local_path"`
}

func newNovelSnapshotCreateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "create <name> <model-keys...>",
		Short:       "Pin the exact local files used for a print job, by hash, so you can prove what you used later.",
		Example:     "  printgoat-pp-cli snapshot create batch-march-orders printables:3161 thingiverse:763622 --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("snapshot create requires a <name> and at least one <model-key>\nUsage: %s <name> <model-keys...>", cmd.CommandPath()))
			}
			name := args[0]

			type ref struct{ source, id string }
			refs := make([]ref, 0, len(args)-1)
			for _, a := range args[1:] {
				source, id, perr := parseModelRef(a)
				if perr != nil {
					return usageErr(perr)
				}
				refs = append(refs, ref{source, id})
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			allDownloads, derr := readDownloadRecords(s.DB())
			if derr != nil {
				return fmt.Errorf("reading local downloads: %w", derr)
			}
			byKey := map[string][]downloadRecord{}
			for _, d := range allDownloads {
				k := modelKey(d.Source, d.ModelID)
				byKey[k] = append(byKey[k], d)
			}

			var files []snapshotFileEntry
			var notFound []string
			for _, r := range refs {
				k := modelKey(r.source, r.id)
				records := byKey[k]
				if len(records) == 0 {
					notFound = append(notFound, k)
					continue
				}
				for _, rec := range records {
					fileName := rec.LocalPath
					if fileName != "" {
						fileName = filepath.Base(fileName)
					}
					files = append(files, snapshotFileEntry{
						Source: rec.Source, ModelID: rec.ModelID,
						FileName: fileName, SHA256: rec.SHA256, LocalPath: rec.LocalPath,
					})
				}
			}

			filesJSON, merr := marshalJSONNoEscape(files)
			if merr != nil {
				return fmt.Errorf("encoding snapshot: %w", merr)
			}
			createdAt := time.Now().UTC().Format(time.RFC3339)
			if _, err := s.DB().ExecContext(ctx,
				`INSERT INTO printgoat_job_snapshots (name, files_json, created_at) VALUES (?, ?, ?)
				 ON CONFLICT(name) DO UPDATE SET files_json = excluded.files_json, created_at = excluded.created_at`,
				name, string(filesJSON), createdAt,
			); err != nil {
				return fmt.Errorf("recording snapshot: %w", err)
			}

			out := map[string]any{
				"name":       name,
				"file_count": len(files),
				"files":      files,
				"created_at": createdAt,
			}
			if len(notFound) > 0 {
				out["not_found_in_downloads"] = notFound
			}
			if len(files) == 0 {
				out["message"] = "no downloaded files found for the given model keys; nothing pinned"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
