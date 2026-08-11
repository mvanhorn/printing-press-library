package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// Notification inbox (GAP-032).
//
// The whole family is VIEWER-SCOPED. Query.notifications is documented as
// "The authenticated user's notifications", and every mutation here acts on
// the same inbox. There is no way to read or mutate another user's
// notifications through this API, so none of these commands take a --user
// flag and every help string says so.
//
// Two shapes of write exist upstream and both are exposed, because they are
// not interchangeable:
//
//	notificationUpdate(id:, input:)   one notification, by notification id
//	notification*All(input:, ...)     a notification AND everything Linear
//	                                  groups with it, addressed by ENTITY
//
// The *All family takes NotificationEntityInput, which names an issue, an
// initiative, a project update, an initiative update, an OAuth client
// approval, or a single notification id. It does not take a list of
// notification ids, so `notifications read-all` is entity-scoped rather than
// inbox-wide. That is what is live, so that is what ships.
//
// NotificationEntityInput.projectId is deprecated ("[DEPRECATED] The id of
// the project related to the notification") and is deliberately not exposed.

// resourceTypeNotifications is the envelope resource type for every read and
// every rendered mutation payload in this file.
const resourceTypeNotifications = "notifications"

// notificationListPageCap bounds the client-side unread walk. --unread-only
// cannot be pushed into NotificationFilter (it has no readAt comparator), so
// filling a --limit of unread rows may need several pages. The cap keeps a
// fully-read inbox from turning one command into an unbounded crawl.
const notificationListPageCap = 20

func newNotificationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "Read and triage the authenticated user's Linear inbox",
		Long: `Read and triage Linear notifications.

Every subcommand is viewer-scoped: Linear's notification API only ever
addresses the inbox of the token's own user, so there is no way to read or
mutate someone else's notifications and no flag pretends otherwise.

Single-notification verbs (read, unread, snooze, unsnooze, archive,
unarchive) take a notification UUID. The bulk verbs (read-all, unread-all,
snooze-all, unsnooze-all, archive-all) do NOT take a list of notification
ids: Linear addresses them by the entity the notifications are about, so
they take exactly one of --notification, --issue, --initiative,
--project-update, --initiative-update or --oauth-client-approval and act on
every notification grouped with it.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNotificationsListCmd(flags))
	cmd.AddCommand(newNotificationsGetCmd(flags))
	cmd.AddCommand(newNotificationsUnreadCountCmd(flags))
	cmd.AddCommand(newNotificationsReadCmd(flags))
	cmd.AddCommand(newNotificationsUnreadCmd(flags))
	cmd.AddCommand(newNotificationsSnoozeCmd(flags))
	cmd.AddCommand(newNotificationsUnsnoozeCmd(flags))
	cmd.AddCommand(newNotificationsArchiveCmd(flags))
	cmd.AddCommand(newNotificationsUnarchiveCmd(flags))
	cmd.AddCommand(newNotificationsReadAllCmd(flags))
	cmd.AddCommand(newNotificationsUnreadAllCmd(flags))
	cmd.AddCommand(newNotificationsSnoozeAllCmd(flags))
	cmd.AddCommand(newNotificationsUnsnoozeAllCmd(flags))
	cmd.AddCommand(newNotificationsArchiveAllCmd(flags))
	return cmd
}

// ---------------------------------------------------------------------------
// reads
// ---------------------------------------------------------------------------

func newNotificationsListCmd(flags *rootFlags) *cobra.Command {
	var unreadOnly, snoozedOnly, includeArchived bool
	var typeFilter, after, orderBy string
	var limit, maxPages int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the authenticated user's notifications",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List notifications from the authenticated user's inbox.

--type is pushed into NotificationFilter.type, so the API does that
narrowing. --unread-only and --snoozed-only cannot be: NotificationFilter
carries id, createdAt, updatedAt, type, subscriptionType and archivedAt
comparators and has no readAt or snoozedUntilAt field. Those two flags are
applied here, after the fetch, and the walker keeps pulling pages until it
has --limit matches or runs out of pages or hits --max-pages. The pageInfo
block reports how many pages were fetched and how many rows were scanned.

Notification is a GraphQL interface with 13 implementations. Rows carry the
shared interface fields plus, for IssueNotification only, the issue and team
the notification is about.`,
		Example: `  linear-pp-cli notifications list --agent
  linear-pp-cli notifications list --unread-only --limit 20 --agent
  linear-pp-cli notifications list --type issueAssignedToYou --agent
  linear-pp-cli notifications list --include-archived --order-by updatedAt --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit expects a positive count, got %d", limit))
			}
			if maxPages <= 0 {
				maxPages = notificationListPageCap
			}
			switch orderBy {
			case "", "createdAt", "updatedAt":
			default:
				return usageErr(fmt.Errorf("--order-by accepts createdAt or updatedAt (the two PaginationOrderBy values), got %q", orderBy))
			}

			filter := map[string]any{}
			if typeFilter != "" {
				filter["type"] = map[string]any{"eq": typeFilter}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			pageSize := limit
			if unreadOnly || snoozedOnly || pageSize > 100 {
				pageSize = 100
			}
			cursor := after
			rows := make([]map[string]any, 0, limit)
			scanned := 0
			pages := 0
			hasNext := false
			endCursor := ""
			for {
				vars := map[string]any{
					"first":           pageSize,
					"after":           nil,
					"includeArchived": includeArchived,
				}
				if cursor != "" {
					vars["after"] = cursor
				}
				if len(filter) > 0 {
					vars["filter"] = filter
				}
				if orderBy != "" {
					vars["orderBy"] = orderBy
				}
				var resp struct {
					Notifications struct {
						Nodes    []map[string]any `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"notifications"`
				}
				if err := c.QueryInto(client.NotificationsQuery, vars, &resp); err != nil {
					return classifyLiveReadError(err, flags)
				}
				pages++
				scanned += len(resp.Notifications.Nodes)
				hasNext = resp.Notifications.PageInfo.HasNextPage
				endCursor = resp.Notifications.PageInfo.EndCursor
				for _, node := range resp.Notifications.Nodes {
					if unreadOnly && !notificationFieldIsNull(node, "readAt") {
						continue
					}
					if snoozedOnly && notificationFieldIsNull(node, "snoozedUntilAt") {
						continue
					}
					rows = append(rows, node)
					if len(rows) >= limit {
						break
					}
				}
				if len(rows) >= limit || !hasNext || pages >= maxPages {
					break
				}
				cursor = endCursor
			}

			out, err := json.Marshal(map[string]any{
				"notifications": rows,
				"pageInfo": map[string]any{
					"hasNextPage":    hasNext,
					"endCursor":      endCursor,
					"pagesFetched":   pages,
					"nodesScanned":   scanned,
					"clientFiltered": unreadOnly || snoozedOnly,
					"pageCapHit":     pages >= maxPages && hasNext && len(rows) < limit,
				},
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, resourceTypeNotifications, true)
		},
	}
	cmd.Flags().BoolVar(&unreadOnly, "unread-only", false, "Keep only notifications with a null readAt. Applied client-side because NotificationFilter has no readAt comparator")
	cmd.Flags().BoolVar(&snoozedOnly, "snoozed-only", false, "Keep only notifications with a non-null snoozedUntilAt. Applied client-side for the same reason")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include archived notifications")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter on Notification.type exactly (NotificationFilter.type, server-side)")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Pagination order: createdAt (Linear's default) or updatedAt")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from pageInfo.endCursor for the next page")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum notifications to return after filtering")
	cmd.Flags().IntVar(&maxPages, "max-pages", notificationListPageCap, "Page ceiling for the client-side unread and snoozed walk")
	return cmd
}

// notificationFieldIsNull reports whether a decoded node's field is absent or
// JSON null. readAt and snoozedUntilAt are nullable DateTime, and "unread"
// means readAt is null.
func notificationFieldIsNull(node map[string]any, field string) bool {
	value, ok := node[field]
	return !ok || value == nil
}

func newNotificationsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <notification-id>",
		Short:       "Get one notification by id",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Read one notification from the authenticated user's inbox by UUID.

Run 'notifications list' to find ids. A notification belonging to another
user is not readable through this API at all.`,
		Example: `  linear-pp-cli notifications get <notification-uuid> --agent`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<notification-id> is required"))
			}
			id, err := requireNotificationID(args[0])
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var resp struct {
				Notification json.RawMessage `json:"notification"`
			}
			if err := c.QueryInto(client.NotificationQuery, map[string]any{"id": id}, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			if len(resp.Notification) == 0 || string(resp.Notification) == "null" {
				return notFoundErr(fmt.Errorf("notification %s not found in this user's inbox", id))
			}
			return renderLiveObject(cmd, flags, resp.Notification, resourceTypeNotifications)
		},
	}
	return cmd
}

func newNotificationsUnreadCountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "unread-count",
		Short:       "Report the authenticated user's unread notification count",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Read Query.notificationsUnreadCount, the inbox badge number.

Linear marks this query "[Internal]". It is exposed anyway because
--unread-only on 'notifications list' is a client-side filter over a bounded
page walk, so this is the only authoritative answer to "did I see all of
them".`,
		Example: `  linear-pp-cli notifications unread-count --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var resp struct {
				Count int `json:"notificationsUnreadCount"`
			}
			if err := c.QueryInto(client.NotificationsUnreadCountQuery, nil, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			out, err := json.Marshal(map[string]any{"unreadCount": resp.Count})
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, out, resourceTypeNotifications)
		},
	}
	return cmd
}

// ---------------------------------------------------------------------------
// single-notification writes, all through notificationUpdate or the archive pair
// ---------------------------------------------------------------------------

// requireNotificationID rejects anything that is not a UUID. Notification ids
// have no human-readable form the way issues do, so a non-UUID argument is
// always a mistake worth naming.
func requireNotificationID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", usageErr(fmt.Errorf("<notification-id> is required"))
	}
	if !store.IsUUID(value) {
		return "", usageErr(fmt.Errorf("<notification-id> expects a notification UUID, got %q. Run 'linear-pp-cli notifications list' to find it", value))
	}
	return value, nil
}

// runNotificationUpdate is the shared body of read, unread, snooze and
// unsnooze. input is the NotificationUpdateInput to send, already built.
func runNotificationUpdate(cmd *cobra.Command, flags *rootFlags, rawID, event string, input map[string]any) error {
	if flags.dryRun {
		return renderMutationDryRun(cmd, flags, event, "notificationUpdate", map[string]any{
			"id":    rawID,
			"input": input,
		})
	}
	id, err := requireNotificationID(rawID)
	if err != nil {
		return err
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	resp, err := c.Mutate(client.NotificationUpdateMutation, map[string]any{"id": id, "input": input})
	if err != nil {
		return classifyGraphQLMutationError("notificationUpdate", err, flags)
	}
	notification, err := extractMutationObject(resp, "notificationUpdate", "notification")
	if err != nil {
		return err
	}
	return renderLiveObject(cmd, flags, notification, resourceTypeNotifications)
}

func newNotificationsReadCmd(flags *rootFlags) *cobra.Command {
	var atFlag string
	cmd := &cobra.Command{
		Use:   "read <notification-id>",
		Short: "Mark one notification as read",
		Long: `Mark one notification read by writing NotificationUpdateInput.readAt.

Defaults to now. Pass --at with an RFC3339 timestamp to record a different
read time. This touches exactly one notification: use 'notifications
read-all' to clear everything Linear groups with it.`,
		Example: `  linear-pp-cli notifications read <notification-uuid> --agent
  linear-pp-cli notifications read <notification-uuid> --dry-run --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			readAt, err := resolveNotificationTimestamp("--at", atFlag)
			if err != nil {
				return err
			}
			return runNotificationUpdate(cmd, flags, args[0], "would_mark_notification_read", map[string]any{"readAt": readAt})
		},
	}
	cmd.Flags().StringVar(&atFlag, "at", "", "RFC3339 timestamp to record as readAt (default: now)")
	return cmd
}

func newNotificationsUnreadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unread <notification-id>",
		Short: "Mark one notification as unread",
		Long: `Mark one notification unread by writing a null NotificationUpdateInput
