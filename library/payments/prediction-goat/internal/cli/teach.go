// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// teach.go implements the LLM-driven learning surface for the
// prediction-goat CLI: `teach` (fire-and-forget, silent on success,
// safe to background with &), `recall` (pre-discovery short-circuit
// for known queries), `learnings list` (human inspection), and
// `forget` (human undo).
//
// The teach call writes one row per `--resource` under a normalized
// query_pattern. Repeating the same call bumps confidence instead of
// duplicating; the row's source is preserved on bump.
//
// `recall` returns rows whose normalized query_pattern overlaps the
// supplied query by token-set Jaccard >= 0.6 (configurable).
//
// All four commands honor `--no-learn` (per-invocation) and
// `PREDICTION_GOAT_NO_LEARN=true` (environment-wide). When disabled,
// `teach` is a silent no-op (exit 0) and `recall` returns
// `{found: false, results: []}`.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/recipes"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// noLearnEnvVar is the environment variable that disables both teach
// (write side) and the rerank apply layer (read side) for a session.
// Used by deterministic agent flows that don't want a learning row to
// silently change subsequent query results.
const noLearnEnvVar = "PREDICTION_GOAT_NO_LEARN"

// learningsAuditFileName is the JSONL audit log alongside the DB.
const learningsAuditFileName = "learnings.jsonl"

// teachLogFileName is the error log for backgrounded teach failures.
// Stdout/stderr from a backgrounded LLM-fired teach must never leak
// into the user-facing response, so failures go here instead.
const teachLogFileName = "teach.log"

// noLearnEnabled reports whether the env var has the value "true" or
// "1" (case-insensitive). Other values are treated as not set.
func noLearnEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(noLearnEnvVar)))
	return v == "true" || v == "1" || v == "yes"
}

// learningsStateDir returns the directory hosting the DB, audit log,
// and teach.log. Created on first use with 0o700 so a multi-user
// machine doesn't accidentally expose one user's learned queries.
func learningsStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "prediction-goat-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// learningsAuditPath returns ~/.local/share/prediction-goat-pp-cli/learnings.jsonl.
func learningsAuditPath() (string, error) {
	dir, err := learningsStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, learningsAuditFileName), nil
}

// teachLogPath returns ~/.local/share/prediction-goat-pp-cli/teach.log.
func teachLogPath() (string, error) {
	dir, err := learningsStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, teachLogFileName), nil
}

// writeTeachLog appends a single line to teach.log. Best-effort: a
// failure to open the log file is silently swallowed, since the call
// site is already in the error path of a backgrounded teach.
func writeTeachLog(line string) {
	p, err := teachLogPath()
	if err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Fprintf(f, "%s %s\n", ts, line)
}

// appendLearningsAudit records one event in the JSONL audit log.
func appendLearningsAudit(entry map[string]any) error {
	p, err := learningsAuditPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	entry["ts"] = time.Now().UTC().Format(time.RFC3339)
	return json.NewEncoder(f).Encode(entry)
}

