// Hand-authored. Do not regenerate over this file with `printing-press generate`
// without merging — it owns the `task attach` and `task attachments` commands.
//
// PATCH(task-attach-and-attachments): `task attach` does a real
// multipart/form-data upload (one POST per file) to
// /v2/task/{task_id}/attachment; `task attachments` lists a task's
// attachments with comment counts from GET /v2/task/{task_id}. Both are
// registered under the generated `task` command in root.go.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup/internal/client"

	"github.com/spf13/cobra"
)

// taskIDParams builds the custom_task_ids/team_id query params exactly the
// way `task get` does.
func taskIDParams(customTaskIDs bool, teamID string) map[string]string {
	params := map[string]string{}
	if customTaskIDs {
		params["custom_task_ids"] = "true"
	}
	if teamID != "" {
		params["team_id"] = teamID
	}
	return params
}

func newTaskAttachCmd(flags *rootFlags) *cobra.Command {
	var customTaskIDs bool
	var teamID string

	cmd := &cobra.Command{
		Use:   "attach <task_id> <file> [<file>...]",
		Short: "Upload one or more local files to a task as attachments (multipart/form-data)",
		Long: `Upload local files to a ClickUp task. Each file is sent as its own
multipart/form-data POST to /v2/task/{task_id}/attachment with the file part
named "attachment". Every file is checked (exists, regular file) before any
upload starts. Failed uploads are reported in "failed" and the command exits
non-zero; 5xx responses are not retried to avoid duplicate attachments.`,
		Example: `  clickup-pp-cli task attach abc123xyz ./report.pdf
  clickup-pp-cli task attach PROJ-123 ./notes.md ./diagram.png --custom-task-ids --team-id 1234567
  clickup-pp-cli task attach PROJ-123 ./report.pdf --custom-task-ids --team-id 1234567 --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("at least one file is required\nUsage: %s", cmd.UseLine()))
			}
			taskID := args[0]

			files := make([]client.UploadFile, 0, len(args)-1)
			for _, p := range args[1:] {
				f, err := client.StatUploadFile(p)
				if err != nil {
					return usageErr(err)
				}
				files = append(files, f)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/v2/task/{task_id}/attachment", "task_id", taskID)
			params := taskIDParams(customTaskIDs, teamID)

			if flags.dryRun {
				return printAttachDryRun(cmd, flags, c, taskID, path, params, files)
			}
			// The CLI client is in live mode here; PostMultipart never retries 5xx.
			result := attachResult{TaskID: taskID, Uploaded: []map[string]any{}, Failed: []map[string]any{}}
			var firstErr error
			for _, f := range files {
				data, status, err := c.PostMultipart(path, params, "attachment", f, nil)
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					entry := map[string]any{"file": f.Path, "error": err.Error()}
					if status != 0 {
						entry["status"] = status
					}
					result.Failed = append(result.Failed, entry)
					continue
				}
				entry := map[string]any{}
				_ = json.Unmarshal(data, &entry)
				if entry == nil {
					entry = map[string]any{}
				}
				entry["file"] = f.Path
				result.Uploaded = append(result.Uploaded, entry)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				for _, u := range result.Uploaded {
					fmt.Fprintf(w, "uploaded  %s -> %v (%v) %v\n", u["file"], u["title"], u["id"], u["url"])
				}
				for _, f := range result.Failed {
					fmt.Fprintf(w, "FAILED    %s: %v\n", f["file"], f["error"])
				}
			} else if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}
			if firstErr != nil {
				return classifyAPIError(fmt.Errorf("%d of %d uploads failed: %w", len(result.Failed), len(files), firstErr), flags)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&customTaskIDs, "custom-task-ids", false, "Reference the task by its custom task id (e.g. PROJ-123); requires --team-id")
	cmd.Flags().StringVar(&teamID, "team-id", "", "Workspace ID; required when --custom-task-ids is set")
	return cmd
}

type attachResult struct {
	TaskID   string           `json:"task_id"`
	Uploaded []map[string]any `json:"uploaded"`
	Failed   []map[string]any `json:"failed"`
}

func printAttachDryRun(cmd *cobra.Command, flags *rootFlags, c *client.Client, taskID, path string, params map[string]string, files []client.UploadFile) error {
	url := c.BaseURL + path
	if len(params) > 0 {
		q := make([]string, 0, len(params))
		if v, ok := params["custom_task_ids"]; ok {
			q = append(q, "custom_task_ids="+v)
		}
		if v, ok := params["team_id"]; ok {
			q = append(q, "team_id="+v)
		}
		url += "?" + strings.Join(q, "&")
	}
	auth := c.MaskedAuthHeader()
	w := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(w) {
		preview := map[string]any{
			"dry_run":       true,
			"action":        "task attach",
			"task_id":       taskID,
			"method":        "POST",
			"url":           url,
			"field":         "attachment",
			"files":         files,
			"authorization": auth,
		}
		return printJSONFiltered(w, preview, flags)
	}
	fmt.Fprintf(w, "POST %s (multipart/form-data, one request per file)\n", url)
	for _, f := range files {
		fmt.Fprintf(w, "  attachment: %s (%s, %d bytes) <- %s\n", f.Name, f.ContentType, f.Size, f.Path)
	}
	if auth != "" {
		fmt.Fprintf(w, "  Authorization: %s\n", auth)
	}
	fmt.Fprintln(w, "\n(dry run - no request sent)")
	return nil
}

// attachmentRow is one row of `task attachments` output.
type attachmentRow struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Extension        string `json:"extension"`
	Size             int64  `json:"size"`
	URL              string `json:"url"`
	TotalComments    int64  `json:"total_comments"`
	ResolvedComments int64  `json:"resolved_comments"`
	OpenComments     int64  `json:"open_comments"`
}

// flexInt decodes a JSON number or numeric string (ClickUp is inconsistent).
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // tolerate unexpected shapes rather than failing the listing
	}
	*f = flexInt(n)
	return nil
}

// attachmentRowsFromTask extracts attachment rows from a GET /v2/task/{id}
// response. It also accepts the CLI's provenance envelope ({"results": {...}}).
func attachmentRowsFromTask(data json.RawMessage) ([]attachmentRow, error) {
	type rawAttachment struct {
		ID               string  `json:"id"`
		Title            string  `json:"title"`
		Extension        string  `json:"extension"`
		Size             flexInt `json:"size"`
		URL              string  `json:"url"`
		TotalComments    flexInt `json:"total_comments"`
		ResolvedComments flexInt `json:"resolved_comments"`
	}
	var task struct {
		Attachments []rawAttachment `json:"attachments"`
		Results     *struct {
			Attachments []rawAttachment `json:"attachments"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("parsing task response: %w", err)
	}
	raw := task.Attachments
	if raw == nil && task.Results != nil {
		raw = task.Results.Attachments
	}
	rows := make([]attachmentRow, 0, len(raw))
	for _, a := range raw {
		open := int64(a.TotalComments) - int64(a.ResolvedComments)
		if open < 0 {
			open = 0
		}
		rows = append(rows, attachmentRow{
			ID:               a.ID,
			Title:            a.Title,
			Extension:        a.Extension,
			Size:             int64(a.Size),
			URL:              a.URL,
			TotalComments:    int64(a.TotalComments),
			ResolvedComments: int64(a.ResolvedComments),
			OpenComments:     open,
		})
	}
	return rows, nil
}

