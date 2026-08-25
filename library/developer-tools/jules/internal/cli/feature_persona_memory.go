// Feature 8: Persona Memory Learning
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

// personaResourceType is the local-store resource_type used to persist
// personas in the generic "resources" table (see internal/store). Like
// checkpoints, personas are a CLI-local concept with no corresponding API
// resource, so the generic key/value resource helpers are the right fit
// rather than a generated typed schema.
const personaResourceType = "personas"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		personaCmd := newPersonaCmd(flags)
		addNovelCommandIfAbsent(root, personaCmd)
	})
}

func newPersonaCmd(flags *rootFlags) *cobra.Command {
	var personaName string
	var sessionID string
	var outcome string

	cmd := &cobra.Command{
		Use:   "persona",
		Short: "Record and reuse Jules work patterns as personas",
		Long: `Learn and replay successful Jules execution patterns.

Personas capture:
- The prompt and title of the session they were recorded from
- The recorded outcome (e.g. success, failure)
- When they were recorded

Personas are stored locally (jules-pp-cli's SQLite store) and are not sent to
the Jules API; reuse one by looking up its prompt with 'persona show' and
passing it to 'sessions create --prompt'.`,
		Example: `  # Record a session's outcome as a persona pattern
  jules-pp-cli persona record --name "refactor-pattern" --session-id abc123 --outcome success

  # List learned personas
  jules-pp-cli persona list

  # Show a persona's captured prompt and reuse it for a new session
  jules-pp-cli persona show --name "refactor-pattern" --json
  jules-pp-cli sessions create --prompt "<the persona's saved prompt>"

  # Delete a persona
  jules-pp-cli persona delete --name "refactor-pattern"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			subcommand := args[0]
			switch subcommand {
			case "record":
				if personaName == "" {
					return fmt.Errorf("--name is required")
				}
				if sessionID == "" {
					return fmt.Errorf("--session-id is required")
				}
				return personaRecord(cmd.Context(), flags, personaName, sessionID, outcome, cmd.OutOrStdout())
			case "list":
				return personaList(cmd.Context(), flags, cmd.OutOrStdout())
			case "show":
				if personaName == "" {
					return fmt.Errorf("--name is required")
				}
				return personaShow(cmd.Context(), flags, personaName, cmd.OutOrStdout())
			case "delete":
				if personaName == "" {
					return fmt.Errorf("--name is required")
				}
				return personaDelete(cmd.Context(), flags, personaName, cmd.OutOrStdout())
			default:
				return cmd.Help()
			}
		},
	}

	cmd.Flags().StringVar(&personaName, "name", "", "Persona name")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID to record the persona from (required for record)")
	cmd.Flags().StringVar(&outcome, "outcome", "", "Outcome to record alongside the persona (e.g. success, failure)")

	return cmd
}

type personaEntry struct {
	Name       string `json:"name"`
	SessionID  string `json:"sessionID"`
	Outcome    string `json:"outcome"`
	Prompt     string `json:"prompt"`
	Title      string `json:"title"`
	State      string `json:"state"`
	RecordedAt string `json:"recordedAt"`
}

func personaRecord(ctx context.Context, flags *rootFlags, personaName, sessionID, outcome string, out io.Writer) error {
	if flags.dryRun {
		if flags.asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"dry_run": true, "name": personaName, "session_id": sessionID, "outcome": outcome})
		}
		fmt.Fprintf(out, "Dry run: would record persona %q from session %s (no local write performed)\n", personaName, sessionID)
		return nil
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/sessions/%s", sessionID)
	data, err := c.Get(ctx, path, map[string]string{})
	if err != nil {
		return fmt.Errorf("fetching session: %w", err)
	}

	var session map[string]any
	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Errorf("decoding session: %w", err)
	}

	prompt, _ := session["prompt"].(string)
	title, _ := session["title"].(string)
	state, _ := session["state"].(string)

	rec := personaEntry{
		Name:       personaName,
		SessionID:  sessionID,
		Outcome:    outcome,
		Prompt:     prompt,
		Title:      title,
		State:      state,
		RecordedAt: time.Now().Format(time.RFC3339),
	}

	recJSON, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encoding persona: %w", err)
	}

	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local persona store: %w", err)
	}
	defer db.Close()

	if err := db.Upsert(personaResourceType, personaName, recJSON); err != nil {
		return fmt.Errorf("saving persona: %w", err)
	}

	if flags.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"recorded": true, "name": personaName, "session_id": sessionID, "outcome": outcome})
	}
	fmt.Fprintf(out, "Persona recorded: %s (from session %s, outcome=%s)\n", personaName, sessionID, outcome)
	return nil
}

func personaList(ctx context.Context, flags *rootFlags, out io.Writer) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local persona store: %w", err)
	}
	defer db.Close()

	all, err := db.List(personaResourceType, 0)
	if err != nil {
		return fmt.Errorf("listing personas: %w", err)
	}

	var records []personaEntry
	for _, raw := range all {
		var rec personaEntry
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })

	if flags.asJSON {
		if records == nil {
			records = []personaEntry{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"personas": records})
	}

	fmt.Fprintf(out, "Learned Personas:\n")
	if len(records) == 0 {
		fmt.Fprintf(out, "  (no personas recorded yet)\n")
		fmt.Fprintf(out, "\nRecord a persona from a successful session:\n")
		fmt.Fprintf(out, "  jules-pp-cli persona record --name <name> --session-id <id> --outcome success\n")
		return nil
	}
	for _, rec := range records {
		fmt.Fprintf(out, "  %s (session=%s, outcome=%s, recorded=%s)\n", rec.Name, rec.SessionID, rec.Outcome, rec.RecordedAt)
	}
	return nil
}

func personaShow(ctx context.Context, flags *rootFlags, personaName string, out io.Writer) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local persona store: %w", err)
	}
	defer db.Close()

	raw, err := db.Get(personaResourceType, personaName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no persona named %q; run 'persona list' to see recorded personas", personaName)
		}
		return fmt.Errorf("loading persona: %w", err)
	}

	var rec personaEntry
	if err := json.Unmarshal(raw, &rec); err != nil {
		return fmt.Errorf("decoding persona: %w", err)
	}

	if flags.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rec)
	}

	fmt.Fprintf(out, "Persona: %s\n", rec.Name)
	fmt.Fprintf(out, "  Session:   %s\n", rec.SessionID)
	fmt.Fprintf(out, "  Outcome:   %s\n", rec.Outcome)
	fmt.Fprintf(out, "  State:     %s\n", rec.State)
	fmt.Fprintf(out, "  Title:     %s\n", rec.Title)
	fmt.Fprintf(out, "  Prompt:    %s\n", rec.Prompt)
	fmt.Fprintf(out, "  Recorded:  %s\n", rec.RecordedAt)
	return nil
}

func personaDelete(ctx context.Context, flags *rootFlags, personaName string, out io.Writer) error {
	db, err := store.OpenWithContext(ctx, defaultDBPath("jules-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local persona store: %w", err)
	}
	defer db.Close()

	res, err := db.DB().ExecContext(ctx, `DELETE FROM resources WHERE resource_type = ? AND id = ?`, personaResourceType, personaName)
	if err != nil {
		return fmt.Errorf("deleting persona: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no persona named %q; run 'persona list' to see recorded personas", personaName)
	}

	fmt.Fprintf(out, "Persona deleted: %s\n", personaName)
	return nil
}
