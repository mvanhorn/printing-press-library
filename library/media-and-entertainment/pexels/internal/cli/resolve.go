// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source live

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/pexels"
)

type resolveResult struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	ChosenLabel  string `json:"chosen_label"`
	ChosenURL    string `json:"chosen_url"`
	ChosenWidth  int    `json:"chosen_width"`
	ChosenHeight int    `json:"chosen_height"`
	Attribution  string `json:"attribution"`
}

type resolvePhoto struct {
	ID           int64             `json:"id"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Photographer string            `json:"photographer"`
	Src          map[string]string `json:"src"`
}

type resolveVideo struct {
	ID   int64 `json:"id"`
	User struct {
		Name string `json:"name"`
	} `json:"user"`
	VideoFiles []struct {
		Quality  string `json:"quality"`
		FileType string `json:"file_type"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Link     string `json:"link"`
	} `json:"video_files"`
}

func newNovelResolveCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagTargetWidth int
	var flagTargetHeight int
	var flagID string

	cmd := &cobra.Command{
		Use:         "resolve [id]",
		Short:       "Pick the smallest photo size or video file that meets a target dimension without upscaling.",
		Example:     "2014422 --target-width 1280 --target-height 720 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--id=2014422"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the best media rendition meeting your target dimensions")
				return nil
			}
			// id comes from the positional arg OR the --id flag (the flag form
			// lets agents and the verifier supply it without a positional).
			id := flagID
			if len(args) >= 1 && args[0] != "" {
				id = args[0]
			}
			if id == "" {
				return usageErr(fmt.Errorf("id is required (positional <id> or --id)\nUsage: %s [id]", cmd.CommandPath()))
			}
			// Pexels media ids are numeric; validating up front avoids sending a
			// malformed path segment to the API and gives a clear usage error.
			if _, convErr := strconv.Atoi(id); convErr != nil {
				return usageErr(fmt.Errorf("id must be a numeric Pexels media id, got %q", id))
			}
			if flagType != "photo" && flagType != "video" {
				return usageErr(fmt.Errorf("--type must be photo or video, got %q", flagType))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			cfg, _ := config.Load(flags.configPath)
			key := ""
			if cfg != nil {
				key = cfg.PexelsApiKey
			}
			client := pexels.New(key)

			if flagType == "video" {
				body, _, err := client.Get(ctx, "/videos/videos/"+id, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var v resolveVideo
				if err := json.Unmarshal(body, &v); err != nil {
					return fmt.Errorf("decode video: %w", err)
				}
				files := make([]pexels.VideoFile, 0, len(v.VideoFiles))
				for _, f := range v.VideoFiles {
					files = append(files, pexels.VideoFile{
						Quality: f.Quality, FileType: f.FileType,
						Width: f.Width, Height: f.Height, Link: f.Link,
					})
				}
				chosen, ok := pexels.PickVideoFile(files, flagTargetWidth, flagTargetHeight)
				if !ok {
					return fmt.Errorf("video %s has no downloadable files", id)
				}
				out := resolveResult{
					ID: v.ID, Type: "video",
					ChosenLabel: chosen.Quality, ChosenURL: chosen.Link,
					ChosenWidth: chosen.Width, ChosenHeight: chosen.Height,
					Attribution: fmt.Sprintf("Video by %s on Pexels", v.User.Name),
				}
				return emitResolve(cmd, flags, out)
			}

			body, _, err := client.Get(ctx, "/photos/"+id, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var p resolvePhoto
			if err := json.Unmarshal(body, &p); err != nil {
				return fmt.Errorf("decode photo: %w", err)
			}
			label, url, w, h := pexels.PickPhotoSize(p.Src, p.Width, p.Height, flagTargetWidth, flagTargetHeight)
			if url == "" {
				return fmt.Errorf("photo %s has no downloadable sources", id)
			}
			out := resolveResult{
				ID: p.ID, Type: "photo",
				ChosenLabel: label, ChosenURL: url,
				ChosenWidth: w, ChosenHeight: h,
				Attribution: fmt.Sprintf("Photo by %s on Pexels", p.Photographer),
			}
			return emitResolve(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "photo", "media type: photo or video")
	cmd.Flags().StringVar(&flagID, "id", "", "Pexels media id (alternative to the positional argument)")
	cmd.Flags().IntVar(&flagTargetWidth, "target-width", 0, "minimum required width in pixels (0 = no constraint)")
	cmd.Flags().IntVar(&flagTargetHeight, "target-height", 0, "minimum required height in pixels (0 = no constraint)")
	return cmd
}

func emitResolve(cmd *cobra.Command, flags *rootFlags, out resolveResult) error {
	stdout := cmd.OutOrStdout()
	if flags.asJSON || flags.agent || !isTerminal(stdout) {
		return printJSONFiltered(stdout, out, flags)
	}
	fmt.Fprintf(stdout, "%s #%d: %s (%dx%d)\n  %s\n  %s\n",
		out.Type, out.ID, out.ChosenLabel, out.ChosenWidth, out.ChosenHeight, out.ChosenURL, out.Attribution)
	return nil
}