func newTaskAttachmentsCmd(flags *rootFlags) *cobra.Command {
	var customTaskIDs bool
	var teamID string

	cmd := &cobra.Command{
		Use:   "attachments <task_id>",
		Short: "List a task's attachments with size, URL and comment counts (total/resolved/open)",
		Long: `List the attachments on a ClickUp task (from GET /v2/task/{task_id}) as rows:
id, title, extension, size, url, total_comments, resolved_comments and
open_comments (total - resolved). The public API exposes comment counts only,
not comment text.`,
		Example: `  clickup-pp-cli task attachments abc123xyz
  clickup-pp-cli task attachments PROJ-123 --custom-task-ids --team-id 1234567 --json
  clickup-pp-cli task attachments PROJ-123 --custom-task-ids --team-id 1234567 --json --select title,open_comments`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/v2/task/{task_id}", "task_id", args[0])
			data, err := c.Get(path, taskIDParams(customTaskIDs, teamID))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				// PATCH(clickup-task-attachments-dry-run-json): the request preview
				// goes to stderr; JSON callers still get a JSON preview on stdout.
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run": true,
						"action":  "task attachments",
						"method":  "GET",
						"url":     c.BaseURL + path,
					}, flags)
				}
				return nil
			}
			rows, err := attachmentRowsFromTask(data)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				table := make([][]string, 0, len(rows))
				for _, r := range rows {
					table = append(table, []string{
						r.ID, r.Title, r.Extension, strconv.FormatInt(r.Size, 10),
						strconv.FormatInt(r.TotalComments, 10), strconv.FormatInt(r.ResolvedComments, 10),
						strconv.FormatInt(r.OpenComments, 10), r.URL,
					})
				}
				return flags.printTable(cmd, []string{"ID", "TITLE", "EXT", "SIZE", "COMMENTS", "RESOLVED", "OPEN", "URL"}, table)
			}
			// --agent turns on --compact, whose generic list allow-list would drop
			// size and the comment counts; these rows are already compact, so
			// only --select narrows them.
			out := *flags
			out.compact = false
			return printJSONFiltered(cmd.OutOrStdout(), rows, &out)
		},
	}
	cmd.Flags().BoolVar(&customTaskIDs, "custom-task-ids", false, "Reference the task by its custom task id (e.g. PROJ-123); requires --team-id")
	cmd.Flags().StringVar(&teamID, "team-id", "", "Workspace ID; required when --custom-task-ids is set")
	return cmd
}

var errNoFile = errors.New("--file is required: path to the local file to upload")
