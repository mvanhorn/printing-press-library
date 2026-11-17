// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil"
)

// jobIDPattern is the shape of an Autom job id; anything else could rewrite
// the request path.
var jobIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,128}$`)

// jobDone reports whether an Autom job status is final.
func jobDone(status string) bool {
	switch status {
	case "Queued", "Running", "":
		return false
	}
	return true
}

func newJobsGetCmd(flags *rootFlags) *cobra.Command {
	var wait bool
	var interval, maxWait string
	cmd := &cobra.Command{
		Use:   "get [job-id]",
		Short: "Read an async job (started with --is-async) and optionally wait until it finishes",
		Long: `Read the status and result of a job started by any search command with --is-async.
With --wait, poll until the job is CompletedJob or failed. Polling is free; Autom bills the
search once when the job completes. Exit code 5 when the job ends in Error or BadRequest, 3 for NotFound.`,
		Example: strings.Trim(`
  autom-pp-cli google search --query "serp api pricing" --is-async --json
  autom-pp-cli jobs get 6f1c2b8e-1d2a-4c7f-9a51-0f3e2b7d9c10 --wait --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "job-id=example-value",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "jobs get")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a job id is required (it is the job_id returned by a search run with --is-async)"))
			}
			id := strings.TrimSpace(args[0])
			if !jobIDPattern.MatchString(id) {
				return usageErr(fmt.Errorf("invalid job id %q: expected letters, digits and dashes", id))
			}
			every, err := cliutil.ParseDurationLoose(interval)
			if err != nil || every <= 0 {
				return usageErr(fmt.Errorf("invalid --interval %q", interval))
			}
			limit, err := cliutil.ParseDurationLoose(maxWait)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --max-wait %q", maxWait))
			}
			// With --wait the whole poll may outlast --timeout (60s by default):
			// bound it by --max-wait plus one request instead.
			ctx, cancel := boundCtx(cmd.Context(), flags)
			if wait {
				cancel()
				ctx, cancel = context.WithTimeout(cmd.Context(), limit+flags.timeout)
			}
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			deadline := time.Now().Add(limit)
			path := replacePathParam("/v1/jobs/{id}", "id", id)
			for {
				raw, err := c.GetNoCache(ctx, path, fullResponse(nil))
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var job struct {
					Status   string `json:"status"`
					Progress *int   `json:"progress"`
				}
				_ = json.Unmarshal(raw, &job)
				if !wait || jobDone(job.Status) || time.Now().After(deadline) {
					if wait && !jobDone(job.Status) {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: job still %s after --max-wait %s\n", job.Status, maxWait)
					}
					if err := printOutputWithFlags(cmd.OutOrStdout(), raw, flags); err != nil {
						return err
					}
					switch job.Status {
					case "NotFound":
						return notFoundErr(fmt.Errorf("job %s not found", id))
					case "Error", "BadRequest":
						return apiErr(fmt.Errorf("job %s ended in %s", id, job.Status))
					}
					return nil
				}
				if job.Progress != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "job %s: %d%%\n", job.Status, *job.Progress)
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(every):
				}
			}
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until the job finishes")
	cmd.Flags().StringVar(&interval, "interval", "3s", "Time between polls with --wait")
	cmd.Flags().StringVar(&maxWait, "max-wait", "2m", "Give up waiting after this long")
	return cmd
}

func newJobsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "jobs",
		Short:       "Async jobs started with --is-async",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newJobsGetCmd(flags))
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newJobsCmd(flags))
	})
}
