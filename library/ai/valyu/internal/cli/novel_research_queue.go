// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/valyu/internal/store"
)

func newResearchQueueCmd(flags *rootFlags) *cobra.Command {
	var file string
	var mode string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Submit and manage a batch of deepresearch tasks.",
		Long: `Submit multiple research topics from a file, then track and download results.

Subcommands:
  queue --file topics.txt     Submit topics from file
  queue status                Check status of all queued tasks
  queue download --dir ./out  Download completed task outputs`,
		Annotations: map[string]string{"pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return parentNoSubcommandRunE(flags)(cmd, args)
			}

			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("opening topics file: %w", err)
			}
			defer f.Close()

			var topics []string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					topics = append(topics, line)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading topics file: %w", err)
			}
			if len(topics) == 0 {
				return fmt.Errorf("no topics found in %s", file)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "Topic\tTask ID\tStatus")
			fmt.Fprintln(tw, "-----\t-------\t------")

			for _, topic := range topics {
				body := map[string]any{
					"query":         topic,
					"mode":          mode,
					"output_format": outputFormat,
				}
				data, _, apiErr := c.PostWithParams(cmd.Context(), "/v1/deepresearch/tasks", map[string]string{}, body)
				if apiErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error submitting %q: %v\n", topic, classifyAPIError(apiErr, flags))
					continue
				}
				taskID := extractResearchTaskID(data)
				if taskID == "" {
					taskID = "(unknown)"
				}
				db.DB().Exec(
					`INSERT OR IGNORE INTO research_queue (task_id, label, mode, output_fmt) VALUES (?, ?, ?, ?)`,
					taskID, topic, mode, outputFormat,
				)
				fmt.Fprintf(tw, "%s\t%s\tsubmitted\n", truncateStr(topic, 50), taskID)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to topics file (one topic per line)")
	cmd.Flags().StringVar(&mode, "mode", "standard", "Research mode (standard, deep, quick)")
	cmd.Flags().StringVar(&outputFormat, "output-format", "markdown", "Output format (markdown, pdf)")

	cmd.AddCommand(newResearchQueueStatusCmd(flags))
	cmd.AddCommand(newResearchQueueDownloadCmd(flags))
	return cmd
}

func newResearchQueueStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Check status of queued deepresearch tasks.",
		Example: "  valyu-pp-cli deepresearch queue status\n  valyu-pp-cli deepresearch queue status --json",
		Annotations: map[string]string{"pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, task_id, label, mode, state, submitted_at FROM research_queue ORDER BY submitted_at DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying research queue: %w", err)
			}
			defer rows.Close()

			type queueItem struct {
				ID          int64
				TaskID      string
				Label       string
				Mode        string
				State       string
				SubmittedAt string
			}
			var items []queueItem
			for rows.Next() {
				var it queueItem
				if err := rows.Scan(&it.ID, &it.TaskID, &it.Label, &it.Mode, &it.State, &it.SubmittedAt); err != nil {
					return err
				}
				items = append(items, it)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			c, clientErr := flags.newClient()
			if clientErr == nil {
				for i, it := range items {
					if it.State == "submitted" || it.State == "running" || it.State == "processing" {
						statusData, statusErr := c.GetNoCache(cmd.Context(),
							"/v1/deepresearch/tasks/"+it.TaskID+"/status", map[string]string{},
						)
						if statusErr == nil {
							newState := extractStringField(statusData, "status")
							if newState == "" {
								newState = extractStringField(statusData, "state")
							}
							if newState != "" && newState != it.State {
								items[i].State = newState
								completedAt := ""
								if newState == "completed" || newState == "done" {
									completedAt = time.Now().UTC().Format(time.RFC3339)
								}
								db.DB().ExecContext(cmd.Context(),
									`UPDATE research_queue SET state = ?, completed_at = ? WHERE id = ?`,
									newState, completedAt, it.ID,
								)
							}
						}
					}
				}
			}

			if flags.asJSON {
				out, _ := json.Marshal(items)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tLabel\tTask ID\tMode\tState\tSubmitted At")
			fmt.Fprintln(tw, "--\t-----\t-------\t----\t-----\t------------")
			for _, it := range items {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
					it.ID, truncateStr(it.Label, 40), truncateStr(it.TaskID, 20), it.Mode, it.State, it.SubmittedAt,
				)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newResearchQueueDownloadCmd(flags *rootFlags) *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:         "download",
		Short:       "Download completed research task outputs.",
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = "."
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, task_id, label FROM research_queue WHERE state IN ('completed','done') ORDER BY submitted_at DESC`,
			)
			if err != nil {
				return fmt.Errorf("querying completed tasks: %w", err)
			}
			defer rows.Close()

			type task struct {
				ID     int64
				TaskID string
				Label  string
			}
			var tasks []task
			for rows.Next() {
				var t task
				if err := rows.Scan(&t.ID, &t.TaskID, &t.Label); err != nil {
					return err
				}
				tasks = append(tasks, t)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if len(tasks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No completed tasks to download.")
				return nil
			}

			c, clientErr := flags.newClient()
			if clientErr != nil {
				return clientErr
			}

			for _, t := range tasks {
				statusData, statusErr := c.GetNoCache(cmd.Context(),
					"/v1/deepresearch/tasks/"+t.TaskID+"/status", map[string]string{},
				)
				if statusErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error fetching task %s: %v\n", t.TaskID, statusErr)
					continue
				}
				output := extractStringField(statusData, "output")
				if output == "" {
					output = extractStringField(statusData, "result")
				}
				if output == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "task %s: no output yet\n", t.TaskID)
					continue
				}

				// Sanitize label for filename
				label := strings.Map(func(r rune) rune {
					if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
						return '_'
					}
					return r
				}, t.Label)
				if len(label) > 80 {
					label = label[:80]
				}
				outFile := filepath.Join(dir, label+".md")
				if err := os.WriteFile(outFile, []byte(output), 0o644); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error writing %s: %v\n", outFile, err)
					continue
				}
				db.DB().ExecContext(cmd.Context(),
					`UPDATE research_queue SET state = 'downloaded' WHERE id = ?`, t.ID,
				)
				fmt.Fprintf(cmd.OutOrStdout(), "downloaded: %s\n", outFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to save downloaded outputs")
	return cmd
}

// extractResearchTaskID extracts the task ID from a deepresearch submit response.
func extractResearchTaskID(data json.RawMessage) string {
	var resp struct {
		TaskID string `json:"task_id"`
		ID     string `json:"id"`
	}
	json.Unmarshal(data, &resp)
	if resp.TaskID != "" {
		return resp.TaskID
	}
	return resp.ID
}
