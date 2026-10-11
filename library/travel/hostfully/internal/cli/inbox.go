// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
	"github.com/spf13/cobra"
)

const hostfullyInboxAPIVersion = "/api/v3.3"

type inboxParticipant struct {
	UID             string `json:"uid,omitempty"`
	ParticipantUID  string `json:"participantUid,omitempty"`
	ParticipantType string `json:"participantType,omitempty"`
}

type inboxReadStatus struct {
	ParticipantUID string `json:"participantUid,omitempty"`
	LastRead       string `json:"lastRead,omitempty"`
	ReadStatus     string `json:"readStatus,omitempty"`
}

type inboxThread struct {
	UID                      string             `json:"uid"`
	CreationDate             string             `json:"creationDate,omitempty"`
	LastUpdateDate           string             `json:"lastUpdateDate,omitempty"`
	Participants             []inboxParticipant `json:"participants,omitempty"`
	ParticipantsReadStatuses []inboxReadStatus  `json:"participantsReadStatuses,omitempty"`
}

type inboxThreadList struct {
	Threads  []inboxThread  `json:"threads"`
	Metadata map[string]any `json:"_metadata,omitempty"`
	Paging   map[string]any `json:"_paging,omitempty"`
}

type inboxContent struct {
	Subject string `json:"subject,omitempty"`
	Text    string `json:"text"`
}

type inboxMessage struct {
	UID                string           `json:"uid"`
	CreatedUTCDateTime string           `json:"createdUtcDateTime,omitempty"`
	Status             string           `json:"status,omitempty"`
	Type               string           `json:"type,omitempty"`
	SenderType         string           `json:"senderType,omitempty"`
	Content            inboxContent     `json:"content"`
	ThreadUID          string           `json:"threadUid,omitempty"`
	Attachments        []map[string]any `json:"attachments,omitempty"`
}

type inboxMessageList struct {
	Messages []inboxMessage `json:"messages"`
	Metadata map[string]any `json:"_metadata,omitempty"`
	Paging   map[string]any `json:"_paging,omitempty"`
}