// newTeachCmd builds the `teach` cobra command — the LLM-facing write
// surface. Silent on success, safe to background, errors only to
// teach.log.
func newTeachCmd(flags *rootFlags) *cobra.Command {
	var query string
	var resources []string
	var venueArg string
	var resourceType string
	var quiet bool
	var dbPath string
	var notes string
	// noValidate suppresses the U6 teach-time resource-shape validator
	// for scripted / batch teaches where the caller has already vetted
	// the resource shape and doesn't want a structured warning written
	// to teach.log. The teach itself is unaffected by this flag.
	var noValidate bool
	// Playbook side -- optional. When any of these is set, after the
	// resource learning lands, also upsert a learning_playbooks row
	// keyed on the query family. Failures here log to teach.log but
	// don't fail the resource learning (graceful degrade). PATCH
	// (learn-loop-backport-u13-integrated-playbook): mirrors ESPN's
	// teach --playbook-file path so SKILL.md Step 5 records both the
	// resource learning and the structured playbook in ONE call
	// instead of two (teach + teach-playbook). Greptile P-equivalent
	// gap from the U1-U12 backport.
	var playbookFile string
	var playbookNotesInline string
	var playbookNotesFile string

	cmd := &cobra.Command{
		Use:   "teach",
		Short: "Record a query -> resource mapping for future rerank (LLM-fired, silent)",
		Long: `Record one or more "this query → this resource" mappings so the next
query that matches surfaces the right tickers without re-running discovery.

This command is designed to be backgrounded by an LLM right before it
emits the user-facing response: silent on success, errors only to
~/.local/share/prediction-goat-pp-cli/teach.log, safe to fire-and-forget.

Disabling: pass --no-learn or set PREDICTION_GOAT_NO_LEARN=true.`,
		Example: `  prediction-goat-pp-cli teach --query "portugal world cup odds" \
    --resource KXMENWORLDCUP-26-PT \
    --resource will-portugal-win-the-2026-fifa-world-cup-912 &`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Silence everything on the cobra command — even usage errors —
			// because this command is designed to be backgrounded.
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			// Disabled? Silent no-op.
			if flags.noLearn || noLearnEnabled() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(query) == "" {
				writeTeachLog(fmt.Sprintf("teach: missing --query (args=%v resources=%v)", args, resources))
				return silentCodeErr(2)
			}
			if len(resources) == 0 {
				writeTeachLog(fmt.Sprintf("teach: missing --resource for query=%q", query))
				return silentCodeErr(2)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				writeTeachLog(fmt.Sprintf("teach: open db: %v", err))
				return silentCodeErr(1)
			}
			defer db.Close()

			// Extract entities once per teach call. The same case-
			// preserving entity slice lands on every row this teach
			// writes, so the recall path's match validator sees a
			// consistent entity signal regardless of how many resources
			// the teach is fanning out across.
			normalized := learn.Normalize(query, learn.DefaultPredictionGoatConfig())
			// Apply entity_lookups promotion symmetrically with recall so
			// lowercase / numeric-prefix aliases (e.g., "usa", "49ers")
			// land in query_entities even when the capitalization-based
			// extractor missed them. Without this, recall's cross-alias
			// canonical resolver has nothing to compare against on the
			// stored side and a query taught under "usa" stays a literal
			// match only.
			resolver := learn.NewCanonicalResolver(cmd.Context(), db.DB())
			normalized = learn.PromoteEntities(normalized, resolver)
			confidences := make(map[string]int, len(resources))
			for _, rid := range resources {
				rid = strings.TrimSpace(rid)
				if rid == "" {
					continue
				}
				_, _, uerr := db.UpsertLearning(store.UpsertLearningInput{
					Query:         query,
					QueryEntities: normalized.Entities,
					ResourceID:    rid,
					ResourceType:  resourceType,
					Venue:         venueArg,
					Source:        store.LearningSourceTaught,
					Notes:         notes,
				})
				if uerr != nil {
					writeTeachLog(fmt.Sprintf("teach: upsert %q for query=%q: %v", rid, query, uerr))
					return silentCodeErr(1)
				}
				// Read back confidence for the audit log.
				rows, _ := db.ListLearnings(store.ListLearningsFilter{Query: query, ResourceID: rid})
				if len(rows) > 0 {
					confidences[rid] = rows[0].Confidence
				}

				// U6: teach-time resource-shape validation. Soft
				// warnings only -- the teach has already succeeded.
				// Warnings land in teach.log; learnings list
				// --warnings surfaces them. Disabled via
				// --no-validate for scripted/batch teaches.
				if !noValidate {
					for _, w := range learn.ValidateResourceShape(cmd.Context(), db.DB(), query, rid, resourceType) {
						if err := learn.AppendTeachLogWarning("teach", query, w); err != nil {
							// Log-side failure is informational. The
							// existing writeTeachLog helper goes to
							// the same file in plain-text form so a
							// human inspecting the log still sees
							// the diagnostic line.
							writeTeachLog(fmt.Sprintf("teach: warn append: %v", err))
						}
					}
				}
			}

			if err := appendLearningsAudit(map[string]any{
				"action":      "teach",
				"query":       query,
				"normalized":  store.NormalizeQuery(query),
				"resources":   resources,
				"venue":       venueArg,
				"confidences": confidences,
			}); err != nil {
				// Audit failure is non-fatal; the row is already in the DB.
				writeTeachLog(fmt.Sprintf("teach: audit append: %v", err))
			}

			// Optional integrated playbook upsert. Any of three flags may
			// be set; all three empty means skip. Failures log to
			// teach.log but don't fail the resource learning above --
			// the agent's primary write (resource learning) already
			// succeeded, so degraded playbook recording is acceptable.
			// Mirrors ESPN's upsertPlaybookFromTeach call site so
			// SKILL.md Step 5's one-call protocol works.
			if strings.TrimSpace(playbookFile) != "" || strings.TrimSpace(playbookNotesInline) != "" || strings.TrimSpace(playbookNotesFile) != "" {
				if pbErr := upsertPlaybookFromTeach(db, cmd.Context(), query, playbookFile, playbookNotesInline, playbookNotesFile, normalized); pbErr != nil {
					writeTeachLog(fmt.Sprintf("teach: playbook upsert: %v", pbErr))
				}
			}

			// U10: post-teach generalization. Recipe extraction looks
			// at the most-recent N teaches, groups them by structural
			// signature, and writes a search_recipes row when a kind
			// binding holds across every member of a group. The call
			// is cheap and idempotent (the unique index silences
			// duplicate triples), so firing it on every teach is
			// safe even when nothing new is groupable.
			//
			// Errors are non-fatal and routed to teach.log alongside
			// the audit-append failure path: a recipe-extraction
			// failure is a learning-side optimization miss, not a
			// reason to fail the underlying teach.
			//
			// TODO: when U6 lands the learn.Teach helper, move this
			// trigger inside that helper so the order
			// (UpsertLearning -> ValidateResourceShape ->
			// recipes.Extract) is centralized in one place.
			if _, extractErr := recipes.Extract(db.DB()); extractErr != nil {
				writeTeachLog(fmt.Sprintf("teach: recipes.Extract: %v", extractErr))
			}

			if !quiet && flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"recorded":   true,
					"query":      query,
					"normalized": store.NormalizeQuery(query),
					"resources":  resources,
				}, flags)
			}
			// Default: silent on success.
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "User's original natural-language question (required)")
	cmd.Flags().StringSliceVar(&resources, "resource", nil, "Resource ID (ticker or slug) — repeat for multiple")
	cmd.Flags().StringVar(&venueArg, "venue", "", "Venue scope: polymarket | kalshi (default: both)")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "Resource type (e.g. kalshi_markets, markets) — recommended for synthetic-hit lookup")
	cmd.Flags().BoolVar(&quiet, "quiet", true, "Silent on success (default true — designed for background invocation)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().StringVar(&notes, "notes", "", "Optional free-form note recorded with the learning")
	cmd.Flags().BoolVar(&noValidate, "no-validate", false, "Suppress the U6 teach-time resource-shape validator (warnings to teach.log)")
	cmd.Flags().StringVar(&playbookFile, "playbook-file", "", "Optional path to a JSON playbook recording the CLI choreography for this query family")
	cmd.Flags().StringVar(&playbookNotesInline, "playbook-notes", "", "Optional inline gotchas/workarounds for this query family (stored alongside the playbook)")
	cmd.Flags().StringVar(&playbookNotesFile, "playbook-notes-file", "", "Optional path to a markdown file with playbook notes")
	return cmd
}

