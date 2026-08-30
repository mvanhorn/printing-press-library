// Feature 3: Working Tree Checkpoint/Restore
// pp:data-source local
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/jules/internal/store"
	"github.com/spf13/cobra"
)

// checkpointResourceType is the local-store resource_type used to persist
// checkpoints in the generic "resources" table (see internal/store). This
// feature has no corresponding API resource -- checkpoints are a CLI-local
// concept -- so there's no generated internal/types/store schema to route
// through; the generic key/value resource helpers are the right fit.
const checkpointResourceType = "checkpoints"

// checkpointRecordID builds the composite key a checkpoint is stored under:
// scoped by session so checkpointList can filter to one session's saves.
func checkpointRecordID(sessionID, label string) string {
	return sessionID + "::" + label
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		checkpointCmd := newCheckpointCmd(flags)
		addNovelCommandIfAbsent(root, checkpointCmd)
	})
}

func newCheckpointCmd(flags *rootFlags) *cobra.Command {
	var sessionID string
	var label string

	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Save and restore working tree checkpoints for sessions",
		Long: `Manage checkpoints of session state and work in progress.

Checkpoints enable safe mid-session work recovery, preventing work loss from interruptions.`,
		Example: `  # Save current session state
  jules-pp-cli checkpoint save --session-id abc123 --label "after-pr-review"

  # List checkpoints for a session
  jules-pp-cli checkpoint list --session-id abc123

  # Restore from checkpoint
  jules-pp-cli checkpoint restore --session-id abc123 --label "after-pr-review"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			switch args[0] {
			case "save":
				return checkpointSave(cmd.Context(), flags, sessionID, label, cmd.OutOrStdout())
			case "list":
				return checkpointList(cmd.Context(), flags, sessionID, cmd.OutOrStdout())
			case "restore":
				if label == "" {
					return fmt.Errorf("--label is required for restore action")
				}
				return checkpointRestore(cmd.Context(), flags, sessionID, label, cmd.OutOrStdout())
			default:
				return cmd.Help()
			}
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (required)")
	cmd.Flags().StringVar(&label, "label", "", "Checkpoint label (required for save/restore)")
	_ = cmd.MarkFlagRequired("session-id")

	return cmd
}

func checkpointSave(ctx context.Context, flags *rootFlags, sessionID, label string, out io.Writer) error {
	if label == "" {
		label = fmt.Sprintf("checkpoint-%d", time.Now().Unix())
	}

	if flags.dryRun {
		if flags.asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"dry_run": true, "session_id": sessionID, "label": label})
		}
		fmt.Fprintf(out, "Dry run: would save checkpoint %q for session %s (no local write performed)\n", label, sessionID)
		return nil
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	// Fetch current session state
	path := fmt.Sprintf("/sessions/%s", sessionID)
	data, err := c.Get(ctx, path, map[string]string{})
	if err != nil {
		return fmt.Errorf("fetching session: %w", err)
	}

	var sessionData map[string]any
	if err := json.Unmarshal(data, &sessionData); err != nil {
		return err
	}

	// Fetch activities to include in checkpoint
	activitiesPath := fmt.Sprintf("/sessions/%s/activities", sessionID)
	activitiesData, err := c.Get(ctx, activitiesPath, map[string]string{"pageSize": "100"})
	if err == nil {
		var activities map[string]any
		if err := json.Unmarshal(activitiesData, &activities); err == nil {
			sessionData["_checkpoint_activities"] = activities
		}
	}

	checkpoint := map[string]any{
		"sessionID":   sessionID,
		"label":       label,
		"timestamp":   time.Now().Format(time.RFC3339),
		"sessionData": sessionData,
	}

	checkpointJSON, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("encoding checkpoint: %w", err)
	}

	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local checkpoint store: %w", err)
	}
	defer db.Close()

	if err := db.Upsert(checkpointResourceType, checkpointRecordID(sessionID, label), checkpointJSON); err != nil {
		return fmt.Errorf("saving checkpoint: %w", err)
	}

	if flags.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"saved": true, "session_id": sessionID, "label": label})
	}
	fmt.Fprintf(out, "Checkpoint saved: %s (session %s)\n", label, sessionID)
	return nil
}

func checkpointList(ctx context.Context, flags *rootFlags, sessionID string, out io.Writer) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local checkpoint store: %w", err)
	}
	defer db.Close()

	all, err := db.List(checkpointResourceType, 0)
	if err != nil {
		return fmt.Errorf("listing checkpoints: %w", err)
	}

	type checkpointSummary struct {
		Label     string `json:"label"`
		Timestamp string `json:"timestamp"`
	}
	var matches []checkpointSummary
	for _, raw := range all {
		var cp map[string]any
		if err := json.Unmarshal(raw, &cp); err != nil {
			continue
		}
		if sid, _ := cp["sessionID"].(string); sid != sessionID {
			continue
		}
		label, _ := cp["label"].(string)
		ts, _ := cp["timestamp"].(string)
		matches = append(matches, checkpointSummary{Label: label, Timestamp: ts})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Timestamp < matches[j].Timestamp })

	if flags.asJSON {
		if matches == nil {
			matches = []checkpointSummary{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"session_id": sessionID, "checkpoints": matches})
	}

	fmt.Fprintf(out, "Checkpoints for session %s:\n", sessionID)
	if len(matches) == 0 {
		fmt.Fprintf(out, "  (no checkpoints saved for this session)\n")
		return nil
	}
	for _, m := range matches {
		fmt.Fprintf(out, "  %s (saved %s)\n", m.Label, m.Timestamp)
	}
	return nil
}

func checkpointRestore(ctx context.Context, flags *rootFlags, sessionID, label string, out io.Writer) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local checkpoint store: %w", err)
	}
	defer db.Close()

	raw, err := db.Get(checkpointResourceType, checkpointRecordID(sessionID, label))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no checkpoint %q found for session %s; run 'checkpoint list --session-id %s' to see saved checkpoints", label, sessionID, sessionID)
		}
		return fmt.Errorf("loading checkpoint: %w", err)
	}

	var checkpoint map[string]any
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		return fmt.Errorf("decoding saved checkpoint: %w", err)
	}

	savedSessionData, _ := checkpoint["sessionData"].(map[string]any)
	savedState, _ := savedSessionData["state"].(string)
	savedAt, _ := checkpoint["timestamp"].(string)

	// Compare against the session's current state so the operator can see
	// whether anything has moved since the checkpoint was taken. This is a
	// read-only comparison: applying a checkpoint's contents back onto a
	// live session is not something the Jules API exposes a write for.
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/sessions/%s", sessionID)
	data, err := c.Get(ctx, path, map[string]string{})
	if err != nil {
		return fmt.Errorf("fetching current session state: %w", err)
	}
	var currentSessionData map[string]any
	if err := json.Unmarshal(data, &currentSessionData); err != nil {
		return fmt.Errorf("decoding current session state: %w", err)
	}
	currentState, _ := currentSessionData["state"].(string)

	if flags.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"session_id":       sessionID,
			"label":            label,
			"saved_at":         savedAt,
			"checkpoint_state": savedState,
			"current_state":    currentState,
			"changed":          savedState != currentState,
		})
	}

	fmt.Fprintf(out, "Checkpoint %q (session %s, saved %s):\n", label, sessionID, savedAt)
	fmt.Fprintf(out, "  Checkpoint state: %s\n", savedState)
	fmt.Fprintf(out, "  Current state:    %s\n", currentState)
	if savedState != currentState {
		fmt.Fprintf(out, "  Session has changed since this checkpoint was saved.\n")
	} else {
		fmt.Fprintf(out, "  Session state matches the checkpoint.\n")
	}

	return nil
}