.readAt. readAt is a nullable DateTime, so clearing it is how Linear models
unread.

This touches exactly one notification: use 'notifications unread-all' for
the whole group.`,
		Example: `  linear-pp-cli notifications unread <notification-uuid> --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNotificationUpdate(cmd, flags, args[0], "would_mark_notification_unread", map[string]any{"readAt": nil})
		},
	}
	return cmd
}

func newNotificationsSnoozeCmd(flags *rootFlags) *cobra.Command {
	var until string
	cmd := &cobra.Command{
		Use:   "snooze <notification-id> --until <when>",
		Short: "Snooze one notification until a given time",
		Long: `Snooze one notification by writing NotificationUpdateInput.snoozedUntilAt.
Linear returns it to the inbox once that time passes.

--until accepts an RFC3339 timestamp or a relative offset from now: 45m, 6h,
3d, 2w. This touches exactly one notification: use 'notifications
snooze-all' for the whole group.`,
		Example: `  linear-pp-cli notifications snooze <notification-uuid> --until 3d --agent
  linear-pp-cli notifications snooze <notification-uuid> --until 2026-09-01T09:00:00Z --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(until) == "" {
				return usageErr(fmt.Errorf("--until is required: pass an RFC3339 timestamp or a relative offset such as 3d"))
			}
			snoozedUntilAt, err := parseNotificationDeadline("--until", until)
			if err != nil {
				return err
			}
			return runNotificationUpdate(cmd, flags, args[0], "would_snooze_notification", map[string]any{"snoozedUntilAt": snoozedUntilAt})
		},
	}
	cmd.Flags().StringVar(&until, "until", "", "RFC3339 timestamp or relative offset (45m, 6h, 3d, 2w) to snooze until (required)")
	return cmd
}

func newNotificationsUnsnoozeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unsnooze <notification-id>",
		Short: "Wake one snoozed notification",
		Long: `Wake one snoozed notification by writing a null NotificationUpdateInput
.snoozedUntilAt.

This clears the snooze on exactly one notification and does not stamp
unsnoozedAt. Use 'notifications unsnooze-all' for the group form, which goes
through notificationUnsnoozeAll and does record unsnoozedAt.`,
		Example: `  linear-pp-cli notifications unsnooze <notification-uuid> --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNotificationUpdate(cmd, flags, args[0], "would_unsnooze_notification", map[string]any{"snoozedUntilAt": nil})
		},
	}
	return cmd
}

func newNotificationsArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <notification-id>",
		Short: "Archive one notification",
		Long: `Archive one notification via notificationArchive. Reverse it with
'notifications unarchive'. Archived notifications stay readable through
'notifications list --include-archived'.

Pass --ignore-missing to treat an already-gone notification as a
successful no-op.`,
		Example: `  linear-pp-cli notifications archive <notification-uuid> --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNotificationArchive(cmd, flags, args[0], true)
		},
	}
	return cmd
}

func newNotificationsUnarchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unarchive <notification-id>",
		Short:   "Restore one archived notification",
		Long:    `Restore an archived notification via notificationUnarchive.`,
		Example: `  linear-pp-cli notifications unarchive <notification-uuid> --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNotificationArchive(cmd, flags, args[0], false)
		},
	}
	return cmd
}