// upsertPlaybookFromTeach loads playbook + notes inputs and writes a
// learning_playbooks row keyed on the normalized query family. Helper
// for the teach command's optional integrated playbook write. Mirrors
// ESPN's upsertPlaybookFromTeach so SKILL.md Step 5's one-call form
// works in prediction-goat. resolvePlaybookInputs lives in
// teach_playbook.go (same package).
func upsertPlaybookFromTeach(s *store.Store, ctx context.Context, query, playbookFile, notesInline, notesFile string, normalized learn.NormalizedQuery) error {
	_ = ctx
	playbookJSON, notes, err := resolvePlaybookInputs(playbookFile, notesInline, notesFile)
	if err != nil {
		return err
	}
	if playbookJSON == "" && notes == "" {
		return nil
	}
	family := learn.QueryFamily(normalized)
	if family == "" {
		return fmt.Errorf("query normalized to empty family")
	}
	_, _, err = s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily:  family,
		PlaybookJSON: playbookJSON,
		NotesText:    notes,
	})
	return err
}

// recallEnvelope is the JSON shape returned by `recall --agent`. The
// LLM consumes this before deciding whether to skip discovery. The
// shape mirrors learn.Result so the agent-context schema can refer
// to a single contract; field order is preserved across versions for
// stable parsing by older agents that only key on {found, results}.
//
// U3 added query_entities, mismatches, and per-row entity_match +
// resource_entities + warnings. Existing consumers that only read
// {found, query, normalized, match_score, results[*].{resource_id,
// confidence, source, last_observed_at}} continue to work.
type recallEnvelope struct {
	Found         bool                    `json:"found"`
	Query         string                  `json:"query"`
	Normalized    string                  `json:"normalized"`
	QueryEntities []string                `json:"query_entities"`
	MatchScore    float64                 `json:"match_score,omitempty"`
	Results       []recallEnvelopeResult  `json:"results"`
	Mismatches    []recallEnvelopeResult  `json:"mismatches,omitempty"`
	Warnings      []string                `json:"warnings,omitempty"`
	Playbook      *learn.ResolvedPlaybook `json:"playbook,omitempty"`
	Notes         string                  `json:"notes,omitempty"`
}

