// Copyright 2026 beetz12. Licensed under Apache-2.0.
// `messages` - read your care.com message inbox. care.com's messaging runs on
// Stream Chat (getstream.io): the /app/messages page embeds a per-user
// streamToken + streamApiKey, which we use to read conversations + message text
// directly from Stream's REST API. Hand-authored; safe across regen.

package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/care/internal/client"

	"github.com/spf13/cobra"
)

const careUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

var reStreamToken = regexp.MustCompile(`"streamToken"\s*:\s*"([^"]+)"`)
var reStreamKey = regexp.MustCompile(`"streamApiKey"\s*:\s*"([^"]+)"`)

type careStreamCreds struct{ APIKey, Token, UserID string }

func careHTTP() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// careFetchStreamCreds pulls the Stream token + app key from the members page.
func careFetchStreamCreds(ctx context.Context, hc *http.Client) (*careStreamCreds, error) {
	ck := client.SessionCookie()
	if ck == "" {
		return nil, fmt.Errorf("no care.com session; run: care-pp-cli auth refresh")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.care.com/app/messages", nil)
	req.Header.Set("Cookie", ck)
	req.Header.Set("User-Agent", careUA)
	req.Header.Set("Accept", "text/html")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	html := string(b)
	tk := reStreamToken.FindStringSubmatch(html)
	ky := reStreamKey.FindStringSubmatch(html)
	if tk == nil || ky == nil {
		if !strings.Contains(html, `"isLoggedIn":true`) {
			return nil, fmt.Errorf("not authenticated; run: care-pp-cli auth refresh")
		}
		return nil, fmt.Errorf("could not find Stream credentials on the messages page (care.com may have changed its layout)")
	}
	creds := &careStreamCreds{APIKey: ky[1], Token: tk[1]}
	if parts := strings.Split(tk[1], "."); len(parts) == 3 {
		if pb, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			var c struct {
				UserID string `json:"user_id"`
			}
			_ = json.Unmarshal(pb, &c)
			creds.UserID = c.UserID
		}
	}
	if creds.UserID == "" {
		return nil, fmt.Errorf("could not read user id from Stream token")
	}
	return creds, nil
}

// --- Stream Chat response shapes (subset) ---

type streamUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type streamMessage struct {
	Text      string     `json:"text"`
	User      streamUser `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	Type      string     `json:"type"`
}
type streamChannelState struct {
	Channel struct {
		CID string `json:"cid"`
	} `json:"channel"`
	Messages []streamMessage `json:"messages"`
	Members  []struct {
		User streamUser `json:"user"`
	} `json:"members"`
}

func careStreamQueryChannels(ctx context.Context, hc *http.Client, creds *careStreamCreds, cidFilter string, msgLimit, limit int) ([]streamChannelState, error) {
	var filter map[string]any
	if cidFilter != "" {
		filter = map[string]any{"cid": cidFilter}
	} else {
		filter = map[string]any{"members": map[string]any{"$in": []string{creds.UserID}}}
	}
	payload := map[string]any{
		"filter_conditions": filter,
		"sort":              []map[string]any{{"field": "last_message_at", "direction": -1}},
		"state":             true,
		"watch":             false,
		"presence":          false,
		"limit":             limit,
		"message_limit":     msgLimit,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://chat.stream-io-api.com/channels?api_key="+creds.APIKey, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", creds.Token)
	req.Header.Set("stream-auth-type", "jwt")
	req.Header.Set("X-Stream-Client", "stream-chat-javascript-client")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Stream Chat error (HTTP %d): %s", resp.StatusCode, truncate(string(rb), 160))
	}
	var out struct {
		Channels []streamChannelState `json:"channels"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return nil, err
	}
	return out.Channels, nil
}

// otherName returns the display name of the participant who is not me.
func (s streamChannelState) otherName(myUID string) string {
	for _, m := range s.Members {
		if m.User.ID != myUID && m.User.Name != "" {
			return m.User.Name
		}
	}
	for _, m := range s.Members {
		if m.User.ID != myUID {
			return m.User.ID
		}
	}
	return "(unknown)"
}

func newCareMessagesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Read your care.com message inbox (conversations with caregivers)",
		Long:  "Lists your message conversations (all caregivers you're talking to, across every job) and reads individual threads. Backed by care.com's Stream Chat messaging.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCareMessagesList(cmd, flags)
		},
	}
	cmd.Flags().Int("limit", 20, "max conversations to list")
	cmd.AddCommand(newCareMessagesReadCmd(flags))
	cmd.AddCommand(newCareMessagesReplyCmd(flags))
	return cmd
}

// otherUID returns the care.com member UUID of the participant who is not me
// (Stream user ids are care.com member UUIDs).
func (s streamChannelState) otherUID(myUID string) string {
	for _, m := range s.Members {
		if m.User.ID != myUID {
			return m.User.ID
		}
	}
	return ""
}

