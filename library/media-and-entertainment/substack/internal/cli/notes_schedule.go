// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/notebuilder"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type scheduleResult struct {
	QueuedID       int64  `json:"queued_id,omitempty"`
	ScheduledAt    string `json:"scheduled_at"`
	Status         string `json:"status"`
	BodyJSONLength int    `json:"body_json_length"`
	Sent           bool   `json:"sent,omitempty"`
	VerifyEnv      bool   `json:"verify_env_short_circuit,omitempty"`
}

type cadenceViolation struct {
	Error            string `json:"error"`
	OffendingNoteID  string `json:"offending_note_id"`
	TimeDeltaMinutes int    `json:"time_delta_minutes"`
	Message          string `json:"message"`
}

func newNotesScheduleCmd(flags *rootFlags) *cobra.Command {
	var at string
	var body string
	var bodyFile string
	var guard bool
	var send bool

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Schedule a Note locally with cadence guard (refuses bursts within 30 min)",
		Example: strings.Trim(`
  substack-pp-cli notes schedule --at 2030-01-01T09:00:00Z --body 'morning take' --json
  substack-pp-cli notes schedule --at 2030-01-01T09:00:00Z --body-file /tmp/note.md --send
`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), scheduleResult{Status: "dry_run"}, flags)
			}
			if strings.TrimSpace(at) == "" {
				return usageErr(fmt.Errorf("--at is required (ISO-8601 timestamp)"))
			}
			tAt, err := time.Parse(time.RFC3339, at)
			if err != nil {
				return usageErr(fmt.Errorf("--at must be ISO-8601 (e.g. 2030-01-01T09:00:00Z): %w", err))
			}
			markdown := body
			if bodyFile != "" {
				data, err := os.ReadFile(bodyFile)
				if err != nil {
					return fmt.Errorf("reading --body-file: %w", err)
				}
				markdown = string(data)
			}
			if strings.TrimSpace(markdown) == "" {
				return usageErr(fmt.Errorf("--body or --body-file is required"))
			}
			pmJSON, err := notebuilder.BuildProseMirrorJSON(markdown)
			if err != nil {
				return fmt.Errorf("building ProseMirror: %w", err)
			}

			if guard {
				if violation, err := checkCadenceGuard(flags, tAt); err == nil && violation != nil {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), violation, flags); printErr != nil {
						return printErr
					}
					return &cliError{code: 2, err: fmt.Errorf("cadence_violation: %s", violation.Message)}
				}
			}

			result := scheduleResult{
				ScheduledAt:    tAt.UTC().Format(time.RFC3339),
				BodyJSONLength: len(pmJSON),
			}

			id, err := queueNote(flags, pmJSON, tAt)
			if err != nil {
				return fmt.Errorf("queueing note: %w", err)
			}
			result.QueuedID = id
			result.Status = "queued"

			if send {
				if cliutil.IsVerifyEnv() {
					result.VerifyEnv = true
					fmt.Fprintln(cmd.ErrOrStderr(), "verify mode short-circuit — would have POSTed to /comment/feed")
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				if err := postScheduledNote(flags, pmJSON); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: send failed: %v\n", err)
				} else {
					result.Sent = true
					result.Status = "fired"
					_ = markFired(flags, id)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "ISO-8601 timestamp when the Note should fire (required)")
	cmd.Flags().StringVar(&body, "body", "", "Note body in Markdown")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read Note body from a Markdown file")
	cmd.Flags().BoolVar(&guard, "guard", true, "Refuse to queue if any own Note in last 24h is within 30 min of --at")
	cmd.Flags().BoolVar(&send, "send", false, "Also fire the Note immediately (otherwise just queue locally)")
	return cmd
}

func checkCadenceGuard(flags *rootFlags, at time.Time) (*cadenceViolation, error) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return nil, nil
	}
	defer st.Close()
	rows, err := st.DB().Query(
		`SELECT id, scheduled_at FROM notes_queue
		 WHERE scheduled_at IS NOT NULL
		 AND scheduled_at >= datetime(?, '-24 hours')
		 AND scheduled_at <= datetime(?, '+24 hours')`,
		at.UTC().Format(time.RFC3339), at.UTC().Format(time.RFC3339))
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var sched string
		if err := rows.Scan(&id, &sched); err != nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, sched)
		if err != nil {
			continue
		}
		delta := math.Abs(t.Sub(at).Minutes())
		if delta < 30 {
			return &cadenceViolation{
				Error:            "cadence_violation",
				OffendingNoteID:  fmt.Sprintf("%d", id),
				TimeDeltaMinutes: int(delta),
				Message:          fmt.Sprintf("another Note is queued within %.0f minutes of %s", delta, at.UTC().Format(time.RFC3339)),
			}, nil
		}
	}
	return nil, nil
}

func queueNote(flags *rootFlags, body []byte, at time.Time) (int64, error) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return 0, err
	}
	defer st.Close()
	res, err := st.DB().Exec(
		`INSERT INTO notes_queue(body_json, scheduled_at, status) VALUES(?, ?, 'queued')`,
		string(body), at.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func markFired(flags *rootFlags, id int64) error {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return err
	}
	defer st.Close()
	_, err = st.DB().Exec(`UPDATE notes_queue SET status = 'fired' WHERE id = ?`, id)
	return err
}

func postScheduledNote(flags *rootFlags, pmJSON []byte) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	_, _, err = c.Post("/comment/feed", map[string]any{
		"type":     "feed",
		"bodyJson": string(pmJSON),
	})
	return err
}
