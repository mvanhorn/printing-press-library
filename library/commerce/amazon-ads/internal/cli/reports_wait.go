package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newReportsWaitCmd(flags *rootFlags) *cobra.Command {
	var statusPath string
	var waitTimeout time.Duration
	var waitInterval time.Duration

	cmd := &cobra.Command{
		Use:   "wait <reportId>",
		Short: "Poll an Amazon Ads report until it reaches a terminal status",
		Long: `Poll an Amazon Ads report status endpoint until the report reaches a terminal status.

By default this checks /v2/reports/{reportId}, which covers Sponsored Ads
reports. Use --status-path for DSP or other report status endpoints.`,
		Example: `  amazon-ads-pp-cli reports wait 550e8400-e29b-41d4-a716-446655440000
  amazon-ads-pp-cli reports wait 550e8400-e29b-41d4-a716-446655440000 --wait-timeout 20m
  amazon-ads-pp-cli reports wait 550e8400-e29b-41d4-a716-446655440000 --status-path /accounts/ACCOUNT_ID/dsp/reports/{id}`,
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"mcp:open-world": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			reportID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_ = RecordJob(JobRow{
				JobID:          reportID,
				Resource:       "reports",
				Endpoint:       "wait",
				Status:         "waiting",
				StatusResource: "reports",
				StatusEndpoint: statusPath,
			})
			final, err := WaitForJob(cmd.Context(), c, statusPath, reportID, WaitOptions{
				Interval: waitInterval,
				Timeout:  waitTimeout,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			status, _ := final["status"].(string)
			if status == "" {
				status = "terminal"
			}
			_ = RecordJob(JobRow{
				JobID:          reportID,
				Resource:       "reports",
				Endpoint:       "wait",
				Status:         status,
				StatusResource: "reports",
				StatusEndpoint: statusPath,
			})
			return printCommandJSON(cmd, flags, map[string]any{
				"report_id":   reportID,
				"status":      status,
				"status_path": statusPath,
				"terminal":    true,
				"report":      final,
			})
		},
	}
	cmd.Flags().StringVar(&statusPath, "status-path", "/v2/reports/{id}", "Report status path; use {id} for the report ID placeholder")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "Maximum duration to wait (0 = no timeout)")
	cmd.Flags().DurationVar(&waitInterval, "wait-interval", 2*time.Second, "Initial poll interval")
	return cmd
}

func addReportsWaitCommand(parent *cobra.Command, flags *rootFlags) {
	if parent == nil {
		return
	}
	for _, child := range parent.Commands() {
		if child.Name() == "wait" {
			return
		}
	}
	parent.AddCommand(newReportsWaitCmd(flags))
}
