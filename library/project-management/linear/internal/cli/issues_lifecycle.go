package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// Issue lifecycle and subscription surface (GAP-012, GAP-031).
//
// issueArchive was reachable only through pp-cleanup, which by construction
// only touches ids in the pp_created fixture ledger, so closing out real work
// had no archive path at all. issueUnarchive, issueDelete, issueSubscribe, and
// issueUnsubscribe had no surface whatsoever. All five are live in
// api-inventory.json.
//
// The subcommands are registered on the issues parent in issues.go.

// resolveIssueTarget turns an issue identifier or UUID into the UUID a mutation
// needs and enforces --trust-mode strict against the pp_created ledger. Every
// issue mutation in this file goes through it, so strict mode cannot be
// bypassed by reaching for a different verb.
func resolveIssueTarget(c graphqlQueryer, flags *rootFlags, dbPath, issueRef string) (string, error) {
	return resolveIssueTargetWith(c, flags, dbPath, issueRef, false)
}

// resolveIssueTargetWith is resolveIssueTarget with archived rows opted into.
// Only `issues unarchive` passes true: its subject is archived by definition,
// so resolving through the archive-excluding lookup made every TEAM-NUMBER
// reference exit 3 not_found and left the UUID as the only usable handle. The
// ledger check is identical on both paths, so widening the lookup widens
// nothing else.
func resolveIssueTargetWith(c graphqlQueryer, flags *rootFlags, dbPath, issueRef string, includeArchived bool) (string, error) {
	issueID, err := resolveIssueIDWith(c, issueRef, includeArchived)
	if err != nil {
		return "", classifyLiveReadError(err, flags)
	}
	if err := enforceIssueTrustMode(flags, resolveDBPath(dbPath), issueID, issueRef); err != nil {
		return "", err
	}
	return issueID, nil
}

// trustModeDryRunGuard applies the strict-mode ledger check on the --dry-run
// path. A dry run must not reach the network, so the reference is checked
// against the ledger as given: the ledger records both the UUID and the
// identifier the create mutation returned, which is what makes an offline
// answer possible for a TEAM-NUMBER reference.
//
// It fails closed, like the live gate. Reporting "would update" for a target
// the real invocation rejects after resolution is the failure this guard
// exists to prevent, and a reference the ledger cannot vouch for offline is
// exactly that case.
func trustModeDryRunGuard(flags *rootFlags, dbPath, issueRef string) error {
	if flags == nil || flags.trustMode != "strict" {
		return nil
	}
	db, err := store.Open(resolveDBPath(dbPath))
	if err != nil {
		return usageErr(fmt.Errorf("trust-mode=strict: cannot read the local pp_created ledger at %s: %w\nRun 'linear-pp-cli sync' to create the store, or drop --trust-mode strict", resolveDBPath(dbPath), err))
	}
	defer db.Close()

	created, err := db.IsPPCreatedRef(issueRef)
	if err != nil {
		return usageErr(fmt.Errorf("trust-mode=strict: reading the local pp_created ledger failed: %w", err))
	}
	if !created {
		return usageErr(fmt.Errorf("trust-mode=strict: %s is not in the local pp_created ledger, so this CLI did not create it and the real invocation would refuse it.\nRun 'linear-pp-cli pp-test list' to see the fixtures this CLI owns, or drop --trust-mode strict to mutate pre-existing workspace issues", issueRef))
	}
	return nil
}

// inheritedDBPath reads the --db value the issues parent declares as a
// persistent flag. These subcommands are attached through the extension
// registry and therefore never see the parent's dbPath variable, and declaring
// a second --db of their own would shadow the inherited one.
func inheritedDBPath(cmd *cobra.Command) string {
	if f := cmd.Flags().Lookup("db"); f != nil {
		return f.Value.String()
	}
	return ""
}

