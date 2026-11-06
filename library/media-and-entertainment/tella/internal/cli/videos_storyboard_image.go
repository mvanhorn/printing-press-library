// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func newNovelVideosStoryboardImageCmd(flags *rootFlags) *cobra.Command {
	var clip, source, output string
	var overwrite bool
	cmd := &cobra.Command{Use: "storyboard-image [video]", Short: "Download an official WebP storyboard to a private local file", Args: workflowArgs(1), Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"}, Example: "  tella-pp-cli videos storyboard-image vid_example --clip cl_example --output storyboard.webp"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && source == "" && clip == "" && output == "" {
			return cmd.Help()
		}
		if clip != "" && source != "" {
			return usageErr(fmt.Errorf("--clip and --source are mutually exclusive"))
		}
		if workflowOffline(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "apply": false, "workflow": "videos storyboard-image", "video": args, "clip": clip, "source": source, "output": output, "steps": []string{"read storyboard metadata", "download WebP to output"}}, flags)
		}

		if output == "" {
			return usageErr(fmt.Errorf("--output FILE is required"))
		}
		path := ""
		if source == "" {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("video is required unless --source is supplied"))
			}
			path = videoPath(args[0]) + "/storyboard"
		}
		if clip != "" {
			if err := workflowArgs(1)(cmd, []string{clip}); err != nil {
				return err
			}
			path = clipPath(args[0], clip) + "/storyboard"
		}
		if source != "" {
			if err := workflowArgs(1)(cmd, []string{source}); err != nil {
				return err
			}
			path = "/v1/sources/" + url.PathEscape(source) + "/storyboard"
		}
		target, err := checkLessonTarget(output, overwrite)
		if err != nil {
			return err
		}
		if flags.dryRun {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "path": path, "response": "json", "output": target}, flags)
		}
		c, err := workflowClient(flags)
		if err != nil {
			return err
		}
		raw, err := c.Get(cmd.Context(), path, map[string]string{"response": "json"})
		if err != nil {
			return err
		}
		var sheet struct {
			URL        string `json:"url"`
			Columns    int    `json:"columns"`
			Rows       int    `json:"rows"`
			IntervalMs int    `json:"intervalMs"`
		}
		if err = json.Unmarshal(raw, &sheet); err != nil {
			return err
		}
		if sheet.URL == "" || sheet.Columns <= 0 || sheet.Rows <= 0 || sheet.IntervalMs < 0 {
			return fmt.Errorf("invalid storyboard metadata")
		}
		actual, size, err := downloadLessonMedia(cmd.Context(), sheet.URL, target, "webp", overwrite, lessonMediaTimeout(flags))
		if err != nil {
			return err
		}
		result := map[string]any{"path": actual, "bytes": size, "columns": sheet.Columns, "rows": sheet.Rows, "intervalMs": sheet.IntervalMs}
		if width, height, ok := storyboardDimensions(cmd.Context(), actual); ok {
			result["width"] = width
			result["height"] = height
		}
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	cmd.Flags().StringVar(&clip, "clip", "", "Clip storyboard (mutually exclusive with --source)")
	cmd.Flags().StringVar(&source, "source", "", "Standalone source storyboard")
	cmd.Flags().StringVar(&output, "output", "", "Destination WebP file")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace an existing regular file")
	return cmd
}
func storyboardDimensions(ctx context.Context, file string) (int, int, bool) {
	bin, err := exec.LookPath("sips")
	if err != nil {
		return 0, 0, false
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	data, err := exec.CommandContext(bounded, bin, "-g", "pixelWidth", "-g", "pixelHeight", file).Output()
	if err != nil {
		return 0, 0, false
	}
	width, height := 0, 0
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] == "pixelWidth:" {
			width, _ = strconv.Atoi(fields[1])
		}
		if fields[0] == "pixelHeight:" {
			height, _ = strconv.Atoi(fields[1])
		}
	}
	return width, height, width > 0 && height > 0
}
