package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/spf13/cobra"
)

func newAssetsLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "Upload and manage image assets via the Framer Server API",
	}
	cmd.AddCommand(newAssetsUploadLiveCmd(flags))
	return cmd
}

func newAssetsUploadLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <file-or-directory> [more files...]",
		Short: "Upload images to Framer and get back asset URLs",
		Long: strings.Trim(`
Upload one or more image files (PNG, JPG, JPEG, WebP, GIF, SVG) to your
Framer project. Accepts individual files or a directory (uploads all
supported images in it).

Returns the Framer asset URL for each uploaded file.`, "\n"),
		Example: strings.Trim(`
  # Upload a single image
  framer-pp-cli assets upload logo.png

  # Upload multiple images
  framer-pp-cli assets upload hero.jpg team.png favicon.svg

  # Upload all images in a directory
  framer-pp-cli assets upload ./images/

  # JSON output for scripting
  framer-pp-cli assets upload ./images/ --json`, "\n"),
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				paths, _ := resolveImagePaths(args)
				fmt.Fprintf(cmd.OutOrStdout(), "would upload %d files\n", len(paths))
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			paths, err := resolveImagePaths(args)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return fmt.Errorf("no supported image files found (PNG, JPG, JPEG, WebP, GIF, SVG)")
			}

			payload, _ := json.Marshal(map[string]interface{}{
				"paths": paths,
			})

			var results []struct {
				Name string `json:"name"`
				ID   string `json:"id"`
				URL  string `json:"url"`
			}
			if err := bc.CallInto(&results, "assets-upload", string(payload)); err != nil {
				return fmt.Errorf("upload failed: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, results)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %d assets:\n", len(results))
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", r.Name, r.URL)
			}
			return nil
		},
	}
	return cmd
}

var supportedImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".webp": true, ".gif": true, ".svg": true,
}

func resolveImagePaths(args []string) ([]string, error) {
	var paths []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", arg, err)
		}
		if info.IsDir() {
			entries, err := os.ReadDir(arg)
			if err != nil {
				return nil, fmt.Errorf("reading directory %s: %w", arg, err)
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(e.Name()))
				if supportedImageExts[ext] {
					abs, _ := filepath.Abs(filepath.Join(arg, e.Name()))
					paths = append(paths, abs)
				}
			}
		} else {
			ext := strings.ToLower(filepath.Ext(arg))
			if !supportedImageExts[ext] {
				return nil, fmt.Errorf("unsupported image type: %s (supported: PNG, JPG, WebP, GIF, SVG)", arg)
			}
			abs, _ := filepath.Abs(arg)
			paths = append(paths, abs)
		}
	}
	return paths, nil
}
