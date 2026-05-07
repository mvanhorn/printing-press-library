package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// runStartedAt is the process-wide default session for fixtures created during
// this invocation. Captured at package-init so the session value is stable
// across all commands in a single CLI run.
var runStartedAt = time.Now().UTC().Format("20060102-150405")

// ppCurrentSession returns the session tag for fixtures created in this run.
// Order of precedence: PP_SESSION env var, then the run-started timestamp.
func ppCurrentSession() string {
	if v := os.Getenv("PP_SESSION"); v != "" {
		return v
	}
	return runStartedAt
}

// resolvePPSession picks the session value to use for a command. Priority:
// (1) explicit --session/--pp-session flag, (2) the special "current" string
// expanding to PP_SESSION/run-timestamp, (3) all sessions when empty.
func resolvePPSession(flags *rootFlags, override string) string {
	if override != "" {
		return override
	}
	if flags != nil && flags.ppSession != "" {
		return flags.ppSession
	}
	return ""
}

// archiveIssueGraphQL invokes Linear's issueArchive mutation. Kept here (not
// in client.go) because it's only used by pp-cleanup; pp-cleanup is the only
// command in the CLI that mutates Linear from a local fixture list.
func archiveIssueGraphQL(c interface {
	Mutate(string, map[string]any) (json.RawMessage, error)
}, issueID string) error {
	const mutation = `mutation ArchiveIssue($id: String!) {
		issueArchive(id: $id) { success }
	}`
	resp, err := c.Mutate(mutation, map[string]any{"id": issueID})
	if err != nil {
		return err
	}
	var parsed struct {
		IssueArchive struct {
			Success bool `json:"success"`
		} `json:"issueArchive"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return fmt.Errorf("parsing issueArchive response: %w", err)
	}
	if !parsed.IssueArchive.Success {
		return fmt.Errorf("Linear reported issueArchive(%s) success=false", issueID)
	}
	return nil
}

func newPPCleanupCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var session string
	var dryRun bool
	var force bool
	cmd := &cobra.Command{
		Use:   "pp-cleanup",
		Short: "Archive Linear issues this CLI created (scoped to the pp_created ledger)",
		Long: `Archive every issue this CLI recorded in the pp_created ledger for the
named session. Touches ONLY issues this CLI made — pre-existing tickets
in the workspace are never affected. The archive call hits Linear's real
issueArchive mutation; the local ledger is only used to enumerate which
fixtures to archive.`,
		Example: `  # Dry-run: show what would be archived
  linear-pp-cli pp-cleanup --session current --dry-run

  # Archive every fixture in the current session
  linear-pp-cli pp-cleanup --session current --yes

  # Archive every fixture across every session
  linear-pp-cli pp-cleanup --session all --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			sess := resolvePPSession(flags, session)
			if sess == "current" {
				sess = ppCurrentSession()
			}
			query := sess
			if strings.EqualFold(sess, "all") {
				query = ""
			}
			fixtures, err := db.ListPPFixtures(query)
			if err != nil {
				return err
			}
			if len(fixtures) == 0 {
				fmt.Println("No active fixtures to archive.")
				return nil
			}

			// Honor verify-mode: don't actually mutate
			if dryRun || flags.dryRun {
				if flags.asJSON {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{
						"event":   "would_archive",
						"count":   len(fixtures),
						"session": sess,
						"items":   fixtures,
					})
				}
				fmt.Printf("Would archive %d fixtures (session=%q):\n", len(fixtures), sess)
				for _, f := range fixtures {
					ident := f.Identifier
					if ident == "" {
						ident = f.IssueID[:8]
					}
					fmt.Printf("  %s  %s\n", ident, f.Title)
				}
				return nil
			}

			// Confirmation gate (unless --force or --yes)
			if !force && !flags.yes {
				if flags.noInput {
					return fmt.Errorf("pp-cleanup needs explicit confirmation: pass --yes or --force (or remove --no-input)")
				}
				fmt.Fprintf(os.Stderr, "Archive %d Linear issues recorded by this CLI in session %q? [y/N] ", len(fixtures), sess)
				var resp string
				fmt.Scanln(&resp)
				if !strings.EqualFold(resp, "y") && !strings.EqualFold(resp, "yes") {
					return fmt.Errorf("aborted")
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			archived := 0
			failed := 0
			results := make([]map[string]any, 0, len(fixtures))
			for _, f := range fixtures {
				err := archiveIssueGraphQL(c, f.IssueID)
				if err != nil {
					failed++
					results = append(results, map[string]any{
						"identifier": f.Identifier,
						"issue_id":   f.IssueID,
						"status":     "error",
						"error":      err.Error(),
					})
					if !flags.asJSON {
						fmt.Fprintf(os.Stderr, "  fail %s: %v\n", f.Identifier, err)
					}
					continue
				}
				_ = db.MarkPPFixtureArchived(f.IssueID)
				archived++
				results = append(results, map[string]any{
					"identifier": f.Identifier,
					"issue_id":   f.IssueID,
					"status":     "archived",
				})
				if !flags.asJSON {
					ident := f.Identifier
					if ident == "" {
						ident = f.IssueID[:8]
					}
					fmt.Printf("  archived %s\n", ident)
				}
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"event":    "pp_cleanup_summary",
					"session":  sess,
					"archived": archived,
					"failed":   failed,
					"items":    results,
				})
			}
			fmt.Fprintf(os.Stderr, "\n%d archived, %d failed\n", archived, failed)
			if failed > 0 {
				return fmt.Errorf("%d fixture(s) failed to archive", failed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&session, "session", "current", "Session tag to clean up; 'current' resolves to PP_SESSION/run timestamp; 'all' archives every active fixture")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be archived without calling the API")
	cmd.Flags().BoolVar(&force, "force", false, "Skip the confirmation prompt (alias for --yes)")
	return cmd
}