func newNovelInboxCmd(flags *rootFlags) *cobra.Command {
	var agencyUID string
	var leadUID string
	var since string
	var limit int
	var cursor string

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Read and reply to Hostfully guest messages",
		Long: `Reads Hostfully's v3.3 reservation-scoped messaging inbox. List threads,
inspect a conversation, filter messages, or send a reply through the booking
channel. Replies are remote writes; preview with --dry-run first.`,
		Example: `  hostfully-pp-cli inbox --agent
  hostfully-pp-cli inbox show <thread-uid> --agent
  hostfully-pp-cli inbox reply <thread-uid> "Your room is ready." --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if since != "" {
				return runInboxMessages(cmd, flags, agencyUID, "", leadUID, "", since, limit, cursor, true)
			}
			return runInboxThreads(cmd, flags, agencyUID, leadUID, limit, cursor)
		},
	}
	cmd.Flags().StringVar(&agencyUID, "agency-uid", "", "Agency UID (default: HOSTFULLY_AGENCY_ID)")
	cmd.Flags().StringVar(&leadUID, "lead-uid", "", "Filter threads to a reservation/lead UID")
	cmd.Flags().StringVar(&since, "since", "", "Show messages created within this duration (for example 10m, 2h, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Results per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")

	cmd.AddCommand(newInboxThreadsCmd(flags))
	cmd.AddCommand(newInboxShowCmd(flags))
	cmd.AddCommand(newInboxMessagesCmd(flags))
	cmd.AddCommand(newInboxMessageCmd(flags))
	cmd.AddCommand(newInboxReplyCmd(flags))
	return cmd
}

func newInboxThreadsCmd(flags *rootFlags) *cobra.Command {
	var agencyUID, leadUID, cursor string
	var limit int
	cmd := &cobra.Command{
		Use:         "threads",
		Example:     "  hostfully-pp-cli inbox threads --limit 20 --agent",
		Aliases:     []string{"list"},
		Short:       "List recent guest message threads, newest first, optionally filtered to one reservation",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInboxThreads(cmd, flags, agencyUID, leadUID, limit, cursor)
		},
	}
	cmd.Flags().StringVar(&agencyUID, "agency-uid", "", "Agency UID (default: HOSTFULLY_AGENCY_ID)")
	cmd.Flags().StringVar(&leadUID, "lead-uid", "", "Filter to a reservation/lead UID")
	cmd.Flags().IntVar(&limit, "limit", 25, "Results per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	return cmd
}

func runInboxThreads(cmd *cobra.Command, flags *rootFlags, agencyUID, leadUID string, limit int, cursor string) error {
	c, err := newInboxClient(flags)
	if err != nil {
		return err
	}
	params, err := inboxScopeParams(cmd.Context(), c, agencyUID, leadUID, "")
	if err != nil {
		return usageErr(err)
	}
	addInboxPaging(params, limit, cursor)
	data, err := c.Get(cmd.Context(), "/threads", params)
	if err != nil {
		return classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	var result inboxThreadList
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parsing inbox threads: %w", err)
	}
	if len(result.Threads) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No Hostfully inbox threads found.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "THREAD UID\tUPDATED\tLEAD UID\tINBOX")
	for _, thread := range result.Threads {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", thread.UID, thread.LastUpdateDate, threadLeadUID(thread), threadReadState(thread))
	}
	return w.Flush()
}

func newInboxShowCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:         "show <thread-uid>",
		Short:       "Show a conversation and its messages",
		Example:     "  hostfully-pp-cli inbox show 00000000-0000-4000-8000-0000000000aa --agent",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInboxClient(flags)
			if err != nil {
				return err
			}
			threadData, err := c.Get(cmd.Context(), "/threads/"+args[0], nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			params := map[string]string{"threadUid": args[0]}
			addInboxPaging(params, limit, cursor)
			messages, err := fetchInboxMessages(cmd.Context(), c, params, true)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var threadEnvelope struct {
				Thread inboxThread `json:"thread"`
			}
			if err := json.Unmarshal(threadData, &threadEnvelope); err != nil {
				return fmt.Errorf("parsing inbox thread: %w", err)
			}
			result := struct {
				Thread   inboxThread    `json:"thread"`
				Messages []inboxMessage `json:"messages"`
				Paging   map[string]any `json:"_paging,omitempty"`
			}{threadEnvelope.Thread, messages.Messages, messages.Paging}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Thread: %s\nLead: %s\nUpdated: %s\n\n", result.Thread.UID, threadLeadUID(result.Thread), result.Thread.LastUpdateDate)
			return printInboxMessagesTable(cmd, result.Messages)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Messages per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	return cmd
}

func newInboxMessagesCmd(flags *rootFlags) *cobra.Command {
	var agencyUID, threadUID, leadUID, createdAt, since, cursor string
	var limit int
	var all bool
	cmd := &cobra.Command{
		Use:         "messages",
		Example:     "  hostfully-pp-cli inbox messages --since 24h --agent",
		Short:       "List or filter inbox messages",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInboxMessages(cmd, flags, agencyUID, threadUID, leadUID, createdAt, since, limit, cursor, all)
		},
	}
	cmd.Flags().StringVar(&agencyUID, "agency-uid", "", "Agency UID (default: HOSTFULLY_AGENCY_ID)")
	cmd.Flags().StringVar(&threadUID, "thread-uid", "", "Filter to a thread UID")
	cmd.Flags().StringVar(&leadUID, "lead-uid", "", "Filter to a reservation/lead UID")
	cmd.Flags().StringVar(&createdAt, "created-at", "", "Only messages at or after this RFC3339 date-time")
	cmd.Flags().StringVar(&since, "since", "", "Only messages created within this duration (for example 10m, 2h, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Results per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	cmd.Flags().BoolVar(&all, "all", false, "Follow pagination until all matching messages are returned")
	return cmd
}

func runInboxMessages(cmd *cobra.Command, flags *rootFlags, agencyUID, threadUID, leadUID, createdAt, since string, limit int, cursor string, all bool) error {
	if createdAt != "" && since != "" {
		return usageErr(fmt.Errorf("--created-at and --since are mutually exclusive"))
	}
	if since != "" {
		var err error
		createdAt, err = inboxCreatedAtSince(since, time.Now())
		if err != nil {
			return usageErr(err)
		}
	}
	c, err := newInboxClient(flags)
	if err != nil {
		return err
	}
	params, err := inboxScopeParams(cmd.Context(), c, agencyUID, leadUID, threadUID)
	if err != nil {
		return usageErr(err)
	}
	if createdAt != "" {
		params["createdAt"] = createdAt
	}
	addInboxPaging(params, limit, cursor)
	result, err := fetchInboxMessages(cmd.Context(), c, params, all || since != "")
	if err != nil {
		return classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encoding inbox messages: %w", err)
		}
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	return printInboxMessagesTable(cmd, result.Messages)
}

func fetchInboxMessages(ctx context.Context, c *client.Client, params map[string]string, all bool) (inboxMessageList, error) {
	query := make(map[string]string, len(params))
	for key, value := range params {
		query[key] = value
	}
	var combined inboxMessageList
	for page := 1; ; page++ {
		data, err := c.Get(ctx, "/messages", query)
		if err != nil {
			return inboxMessageList{}, err
		}
		var current inboxMessageList
		if err := json.Unmarshal(data, &current); err != nil {
			return inboxMessageList{}, fmt.Errorf("parsing inbox messages page %d: %w", page, err)
		}
		combined.Messages = append(combined.Messages, current.Messages...)
		combined.Metadata = current.Metadata
		combined.Paging = current.Paging
		nextCursor, _ := current.Paging["_nextCursor"].(string)
		if !all || nextCursor == "" {
			if combined.Metadata == nil {
				combined.Metadata = map[string]any{}
			}
			combined.Metadata["count"] = len(combined.Messages)
			return combined, nil
		}
		if page >= 100 {
			return inboxMessageList{}, fmt.Errorf("inbox pagination exceeded 100 pages; resume with --cursor %s", nextCursor)
		}
		query["_cursor"] = nextCursor
	}
}

func inboxCreatedAtSince(value string, now time.Time) (string, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return "", fmt.Errorf("invalid --since duration %q: use a positive duration such as 10m, 2h, or 24h", value)
	}
	return now.UTC().Add(-duration).Format(time.RFC3339), nil
}

func newInboxMessageCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "message <message-uid>",
		Example:     "  hostfully-pp-cli inbox message 00000000-0000-4000-8000-0000000000bb --agent",
		Short:       "Retrieve one inbox message",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInboxClient(flags)
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/messages/"+args[0], nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func newInboxReplyCmd(flags *rootFlags) *cobra.Command {
	var leadUID, messageType, subject string
	cmd := &cobra.Command{
		Use:   "reply <thread-uid> <text>",
		Short: "Reply to a guest through the reservation's messaging channel",
		Long: `Sends a Hostfully message against a reservation-scoped thread. The command
resolves the lead UID from the thread when --lead-uid is omitted. Use --dry-run
to inspect the payload without making a request.`,
		Example: `  hostfully-pp-cli inbox reply <thread-uid> "Your door code is 1234." --dry-run
  hostfully-pp-cli inbox reply <thread-uid> "Your room is ready." --yes
  hostfully-pp-cli inbox reply <thread-uid> "Receipt attached separately." --type email --subject "Receipt" --yes`,
		Args:        cobra.MinimumNArgs(2),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			threadUID := strings.TrimSpace(args[0])
			text := strings.TrimSpace(strings.Join(args[1:], " "))
			if text == "" {
				return usageErr(fmt.Errorf("reply text cannot be empty"))
			}
			typeValue, err := normalizeInboxMessageType(messageType)
			if err != nil {
				return usageErr(err)
			}
			body := map[string]any{
				"type":      typeValue,
				"threadUid": threadUID,
				"content": map[string]string{
					"text":    text,
					"subject": subject,
				},
			}
			if leadUID != "" {
				body["leadUid"] = leadUID
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"method":  "POST",
					"path":    hostfullyInboxAPIVersion + "/messages",
					"body":    body,
				}, flags)
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("sending a guest message requires confirmation: pass --yes (or preview with --dry-run)"))
			}
			c, err := newInboxClient(flags)
			if err != nil {
				return err
			}
			if leadUID == "" {
				resolved, err := fetchThreadLeadUID(cmd.Context(), c, threadUID)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				body["leadUid"] = resolved
			}
			data, _, err := c.Post(cmd.Context(), "/messages", body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&leadUID, "lead-uid", "", "Reservation/lead UID (auto-resolved from the thread when omitted)")
	cmd.Flags().StringVar(&messageType, "type", "direct-message", "Delivery type: direct-message or email")
	cmd.Flags().StringVar(&subject, "subject", "", "Email subject (optional)")
	return cmd
}

func newInboxClient(flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if strings.HasSuffix(base, "/api/v3") {
		base = strings.TrimSuffix(base, "/api/v3") + hostfullyInboxAPIVersion
	}
	c.BaseURL = base
	return c, nil
}

func inboxScopeParams(ctx context.Context, c *client.Client, agencyUID, leadUID, threadUID string) (map[string]string, error) {
	params := map[string]string{}
	if threadUID != "" {
		params["threadUid"] = threadUID
	}
	if leadUID != "" {
		params["leadUid"] = leadUID
	}
	if agencyUID == "" && leadUID == "" && threadUID == "" && c.Config != nil {
		agencyUID = c.Config.AgencyUID()
	}
	if agencyUID != "" {
		params["agencyUid"] = agencyUID
	}
	if len(params) == 0 {
		return nil, fmt.Errorf("an inbox scope is required: set HOSTFULLY_AGENCY_ID or pass --agency-uid, --lead-uid, or --thread-uid")
	}
	return params, nil
}

func addInboxPaging(params map[string]string, limit int, cursor string) {
	if limit > 0 {
		params["_limit"] = fmt.Sprintf("%d", limit)
	}
	if cursor != "" {
		params["_cursor"] = cursor
	}
}

func fetchThreadLeadUID(ctx context.Context, c *client.Client, threadUID string) (string, error) {
	data, err := c.Get(ctx, "/threads/"+threadUID, nil)
	if err != nil {
		return "", err
	}
	var envelope struct {
		Thread inboxThread `json:"thread"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("parsing inbox thread: %w", err)
	}
	leadUID := threadLeadUID(envelope.Thread)
	if leadUID == "" {
		return "", fmt.Errorf("thread %s has no LEAD participant; pass --lead-uid explicitly", threadUID)
	}
	return leadUID, nil
}

