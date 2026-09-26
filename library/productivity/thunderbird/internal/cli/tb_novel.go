// pp:data-source local

package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

var tbNow = time.Now

const (
	tbDirOther = iota
	tbDirSent
	tbDirInbound
)

// tbMailbox classifies messages: only mail in a sent folder is sent by me, own-address mail elsewhere is neither side.
type tbMailbox struct {
	sentFolders map[string]bool
	own         map[string]bool
}

func tbLoadMailbox(db *store.Store) (*tbMailbox, error) {
	folders, err := tbLoadDocs[struct {
		ID         string `json:"id"`
		Path       string `json:"path"`
		SentFolder *bool  `json:"sent_folder"`
	}](db, "folders")
	if err != nil {
		return nil, err
	}
	ids, err := tbLoadDocs[tbIdentityDoc](db, "identities")
	if err != nil {
		return nil, err
	}
	mb := &tbMailbox{sentFolders: map[string]bool{}, own: tbOwnAddresses(ids)}
	for _, f := range folders {
		// Stores synced before sent_folder existed fall back to the folder name.
		if f.SentFolder != nil && *f.SentFolder || f.SentFolder == nil && tbprofile.IsSentFolderName(f.Path) {
			mb.sentFolders[strings.ToLower(f.ID)] = true
		}
	}
	return mb, nil
}

func (mb *tbMailbox) directionLabel(d tbMessageDoc) string {
	if mb.sentByMe(d) {
		return "out"
	}
	return "in"
}

func (mb *tbMailbox) sentByMe(d tbMessageDoc) bool {
	return mb.sentFolders[strings.ToLower(d.FolderKey)]
}

func (mb *tbMailbox) isOwn(addr string) bool { return mb.own[strings.ToLower(addr)] }

func (mb *tbMailbox) direction(d tbMessageDoc) int {
	switch {
	case mb.sentByMe(d):
		return tbDirSent
	case mb.isOwn(d.FromAddr):
		return tbDirOther
	default:
		return tbDirInbound
	}
}

var tbNoReplyRE = regexp.MustCompile(`(?i)^(no-?reply|do-?not-?reply|mailer-daemon|postmaster|bounces?|notifications?)([+._-].*)?@`)

const (
	tbBulkList      = "list"
	tbBulkAutomated = "automated"
	tbBulkOneWay    = "one_way"

	tbOneWayMinReceived = 20
)

// tbBulk classifies bulk mail; one-way senders are derived from the whole window.
type tbBulk struct {
	oneWay map[string]bool
}

func tbNewBulk(docs []tbMessageDoc, mb *tbMailbox) *tbBulk {
	received, sentTo, seen := map[string]int{}, map[string]bool{}, map[string]bool{}
	for _, d := range docs {
		if tbprofile.IsTrashOrJunkFolderName(d.FolderPath) {
			continue
		}
		switch mb.direction(d) {
		case tbDirInbound:
			if k := tbDedupKey(d); !seen[k] {
				seen[k] = true
				received[strings.ToLower(d.FromAddr)]++
			}
		case tbDirSent:
			for _, a := range tbRecipients(d) {
				sentTo[strings.ToLower(a)] = true
			}
		}
	}
	b := &tbBulk{oneWay: map[string]bool{}}
	for a, n := range received {
		if a != "" && n >= tbOneWayMinReceived && !sentTo[a] {
			b.oneWay[a] = true
		}
	}
	return b
}

func (b *tbBulk) kind(d tbMessageDoc) string {
	switch {
	case d.ListID != "":
		return tbBulkList
	case d.Automated || d.ListUnsubscribe != "" || tbNoReplyRE.MatchString(d.FromAddr):
		return tbBulkAutomated
	case b.oneWay[strings.ToLower(d.FromAddr)]:
		return tbBulkOneWay
	}
	return ""
}

func tbRecipients(d tbMessageDoc) []string {
	return append(append([]string{}, d.To...), d.Cc...)
}

func tbMsgTime(d tbMessageDoc) time.Time {
	t, _ := time.Parse(time.RFC3339, d.Date)
	return t
}

// tbDedupKey identifies one message across folder copies.
func tbDedupKey(d tbMessageDoc) string {
	if d.MessageID != "" {
		return d.MessageID
	}
	return d.ID
}

// tbWindowStart resolves --since, or the hidden --days alias when it was given.
func tbWindowStart(cmd *cobra.Command, since string, days int, now time.Time) (time.Time, error) {
	if cmd.Flags().Changed("days") {
		if days <= 0 {
			return time.Time{}, usageErr(fmt.Errorf("--days must be a positive number of days"))
		}
		return now.AddDate(0, 0, -days), nil
	}
	t, err := tbParseTimeBound(since, now, false)
	if err != nil {
		return time.Time{}, usageErr(err)
	}
	if t.IsZero() {
		return time.Time{}, usageErr(fmt.Errorf("--since must be a duration like 14d or a date like 2025-01-31"))
	}
	return t, nil
}

func tbInClause(n int) string { return "(?" + strings.Repeat(",?", n-1) + ")" }

// tbWindowMessages loads messages dated at or after since, without bodies.
func tbWindowMessages(db *store.Store, since time.Time) ([]tbMessageDoc, error) {
	return tbQueryMessages(db, "json_extract(data,'$.date') >= ?", []any{tbFormatTime(since)}, "", 0)
}