// runNotificationArchive drives notificationArchive and notificationUnarchive.
// Both return NotificationArchivePayload, whose object field is `entity`.
func runNotificationArchive(cmd *cobra.Command, flags *rootFlags, rawID string, archive bool) error {
	mutationName := "notificationUnarchive"
	document := client.NotificationUnarchiveMutation
	event := "would_unarchive_notification"
	if archive {
		mutationName = "notificationArchive"
		document = client.NotificationArchiveMutation
		event = "would_archive_notification"
	}
	if flags.dryRun {
		return renderMutationDryRun(cmd, flags, event, mutationName, map[string]any{"id": rawID})
	}
	id, err := requireNotificationID(rawID)
	if err != nil {
		return err
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	resp, err := c.Mutate(document, map[string]any{"id": id})
	if err != nil {
		return classifyGraphQLMutationError(mutationName, err, flags)
	}
	entity, err := extractMutationObject(resp, mutationName, "entity")
	if err != nil {
		return err
	}
	return renderLiveObject(cmd, flags, entity, resourceTypeNotifications)
}

// ---------------------------------------------------------------------------
// entity-scoped batch writes
// ---------------------------------------------------------------------------

// notificationEntityFlags collects the NotificationEntityInput selectors. The
// input's own description is "Exactly one entity identifier should be
// provided", so passing none or more than one is a usage error rather than a
// silent pick.
//
// projectId is omitted on purpose: Linear marks it "[DEPRECATED]".
type notificationEntityFlags struct {
	notification        string
	issue               string
	initiative          string
	projectUpdate       string
	initiativeUpdate    string
	oauthClientApproval string
}

func (f *notificationEntityFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.notification, "notification", "", "Target one notification UUID and everything Linear groups with it")
	cmd.Flags().StringVar(&f.issue, "issue", "", "Target every notification about an issue (identifier such as ENG-123, or a UUID)")
	cmd.Flags().StringVar(&f.initiative, "initiative", "", "Target every notification about an initiative UUID")
	cmd.Flags().StringVar(&f.projectUpdate, "project-update", "", "Target every notification about a project update UUID")
	cmd.Flags().StringVar(&f.initiativeUpdate, "initiative-update", "", "Target every notification about an initiative update UUID")
	cmd.Flags().StringVar(&f.oauthClientApproval, "oauth-client-approval", "", "Target every notification about an OAuth client approval UUID")
}

// input builds NotificationEntityInput. c may be nil on the --dry-run path,
// in which case an issue identifier is echoed back unresolved rather than
// triggering a network call.
func (f notificationEntityFlags) input(c graphqlQueryer) (map[string]any, string, error) {
	type selector struct {
		flag  string
		field string
		value string
	}
	candidates := []selector{
		{"--notification", "id", f.notification},
		{"--issue", "issueId", f.issue},
		{"--initiative", "initiativeId", f.initiative},
		{"--project-update", "projectUpdateId", f.projectUpdate},
		{"--initiative-update", "initiativeUpdateId", f.initiativeUpdate},
		{"--oauth-client-approval", "oauthClientApprovalId", f.oauthClientApproval},
	}
	var chosen []selector
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.value) != "" {
			candidate.value = strings.TrimSpace(candidate.value)
			chosen = append(chosen, candidate)
		}
	}
	if len(chosen) == 0 {
		return nil, "", usageErr(fmt.Errorf("pass exactly one target: --notification, --issue, --initiative, --project-update, --initiative-update or --oauth-client-approval. Linear's NotificationEntityInput addresses notifications by the entity they are about, not by a list of notification ids"))
	}
	if len(chosen) > 1 {
		names := make([]string, 0, len(chosen))
		for _, candidate := range chosen {
			names = append(names, candidate.flag)
		}
		return nil, "", usageErr(fmt.Errorf("pass exactly one target, got %s", strings.Join(names, " and ")))
	}
	picked := chosen[0]
	value := picked.value
	if picked.field == "issueId" && !store.IsUUID(value) && c != nil {
		resolved, err := resolveIssueID(c, value)
		if err != nil {
			return nil, "", err
		}
		value = resolved
	} else if picked.field != "issueId" && !store.IsUUID(value) {
		return nil, "", usageErr(fmt.Errorf("%s expects a UUID, got %q", picked.flag, value))
	}
	return map[string]any{picked.field: value}, fmt.Sprintf("%s %s", picked.flag, picked.value), nil
}

