// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// modelFile is the normalized per-file row `files` prints, regardless of
// source. Note is used for source-specific caveats (e.g. Printables' file
// listing carries no per-file download URL on its own).
type modelFile struct {
	Source      string `json:"source"`
	ModelID     string `json:"model_id"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Format      string `json:"format,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	Note        string `json:"note,omitempty"`
}

func newFilesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files <source>:<id>",
		Short: "List the files available for a model",
		Long: `Lists a model's individual files (STL, 3MF, G-code, etc.) by source and ID.

Printables: the file list comes from the model-detail GraphQL query, which
does not include a per-file download URL. Run 'download' to resolve a
signed link and fetch the file, or 'download --dry-run' to preview the
request without downloading anything.

Thingiverse: each file includes a direct CDN URL when the API provides one.

Cults3D: the API has no file-listing endpoint at all — this command says so
plainly rather than guessing at a shape that doesn't exist.`,
		Example: `  printgoat-pp-cli files printables:12345
  printgoat-pp-cli files thingiverse:763622 --agent
  printgoat-pp-cli files cults3d:12345`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			source, id, perr := parseSourceID(args[0])
			if perr != nil {
				return usageErr(perr)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			switch source {
			case "printables":
				pf, err := printablesFiles(ctx, c, id)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				files := make([]modelFile, 0, len(pf))
				const note = "no per-file download URL in this listing; run 'download printables:<id>' to resolve one"
				for _, f := range pf {
					files = append(files, modelFile{
						Source:    "printables",
						ModelID:   id,
						ID:        f.ID,
						Name:      f.Name,
						Format:    f.Format,
						SizeBytes: f.SizeBytes,
						Note:      note,
					})
				}
				return outputModelFiles(cmd, flags, files)
			case "thingiverse":
				entries, err := thingiverseFiles(ctx, c, id)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				files := make([]modelFile, 0, len(entries))
				for _, e := range entries {
					files = append(files, modelFile{
						Source:      "thingiverse",
						ModelID:     id,
						ID:          rawIDString(e.ID),
						Name:        e.Name,
						Format:      inferFormatFromName(e.Name),
						SizeBytes:   e.Size,
						DownloadURL: thingiverseFileURL(e),
					})
				}
				return outputModelFiles(cmd, flags, files)
			case "cults3d":
				return outputCults3DFilesUnsupported(cmd, flags, id)
			default:
				return usageErr(fmt.Errorf("unknown source %q: must be one of printables, thingiverse, cults3d", source))
			}
		},
	}
	return cmd
}

// parseSourceID splits a "<source>:<id>" positional argument, lower-casing
// and validating the source name against the three sites printgoat knows.
func parseSourceID(arg string) (string, string, error) {
	source, id, ok := strings.Cut(arg, ":")
	source = strings.ToLower(strings.TrimSpace(source))
	id = strings.TrimSpace(id)
	if !ok || source == "" || id == "" {
		return "", "", fmt.Errorf("expected <source>:<id> (e.g. printables:12345), got %q", arg)
	}
	switch source {
	case "printables", "thingiverse", "cults3d":
	default:
		return "", "", fmt.Errorf("unknown source %q: must be one of printables, thingiverse, cults3d", source)
	}
	return source, id, nil
}

func outputModelFiles(cmd *cobra.Command, flags *rootFlags, files []modelFile) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(files) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "No files found.")
			return nil
		}
		return printAutoTable(cmd.OutOrStdout(), modelFilesToRows(files))
	}
	// printJSONFiltered defaults its envelope's meta.source to "local"; this
	// command only ever hits live APIs, so marshal and route through
	// printOutputWithFlagsMeta directly to report "live" instead (matching
	// generated read commands' convention).
	raw, err := marshalJSONNoEscape(files)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": "live"})
}

func modelFilesToRows(files []modelFile) []map[string]any {
	rows := make([]map[string]any, len(files))
	for i, f := range files {
		row := map[string]any{
			"source":   f.Source,
			"model_id": f.ModelID,
			"id":       f.ID,
			"name":     f.Name,
		}
		if f.Format != "" {
			row["format"] = f.Format
		}
		if f.SizeBytes != 0 {
			row["size_bytes"] = f.SizeBytes
		}
		if f.DownloadURL != "" {
			row["download_url"] = f.DownloadURL
		}
		if f.Note != "" {
			row["note"] = f.Note
		}
		rows[i] = row
	}
	return rows
}

// outputCults3DFilesUnsupported reports, plainly and without a network
// call, that Cults3D has no file-listing endpoint — the API excludes
// third-party file access by design, so there is nothing here worth
// guessing at.
func outputCults3DFilesUnsupported(cmd *cobra.Command, flags *rootFlags, id string) error {
	const note = "Cults3D does not expose a file-listing endpoint via its API (by design). Use 'search --source cults3d' to find the listing page, or 'download cults3d:<id> --open' to open it in a browser."
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"source":   "cults3d",
			"model_id": id,
			"files":    []modelFile{},
			"note":     note,
		}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), note)
	return nil
}