type recallEnvelopeResult struct {
	ResourceID       string     `json:"resource_id"`
	ResourceType     string     `json:"resource_type,omitempty"`
	Venue            string     `json:"venue,omitempty"`
	Action           string     `json:"action"`
	Confidence       int        `json:"confidence"`
	MatchScore       float64    `json:"match_score"`
	EntityMatch      string     `json:"entity_match,omitempty"`
	ResourceEntities []string   `json:"resource_entities,omitempty"`
	Source           string     `json:"source"`
	LastObservedAt   *time.Time `json:"last_observed_at,omitempty"`
	AliasTarget      string     `json:"alias_target,omitempty"`
	Warnings         []string   `json:"warnings,omitempty"`
}

// newRecallCmd builds the read side of the loop: LLM calls `recall`
// FIRST when starting work on a new user question, and uses the
// returned tickers to short-circuit discovery.
//
// U3 wired the body to learn.Recall, which adds entity-aware match
// validation: results whose resource entities don't overlap with the
// query entities land in the mismatches array (debug-only, surfaced
// with --debug-mismatches) rather than poisoning the default results
// path. Per docs/plans/2026-05-23-002 section U3.
func newRecallCmd(flags *rootFlags) *cobra.Command {
	var minConf int
	var limit int
	var dbPath string
	var debugMismatches bool

	cmd := &cobra.Command{
		Use:   "recall <query>",
		Short: "Check prior learnings for a query before running discovery (LLM-fired, pre-discovery)",
		Long: `Returns prior learnings matching the supplied query by token-set
overlap (Jaccard >= 0.6) plus entity-aware validation. The LLM should
call this FIRST when starting work on a new user question; if
found=true AND results[0].entity_match == "exact" AND confidence >= 2,
the LLM can skip topic/compare entirely and go straight to live price
fetch for the returned tickers.

Each result carries an entity_match verdict (exact | partial | unknown)
and an optional warnings array. The most important warning is
parent_event_when_child_exists — when present, the better target is the
named child ticker, not the matched parent.

Empty match returns {"found": false, "results": []} with exit 0 — this
is an information query, not a not-found error.

Pass --debug-mismatches to surface candidates that cleared the Jaccard
threshold but failed entity validation (e.g., England query against a
Portugal-tagged learning). Useful for diagnosing why a high-overlap
candidate was dropped.

Disabling: PREDICTION_GOAT_NO_LEARN=true returns the empty shape even
when learnings exist.`,
		Example: `  prediction-goat-pp-cli recall "portugal world cup odds" --agent
  prediction-goat-pp-cli recall "lakers tonight" --agent --min-confidence 2
  prediction-goat-pp-cli recall "england world cup odds" --agent --debug-mismatches`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			envelope := recallEnvelope{
				Query:         query,
				Normalized:    store.NormalizeQuery(query),
				QueryEntities: []string{},
				Results:       []recallEnvelopeResult{},
			}
			if flags.noLearn || noLearnEnabled() {
				return emitRecall(cmd, flags, envelope)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("recall: %w", err)
			}
			defer db.Close()

			result, err := learn.Recall(cmd.Context(), db.DB(), query, learn.Opts{
				MinConfidence:   minConf,
				Limit:           limit,
				DebugMismatches: debugMismatches,
			})
			if err != nil {
				return fmt.Errorf("recall: %w", err)
			}
			envelope.Found = result.Found
			envelope.Normalized = result.Normalized
			envelope.QueryEntities = result.QueryEntities
			envelope.MatchScore = result.MatchScore
			envelope.Results = toEnvelopeResults(result.Results)
			if debugMismatches {
				envelope.Mismatches = toEnvelopeResults(result.Mismatches)
			}
			envelope.Warnings = result.Warnings
			// U7 backport: surface the playbook + notes when the query
			// family matches a stored learning_playbooks row. The agent
			// reads playbook.steps and notes before any discovery call.
			envelope.Playbook = result.Playbook
			envelope.Notes = result.Notes
			return emitRecall(cmd, flags, envelope)
		},
	}
	cmd.Flags().IntVar(&minConf, "min-confidence", 1, "Minimum confidence to include in results")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of learnings to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().BoolVar(&debugMismatches, "debug-mismatches", false, "Include cross-entity mismatches in the envelope under mismatches[]")
	return cmd
}