// notificationBatchSpec describes one entity-scoped bulk verb. All five share
// the same shape: build NotificationEntityInput, optionally add a timestamp
// argument, confirm, send, render the returned notifications array. Only the
// behavioural parts live here; each command declares its own Use, Short, Long
// and Example literally so the command surface stays greppable from this file.
type notificationBatchSpec struct {
	event        string
	mutationName string
	document     string
	// timestampArg is the name of the required DateTime argument, empty when
	// the mutation takes none.
	timestampArg string
	// timestampFlag is the user-facing flag feeding timestampArg. Empty means
	// the timestamp is always "now" and is not configurable.
	timestampFlag string
	// timestampRequired marks a mutation whose timestamp is a deadline the
	// caller must choose rather than a stamp of the current moment.
	timestampRequired bool
	confirmPrompt     string
}

// bindNotificationBatchFlags registers the entity selectors every bulk verb
// takes, plus the timestamp flag the spec names when it has one.
func bindNotificationBatchFlags(cmd *cobra.Command, entity *notificationEntityFlags, timestampValue *string, spec notificationBatchSpec) {
	entity.register(cmd)
	if spec.timestampFlag != "" {
		usage := "RFC3339 timestamp to record (default: now)"
		if spec.timestampRequired {
			usage = "RFC3339 timestamp or relative offset (45m, 6h, 3d, 2w) (required)"
		}
		cmd.Flags().StringVar(timestampValue, strings.TrimPrefix(spec.timestampFlag, "--"), "", usage)
	}
}

func notificationBatchRunE(flags *rootFlags, entity *notificationEntityFlags, timestampValue *string, spec notificationBatchSpec) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var stamp string
		if spec.timestampArg != "" {
			if spec.timestampRequired {
				if strings.TrimSpace(*timestampValue) == "" {
					return usageErr(fmt.Errorf("%s is required: pass an RFC3339 timestamp or a relative offset such as 3d", spec.timestampFlag))
				}
				resolved, err := parseNotificationDeadline(spec.timestampFlag, *timestampValue)
				if err != nil {
					return err
				}
				stamp = resolved
			} else {
				resolved, err := resolveNotificationTimestamp(spec.timestampFlag, *timestampValue)
				if err != nil {
					return err
				}
				stamp = resolved
			}
		}

		if flags.dryRun {
			input, _, err := entity.input(nil)
			if err != nil {
				return err
			}
			fields := map[string]any{"input": input}
			if spec.timestampArg != "" {
				fields[spec.timestampArg] = stamp
			}
			return renderMutationDryRun(cmd, flags, spec.event, spec.mutationName, fields)
		}

		c, err := flags.newClient()
		if err != nil {
			return err
		}
		input, described, err := entity.input(c)
		if err != nil {
			// A --issue identifier lookup is a live read, so a transport
			// failure gets a typed exit code. usageErr from the selector
			// check already carries its own code and passes through.
			return classifyLiveReadError(err, flags)
		}
		if err := confirmMutation(cmd, flags, fmt.Sprintf(spec.confirmPrompt, described)); err != nil {
			return err
		}
		vars := map[string]any{"input": input}
		if spec.timestampArg != "" {
			vars[spec.timestampArg] = stamp
		}
		resp, err := c.Mutate(spec.document, vars)
		if err != nil {
			return classifyGraphQLMutationError(spec.mutationName, err, flags)
		}
		notifications, err := extractMutationObject(resp, spec.mutationName, "notifications")
		if err != nil {
			return err
		}
		out, err := json.Marshal(map[string]any{"notifications": notifications})
		if err != nil {
			return err
		}
		return renderLivePayload(cmd, flags, out, resourceTypeNotifications, true)
	}
}