func newIssuesArchiveCmd(flags *rootFlags) *cobra.Command {
	var trash bool
	cmd := &cobra.Command{
		Use:   "archive <issue>",
		Short: "Archive a Linear issue",
		Long: `Archive a Linear issue via the issueArchive mutation.

Accepts an issue identifier (ENG-123) or a UUID. This is the general-purpose
archive path: pp-cleanup only ever touches issues in the local pp_created
fixture ledger, so it cannot close out real work.

Pass --trash to send the issue to the trash instead of the archive. Reverse a
plain archive with 'issues unarchive'.`,
		Example: `  linear-pp-cli issues archive ENG-123 --agent
  linear-pp-cli issues archive ENG-123 --trash --yes --agent
  linear-pp-cli issues archive ENG-123 --dry-run --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := inheritedDBPath(cmd)
			if flags.dryRun {
				if err := trustModeDryRunGuard(flags, dbPath, args[0]); err != nil {
					return err
				}
				return renderMutationDryRun(cmd, flags, "would_archive_issue", "issueArchive", map[string]any{
					"input": map[string]any{"id": args[0], "trash": trash},
				})
			}
			if trash {
				if err := confirmMutation(cmd, flags, fmt.Sprintf("Trash issue %s instead of archiving it?", args[0])); err != nil {
					return err
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			issueID, err := resolveIssueTarget(c, flags, dbPath, args[0])
			if err != nil {
				return err
			}
			vars := map[string]any{"id": issueID}
			if trash {
				vars["trash"] = true
			}
			resp, err := c.Mutate(client.IssueArchiveMutation, vars)
			if err != nil {
				return classifyGraphQLMutationError("issueArchive", err, flags)
			}
			issue, err := extractMutationObject(resp, "issueArchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, issue, "issues")
		},
	}
	cmd.Flags().BoolVar(&trash, "trash", false, "Trash the issue instead of archiving it")
	return cmd
}

func newIssuesUnarchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unarchive <issue>",
		Short: "Restore an archived Linear issue",
		Long: `Unarchive a Linear issue via the issueUnarchive mutation. Accepts an issue
identifier (ENG-123) or a UUID.

The identifier lookup opts into archived issues, which is the whole point: an
archived issue is invisible to the ordinary lookup, so without that the only
resolvable handle would be the UUID nobody has written down.`,
		Example: `  linear-pp-cli issues unarchive ENG-123 --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := inheritedDBPath(cmd)
			if flags.dryRun {
				if err := trustModeDryRunGuard(flags, dbPath, args[0]); err != nil {
					return err
				}
				return renderMutationDryRun(cmd, flags, "would_unarchive_issue", "issueUnarchive", map[string]any{
					"input": map[string]any{"id": args[0]},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// includeArchived: true. The subject of an unarchive is archived,
			// so the ordinary resolver can never find it by identifier.
			issueID, err := resolveIssueTargetWith(c, flags, dbPath, args[0], true)
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.IssueUnarchiveMutation, map[string]any{"id": issueID})
			if err != nil {
				return classifyGraphQLMutationError("issueUnarchive", err, flags)
			}
			issue, err := extractMutationObject(resp, "issueUnarchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, issue, "issues")
		},
	}
	return cmd
}

func newIssuesDeleteCmd(flags *rootFlags) *cobra.Command {
	var permanent bool
	cmd := &cobra.Command{
		Use:   "delete <issue>",
		Short: "Trash a Linear issue",
		Long: `Delete (trash) a Linear issue via the issueDelete mutation.

This is the trash, not the archive. Linear holds a trashed issue for a 30-day
grace period and then removes it. Use 'issues archive' when the work is simply
finished.

--permanent sets permanentlyDelete, which skips the grace period entirely and
cannot be undone. Linear allows it for admins only.

Always requires confirmation unless --yes is set. Pass --ignore-missing to
treat an already-deleted issue as a successful no-op.`,
		Example: `  linear-pp-cli issues delete ENG-123 --yes --agent
  linear-pp-cli issues delete ENG-123 --permanent --yes --agent
  linear-pp-cli issues delete ENG-123 --dry-run --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := inheritedDBPath(cmd)
			if flags.dryRun {
				if err := trustModeDryRunGuard(flags, dbPath, args[0]); err != nil {
					return err
				}
				return renderMutationDryRun(cmd, flags, "would_delete_issue", "issueDelete", map[string]any{
					"input": map[string]any{"id": args[0], "permanentlyDelete": permanent},
				})
			}
			prompt := fmt.Sprintf("Trash issue %s? Linear keeps it for 30 days, then removes it.", args[0])
			if permanent {
				prompt = fmt.Sprintf("PERMANENTLY delete issue %s, skipping the 30-day grace period? This cannot be undone.", args[0])
			}
			if err := confirmMutation(cmd, flags, prompt); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			issueID, err := resolveIssueTarget(c, flags, dbPath, args[0])
			if err != nil {
				return err
			}
			vars := map[string]any{"id": issueID}
			if permanent {
				vars["permanentlyDelete"] = true
			}
			resp, err := c.Mutate(client.IssueDeleteMutation, vars)
			if err != nil {
				return classifyGraphQLMutationError("issueDelete", err, flags)
			}
			issue, err := extractMutationObject(resp, "issueDelete", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, issue, "issues")
		},
	}
	cmd.Flags().BoolVar(&permanent, "permanent", false, "Skip the 30-day grace period and delete immediately (admin only, irreversible)")
	return cmd
}

func newIssuesSubscribeCmd(flags *rootFlags) *cobra.Command {
	var sub issueSubscriptionFlags
	cmd := &cobra.Command{
		Use:   "subscribe <issue>",
		Short: "Subscribe a user to a Linear issue",
		Long:  "Subscribe to a Linear issue via the issueSubscribe mutation. Defaults to the authenticated user. Pass --user with a user UUID or --user-email to subscribe someone else.",
		Example: `  linear-pp-cli issues subscribe ENG-123 --agent
  linear-pp-cli issues subscribe ENG-123 --user <user-uuid> --agent`,
		Args: cobra.ExactArgs(1),
		RunE: issueSubscriptionRunE(flags, &sub, issueSubscription{
			event:    "would_subscribe_issue",
			mutation: "issueSubscribe",
			document: client.IssueSubscribeMutation,
		}),
	}
	sub.bind(cmd)
	return cmd
}

func newIssuesUnsubscribeCmd(flags *rootFlags) *cobra.Command {
	var sub issueSubscriptionFlags
	cmd := &cobra.Command{
		Use:   "unsubscribe <issue>",
		Short: "Unsubscribe a user from a Linear issue",
		Long:  "Unsubscribe from a Linear issue via the issueUnsubscribe mutation. Defaults to the authenticated user. Pass --user with a user UUID or --user-email to unsubscribe someone else.",
		Example: `  linear-pp-cli issues unsubscribe ENG-123 --agent
  linear-pp-cli issues unsubscribe ENG-123 --user <user-uuid> --agent`,
		Args: cobra.ExactArgs(1),
		RunE: issueSubscriptionRunE(flags, &sub, issueSubscription{
			event:    "would_unsubscribe_issue",
			mutation: "issueUnsubscribe",
			document: client.IssueUnsubscribeMutation,
		}),
	}
	sub.bind(cmd)
	return cmd
}

// issueSubscriptionFlags carries the two optional subject selectors both
// subscription leaves accept. Each constructor owns its own copy and binds it
// to its own command, so the shared RunE below holds no per-command state.
type issueSubscriptionFlags struct {
	user      string
	userEmail string
}

func (f *issueSubscriptionFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.user, "user", "", "User UUID, defaults to the authenticated user")
	cmd.Flags().StringVar(&f.userEmail, "user-email", "", "User email, defaults to the authenticated user")
}

// issueSubscription describes the two subscription mutations. They take the
// identical argument triple (id, userId, userEmail) and return IssuePayload, so
// they share one implementation.
type issueSubscription struct {
	event    string
	mutation string
	document string
}

func issueSubscriptionRunE(flags *rootFlags, sub *issueSubscriptionFlags, spec issueSubscription) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		dbPath := inheritedDBPath(cmd)
		if sub.user != "" && sub.userEmail != "" {
			return usageErr(fmt.Errorf("pass at most one of --user or --user-email, both default to the authenticated user when omitted"))
		}
		if sub.user != "" && !store.IsUUID(sub.user) {
			return usageErr(fmt.Errorf("--user expects a user UUID (got %q), use --user-email for an address, or run 'linear-pp-cli users' to find the UUID", sub.user))
		}
		vars := map[string]any{"id": args[0]}
		if sub.user != "" {
			vars["userId"] = sub.user
		}
		if sub.userEmail != "" {
			vars["userEmail"] = sub.userEmail
		}
		if flags.dryRun {
			if err := trustModeDryRunGuard(flags, dbPath, args[0]); err != nil {
				return err
			}
			return renderMutationDryRun(cmd, flags, spec.event, spec.mutation, map[string]any{"input": vars})
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		issueID, err := resolveIssueTarget(c, flags, dbPath, args[0])
		if err != nil {
			return err
		}
		vars["id"] = issueID
		resp, err := c.Mutate(spec.document, vars)
		if err != nil {
			return classifyGraphQLMutationError(spec.mutation, err, flags)
		}
		issue, err := extractMutationObject(resp, spec.mutation, "issue")
		if err != nil {
			return err
		}
		return renderLiveObject(cmd, flags, issue, "issues")
	}
}