// toEnvelopeResults adapts the learn.Hit slice to the CLI's
// recallEnvelopeResult slice. Field-for-field copy; kept as its own
// helper so the typeshape mapping lives in one place (rather than
// inline in both Results and Mismatches assignments).
func toEnvelopeResults(in []learn.Hit) []recallEnvelopeResult {
	if len(in) == 0 {
		return []recallEnvelopeResult{}
	}
	out := make([]recallEnvelopeResult, 0, len(in))
	for _, h := range in {
		out = append(out, recallEnvelopeResult{
			ResourceID:       h.ResourceID,
			ResourceType:     h.ResourceType,
			Venue:            h.Venue,
			Action:           h.Action,
			Confidence:       h.Confidence,
			MatchScore:       h.MatchScore,
			EntityMatch:      h.EntityMatch,
			ResourceEntities: h.ResourceEntities,
			Source:           h.Source,
			LastObservedAt:   h.LastObservedAt,
			AliasTarget:      h.AliasTarget,
			Warnings:         h.Warnings,
		})
	}
	return out
}

// emitRecall renders the recall envelope in the chosen output mode.
func emitRecall(cmd *cobra.Command, flags *rootFlags, env recallEnvelope) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), env, flags)
	}
	if !env.Found {
		fmt.Fprintf(cmd.OutOrStdout(), "no prior learnings for %q\n", env.Query)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%d learning(s) for %q (top match %.2f):\n", len(env.Results), env.Query, env.MatchScore)
	for _, r := range env.Results {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\tconf=%d\tmatch=%.2f\n", r.Action, r.ResourceID, r.Confidence, r.MatchScore)
	}
	return nil
}

// newLearningsCmd is the human-inspection surface. `learnings list`
// lists rows; `learnings forget` is a parent for the forget verb.
// Forget itself is registered as a sibling at root.
func newLearningsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learnings",
		Short: "Inspect the local search-learnings table (taught query -> resource mappings)",
		Long: `Surface for browsing and filtering the search_learnings table that
the LLM populates via the 'teach' command. To delete rows, use the
top-level 'forget' command.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newLearningsListCmd(flags))
	return cmd
}

func newLearningsListCmd(flags *rootFlags) *cobra.Command {
	var queryFilter string
	var sourceFilter string
	var resourceFilter string
	var actionFilter string
	var minConf int
	var limit int
	var dbPath string
	// warningsOnly switches the output to U6 teach.log entries. When
	// true, the envelope omits the row list and returns the structured
	// warnings filtered (optionally) by --resource.
	var warningsOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded learnings",
		Example: `  prediction-goat-pp-cli learnings list --agent
  prediction-goat-pp-cli learnings list --query portugal
  prediction-goat-pp-cli learnings list --source taught --agent
  prediction-goat-pp-cli learnings list --warnings --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// --warnings reads teach.log directly. No DB needed.
			if warningsOnly {
				var filterIDs []string
				if strings.TrimSpace(resourceFilter) != "" {
					filterIDs = []string{resourceFilter}
				}
				entries, err := learn.ReadTeachLogWarnings(filterIDs...)
				if err != nil {
					return fmt.Errorf("learnings list --warnings: %w", err)
				}
				// Always return a non-nil slice so JSON consumers
				// (`jq '.warnings | length'`) don't trip on null.
				if entries == nil {
					entries = []learn.TeachLogEntry{}
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"warnings": entries,
					}, flags)
				}
				if len(entries) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "(no teach.log warnings recorded)")
					return nil
				}
				for _, e := range entries {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\tsuggested=%s\n",
						e.TS, e.Warning, e.Resource, e.Suggested)
				}
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("learnings list: %w", err)
			}
			defer db.Close()

			rows, err := db.ListLearnings(store.ListLearningsFilter{
				Query:         queryFilter,
				Source:        sourceFilter,
				ResourceID:    resourceFilter,
				Action:        actionFilter,
				MinConfidence: minConf,
				Limit:         limit,
			})
			if err != nil {
				return fmt.Errorf("learnings list: %w", err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no learnings recorded)")
				return nil
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\tconf=%d\tsource=%s\n",
					r.QueryPattern, r.Action, r.ResourceID, r.Confidence, r.Source)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&queryFilter, "query", "", "Filter by normalized query substring")
	cmd.Flags().StringVar(&sourceFilter, "source", "", "Filter by source (taught | manual-forget)")
	cmd.Flags().StringVar(&resourceFilter, "resource", "", "Filter by resource_id")
	cmd.Flags().StringVar(&actionFilter, "action", "", "Filter by action (boost | hide | alias_of)")
	cmd.Flags().IntVar(&minConf, "min-confidence", 0, "Filter by minimum confidence")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().BoolVar(&warningsOnly, "warnings", false, "Return U6 teach-time warnings from teach.log instead of stored learning rows")
	return cmd
}

