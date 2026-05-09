package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

// watchKind is one of: uploader, base-model, feature.
type watchEntry struct {
	ID          string `json:"id"`     // uuid for stable references
	Kind        string `json:"kind"`   // uploader | base-model | feature
	Target      string `json:"target"` // user, base-model id, or feature name
	Notify      string `json:"notify"` // jarvis | stdout | file:<path>
	Since       string `json:"since"`  // ISO timestamp; only models lastModified > since match
	CreatedAt   string `json:"created_at"`
	Description string `json:"description,omitempty"`
}

type watchListResponse struct {
	hfx.Envelope
	StateDir string       `json:"state_dir"`
	Entries  []watchEntry `json:"entries"`
}

type watchAddResponse struct {
	hfx.Envelope
	Added    watchEntry `json:"added"`
	StateDir string     `json:"state_dir"`
}

type watchRemoveResponse struct {
	hfx.Envelope
	Removed watchEntry `json:"removed"`
}

type watchCursor struct {
	SchemaVersion int               `json:"schema_version"`
	UpdatedAt     time.Time         `json:"updated_at"`
	LastPoll      map[string]string `json:"last_poll"` // entry-id → ISO timestamp of last successful poll
}

type watchEvent struct {
	EntryID    string      `json:"entry_id"`
	Kind       string      `json:"kind"`
	Target     string      `json:"target"`
	Match      hfModelLite `json:"match"`
	NotifiedTo string      `json:"notified_to"`
	EmittedAt  string      `json:"emitted_at"`
}

type hfModelLite struct {
	ID           string `json:"id"`
	Author       string `json:"author"`
	LastModified string `json:"last_modified"`
	Downloads    int    `json:"downloads"`
}

type watchPollResponse struct {
	hfx.Envelope
	Polled        int          `json:"polled"`
	NewMatches    int          `json:"new_matches"`
	Events        []watchEvent `json:"events"`
	Cursor        string       `json:"cursor_path"`
	WatchListPath string       `json:"watchlist_path"`
	Explain       string       `json:"explain,omitempty"`
}

const (
	watchFile     = "watch.json"
	watchCursorFn = "watch-cursor.json"
)

func newHFWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch <command>",
		Short: "Manage watch subscriptions on uploaders, base models, or features.",
		Long: `Watch subscribes a target (uploader / base-model / feature) to a periodic poll.
Companion 'watch-poll' command checks for new matches since last poll and emits
structured events (stdout, file, or MC API alert pipeline).

State: <state-dir>/watch.json (subscription list, flock-guarded).
       <state-dir>/watch-cursor.json (last-poll timestamps per entry).

Subcommands: add, list, remove.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	cmd.AddCommand(newHFWatchAddCmd(flags))
	cmd.AddCommand(newHFWatchListCmd(flags))
	cmd.AddCommand(newHFWatchRemoveCmd(flags))
	return cmd
}

func newHFWatchAddCmd(flags *rootFlags) *cobra.Command {
	var kind, notify, since, description string
	cmd := &cobra.Command{
		Use:   "add <target>",
		Short: "Add a target to the watch list.",
		Example: `  huggingface-pp-cli watch add unsloth --kind uploader --notify stdout
  huggingface-pp-cli watch add Qwen/Qwen2.5-7B --kind base-model --notify jarvis
  huggingface-pp-cli watch add mtp --kind feature --notify file:/tmp/hf-watch.jsonl`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			kind = strings.ToLower(strings.TrimSpace(kind))
			switch kind {
			case "uploader", "base-model", "feature":
			default:
				return hfNotFound("--kind %q invalid (use: uploader, base-model, feature)", kind)
			}
			if notify == "" {
				notify = "stdout"
			}
			if since == "" {
				since = time.Now().UTC().Format(time.RFC3339)
			}

			stateDir, err := hfx.EnsureStateDir(flags.stateDir, flags.noWrite)
			if err != nil {
				return hfConfigMissing("resolving state dir: %v", err)
			}
			if flags.noWrite {
				return hfConfigMissing("--no-write set; cannot persist watch entry. Re-run without --no-write.")
			}

			entries, _ := loadWatchList(stateDir)
			entry := watchEntry{
				ID:          uuid.New().String(),
				Kind:        kind,
				Target:      target,
				Notify:      notify,
				Since:       since,
				CreatedAt:   time.Now().UTC().Format(time.RFC3339),
				Description: description,
			}
			entries = append(entries, entry)
			if err := saveWatchList(stateDir, entries, flags.noWrite); err != nil {
				return hfConfigMissing("persisting watch list: %v", err)
			}

			resp := watchAddResponse{
				Envelope: hfx.NewEnvelope("watch add"),
				Added:    entry,
				StateDir: stateDir,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watch added: %s (%s) %s → %s\n", entry.ID[:8], entry.Kind, entry.Target, entry.Notify)
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "uploader", "Watch kind: uploader, base-model, feature")
	cmd.Flags().StringVar(&notify, "notify", "stdout", "Notification sink: stdout, file:<path>, jarvis (writes to data/alerts/<id>.json)")
	cmd.Flags().StringVar(&since, "since", "", "Only notify on models lastModified > this RFC3339 timestamp (default: now)")
	cmd.Flags().StringVar(&description, "description", "", "Optional human-readable label")
	return cmd
}

func newHFWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List active watch entries.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, _ := hfx.StateDir(flags.stateDir)
			entries, err := loadWatchList(stateDir)
			if err != nil && !os.IsNotExist(err) {
				return hfConfigMissing("loading watch list: %v", err)
			}
			resp := watchListResponse{
				Envelope: hfx.NewEnvelope("watch list"),
				StateDir: stateDir,
				Entries:  entries,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(watch list empty)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watch list (%d entries) at %s\n\n", len(entries), filepath.Join(stateDir, watchFile))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-10s  %-12s  %-30s  %-12s  %s\n", "ID", "KIND", "TARGET", "NOTIFY", "SINCE")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-10s  %-12s  %-30s  %-12s  %s\n", e.ID[:8], e.Kind, e.Target, e.Notify, e.Since)
			}
			return nil
		},
	}
}

func newHFWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id-prefix>",
		Short: "Remove a watch entry by id prefix (first 8 chars suffice).",
		Example: `  huggingface-pp-cli watch remove fece9a25
  huggingface-pp-cli watch remove fece9a25-159b --json`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stateDir, _ := hfx.StateDir(flags.stateDir)
			entries, err := loadWatchList(stateDir)
			if err != nil {
				return hfConfigMissing("loading watch list: %v", err)
			}
			prefix := args[0]
			out := make([]watchEntry, 0, len(entries))
			var removed watchEntry
			found := false
			for _, e := range entries {
				if !found && strings.HasPrefix(e.ID, prefix) {
					removed = e
					found = true
					continue
				}
				out = append(out, e)
			}
			if !found {
				return hfNotFound("no watch entry with id prefix %q (use 'watch list' to see ids)", prefix)
			}
			if err := saveWatchList(stateDir, out, flags.noWrite); err != nil {
				return hfConfigMissing("persisting watch list: %v", err)
			}
			resp := watchRemoveResponse{
				Envelope: hfx.NewEnvelope("watch remove"),
				Removed:  removed,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s (%s) %s\n", removed.ID[:8], removed.Kind, removed.Target)
			return nil
		},
	}
}

func loadWatchList(stateDir string) ([]watchEntry, error) {
	if stateDir == "" {
		return nil, os.ErrNotExist
	}
	path := filepath.Join(stateDir, watchFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []watchEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func saveWatchList(stateDir string, entries []watchEntry, noWrite bool) error {
	return hfx.WriteJSONLocked(stateDir, filepath.Join(stateDir, watchFile), entries, noWrite)
}

func loadWatchCursor(stateDir string) watchCursor {
	c := watchCursor{SchemaVersion: hfx.SchemaVersion, LastPoll: map[string]string{}}
	path := filepath.Join(stateDir, watchCursorFn)
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	if c.LastPoll == nil {
		c.LastPoll = map[string]string{}
	}
	return c
}

func saveWatchCursor(stateDir string, c watchCursor, noWrite bool) error {
	c.UpdatedAt = time.Now().UTC()
	c.SchemaVersion = hfx.SchemaVersion
	return hfx.WriteJSONLocked(stateDir, filepath.Join(stateDir, watchCursorFn), c, noWrite)
}