// careFindConversation locates a conversation by cid or by caregiver-name substring.
func careFindConversation(ctx context.Context, hc *http.Client, creds *careStreamCreds, query string, msgLimit int) (*streamChannelState, error) {
	if strings.HasPrefix(query, "messaging:") {
		chans, err := careStreamQueryChannels(ctx, hc, creds, query, msgLimit, 1)
		if err != nil {
			return nil, err
		}
		if len(chans) == 0 {
			return nil, fmt.Errorf("no conversation with cid %s", query)
		}
		return &chans[0], nil
	}
	chans, err := careStreamQueryChannels(ctx, hc, creds, "", msgLimit, 30)
	if err != nil {
		return nil, err
	}
	for i := range chans {
		if strings.Contains(strings.ToLower(chans[i].otherName(creds.UserID)), strings.ToLower(query)) {
			return &chans[i], nil
		}
	}
	return nil, fmt.Errorf("no conversation matching %q (list them: care-pp-cli messages)", query)
}

// careResolveJobForConversation returns the care.com job id most relevant to a
// conversation (needed as the careNeedIdentifier when replying).
func careResolveJobForConversation(ctx context.Context, flags *rootFlags, participantUID, cid string) (string, error) {
	vars := map[string]any{
		"loggedInUserRole":     "SEEKER",
		"participantsMetadata": []map[string]any{{"participantId": participantUID, "conversationId": cid}},
	}
	data, err := careGraphQL(ctx, flags, careQConversationsOp, careQConversations, vars)
	if err != nil {
		return "", err
	}
	var wrap struct {
		Conv []struct {
			MostRelevantJob struct {
				Job struct {
					ID string `json:"id"`
				} `json:"job"`
			} `json:"mostRelevantJob"`
		} `json:"conversationMostRelevantCareNeeds"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return "", err
	}
	if len(wrap.Conv) == 0 || wrap.Conv[0].MostRelevantJob.Job.ID == "" {
		return "", fmt.Errorf("could not resolve a job for this conversation")
	}
	return wrap.Conv[0].MostRelevantJob.Job.ID, nil
}

func newCareMessagesReplyCmd(flags *rootFlags) *cobra.Command {
	var message, messageFile string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "reply <caregiver name or cid>",
		Short: "Reply to a caregiver in your inbox (dry-run unless --confirm)",
		Long: "Sends a reply in an existing conversation via care.com's official, MODERATED send path\n" +
			"(sendCareNeedMessage) - not directly to Stream Chat. Resolves the job + recipient from the\n" +
			"conversation automatically. Dry-run by default; pass --confirm to actually send.",
		// NOTE: examples never include --confirm. This command is dry-run by
		// default (see Long); an example carrying --confirm would send for real
		// when copy-pasted or exercised by a live dogfood matrix.
		Example: "  care-pp-cli messages reply \"Jane\" --message \"Are you free to chat this week?\"",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			if message == "" && messageFile != "" {
				b, err := os.ReadFile(messageFile)
				if err != nil {
					return fmt.Errorf("reading --message-file: %w", err)
				}
				message = strings.TrimSpace(string(b))
			}
			if strings.TrimSpace(message) == "" {
				return usageErr(fmt.Errorf("--message or --message-file is required"))
			}
			to := flags.timeout
			if to <= 0 {
				to = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), to)
			defer cancel()
			hc := careHTTP()
			creds, err := careFetchStreamCreds(ctx, hc)
			if err != nil {
				return err
			}
			ch, err := careFindConversation(ctx, hc, creds, query, 1)
			if err != nil {
				return err
			}
			nannyUID := ch.otherUID(creds.UserID)
			nannyName := ch.otherName(creds.UserID)
			if nannyUID == "" {
				return fmt.Errorf("could not identify the caregiver in this conversation")
			}
			jobID, err := careResolveJobForConversation(ctx, flags, nannyUID, ch.Channel.CID)
			if err != nil {
				return err
			}
			name, legacyID, err := careResolveCaregiver(ctx, flags, nannyUID)
			if err != nil {
				return err
			}
			if name == "" {
				name = nannyName
			}
			if legacyID == "" {
				return fmt.Errorf("could not resolve %s's member id", nannyName)
			}
			w := cmd.OutOrStdout()
			markContacted := func() {
				// Surface (don't swallow) a failed local write so the inbox/dedupe
				// state is known to be stale after a successful moderated reply.
				if merr := careMarkContacted(nannyUID); merr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: reply sent, but recording the contact locally failed: %v\n", merr)
				}
			}
			if flags.asJSON {
				out := map[string]any{
					"to": name, "member": legacyID, "job": jobID,
					"message": message, "dry_run": !confirm, "sent": false,
				}
				if confirm {
					if err := careSendMessage(ctx, flags, jobID, legacyID, message); err != nil {
						return err
					}
					out["sent"] = true
					markContacted()
				}
				b, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(w, string(b))
				return nil
			}
			fmt.Fprintf(w, "To:      %s (member %s)\n", name, legacyID)
			fmt.Fprintf(w, "Job:     %s\n", jobID)
			fmt.Fprintf(w, "Message:\n%s\n\n", message)
			if !confirm {
				fmt.Fprintln(w, "DRY RUN - nothing sent. Re-run with --confirm to send (via care.com's moderated path).")
				return nil
			}
			if err := careSendMessage(ctx, flags, jobID, legacyID, message); err != nil {
				return err
			}
			fmt.Fprintf(w, "Reply sent to %s.\n", name)
			markContacted()
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "reply text")
	cmd.Flags().StringVar(&messageFile, "message-file", "", "read reply text from a file")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "actually send (default is dry-run)")
	return cmd
}

func runCareMessagesList(cmd *cobra.Command, flags *rootFlags) error {
	if dryRunOK(flags) {
		fmt.Fprintln(cmd.OutOrStdout(), "would list message conversations")
		return nil
	}
	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 {
		limit = 20
	}
	to := flags.timeout
	if to <= 0 {
		to = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), to)
	defer cancel()
	hc := careHTTP()
	creds, err := careFetchStreamCreds(ctx, hc)
	if err != nil {
		return err
	}
	chans, err := careStreamQueryChannels(ctx, hc, creds, "", 1, limit)
	if err != nil {
		return err
	}
	type conv struct {
		Nanny    string    `json:"nanny"`
		LastFrom string    `json:"last_from"`
		LastText string    `json:"last_message"`
		LastAt   time.Time `json:"last_at"`
		CID      string    `json:"cid"`
	}
	out := []conv{}
	for _, ch := range chans {
		c := conv{Nanny: ch.otherName(creds.UserID), CID: ch.Channel.CID}
		if n := len(ch.Messages); n > 0 {
			m := ch.Messages[n-1]
			c.LastText = m.Text
			c.LastAt = m.CreatedAt
			if m.User.ID == creds.UserID {
				c.LastFrom = "you"
			} else if m.User.ID == "care" {
				c.LastFrom = "care.com"
			} else {
				c.LastFrom = c.Nanny
			}
		}
		out = append(out, c)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}
	w := cmd.OutOrStdout()
	if len(out) == 0 {
		fmt.Fprintln(w, "No conversations.")
		return nil
	}
	fmt.Fprintf(w, "%-20s %-8s %-12s  %s\n", "CAREGIVER", "LAST", "WHEN", "MESSAGE")
	for _, c := range out {
		when := ""
		if !c.LastAt.IsZero() {
			when = c.LastAt.Local().Format("Jan 2 15:04")
		}
		fmt.Fprintf(w, "%-20s %-8s %-12s  %s\n",
			truncate(c.Nanny, 20), truncate(c.LastFrom, 8), when, truncate(strings.ReplaceAll(c.LastText, "\n", " "), 50))
	}
	fmt.Fprintf(w, "\n%d conversations. Read one: care-pp-cli messages read \"<caregiver name>\"\n", len(out))
	return nil
}

func newCareMessagesReadCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "read <caregiver name or cid>",
		Short:       "Read the full message thread with one caregiver",
		Example:     "  care-pp-cli messages read \"Jane\"",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			to := flags.timeout
			if to <= 0 {
				to = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), to)
			defer cancel()
			hc := careHTTP()
			creds, err := careFetchStreamCreds(ctx, hc)
			if err != nil {
				return err
			}
			var ch *streamChannelState
			if strings.HasPrefix(query, "messaging:") {
				chans, err := careStreamQueryChannels(ctx, hc, creds, query, 100, 1)
				if err != nil {
					return err
				}
				if len(chans) > 0 {
					ch = &chans[0]
				}
			} else {
				chans, err := careStreamQueryChannels(ctx, hc, creds, "", 100, 30)
				if err != nil {
					return err
				}
				for i := range chans {
					if strings.Contains(strings.ToLower(chans[i].otherName(creds.UserID)), strings.ToLower(query)) {
						ch = &chans[i]
						break
					}
				}
			}
			if ch == nil {
				return fmt.Errorf("no conversation matching %q (list them: care-pp-cli messages)", query)
			}
			nanny := ch.otherName(creds.UserID)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				type msg struct {
					From string    `json:"from"`
					At   time.Time `json:"at"`
					Text string    `json:"text"`
				}
				msgs := []msg{}
				for _, m := range ch.Messages {
					from := nanny
					if m.User.ID == creds.UserID {
						from = "you"
					} else if m.User.ID == "care" {
						from = "care.com"
					}
					msgs = append(msgs, msg{From: from, At: m.CreatedAt, Text: m.Text})
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"caregiver": nanny, "cid": ch.Channel.CID, "messages": msgs}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Conversation with %s\n\n", nanny)
			for _, m := range ch.Messages {
				from := nanny
				if m.User.ID == creds.UserID {
					from = "You"
				} else if m.User.ID == "care" {
					from = "care.com"
				}
				when := m.CreatedAt.Local().Format("Jan 2 15:04")
				fmt.Fprintf(w, "[%s] %s:\n%s\n\n", when, from, strings.TrimSpace(m.Text))
			}
			return nil
		},
	}
}
