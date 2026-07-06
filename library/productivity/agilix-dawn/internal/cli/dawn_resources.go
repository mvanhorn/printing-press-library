// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: list the media/material resources attached to a course.
//
// Files live in concept.resource[]. Uploading is a browser-mediated signed-URL
// flow (POST /api/resource expects JSON, not bytes, and returns an upload
// target that the web app PUTs to) that this CLI does not replicate; this
// command covers the read side (inventory a course's files).

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type resourceFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	LastModified string `json:"lastModified"`
	Language     string `json:"language,omitempty"`
}

func newResourceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Inspect a course's media/material files",
		Long: "Inspect the file resources (video, images, PDFs, SCORM, etc.) attached to a course.\n\n" +
			"Note: uploading files is a browser-mediated signed-URL flow the Dawn web editor performs; " +
			"this CLI covers the read side. Upload via the Dawn editor, then list them here.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newResourceListCmd(flags))
	return cmd
}

func newResourceListCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:         "list <course-id>",
		Short:       "List the file resources attached to a course",
		Example:     "  agilix-dawn-pp-cli resource list c_f4bff87c0cab456984f2860af3e427d0 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "course-id=c_f4bff87c0cab456984f2860af3e427d0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list a course's resources")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id is required (a concept id, c_...)"))
			}
			if format != "" && format != "table" && format != "csv" && format != "json" {
				return usageErr(fmt.Errorf("--format must be table, csv, or json"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			files := make([]resourceFile, 0)
			for _, r := range arrOf(obj["resource"]) {
				rm, ok := asMap(r)
				if !ok {
					continue
				}
				b, _ := json.Marshal(rm)
				var f resourceFile
				if json.Unmarshal(b, &f) == nil {
					files = append(files, f)
				}
			}
			if format == "json" || flags.asJSON {
				return flags.printJSON(cmd, files)
			}
			w := cmd.OutOrStdout()
			if format == "csv" {
				cw := csv.NewWriter(w)
				_ = cw.Write([]string{"id", "name", "size", "contentType", "lastModified"})
				for _, f := range files {
					_ = cw.Write([]string{f.ID, f.Name, strconv.FormatInt(f.Size, 10), f.ContentType, f.LastModified})
				}
				cw.Flush()
				return cw.Error()
			}
			if len(files) == 0 {
				fmt.Fprintln(w, "no resources attached to this course")
				return nil
			}
			for _, f := range files {
				fmt.Fprintf(w, "%s\t%d bytes\t%s\t%s\n", f.Name, f.Size, f.ContentType, f.ID)
			}
			fmt.Fprintf(w, "\n%d resources\n", len(files))
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, csv, or json")
	return cmd
}
