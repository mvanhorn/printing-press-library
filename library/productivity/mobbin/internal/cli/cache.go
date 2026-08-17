// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/imagecache"
	"github.com/spf13/cobra"
)

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cache",
		Short:       "Inspect and prune the local Mobbin image cache.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "stats",
		Short:       "Show cached image file count and bytes.",
		Example:     "  mobbin-pp-cli cache stats --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := imagecache.New("")
			if err != nil {
				return err
			}
			bytes, files, err := c.Stats()
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"root": c.Root, "bytes": bytes, "files": files})
		},
	})
	var older string
	prune := &cobra.Command{
		Use:     "prune",
		Short:   "Delete cached images older than a duration.",
		Example: "  mobbin-pp-cli cache prune --older-than 30d",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseSince(older)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --older-than %q: %w", older, err))
			}
			c, err := imagecache.New("")
			if err != nil {
				return err
			}
			deleted, err := c.Prune(d)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"root": c.Root, "deleted": deleted, "older_than": older, "at": time.Now().UTC().Format(time.RFC3339)})
		},
	}
	prune.Flags().StringVar(&older, "older-than", "30d", "Delete files older than this duration, e.g. 30d or 720h")
	cmd.AddCommand(prune)
	return cmd
}
