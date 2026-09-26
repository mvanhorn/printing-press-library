// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

type tbMessageDetail struct {
	tbMessageRow
	To              []string               `json:"to"`
	Cc              []string               `json:"cc"`
	InReplyTo       string                 `json:"in_reply_to"`
	References      []string               `json:"references"`
	Forwarded       bool                   `json:"forwarded"`
	ListID          string                 `json:"list_id"`
	ListUnsubscribe string                 `json:"list_unsubscribe"`
	BodyText        string                 `json:"body_text"`
	Attachments     []tbprofile.Attachment `json:"attachments"`
	Headers         map[string][]string    `json:"headers,omitempty"`
	Source          string                 `json:"source"`
}

// tbBuildDetail prefers the original mbox bytes; raw is nil when only the stored document is available.
func tbBuildDetail(d *tbMessageDoc, raw []byte, withHeaders bool, atts []tbprofile.Attachment) tbMessageDetail {
	det := tbMessageDetail{
		tbMessageRow: tbRowFromDoc(*d), To: d.To, Cc: d.Cc, InReplyTo: d.InReplyTo, References: d.References,
		Forwarded: d.Forwarded, ListID: d.ListID, ListUnsubscribe: d.ListUnsubscribe, BodyText: d.BodyText,
		Attachments: atts, Source: "store",
	}
	if raw != nil {
		m := tbprofile.ParseMessage(raw)
		det.BodyText, det.Attachments, det.Source = m.BodyText, m.Attachments, "mbox"
		if withHeaders {
			det.Headers = map[string][]string(m.Header)
		}
	}
	for _, s := range []*[]string{&det.To, &det.Cc, &det.References} {
		if *s == nil {
			*s = []string{}
		}
	}
	if det.Attachments == nil {
		det.Attachments = []tbprofile.Attachment{}
	}
	return det
}

// tbStoredAttachments is the attachment metadata saved at sync time.
func tbStoredAttachments(db *store.Store, messageID string) []tbprofile.Attachment {
	out := make([]tbprofile.Attachment, 0)
	docs, err := tbMessageAttachments(db, messageID)
	if err != nil {
		return out
	}
	for _, a := range docs {
		out = append(out, tbprofile.Attachment{Index: a.Index, Filename: a.Filename, ContentType: a.ContentType, SizeBytes: a.SizeBytes})
	}
	return out
}

func tbPrintDetail(cmd *cobra.Command, det tbMessageDetail) error {
	w := tbHumanOut(cmd)
	from := det.FromAddr
	if det.FromName != "" {
		from = det.FromName + " <" + det.FromAddr + ">"
	}
	fmt.Fprintf(w, "ID:      %s\nDate:    %s\nFrom:    %s\nTo:      %s\n", det.ID, tbShortDate(det.Date), from, strings.Join(det.To, ", "))
	if len(det.Cc) > 0 {
		fmt.Fprintf(w, "Cc:      %s\n", strings.Join(det.Cc, ", "))
	}
	fmt.Fprintf(w, "Subject: %s\nFolder:  %s / %s\nFlags:   %s\n", det.Subject, det.AccountName, det.FolderPath, det.Flags)
	if len(det.Headers) > 0 {
		keys := make([]string, 0, len(det.Headers))
		for k := range det.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(w, "\nHeaders:")
		for _, k := range keys {
			for _, v := range det.Headers[k] {
				fmt.Fprintf(w, "  %s: %s\n", k, v)
			}
		}
	}
	if len(det.Attachments) > 0 {
		fmt.Fprintln(w, "\nAttachments:")
		for _, a := range det.Attachments {
			fmt.Fprintf(w, "  [%d] %s (%s, %s)\n", a.Index, a.Filename, a.ContentType, tbHumanBytes(a.SizeBytes))
		}
	}
	fmt.Fprintf(w, "\n%s\n", det.BodyText)
	return nil
}
