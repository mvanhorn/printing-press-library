package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// newPPTestCmd is the parent for pp-test subcommands. It shows fixtures the
// CLI created in the current or named session — issues an agent or operator
// can safely mutate or archive without touching pre-existing tickets.
func newPPTestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pp-test",
		Short: "List Linear issues this CLI has created (test-fixture ledger)",
		Long: `Inspect the local pp_created ledger — issues this CLI created during
testing or agent runs. The ledger is populated automatically every time
'issues create' returns successfully. Pair with 'pp-cleanup' to archive
only the fixtures this CLI made.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newPPTestListCmd(flags))
	cmd.AddCommand(newPPTestSessionsCmd(flags))
	return cmd
}

func newPPTestListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var session string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active fixtures (issues this CLI created and has not yet archived)",
		Example: `  linear-pp-cli pp-test list
  linear-pp-cli pp-test list --session current
  linear-pp-cli pp-test list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
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
			fixtures, err := db.ListPPFixtures(sess)
			if err != nil {
				return err
			}
			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(fixtures)
			}
			if len(fixtures) == 0 {
				fmt.Println("No active fixtures. (Run 'issues create' to populate the ledger, or pass --session to scope to a specific session.)")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "IDENTIFIER\tSESSION\tCREATED\tTITLE")
			for _, f := range fixtures {
				title := f.Title
				if len(title) > 50 {
					title = title[:47] + "..."
				}
				ident := f.Identifier
				if ident == "" {
					ident = f.IssueID[:8]
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ident, f.Session, f.CreatedAt, title)
			}
			tw.Flush()
			fmt.Fprintf(os.Stderr, "\n%d active fixtures across pp_created ledger\n", len(fixtures))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&session, "session", "", "Filter by session tag ('current' resolves to PP_SESSION env or current run timestamp)")
	return cmd
}

func newPPTestSessionsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List all distinct session tags with active fixtures",
		Example: `  linear-pp-cli pp-test sessions
  linear-pp-cli pp-test sessions --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			sessions, err := db.ListPPSessions()
			if err != nil {
				return err
			}
			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sessions)
			}
			if len(sessions) == 0 {
				fmt.Println("No sessions with active fixtures.")
				return nil
			}
			for _, s := range sessions {
				fmt.Println(s)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
