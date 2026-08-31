// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/internal/types"

	"github.com/spf13/cobra"
)

func newNovelVideoWatchCmd(flags *rootFlags) *cobra.Command {
	var flagBatch string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Check on an entire submitted batch of generations with one command instead of clicking through each one.",
		Long: "Reads job_name entries from a queue file (script draft-prompts / episode import) and checks all of them " +
			"in one call to POST /v1/video:batchCheckAsyncVideoGenerationStatus, instead of watching each generation " +
			"individually.\n\n" +
			"job_name is not filled in automatically: Flow's generation-submit step is gated by a per-turn reCAPTCHA " +
			"Enterprise challenge (flowCreationAgent:streamChat) that cannot be replayed headlessly, so this CLI cannot " +
			"submit generations on its own. Submit each shot in the real Flow UI, then copy the returned job/workflow " +
			"name into the matching shot's job_name field before running this command.",
		Example: "  flow-pp-cli video watch --batch episode3-queue.json",
		// pp:happy-args points the live dogfood matrix at a dedicated fixture
		// (not the shared root-level episode3-queue.json) carrying a
		// well-formed but non-real job_name. The shared file is unsafe here:
		// the matrix's own "script"/"script draft-prompts" happy-path tests
		// write real (non-dry-run) output to episode3-queue.json earlier in
		// the same run, so any job_name placed there gets clobbered before
		// this test executes. A real job_name only exists after a human
		// submits a generation through Flow's reCAPTCHA-gated UI -- see the
		// Long description above -- so checking this fake one against the
		// real batch-status endpoint legitimately returns a typed
		// not-found/API-error response, not a CLI bug; pp:typed-exit-codes
		// accepts those exit codes as a pass so the matrix can prove the
		// request wiring works without a real submitted generation. See
		// .printing-press-patches/2026-08-30-happy-args-dogfood-fixtures.md.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--batch=testdata/dogfood-fixtures/video-queue.json", "pp:typed-exit-codes": "0,3,4,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagBatch == "" && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if flagBatch == "" {
				return usageErr(fmt.Errorf("--batch is required; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("check batch status for %s", flagBatch))
			}

			raw, err := os.ReadFile(filepath.Clean(flagBatch)) // #nosec G304 -- user-specified batch file is this flag's documented purpose.
			if err != nil {
				return usageErr(fmt.Errorf("reading batch file: %w", err))
			}
			var queue promptQueue
			if err := json.Unmarshal(raw, &queue); err != nil {
				return usageErr(fmt.Errorf("parsing batch file: %w", err))
			}

			var names []string
			shotByName := map[string]queueShot{}
			for _, s := range queue.Shots {
				if s.JobName != "" {
					names = append(names, s.JobName)
					shotByName[s.JobName] = s
				}
			}
			if len(names) == 0 {
				return usageErr(fmt.Errorf("no shot in %s has a job_name set yet -- submit shots in the real Flow UI first, then record each returned job/workflow name in the queue file", flagBatch))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Post(ctx, "/video:batchCheckAsyncVideoGenerationStatus", map[string]any{"names": names})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			var statuses []types.VideoGenerationStatus
			if err := json.Unmarshal(data, &statuses); err != nil {
				return apiErr(fmt.Errorf("parsing batch status response: %w", err))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				for _, s := range statuses {
					items = append(items, map[string]any{
						"name":     s.Name,
						"state":    s.State,
						"progress": s.ProgressPercent,
						"credits":  s.CreditsCost,
						"error":    s.ErrorMessage,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"batch": flagBatch, "checked": len(names), "statuses": statuses}, flags)
		},
	}
	cmd.Flags().StringVar(&flagBatch, "batch", "", "Queue file whose shots' job_name fields should be checked (required)")
	return cmd
}
