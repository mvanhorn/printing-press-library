package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
)

// newZoomScheduleCmd: T6. Creates a cloud meeting and persists the result as a
// saved bookmark in one transaction.
func newZoomScheduleCmd(flags *rootFlags) *cobra.Command {
	var (
		when     string
		duration int
		saveAs   string
		userID   string
		password string
		agenda   string
	)
	cmd := &cobra.Command{
		Use:   "schedule [topic]",
		Short: "Create a cloud meeting and immediately save it as a local bookmark",
		Long: "POSTs to /users/{userId}/meetings (userId defaults to 'me' so it uses the authenticated user's account) " +
			"with the supplied topic + start_time + duration, parses the response, then inserts the resulting ID + " +
			"password into saved_meetings so `zoom-pp-cli saved join <name>` works offline. Requires S2S OAuth.",
		Example: `  zoom-pp-cli schedule "Sprint planning" --when 2026-05-21T15:00:00Z --duration 60 --save-as sprint --json
  zoom-pp-cli schedule "Q3 Planning" --when "2026-08-12 14:00" --save-as q3 --json --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			topic := args[0]
			if when == "" {
				return errors.New("schedule: --when is required (ISO8601, e.g. 2026-05-21T15:00:00Z)")
			}
			start, err := parseSchedTime(when)
			if err != nil {
				return err
			}
			if userID == "" {
				userID = "me"
			}
			if duration == 0 {
				duration = 60
			}

			body := map[string]any{
				"topic":      topic,
				"type":       2, // scheduled meeting
				"start_time": start.Format(time.RFC3339),
				"duration":   duration,
				"timezone":   "UTC",
			}
			if password != "" {
				body["password"] = password
			}
			if agenda != "" {
				body["agenda"] = agenda
			}

			// Dry-run first so the user can test the command shape without
			// needing S2S OAuth configured. Auth check happens only on real
			// invocations.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{
					"would_create":      true,
					"endpoint":          "POST /users/" + userID + "/meetings",
					"body":              body,
					"would_bookmark_as": saveAs,
				})
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			if cfg.AuthHeader() == "" {
				return errors.New("schedule: no Zoom S2S OAuth token available — run `zoom-pp-cli auth set-token` first")
			}

			meeting, err := postZoomMeeting(cmd.Context(), cfg, userID, body)
			if err != nil {
				return err
			}

			out := map[string]any{
				"status":  "created",
				"meeting": meeting,
			}

			if saveAs != "" {
				bm := localstore.SavedMeeting{
					Name:         saveAs,
					MeetingID:    numericField(meeting, "id"),
					Pwd:          stringField(meeting, "password"),
					URL:          stringField(meeting, "join_url"),
					Notes:        agenda,
					ScheduledFor: &start,
				}
				db, closer, derr := openLocalDB(cmd.Context())
				if derr != nil {
					return derr
				}
				defer closer()
				if err := localstore.SaveBookmark(cmd.Context(), db, bm); err != nil {
					return fmt.Errorf("schedule: meeting created but bookmark save failed: %w", err)
				}
				out["bookmark"] = bm
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&when, "when", "", "Scheduled start time (ISO8601 or 'YYYY-MM-DD HH:MM')")
	cmd.Flags().IntVar(&duration, "duration", 60, "Duration in minutes")
	cmd.Flags().StringVar(&saveAs, "save-as", "", "Save the created meeting as this bookmark name")
	cmd.Flags().StringVar(&userID, "user-id", "me", "Owner user ID (defaults to authenticated user)")
	cmd.Flags().StringVar(&password, "password", "", "Optional meeting password")
	cmd.Flags().StringVar(&agenda, "agenda", "", "Optional agenda / notes")
	return cmd
}

func parseSchedTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			if layout != time.RFC3339 {
				t = t.UTC()
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("schedule: cannot parse --when %q (try ISO8601 like 2026-05-21T15:00:00Z)", s)
}

// postZoomMeeting calls the cloud meeting create endpoint directly. We avoid
// the spec-emitted client to keep auth + retry logic narrow.
func postZoomMeeting(ctx context.Context, cfg *config.Config, userID string, body map[string]any) (map[string]any, error) {
	url := strings.TrimRight(cfg.BaseURL, "/") + "/users/" + userID + "/meetings"
	bb, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bb))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", cfg.AuthHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("zoom meeting create: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("zoom meeting create: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("parsing meeting response: %w", err)
	}
	return out, nil
}

func stringField(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// numericField formats an ID-like field as an integer string. Zoom meeting IDs
// arrive as JSON numbers, which json.Unmarshal into map[string]any decodes as
// float64 — and fmt.Sprintf("%v", float64(85123456789)) yields scientific
// notation ("8.5123456789e+10"), which would corrupt the saved bookmark's
// meeting_id and break every later `saved join`. Format float64/json.Number
// with no exponent and no fractional digits; pass strings through verbatim.
func numericField(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch n := v.(type) {
	case string:
		return n
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case json.Number:
		return n.String()
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}
