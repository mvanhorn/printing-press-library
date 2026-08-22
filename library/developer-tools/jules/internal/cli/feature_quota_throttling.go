// Feature 1: Quota-aware Dispatch Throttling
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/jules/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// Find sessions command and wrap its create subcommand with quota awareness
		sessionsCmdIdx := -1
		for idx, cmd := range root.Commands() {
			if cmd.Name() == "sessions" {
				sessionsCmdIdx = idx
				break
			}
		}

		if sessionsCmdIdx >= 0 {
			sessionsCmd := root.Commands()[sessionsCmdIdx]
			for _, subcmd := range sessionsCmd.Commands() {
				if subcmd.Name() == "create" {
					// Replace with quota-aware wrapper
					quotaCreate := newSessionsCreateQuotaAwareCmd(flags)
					sessionsCmd.RemoveCommand(subcmd)
					sessionsCmd.AddCommand(quotaCreate)
					break
				}
			}
		}
	})
}

func newSessionsCreateQuotaAwareCmd(flags *rootFlags) *cobra.Command {
	var bodyAutomationMode string
	var bodyPrompt string
	var bodyRequirePlanApproval bool
	var bodySourceContext string
	var bodyTitle string
	var stdinBody bool
	var quotaSafe bool
	var checkConflicts bool
	var maxRetries int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create new coding task session",
		Long: `Create new coding task session with optional quota safety checks.

Use --quota-safe to check quota before dispatching and implement exponential backoff on rate limits.`,
		Example:     "  jules-pp-cli sessions create --prompt 'fix the bug' --quota-safe",
		Annotations: map[string]string{"pp:endpoint": "sessions.create", "pp:method": "POST", "pp:path": "/sessions"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasChangedLocalFlags(cmd) && len(args) == 0 && !flags.dryRun {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "requires input",
						"usage": cmd.CommandPath() + " --help",
					}, flags); printErr != nil {
						return printErr
					}
					return usageErr(fmt.Errorf("%q requires input; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
				}
				return cmd.Help()
			}
			if !stdinBody {
				if !cmd.Flags().Changed("prompt") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "prompt")
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Check quota before dispatching if --quota-safe is set
			if quotaSafe {
				if err := checkQuotaBeforeDispatch(cmd.Context(), c, cmd.OutOrStdout()); err != nil {
					return err
				}
			}

			// Check for conflicts if --check-conflicts is set
			if checkConflicts {
				var sourceCtx map[string]any
				if bodySourceContext != "" {
					if err := json.Unmarshal([]byte(bodySourceContext), &sourceCtx); err != nil {
						return fmt.Errorf("parsing source context: %w", err)
					}
				}
				hasConflict, conflicts, err := checkConflictsFunc(cmd.Context(), c, sourceCtx, cmd.OutOrStdout())
				if err != nil {
					return err
				}
				if hasConflict && !flags.yes {
					return fmt.Errorf("conflicts detected with in-flight sessions: %v (use --yes to override)", conflicts)
				}
			}

			path := "/sessions"
			params := map[string]string{}
			var body any

			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				bodyMap := map[string]any{}
				body = bodyMap
				if cmd.Flags().Changed("automation-mode") || bodyAutomationMode != "" {
					bodyMap["automationMode"] = bodyAutomationMode
				}
				if cmd.Flags().Changed("prompt") || bodyPrompt != "" {
					bodyMap["prompt"] = bodyPrompt
				}
				if cmd.Flags().Changed("require-plan-approval") {
					bodyMap["requirePlanApproval"] = bodyRequirePlanApproval
				}
				if cmd.Flags().Changed("source-context") || bodySourceContext != "" {
					var parsedSourceContext any
					if err := json.Unmarshal([]byte(bodySourceContext), &parsedSourceContext); err != nil {
						return fmt.Errorf("parsing --source-context JSON: %w", err)
					}
					asMap, ok := parsedSourceContext.(map[string]any)
					if !ok {
						return fmt.Errorf("--source-context must be a JSON object, got JSON %T", parsedSourceContext)
					}
					bodyMap["sourceContext"] = asMap
				}
				if cmd.Flags().Changed("title") || bodyTitle != "" {
					bodyMap["title"] = bodyTitle
				}
			}

			// Dispatch with exponential backoff if quota-safe is enabled
			var data []byte
			if quotaSafe {
				data, _, err = dispatchWithQuotaBackoff(cmd.Context(), c, path, params, body, maxRetries)
			} else {
				data, _, err = c.PostWithParams(cmd.Context(), path, params, body)
			}

			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			return printOutput(cmd.OutOrStdout(), data, flags.asJSON)
		},
	}

	cmd.Flags().StringVar(&bodyAutomationMode, "automation-mode", "", "Automation mode for the session")
	cmd.Flags().StringVar(&bodyPrompt, "prompt", "", "The task description for Jules to execute (required)")
	cmd.Flags().BoolVar(&bodyRequirePlanApproval, "require-plan-approval", false, "Require plan approval before execution")
	cmd.Flags().StringVar(&bodySourceContext, "source-context", "", "Source context as JSON object")
	cmd.Flags().StringVar(&bodyTitle, "title", "", "Title for the session")
	cmd.Flags().BoolVar(&stdinBody, "stdin-body", false, "Read JSON body from stdin")
	cmd.Flags().BoolVar(&quotaSafe, "quota-safe", false, "Check quota and use exponential backoff on rate limits")
	cmd.Flags().BoolVar(&checkConflicts, "check-conflicts", false, "Detect merge conflicts with in-flight sessions before dispatching")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 5, "Maximum retries on rate limit (only with --quota-safe)")

	return cmd
}

// checkQuotaBeforeDispatch queries API to check available quota
func checkQuotaBeforeDispatch(ctx context.Context, c *client.Client, out io.Writer) error {
	// Query sessions to estimate quota usage
	// In a real implementation, this would call an actual quota endpoint
	data, err := c.Get(ctx, "/sessions", map[string]string{"pageSize": "1"})
	if err != nil {
		fmt.Fprintf(out, "Warning: Could not check quota: %v\n", err)
		return nil // Don't fail, just warn
	}

	var sessions map[string]any
	if err := json.Unmarshal(data, &sessions); err != nil {
		fmt.Fprintf(out, "Warning: Could not parse quota check response\n")
		return nil
	}

	// Check for quota warnings in response
	if quota, ok := sessions["quota"].(map[string]any); ok {
		if remaining, ok := quota["remaining"].(float64); ok && remaining < 5 {
			fmt.Fprintf(out, "⚠️  Quota warning: Only %.0f requests remaining\n", remaining)
		}
	}

	return nil
}

// dispatchWithQuotaBackoff implements exponential backoff for quota exhaustion
func dispatchWithQuotaBackoff(ctx context.Context, c *client.Client, path string, params map[string]string, body any, maxRetries int) ([]byte, int, error) {
	var lastErr error
	backoff := time.Millisecond * 100

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			}
			backoff = time.Duration(float64(backoff) * math.Pow(2, 1.5)) // 1.5x exponential backoff
		}

		data, statusCode, err := c.PostWithParams(ctx, path, params, body)

		// Check for quota exhaustion errors (429, 400 FAILED_PRECONDITION)
		if statusCode == 429 || (statusCode == 400 && attempt < maxRetries) {
			lastErr = err
			continue
		}

		return data, statusCode, err
	}

	return nil, 429, fmt.Errorf("quota exhausted after %d retries: %w", maxRetries, lastErr)
}
