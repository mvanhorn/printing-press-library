// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// The "todo add" capture command lives in its own file so its declaring
// source is unambiguous: three commands in this CLI share the leaf name
// "add" (client profile add, top-level add, todo add), and static tooling
// that resolves a command path to a file falls back to the
// <parent>_<leaf>.go basename convention when a command is registered
// through the novel-command hook instead of a literal rootCmd.AddCommand.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/cliutil"
)

func newTodoAddCmd(flags *rootFlags) *cobra.Command {
	var flagStdin bool
	var flagFor, flagNote, flagIcon, flagProfile, flagDB, flagList string

	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Capture one or more to-do tasks",
		Long: `Add tasks to the to-do list.

Pass --stdin to capture a whole brain dump at once, one task per line. That
is the shape this audience actually needs: get everything out of your head
first, organize later.

Blank lines are skipped. Each line becomes its own task.`,
		Example: "  printf 'call pharmacy\\nbook dentist\\n' | tiimo-pp-cli todo add --stdin",
		Annotations: map[string]string{
			"pp:happy-args":       "title=pp-dogfood-fixture",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todo add")
			}
			if handled, err := runWriteHarnessGuard(cmd, flags, "todo add", args); handled {
				return err
			}

			titles := make([]string, 0, 1)
			if flagStdin {
				sc := bufio.NewScanner(cmd.InOrStdin())
				sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				for sc.Scan() {
					line := strings.TrimSpace(sc.Text())
					if line != "" {
						titles = append(titles, line)
					}
				}
				if err := sc.Err(); err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if len(titles) == 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--stdin was set but no task lines were read"))
				}
			} else {
				if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("a task title is required (or pass --stdin)"))
				}
				titles = append(titles, strings.TrimSpace(args[0]))
			}

			durSecs := 0
			if strings.TrimSpace(flagFor) != "" {
				d, err := cliutil.ParseDurationLoose(flagFor)
				if err != nil || d <= 0 {
					var n int
					if _, scanErr := fmt.Sscanf(strings.TrimSpace(flagFor), "%d", &n); scanErr == nil && n > 0 {
						d = time.Duration(n) * time.Minute
					} else {
						_ = cmd.Usage()
						return usageErr(fmt.Errorf("invalid --for %q: want 30m, 1h, or a number of minutes", flagFor))
					}
				}
				durSecs = int(d.Seconds())
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			profileID, err := resolveProfileID(ctx, cmd, flags, flagProfile, flagDB)
			if err != nil {
				return err
			}
			listID := strings.TrimSpace(flagList)
			if listID == "" {
				listID, err = resolveTodoListID(ctx, flags, profileID)
				if err != nil {
					return err
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/api/profiles/" + cliutil.EscapePathParam(profileID) + "/todo-tasks"

			results := make([]todoWriteResult, 0, len(titles))
			for _, title := range titles {
				body := map[string]any{
					"todoTaskListId": listID,
					"title":          title,
					"notes":          flagNote,
					"iconType":       "UnicodeEmoji",
				}
				if durSecs > 0 {
					body["duration"] = durSecs
				}
				if flagIcon != "" {
					body["iconId"] = flagIcon
				}
				data, status, err := c.Post(ctx, path, body)
				if err != nil {
					// Report what did land before failing, so a partial brain
					// dump is never silently lost.
					if len(results) > 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: created %d task(s) before failing on %q\n", len(results), title)
					}
					return classifyAPIError(err, flags)
				}
				if status < 200 || status >= 300 {
					return apiErr(fmt.Errorf("creating task %q failed with status %d", title, status))
				}
				var created struct {
					TaskID string `json:"taskId"`
				}
				_ = json.Unmarshal(data, &created)
				results = append(results, todoWriteResult{
					Action: "created", TaskID: created.TaskID, Title: title, Status: "ok",
				})
			}

			return writeTiimoResult(cmd, flags, results, func(w io.Writer) {
				for _, r := range results {
					fmt.Fprintf(w, "Added to-do: %s\n", r.Title)
				}
				if len(results) > 1 {
					fmt.Fprintf(w, "\n%d task(s) captured.\n", len(results))
				}
				fmt.Fprintln(w, "Run 'tiimo-pp-cli sync' to refresh the local mirror.")
			})
		},
	}
	cmd.Flags().BoolVar(&flagStdin, "stdin", false, "Read one task per line from stdin")
	cmd.Flags().StringVar(&flagFor, "for", "", "Estimated duration (30m, 1h, or minutes)")
	cmd.Flags().StringVar(&flagNote, "note", "", "Notes to attach to the task")
	cmd.Flags().StringVar(&flagIcon, "icon", "", "Emoji to show on the task")
	cmd.Flags().StringVar(&flagList, "list-id", "", "Target to-do list UUID (defaults to the first list)")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name or UUID")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	return cmd
}