func newNotificationsReadAllCmd(flags *rootFlags) *cobra.Command {
	var entity notificationEntityFlags
	var timestampValue string
	spec := notificationBatchSpec{
		event:         "would_mark_notifications_read",
		mutationName:  "notificationMarkReadAll",
		document:      client.NotificationMarkReadAllMutation,
		timestampArg:  "readAt",
		timestampFlag: "--at",
		confirmPrompt: "Mark every notification for %s as read?",
	}
	cmd := &cobra.Command{
		Use:   "read-all",
		Short: "Mark every notification about one entity as read",
		Long: `Mark a notification and everything Linear groups with it as read, via
notificationMarkReadAll.

This is entity-scoped, not inbox-wide. Linear's mutation takes a
NotificationEntityInput, so the target is the issue, initiative, project
update, initiative update or OAuth client approval the notifications are
about, or a single notification id standing in for its own group. There is
no live mutation that clears the entire inbox in one call, so this CLI does
not pretend to offer one.

Confirmation is required unless --yes, because the blast radius is a whole
notification group rather than one row.`,
		Example: `  linear-pp-cli notifications read-all --issue ENG-123 --yes --agent
  linear-pp-cli notifications read-all --notification <notification-uuid> --yes --agent
  linear-pp-cli notifications read-all --issue ENG-123 --dry-run --agent`,
		Args: cobra.NoArgs,
		RunE: notificationBatchRunE(flags, &entity, &timestampValue, spec),
	}
	bindNotificationBatchFlags(cmd, &entity, &timestampValue, spec)
	return cmd
}

func newNotificationsUnreadAllCmd(flags *rootFlags) *cobra.Command {
	var entity notificationEntityFlags
	var timestampValue string
	spec := notificationBatchSpec{
		event:         "would_mark_notifications_unread",
		mutationName:  "notificationMarkUnreadAll",
		document:      client.NotificationMarkUnreadAllMutation,
		confirmPrompt: "Mark every notification for %s as unread?",
	}
	cmd := &cobra.Command{
		Use:   "unread-all",
		Short: "Mark every notification about one entity as unread",
		Long: `Mark a notification and everything Linear groups with it as unread, via
notificationMarkUnreadAll. Entity-scoped, exactly like 'read-all', and takes
no timestamp because the mutation has no DateTime argument.

Confirmation is required unless --yes.`,
		Example: `  linear-pp-cli notifications unread-all --issue ENG-123 --yes --agent`,
		Args:    cobra.NoArgs,
		RunE:    notificationBatchRunE(flags, &entity, &timestampValue, spec),
	}
	bindNotificationBatchFlags(cmd, &entity, &timestampValue, spec)
	return cmd
}

func newNotificationsSnoozeAllCmd(flags *rootFlags) *cobra.Command {
	var entity notificationEntityFlags
	var timestampValue string
	spec := notificationBatchSpec{
		event:             "would_snooze_notifications",
		mutationName:      "notificationSnoozeAll",
		document:          client.NotificationSnoozeAllMutation,
		timestampArg:      "snoozedUntilAt",
		timestampFlag:     "--until",
		timestampRequired: true,
		confirmPrompt:     "Snooze every notification for %s?",
	}
	cmd := &cobra.Command{
		Use:   "snooze-all",
		Short: "Snooze every notification about one entity",
		Long: `Snooze a notification and everything Linear groups with it, via
notificationSnoozeAll. Entity-scoped, exactly like 'read-all'.

--until accepts an RFC3339 timestamp or a relative offset from now: 45m, 6h,
3d, 2w. Confirmation is required unless --yes.`,
		Example: `  linear-pp-cli notifications snooze-all --issue ENG-123 --until 3d --yes --agent`,
		Args:    cobra.NoArgs,
		RunE:    notificationBatchRunE(flags, &entity, &timestampValue, spec),
	}
	bindNotificationBatchFlags(cmd, &entity, &timestampValue, spec)
	return cmd
}

