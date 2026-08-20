// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command: background removal + HD restoration.
// local image -> upload -> submit /draw/hd/prompt -> poll -> persist -> download.
//
// pp:data-source live

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelGenerateRestoreCmd(flags *rootFlags) *cobra.Command {
	var hdFix, removeBack string
	var noWait bool
	var batchDir, outDir, db string
	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "restore [image]",
		Short: "Background removal and/or HD restoration of a local image",
		Long: strings.Trim(`
Upload a local image (or pass a remote URL), run background removal and/or HD
restoration, poll to completion, record the job locally, and print the result
URLs. Cost: 2 credits. A good cheap pre-process before a depth-map or 3D draw —
or use 'generate depthmap --restore-first' to chain both in one command.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli generate restore photo.jpg --hd-fix true
  sculptok-pp-cli generate restore subject.png --remove-back general
  sculptok-pp-cli generate restore --batch ./photos --hd-fix true`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if hdFix != "" {
				body["hdFix"] = hdFix
			}
			if removeBack != "" {
				body["removeBack"] = removeBack
			}
			return executeGenerate(cmd, flags, args, genCmdConfig{
				kind:         "restore",
				noWait:       noWait,
				batchDir:     batchDir,
				outDir:       outDir,
				db:           db,
				pollInterval: pollInterval,
				body:         body,
				recorded:     map[string]string{"hd_fix": hdFix, "remove_back": removeBack},
			})
		},
	}
	cmd.Flags().StringVar(&hdFix, "hd-fix", "", "Apply HD restoration: true|false")
	cmd.Flags().StringVar(&removeBack, "remove-back", "", "Background removal model: anime|general (omit to skip)")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Submit only; do not poll to completion")
	cmd.Flags().StringVar(&batchDir, "batch", "", "Process every image in this directory")
	cmd.Flags().StringVar(&outDir, "out", "", "Download result images into this directory")
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 3*time.Second, "Status poll interval")
	return cmd
}
