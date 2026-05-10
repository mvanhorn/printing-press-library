package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

func ensurePipelineTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS pipelines (
			name TEXT PRIMARY KEY,
			spec TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pipeline_runs (
			name TEXT NOT NULL,
			run_id TEXT NOT NULL,
			step_index INTEGER NOT NULL,
			step_name TEXT NOT NULL,
			input_count INTEGER NOT NULL DEFAULT 0,
			output_count INTEGER NOT NULL DEFAULT 0,
			output_ids TEXT,
			ran_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (name, run_id, step_index)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pipeline_runs_name ON pipeline_runs(name)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating pipeline tables: %w", err)
		}
	}
	return nil
}

func newPipelineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Compose and run named E-utility pipelines",
		Long: strings.TrimSpace(`
Agent Pipeline Compositor -- define named pipelines as pipe-separated
E-utility steps with parameters, then run them with a query. Intermediate
results are stored for inspection.

Pipeline spec format: "esearch pubmed '{query}' | elink gene | efetch gene-summary"
Each step is: <command> [args...]`),
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli pipeline create safety-check "esearch pubmed '{query}' | elink gene"
  ncbi-entrez-pp-cli pipeline run safety-check --query "ibuprofen adverse"
  ncbi-entrez-pp-cli pipeline list
  ncbi-entrez-pp-cli pipeline status safety-check
  ncbi-entrez-pp-cli pipeline delete safety-check`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newPipelineCreateCmd(flags))
	cmd.AddCommand(newPipelineRunCmd(flags))
	cmd.AddCommand(newPipelineListCmd(flags))
	cmd.AddCommand(newPipelineDeleteCmd(flags))
	cmd.AddCommand(newPipelineStatusCmd(flags))

	return cmd
}

func newPipelineCreateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "create <name> <spec>",
		Short: "Create a named pipeline",
		Example: strings.TrimSpace(`
  ncbi-entrez-pp-cli pipeline create safety-check "esearch pubmed '{query}' | elink gene"
  ncbi-entrez-pp-cli pipeline create drug-review "esearch pubmed '{query}' | elink gene | elink pubmed"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 && !flags.dryRun {
				return fmt.Errorf("usage: pipeline create <name> <spec>")
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]
			spec := args[1]

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePipelineTables(db); err != nil {
				return err
			}

			_, err = db.DB().Exec(
				`INSERT INTO pipelines (name, spec, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)
				 ON CONFLICT(name) DO UPDATE SET spec = excluded.spec, created_at = CURRENT_TIMESTAMP`,
				name, spec,
			)
			if err != nil {
				return fmt.Errorf("saving pipeline: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "created",
				"name":   name,
				"spec":   spec,
				"steps":  parsePipelineSpec(spec),
			}, flags)
		},
	}
}

// pipelineStep represents one parsed step from a pipeline spec.
type pipelineStep struct {
	Command string `json:"command"`
	Args    string `json:"args"`
}

// parsePipelineSpec splits "esearch pubmed '{query}' | elink gene" into steps.
func parsePipelineSpec(spec string) []pipelineStep {
	parts := strings.Split(spec, "|")
	var steps []pipelineStep
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.SplitN(p, " ", 2)
		step := pipelineStep{Command: fields[0]}
		if len(fields) > 1 {
			step.Args = strings.TrimSpace(fields[1])
		}
		steps = append(steps, step)
	}
	return steps
}

func newPipelineRunCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string

	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Execute a saved pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("pipeline name required")
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePipelineTables(db); err != nil {
				return err
			}

			// Load pipeline spec
			var spec string
			err = db.DB().QueryRow(`SELECT spec FROM pipelines WHERE name = ?`, name).Scan(&spec)
			if err != nil {
				return fmt.Errorf("pipeline %q not found: %w", name, err)
			}

			// Substitute {query}
			if flagQuery != "" {
				spec = strings.ReplaceAll(spec, "{query}", flagQuery)
			}

			steps := parsePipelineSpec(spec)
			if len(steps) == 0 {
				return fmt.Errorf("pipeline %q has no steps", name)
			}

			runID := fmt.Sprintf("%d", time.Now().UnixMilli())
			var currentIDs []string
			var stepResults []map[string]any

			for i, step := range steps {
				fmt.Fprintf(os.Stderr, "step %d: %s %s\n", i+1, step.Command, step.Args)

				var outputIDs []string

				switch step.Command {
				case "esearch":
					// Parse: esearch <db> '<query>'
					parts := strings.SplitN(step.Args, " ", 2)
					dbName := "pubmed"
					query := step.Args
					if len(parts) >= 2 {
						dbName = parts[0]
						query = strings.Trim(parts[1], "'\"")
					}

					params := map[string]string{
						"db":      dbName,
						"term":    query,
						"retmax":  "100",
						"retmode": "json",
					}
					data, apiErr := c.Get("/esearch.fcgi", params)
					if apiErr != nil {
						fmt.Fprintf(os.Stderr, "warning: esearch failed: %v\n", apiErr)
					} else {
						outputIDs = extractPMIDsFromEsearch(data)
					}

				case "elink":
					// Parse: elink <target_db>
					targetDB := strings.TrimSpace(step.Args)
					if targetDB == "" {
						targetDB = "gene"
					}

					if len(currentIDs) > 0 {
						// Determine source DB from context (use pubmed as default)
						sourceDB := "pubmed"
						if i > 0 {
							// Heuristic: if previous step was elink to gene, source is gene
							prev := steps[i-1]
							if prev.Command == "elink" {
								sourceDB = strings.TrimSpace(prev.Args)
							}
						}

						batchSize := 50
						for b := 0; b < len(currentIDs); b += batchSize {
							end := b + batchSize
							if end > len(currentIDs) {
								end = len(currentIDs)
							}
							batch := currentIDs[b:end]

							linkParams := map[string]string{
								"dbfrom":  sourceDB,
								"db":      targetDB,
								"id":      strings.Join(batch, ","),
								"retmode": "json",
								"cmd":     "neighbor",
							}
							data, apiErr := c.Get("/elink.fcgi", linkParams)
							if apiErr != nil {
								fmt.Fprintf(os.Stderr, "warning: elink failed: %v\n", apiErr)
							} else {
								linked := extractLinkedIDs(data)
								outputIDs = append(outputIDs, linked...)
							}
						}
					}

				case "efetch":
					// efetch stores nothing further in pipeline context, just passes IDs through
					outputIDs = currentIDs

				default:
					fmt.Fprintf(os.Stderr, "warning: unknown pipeline step %q, passing IDs through\n", step.Command)
					outputIDs = currentIDs
				}

				// Store step result
				idsJSON := strings.Join(outputIDs, ",")
				db.DB().Exec(
					`INSERT INTO pipeline_runs (name, run_id, step_index, step_name, input_count, output_count, output_ids, ran_at) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
					name, runID, i, step.Command+" "+step.Args, len(currentIDs), len(outputIDs), idsJSON,
				)

				stepResults = append(stepResults, map[string]any{
					"step":         i + 1,
					"command":      step.Command,
					"args":         step.Args,
					"input_count":  len(currentIDs),
					"output_count": len(outputIDs),
				})

				currentIDs = outputIDs
			}

			result := map[string]any{
				"status":    "complete",
				"pipeline":  name,
				"run_id":    runID,
				"query":     flagQuery,
				"steps":     stepResults,
				"final_ids": len(currentIDs),
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagQuery, "query", "", "Query to substitute into pipeline spec")

	return cmd
}

func newPipelineListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List saved multi-step query pipelines with their step counts and creation dates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # List all saved pipelines
  ncbi-entrez-pp-cli pipeline list --json

  # Agent-friendly list
  ncbi-entrez-pp-cli pipeline list --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePipelineTables(db); err != nil {
				return err
			}

			rows, err := db.DB().Query(`SELECT name, spec, created_at FROM pipelines ORDER BY created_at DESC`)
			if err != nil {
				return err
			}
			defer rows.Close()

			var pipelines []map[string]any
			for rows.Next() {
				var name, spec, createdAt string
				if err := rows.Scan(&name, &spec, &createdAt); err != nil {
					return err
				}
				pipelines = append(pipelines, map[string]any{
					"name":       name,
					"spec":       spec,
					"steps":      len(parsePipelineSpec(spec)),
					"created_at": createdAt,
				})
			}
			if pipelines == nil {
				pipelines = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), pipelines, flags)
		},
	}
}

func newPipelineDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("pipeline name required")
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePipelineTables(db); err != nil {
				return err
			}

			db.DB().Exec(`DELETE FROM pipeline_runs WHERE name = ?`, name)
			res, err := db.DB().Exec(`DELETE FROM pipelines WHERE name = ?`, name)
			if err != nil {
				return fmt.Errorf("deleting pipeline: %w", err)
			}

			affected, _ := res.RowsAffected()
			if affected == 0 {
				return fmt.Errorf("pipeline %q not found", name)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "deleted",
				"name":   name,
			}, flags)
		},
	}
}

func newPipelineStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status <name>",
		Short:       "Show the last run results for a pipeline including per-step input and output counts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Check status of a pipeline
  ncbi-entrez-pp-cli pipeline status safety-check --json

  # Agent-friendly status
  ncbi-entrez-pp-cli pipeline status safety-check --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return fmt.Errorf("pipeline name required")
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensurePipelineTables(db); err != nil {
				return err
			}

			// Get the latest run_id
			var latestRunID string
			err = db.DB().QueryRow(
				`SELECT run_id FROM pipeline_runs WHERE name = ? ORDER BY ran_at DESC LIMIT 1`,
				name,
			).Scan(&latestRunID)
			if err != nil {
				return fmt.Errorf("no runs found for pipeline %q", name)
			}

			rows, err := db.DB().Query(
				`SELECT step_index, step_name, input_count, output_count, output_ids, ran_at
				 FROM pipeline_runs
				 WHERE name = ? AND run_id = ?
				 ORDER BY step_index`,
				name, latestRunID,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			var steps []map[string]any
			for rows.Next() {
				var stepIdx, inputCount, outputCount int
				var stepName, outputIDs, ranAt string
				if err := rows.Scan(&stepIdx, &stepName, &inputCount, &outputCount, &outputIDs, &ranAt); err != nil {
					return err
				}
				steps = append(steps, map[string]any{
					"step":         stepIdx + 1,
					"name":         stepName,
					"input_count":  inputCount,
					"output_count": outputCount,
					"ran_at":       ranAt,
				})
			}
			if steps == nil {
				steps = []map[string]any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"pipeline": name,
				"run_id":   latestRunID,
				"steps":    steps,
			}, flags)
		},
	}
}