func threadLeadUID(thread inboxThread) string {
	for _, participant := range thread.Participants {
		if strings.EqualFold(participant.ParticipantType, "LEAD") {
			return participant.ParticipantUID
		}
	}
	return ""
}

func threadReadState(thread inboxThread) string {
	internalParticipantUIDs := map[string]bool{}
	for _, participant := range thread.Participants {
		switch strings.ToUpper(participant.ParticipantType) {
		case "PMP", "AGENCY", "PM_EMPLOYEE", "AGENT":
			internalParticipantUIDs[participant.ParticipantUID] = true
			internalParticipantUIDs[participant.UID] = true
		}
	}
	for _, status := range thread.ParticipantsReadStatuses {
		if internalParticipantUIDs[status.ParticipantUID] {
			if strings.EqualFold(status.ReadStatus, "UNREAD") {
				return "unread"
			}
			return "read"
		}
	}
	return "unknown"
}

func normalizeInboxMessageType(value string) (string, error) {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-")) {
	case "direct", "direct-message", "dm":
		return "DIRECT_MESSAGE", nil
	case "email":
		return "EMAIL", nil
	default:
		return "", fmt.Errorf("invalid message type %q: must be direct-message or email", value)
	}
}

func printInboxMessagesTable(cmd *cobra.Command, messages []inboxMessage) error {
	if len(messages) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No Hostfully inbox messages found.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tFROM\tTYPE\tSTATUS\tMESSAGE")
	for _, message := range messages {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", message.CreatedUTCDateTime, message.SenderType, message.Type, message.Status, inboxExcerpt(message.Content.Text, 80))
	}
	return w.Flush()
}

func inboxExcerpt(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit > 0 && len(value) > limit {
		if limit <= 3 {
			return value[:limit]
		}
		return value[:limit-3] + "..."
	}
	return value
}
