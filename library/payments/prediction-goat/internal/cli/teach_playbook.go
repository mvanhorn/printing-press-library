// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// teach_playbook.go implements the playbook write surface. Standalone
// `teach-playbook` records a query-family-keyed playbook (structured
// CLI choreography) plus optional free-text notes (gotchas the CLI
// surface doesn't expose). `playbook list` is the inspection
// counterpart to `learnings list`; `playbook amend` is the fire-and-
// forget self-correction surface that appends a timestamped marker to
// an existing family's notes_text using the atomic AppendPlaybookNotes
// store method.
//
// PATCH(learn-loop-backport U8): ported from ESPN PR #851 HEAD
// 9bb0a40a (library/media-and-entertainment/espn/internal/cli/
// teach_playbook.go). Import paths adapted to prediction-goat;
// constructor signatures match prediction-goat's existing
// newTeach* shape (rootFlags pointer only, no separate learnCfg).
// `playbook amend` uses store.AppendPlaybookNotes for race-free
// atomic appends per Greptile round 3 finding on PR #851.

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// newTeachPlaybookCmd builds the standalone command for recording a
// playbook + notes pair for a query family. Use this when only the
// recipe is being recorded (no resource learning to attach). For the
// common end-of-session flow, prefer `teach --playbook-file --notes`
// so both surfaces land in one shot (deferred to a follow-on plan).
func newTeachPlaybookCmd(flags *rootFlags) *cobra.Command {
	var query string
	var playbookFile string
	var notesText string
	var notesFile string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "teach-playbook",
		Short: "Record a CLI playbook + free-text notes for a query family",
		Long: `Stores a structured CLI command sequence (with entity slots) and/or
free-form gotchas/workarounds, keyed on the structural query family.
The recall path surfaces this whenever a future query of the same
family fires, so the agent can replay the choreography and read the
notes verbatim.

At least one of --playbook-file and --notes/--notes-file must be set.

Disabling: pass --no-learn or set ` + noLearnEnvVar + `=true.`,
		Example: `  prediction-goat-pp-cli teach-playbook \
    --query "portugal world cup odds" \
    --playbook-file ~/playbooks/odds-for-team.json \
    --notes-file ~/playbooks/odds-for-team-notes.md`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if noLearnActive(flags) {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(query) == "" {
				return usageErr(fmt.Errorf("--query is required"))
			}
			if strings.TrimSpace(playbookFile) == "" && strings.TrimSpace(notesText) == "" && strings.TrimSpace(notesFile) == "" {
				return usageErr(fmt.Errorf("at least one of --playbook-file, --notes, --notes-file is required"))
			}

			playbookJSON, notes, err := resolvePlaybookInputs(playbookFile, notesText, notesFile)
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("teach-playbook: open db: %w", err)
			}
			defer s.Close()
			normalized := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
			resolver := learn.NewCanonicalResolver(cmd.Context(), s.DB())
			normalized = learn.PromoteEntities(normalized, resolver)
			family := learn.QueryFamily(normalized)
			if family == "" {
				return fmt.Errorf("teach-playbook: query normalized to empty family")
			}

			id, inserted, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
				QueryFamily:  family,
				PlaybookJSON: playbookJSON,
				NotesText:    notes,
			})
			if err != nil {
				return fmt.Errorf("teach-playbook: %w", err)
			}

			if auditErr := appendLearningsAudit(map[string]any{
				"action":         "playbook-teach",
				"query":          query,
				"query_family":   family,
				"playbook_id":    id,
				"newly_inserted": inserted,
			}); auditErr != nil {
				// Audit failure is non-fatal; the row is already in the DB.
				writeTeachLog(fmt.Sprintf("teach-playbook: audit append: %v", auditErr))
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"recorded":     true,
					"playbook_id":  id,
					"inserted":     inserted,
					"query_family": family,
					"has_playbook": playbookJSON != "",
					"has_notes":    notes != "",
				}, flags)
			}
			// Default: silent on success, matching teach / teach-lookup.
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Example query that anchors the family (required)")
	cmd.Flags().StringVar(&playbookFile, "playbook-file", "", "Path to a JSON file with the playbook (steps, entity_slots, expected_tool_calls)")
	cmd.Flags().StringVar(&notesText, "notes", "", "Free-text notes (gotchas, workarounds) -- mutually exclusive with --notes-file")
	cmd.Flags().StringVar(&notesFile, "notes-file", "", "Path to a markdown file with the notes")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// newPlaybookCmd is the inspection + amendment parent. `playbook list`
// lists stored playbooks; `playbook amend` appends notes to an
// existing family.
func newPlaybookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playbook",
		Short: "Inspect or amend stored CLI playbooks",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPlaybookListCmd(flags))
	cmd.AddCommand(newPlaybookAmendCmd(flags))
	return cmd
}

