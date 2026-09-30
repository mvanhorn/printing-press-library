// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type suppressionCheckRow struct {
	Stream      string `json:"stream"`
	StreamName  string `json:"stream_name,omitempty"`
	StreamType  string `json:"stream_type,omitempty"`
	Suppressed  bool   `json:"suppressed"`
	Reason      string `json:"reason,omitempty"`
	Origin      string `json:"origin,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	NextCommand string `json:"next_command,omitempty"`
	Note        string `json:"note,omitempty"`
}

type suppressionCheckView struct {
	Email              string                 `json:"email"`
	SuppressedAnywhere bool                   `json:"suppressed_anywhere"`
	Streams            []suppressionCheckRow  `json:"streams"`
	SkippedStreams     []string               `json:"skipped_streams,omitempty"`
	FetchFailures      []postmarkFetchFailure `json:"fetch_failures,omitempty"`
}

// suppressionNextCommand builds the removal command using the generated
// `suppressions delete <streamId> --suppressions <json>` shape. Spam
// complaints cannot be removed, so they get no command.
func suppressionNextCommand(stream, email, reason, server string) (command, note string) {
	if isSpamComplaint(reason) {
		return "", "SpamComplaint suppressions cannot be removed; the recipient marked your mail as spam"
	}
	payload, _ := json.Marshal([]map[string]string{{"EmailAddress": email}})
	command = fmt.Sprintf("postmark-pp-cli suppressions delete %s --suppressions %s", shellQuoteWord(stream), shellQuoteWord(string(payload))) + postmarkServerArg(server)
	if reason == postmarkHardBounce {
		note = "removing a HardBounce suppression also reactivates the bounce"
	}
	return command, note
}

// suppressionRowFor turns one stream's dump into the check row. The dump's
// EmailAddress filter may match partially, so only an exact address counts.
func suppressionRowFor(stream postmarkStream, entries []suppressionEntry, email, server string) suppressionCheckRow {
	row := suppressionCheckRow{Stream: stream.ID, StreamName: stream.Name, StreamType: stream.MessageStreamType}
	for _, e := range entries {
		if !sameAddr(e.EmailAddress, email) {
			continue
		}
		row.Suppressed = true
		row.Reason, row.Origin, row.CreatedAt = e.SuppressionReason, e.Origin, e.CreatedAt
		row.NextCommand, row.Note = suppressionNextCommand(stream.ID, e.EmailAddress, e.SuppressionReason, server)
		break
	}
	return row
}

func newSuppressionsCheckCmd(flags *rootFlags) *cobra.Command {
	var streamFlag string
	cmd := &cobra.Command{
		Use:   "check <email>",
		Short: "Check whether one address is suppressed on any message stream",
		Long: strings.Trim(`
Use this command to ask whether one address is suppressed on any stream. Do NOT use it for a full delivery diagnosis; use 'diagnose' instead.

Lists the server's active sending streams (or just --stream) and looks the
address up in each stream's suppression list. Each row shows suppressed yes or
no, the reason, the origin, when it was created, and the command that removes
the suppression. SpamComplaint suppressions cannot be removed, so those rows
carry a note instead of a command. Inbound streams are skipped because they do
not send.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli suppressions check jane@example.com --json
  postmark-pp-cli suppressions check jane@example.com --server "Main App"
  postmark-pp-cli suppressions check jane@example.com --stream broadcast --server "Main App" --json`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			"mcp:read-only":  "true",
			"pp:happy-args":  "email=jane@example.com",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "suppressions check")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("<email> is required: the address to look up"))
			}
			email := strings.TrimSpace(args[0])
			if !strings.Contains(email, "@") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("%q is not an email address", email))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newUncachedClient(flags)
			if err != nil {
				return err
			}
			server := ""
			if name, explicit := postmarkSelectedServer(); explicit {
				server = name
			}
			view := suppressionCheckView{Email: email, Streams: make([]suppressionCheckRow, 0)}
			var streams []postmarkStream
			if strings.TrimSpace(streamFlag) != "" {
				streams = []postmarkStream{{ID: strings.TrimSpace(streamFlag)}}
			} else {
				all, err := fetchPostmarkStreams(ctx, c, map[string]string{"MessageStreamType": "All", "IncludeArchivedStreams": "false"})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				for _, s := range all {
					if s.inbound() {
						view.SkippedStreams = append(view.SkippedStreams, s.ID)
						continue
					}
					streams = append(streams, s)
				}
			}
			for _, s := range streams {
				entries, err := dumpStreamSuppressions(ctx, c, s.ID, email)
				if err != nil {
					if isRateLimited(err) {
						return rateLimitErr(err)
					}
					if streamFlag != "" {
						return classifyAPIError(cmd.OutOrStdout(), err, flags)
					}
					view.FetchFailures = append(view.FetchFailures, itemFailure(s.ID, err))
					continue
				}
				row := suppressionRowFor(s, entries, email, server)
				view.SuppressedAnywhere = view.SuppressedAnywhere || row.Suppressed
				view.Streams = append(view.Streams, row)
			}
			warnPartialFailures(cmd.ErrOrStderr(), len(view.FetchFailures), len(streams), "streams could not be checked", remainingNote(len(view.Streams)))
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "suppressed", "reason", "origin", "created_at", "next_command", "note"); err != nil {
					return err
				}
			} else {
				printSuppressionCheckHuman(cmd.OutOrStdout(), view)
			}
			if len(view.FetchFailures) > 0 {
				return apiErr(fmt.Errorf("%d stream(s) could not be checked", len(view.FetchFailures)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&streamFlag, "stream", "", "Check only this message stream ID (default: every sending stream)")
	return cmd
}

func printSuppressionCheckHuman(w io.Writer, v suppressionCheckView) {
	verdict := "not suppressed on any checked stream"
	if v.SuppressedAnywhere {
		verdict = "SUPPRESSED"
	}
	fmt.Fprintf(w, "%s: %s\n", v.Email, verdict)
	if len(v.Streams) > 0 {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "STREAM\tSUPPRESSED\tREASON\tORIGIN\tCREATED")
		for _, r := range v.Streams {
			yes := "no"
			if r.Suppressed {
				yes = "yes"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Stream, yes, r.Reason, r.Origin, r.CreatedAt)
		}
		_ = tw.Flush()
	}
	for _, r := range v.Streams {
		if r.NextCommand != "" {
			fmt.Fprintf(w, "remove from %s: %s\n", r.Stream, r.NextCommand)
		}
		if r.Note != "" {
			fmt.Fprintf(w, "note (%s): %s\n", r.Stream, r.Note)
		}
	}
	if len(v.SkippedStreams) > 0 {
		fmt.Fprintf(w, "skipped inbound stream(s): %s\n", strings.Join(v.SkippedStreams, ", "))
	}
	for _, f := range v.FetchFailures {
		fmt.Fprintf(w, "could not check %s: %s\n", f.ID, f.Error)
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, err := root.Find([]string{"suppressions"})
		if err == nil && parent != root {
			addNovelCommandIfAbsent(parent, newSuppressionsCheckCmd(flags))
		}
	})
}