// newForgetCmd is the destructive surface — a human un-does a bad
// learning, or a session pre-test reset. Requires at least one of
// --resource / --action, or --all to wipe every rule for that query.
func newForgetCmd(flags *rootFlags) *cobra.Command {
	var resourceArg string
	var actionArg string
	var all bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "forget <query>",
		Short: "Delete learnings matching a query (use --all to wipe every rule for that query)",
		Long: `Removes rows from the search_learnings table so a bad teach can be
undone without dropping the whole DB.

Requires at least one of --resource, --action, or --all.`,
		Example: `  prediction-goat-pp-cli forget "portugal world cup" --resource KXMENWORLDCUP-26-PT
  prediction-goat-pp-cli forget "portugal world cup" --all`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("forget: %w", err)
			}
			defer db.Close()

			n, err := db.ForgetLearnings(store.ForgetLearningsFilter{
				Query:      query,
				ResourceID: resourceArg,
				Action:     actionArg,
				All:        all,
			})
			if err != nil {
				return usageErr(fmt.Errorf("forget: %w", err))
			}
			_ = appendLearningsAudit(map[string]any{
				"action":       "forget",
				"query":        query,
				"normalized":   store.NormalizeQuery(query),
				"filter":       map[string]any{"resource": resourceArg, "action": actionArg, "all": all},
				"rows_deleted": n,
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"deleted": n,
					"query":   query,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forgot %d learning(s) for %q\n", n, query)
			return nil
		},
	}
	cmd.Flags().StringVar(&resourceArg, "resource", "", "Delete only the rule for this resource_id")
	cmd.Flags().StringVar(&actionArg, "action", "", "Delete only rules with this action (boost | hide | alias_of)")
	cmd.Flags().BoolVar(&all, "all", false, "Delete every rule for the supplied query")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// silentCodeErr returns an error that ExitCode honors but that carries
// no message. Used by the teach command's error path so a backgrounded
// failure leaks nothing to the parent shell's stderr — the diagnosis
// lives in teach.log instead.
func silentCodeErr(code int) error {
	return &cliError{code: code, err: silentSentinel{}}
}

// silentSentinel implements error with an empty string so cobra has
// nothing to print even if SilenceErrors regresses.
type silentSentinel struct{}

func (silentSentinel) Error() string { return "" }