func newNotificationsUnsnoozeAllCmd(flags *rootFlags) *cobra.Command {
	var entity notificationEntityFlags
	var timestampValue string
	spec := notificationBatchSpec{
		event:         "would_unsnooze_notifications",
		mutationName:  "notificationUnsnoozeAll",
		document:      client.NotificationUnsnoozeAllMutation,
		timestampArg:  "unsnoozedAt",
		timestampFlag: "--at",
		confirmPrompt: "Wake every snoozed notification for %s?",
	}
	cmd := &cobra.Command{
		Use:   "unsnooze-all",
		Short: "Wake every snoozed notification about one entity",
		Long: `Wake a snoozed notification and everything Linear groups with it, via
notificationUnsnoozeAll. Entity-scoped, exactly like 'read-all'.

The mutation records when the wake happened in unsnoozedAt, which defaults
to now and can be overridden with --at. Confirmation is required unless
--yes.`,
		Example: `  linear-pp-cli notifications unsnooze-all --issue ENG-123 --yes --agent`,
		Args:    cobra.NoArgs,
		RunE:    notificationBatchRunE(flags, &entity, &timestampValue, spec),
	}
	bindNotificationBatchFlags(cmd, &entity, &timestampValue, spec)
	return cmd
}

func newNotificationsArchiveAllCmd(flags *rootFlags) *cobra.Command {
	var entity notificationEntityFlags
	var timestampValue string
	spec := notificationBatchSpec{
		event:         "would_archive_notifications",
		mutationName:  "notificationArchiveAll",
		document:      client.NotificationArchiveAllMutation,
		confirmPrompt: "Archive every notification for %s?",
	}
	cmd := &cobra.Command{
		Use:   "archive-all",
		Short: "Archive every notification about one entity",
		Long: `Archive a notification and everything Linear groups with it, via
notificationArchiveAll. Entity-scoped, exactly like 'read-all'.

Archived notifications stay readable through 'notifications list
--include-archived', and 'notifications unarchive' restores them one at a
time. Confirmation is required unless --yes.`,
		Example: `  linear-pp-cli notifications archive-all --issue ENG-123 --yes --agent`,
		Args:    cobra.NoArgs,
		RunE:    notificationBatchRunE(flags, &entity, &timestampValue, spec),
	}
	bindNotificationBatchFlags(cmd, &entity, &timestampValue, spec)
	return cmd
}

// ---------------------------------------------------------------------------
// timestamps
// ---------------------------------------------------------------------------

// resolveNotificationTimestamp turns an optional flag value into the RFC3339
// string a DateTime argument wants, defaulting to now. Used for the "when did
// this happen" stamps (readAt, unsnoozedAt), which are almost always now.
func resolveNotificationTimestamp(flagName, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC().Format(time.RFC3339), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", usageErr(fmt.Errorf("%s expects an RFC3339 timestamp such as 2026-09-01T09:00:00Z, got %q", flagName, value))
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

// parseNotificationDeadline turns a snooze deadline into RFC3339. It accepts
// an absolute RFC3339 timestamp or a relative offset from now, because
// "snooze this for three days" is the shape of the request every time and
// making the caller compute a timestamp is a pointless tax.
//
// Offsets: <number><unit> with unit in m, h, d, w. Go's ParseDuration knows
// nothing about days or weeks, so those two are expanded here.
func parseNotificationDeadline(flagName, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", usageErr(fmt.Errorf("%s is required", flagName))
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.RFC3339), nil
	}
	offset, err := parseRelativeOffset(value)
	if err != nil {
		return "", usageErr(fmt.Errorf("%s expects an RFC3339 timestamp such as 2026-09-01T09:00:00Z or a relative offset such as 45m, 6h, 3d, 2w, got %q", flagName, value))
	}
	if offset <= 0 {
		return "", usageErr(fmt.Errorf("%s expects an offset in the future, got %q", flagName, value))
	}
	return time.Now().UTC().Add(offset).Format(time.RFC3339), nil
}

// parseRelativeOffset parses <number><unit> with unit in m, h, d, w.
func parseRelativeOffset(value string) (time.Duration, error) {
	if len(value) < 2 {
		return 0, fmt.Errorf("offset too short")
	}
	unit := value[len(value)-1]
	amount, err := strconv.Atoi(value[:len(value)-1])
	if err != nil {
		return 0, err
	}
	switch unit {
	case 'm':
		return time.Duration(amount) * time.Minute, nil
	case 'h':
		return time.Duration(amount) * time.Hour, nil
	case 'd':
		return time.Duration(amount) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(amount) * 7 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown unit %q", string(unit))
}