// newPlaybookAmendCmd is the self-correction surface: a one-line
// CLI call that appends a timestamped note to an existing playbook's
// notes_text (or creates a notes-only playbook if the family has
// none). Designed for agents to fire when their debug-protocol
// response identifies a concrete correction worth persisting.
//
// Same fire-and-forget posture as `teach` -- silent on success,
// errors to teach.log, safe to background with &.
//
// CRITICAL (Greptile PR #851 round 3): this command uses
// store.AppendPlaybookNotes (atomic, runs inside a single transaction
// under writeMu) NOT GetPlaybookByFamily + UpsertPlaybook. The atomic
// path is race-free; the read-then-write composition would lose notes
// under concurrent `playbook amend &` background invocations.
func newPlaybookAmendCmd(flags *rootFlags) *cobra.Command {
	var query string
	var addNote string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "amend",
		Short: "Append a note to an existing playbook (LLM-fired self-correction, silent)",
		Long: `Appends a timestamped note to the matching family's playbook notes_text.
If no playbook exists for the family yet, creates a notes-only one.

Designed for agents to fire when their debug-protocol response
identifies a concrete correction worth persisting (a workaround, an
undocumented endpoint shape, a stale field name, observed schema
drift). Same fire-and-forget posture as teach: silent on success,
errors to teach.log, safe to background with &.

Disabling: pass --no-learn or set ` + noLearnEnvVar + `=true.`,
		Example: `  prediction-goat-pp-cli playbook amend \
    --query "<exact recall query>" \
    --add-note "kalshi events list pages by created_at desc; pagination cursor is 'cursor', not 'page'"`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if noLearnActive(flags) {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(query) == "" {
				writeTeachLog(fmt.Sprintf("playbook amend: missing --query (args=%v)", args))
				return silentCodeErr(2)
			}
			if strings.TrimSpace(addNote) == "" {
				writeTeachLog(fmt.Sprintf("playbook amend: missing --add-note for query=%q", query))
				return silentCodeErr(2)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				writeTeachLog(fmt.Sprintf("playbook amend: open db: %v", err))
				return silentCodeErr(1)
			}
			defer s.Close()

			normalized := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
			resolver := learn.NewCanonicalResolver(cmd.Context(), s.DB())
			normalized = learn.PromoteEntities(normalized, resolver)
			family := learn.QueryFamily(normalized)
			if family == "" {
				writeTeachLog(fmt.Sprintf("playbook amend: query normalized to empty family: %q", query))
				return silentCodeErr(2)
			}

			// AppendPlaybookNotes runs the read+update inside a single
			// transaction under writeMu, so two concurrent
			// `playbook amend` calls for the same family cannot
			// race-overwrite each other (e.g. when SKILL.md tells the
			// agent to background amend with `&` across overlapping
			// sessions). Per Greptile PR #851 round 3 finding.
			marker := fmt.Sprintf("\n\n[amend %s]: %s", time.Now().UTC().Format("2006-01-02T15:04Z"), addNote)
			if _, _, err := s.AppendPlaybookNotes(family, marker); err != nil {
				writeTeachLog(fmt.Sprintf("playbook amend: append family=%q: %v", family, err))
				return silentCodeErr(1)
			}

			if auditErr := appendLearningsAudit(map[string]any{
				"action":       "playbook-amend",
				"query":        query,
				"query_family": family,
				"add_note":     addNote,
			}); auditErr != nil {
				writeTeachLog(fmt.Sprintf("playbook amend: audit append: %v", auditErr))
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"amended":      true,
					"query":        query,
					"query_family": family,
				}, flags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "The exact recall query whose family should be amended (required)")
	cmd.Flags().StringVar(&addNote, "add-note", "", "The note text to append to the family's playbook (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newPlaybookListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List stored playbooks (query_family, content presence, last observed)",
		Example:     `  prediction-goat-pp-cli playbook list --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("playbook list: %w", err)
			}
			defer s.Close()
			rows, err := s.ListPlaybooks()
			if err != nil {
				return fmt.Errorf("playbook list: %w", err)
			}
			if rows == nil {
				rows = []store.PlaybookRow{}
			}

			if auditErr := appendLearningsAudit(map[string]any{
				"action": "playbook-list",
				"count":  len(rows),
			}); auditErr != nil {
				writeTeachLog(fmt.Sprintf("playbook list: audit append: %v", auditErr))
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no playbooks recorded)")
				return nil
			}
			for _, r := range rows {
				lastObs := ""
				if r.LastObservedAt != nil {
					lastObs = r.LastObservedAt.UTC().Format(time.RFC3339)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\tplaybook=%v\tnotes=%v\tconf=%d\t%s\n",
					r.QueryFamily, r.PlaybookJSON != "", r.NotesText != "", r.Confidence, lastObs)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// resolvePlaybookInputs loads the playbook JSON (if a file path was
// given) and the notes text (preferring inline --notes over
// --notes-file). Returns (playbookJSON, notesText, error). Either
// output may be empty; the caller validates at-least-one-non-empty.
func resolvePlaybookInputs(playbookFile, notesInline, notesFile string) (string, string, error) {
	var playbookJSON string
	if strings.TrimSpace(playbookFile) != "" {
		// Validate that it parses as a Playbook; reject garbage early.
		pb, err := learn.ParsePlaybookFile(playbookFile)
		if err != nil {
			return "", "", fmt.Errorf("teach-playbook: %w", err)
		}
		out, err := learn.MarshalPlaybook(pb)
		if err != nil {
			return "", "", fmt.Errorf("teach-playbook: re-marshal: %w", err)
		}
		playbookJSON = out
	}
	var notes string
	if strings.TrimSpace(notesInline) != "" {
		notes = notesInline
	} else if strings.TrimSpace(notesFile) != "" {
		data, err := os.ReadFile(notesFile)
		if err != nil {
			return "", "", fmt.Errorf("teach-playbook: read notes file %s: %w", notesFile, err)
		}
		notes = string(data)
	}
	return playbookJSON, notes, nil
}