// teachApplier is the rerank Applier the topic command uses; it lives
// in teach.go (alongside the learnings code) so the apply-side surface
// stays close to the teach-side. compareApplier lives next to it.
//
// resolveLearnedHit is the helper that synthesizes a topicHit from the
// resources table when a boost rule's resource_id isn't already in the
// FTS bundle.
func resolveLearnedHit(ctx context.Context, db *store.Store, h store.LearnedHit) (topicHit, bool) {
	if strings.TrimSpace(h.ResourceID) == "" {
		return topicHit{}, false
	}
	rt := h.ResourceType
	candidates := []string{rt}
	if rt == "" {
		// No resource_type recorded; try the price-bearing tables in order.
		candidates = []string{"kalshi_markets", "markets", "kalshi_events", "events", "kalshi_series", "tags"}
	}
	for _, c := range candidates {
		raw, err := db.Get(c, h.ResourceID)
		if err != nil {
			continue
		}
		hit, ok := topicHitFromJSON(c, h.ResourceID, string(raw))
		if !ok {
			continue
		}
		return hit, true
	}
	return topicHit{}, false
}

// applyLearningsForTopic adapts the rerank layer to topic.go's hit
// slice. Returns the (possibly-rewritten) hits, the count of rules
// that touched the output, and a teach_hint if no high-confidence
// boost fired for this query.
func applyLearningsForTopic(ctx context.Context, db *store.Store, query string, hits []topicHit) ([]topicHit, int, bool) {
	ap := &topicApplier{ctx: ctx, db: db, hits: hits}
	res, err := db.Apply(ctx, query, ap)
	if err != nil {
		writeTeachLog(fmt.Sprintf("apply learnings for topic %q: %v", query, err))
		return hits, 0, false
	}
	for _, w := range res.Warnings {
		writeTeachLog(fmt.Sprintf("apply (topic): %s", w))
	}
	return ap.hits, res.Count, res.HasHighConfidence
}

type topicApplier struct {
	ctx  context.Context
	db   *store.Store
	hits []topicHit
}

func (a *topicApplier) HasHit(rt, rid string) bool {
	for _, h := range a.hits {
		if h.ID == rid && (rt == "" || matchesTopicResourceType(h, rt)) {
			return true
		}
	}
	return false
}

func (a *topicApplier) MoveToFront(rt, rid string) {
	idx := -1
	for i, h := range a.hits {
		if h.ID == rid && (rt == "" || matchesTopicResourceType(h, rt)) {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return
	}
	h := a.hits[idx]
	a.hits = append(a.hits[:idx], a.hits[idx+1:]...)
	a.hits = append([]topicHit{h}, a.hits...)
}

func (a *topicApplier) InsertLearnedHit(h store.LearnedHit) error {
	hit, ok := resolveLearnedHit(a.ctx, a.db, h)
	if !ok {
		return fmt.Errorf("resource not found in local DB")
	}
	a.hits = append([]topicHit{hit}, a.hits...)
	return nil
}

func (a *topicApplier) RemoveHit(rt, rid string) {
	for i, h := range a.hits {
		if h.ID == rid && (rt == "" || matchesTopicResourceType(h, rt)) {
			a.hits = append(a.hits[:i], a.hits[i+1:]...)
			return
		}
	}
}

func (a *topicApplier) ReplaceHit(srcType, srcID, dstType, dstID string) error {
	for i, h := range a.hits {
		if h.ID == srcID && (srcType == "" || matchesTopicResourceType(h, srcType)) {
			hit, ok := resolveLearnedHit(a.ctx, a.db, store.LearnedHit{ResourceType: dstType, ResourceID: dstID})
			if !ok {
				return fmt.Errorf("alias target not found in local DB")
			}
			a.hits[i] = hit
			return nil
		}
	}
	// Source not in bundle; insert the alias target as a learned hit.
	return a.InsertLearnedHit(store.LearnedHit{ResourceType: dstType, ResourceID: dstID})
}

// matchesTopicResourceType maps the resource-table type ("kalshi_markets",
// "markets", etc.) onto the topicHit's (Source, Kind) pair so the
// rerank layer can address a hit by its store-side type.
func matchesTopicResourceType(h topicHit, rt string) bool {
	switch rt {
	case "markets":
		return h.Source == "polymarket" && h.Kind == "market"
	case "events":
		return h.Source == "polymarket" && h.Kind == "event"
	case "tags":
		return h.Source == "polymarket" && h.Kind == "tag"
	case "kalshi_markets":
		return h.Source == "kalshi" && h.Kind == "market"
	case "kalshi_events":
		return h.Source == "kalshi" && h.Kind == "event"
	case "kalshi_series":
		return h.Source == "kalshi" && h.Kind == "series"
	}
	return false
}

// applyLearningsForCompare adapts the rerank layer to compare.go's
// pair slice. Boosts reorder pairs by moving the pair containing the
// boosted ID to the front; hides drop the matching pair; aliases swap
// the venue-side ID. compare's structural pair shape means most rules
// fire less aggressively than they do on topic, which is intentional —
// compare is venue-symmetric and a synthetic insert there would yield
// a one-sided pair the agent already gets from a single-venue topic.
func applyLearningsForCompare(ctx context.Context, db *store.Store, query string, pairs []comparePair) ([]comparePair, int, bool) {
	ap := &compareApplier{ctx: ctx, db: db, pairs: pairs}
	res, err := db.Apply(ctx, query, ap)
	if err != nil {
		writeTeachLog(fmt.Sprintf("apply learnings for compare %q: %v", query, err))
		return pairs, 0, false
	}
	for _, w := range res.Warnings {
		writeTeachLog(fmt.Sprintf("apply (compare): %s", w))
	}
	return ap.pairs, res.Count, res.HasHighConfidence
}

type compareApplier struct {
	ctx   context.Context
	db    *store.Store
	pairs []comparePair
}

func (a *compareApplier) HasHit(rt, rid string) bool {
	for _, p := range a.pairs {
		if pairHasResource(p, rt, rid) {
			return true
		}
	}
	return false
}

func (a *compareApplier) MoveToFront(rt, rid string) {
	idx := -1
	for i, p := range a.pairs {
		if pairHasResource(p, rt, rid) {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return
	}
	p := a.pairs[idx]
	a.pairs = append(a.pairs[:idx], a.pairs[idx+1:]...)
	a.pairs = append([]comparePair{p}, a.pairs...)
}

func (a *compareApplier) InsertLearnedHit(h store.LearnedHit) error {
	// compare pairs are bilateral by design; synthesizing a one-sided
	// pair here would mislead — the agent should run topic with a
	// venue scope instead. Record this as a no-op with a teach.log entry.
	writeTeachLog(fmt.Sprintf("compare apply: skipping synthetic insert for %s/%s (compare is bilateral)", h.ResourceType, h.ResourceID))
	return nil
}

func (a *compareApplier) RemoveHit(rt, rid string) {
	for i, p := range a.pairs {
		if pairHasResource(p, rt, rid) {
			a.pairs = append(a.pairs[:i], a.pairs[i+1:]...)
			return
		}
	}
}

func (a *compareApplier) ReplaceHit(srcType, srcID, dstType, dstID string) error {
	// Aliases on compare are not implemented: the venue-symmetric shape
	// of compare pairs makes the right replacement ambiguous. Log and
	// continue.
	writeTeachLog(fmt.Sprintf("compare apply: skipping alias %s -> %s (compare is bilateral)", srcID, dstID))
	return nil
}

func pairHasResource(p comparePair, rt, rid string) bool {
	if p.PM != nil && p.PM.ID == rid && (rt == "" || rt == "markets") {
		return true
	}
	if p.Kalshi != nil && p.Kalshi.ID == rid && (rt == "" || rt == "kalshi_markets") {
		return true
	}
	return false
}

// teachHintFor returns the suggested teach call the LLM should fire
// in the background before emitting its final response. Empty string
// means no hint is needed (e.g., a high-confidence boost already
// applied for this query).
func teachHintFor(query string, applied int, highConfidence bool, hits int) string {
	if highConfidence {
		return ""
	}
	if hits == 0 {
		return ""
	}
	// Escape the query for inclusion inside a shell-style double-quoted
	// string. The LLM is the consumer; the safest thing to do is keep
	// the hint simple and identical-shape every time.
	safeQuery := strings.ReplaceAll(query, `"`, `\"`)
	return fmt.Sprintf(
		`If you use these results in your final response, run: prediction-goat-pp-cli teach --query "%s" --resource <id> [--resource <id>...] &`,
		safeQuery,
	)
}

// noLearnActive is the combined check used by topic.go and compare.go
// to decide whether to skip the rerank layer entirely. Exported (lower-
// case) within-package so the call sites stay declarative.
func noLearnActive(flags *rootFlags) bool {
	if flags != nil && flags.noLearn {
		return true
	}
	return noLearnEnabled()
}
