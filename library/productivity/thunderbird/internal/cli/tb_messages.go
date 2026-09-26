// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

// tbMessageRow is the compact listing shape of a message.
type tbMessageRow struct {
	ID              string `json:"id"`
	Date            string `json:"date"`
	FromAddr        string `json:"from_addr"`
	FromName        string `json:"from_name"`
	Subject         string `json:"subject"`
	Account         string `json:"account"`
	AccountName     string `json:"account_name"`
	Folder          string `json:"folder"`
	FolderPath      string `json:"folder_path"`
	MessageID       string `json:"message_id"`
	ThreadID        string `json:"thread_id"`
	Read            bool   `json:"read"`
	Flagged         bool   `json:"flagged"`
	Replied         bool   `json:"replied"`
	HasAttachments  bool   `json:"has_attachments"`
	AttachmentCount int    `json:"attachment_count"`
	SizeBytes       int64  `json:"size_bytes"`
	Flags           string `json:"flags"`
	Direction       string `json:"direction,omitempty"`
	TotalMessages   int    `json:"total_messages,omitempty"`
}

func tbRowFromDoc(d tbMessageDoc) tbMessageRow {
	return tbMessageRow{
		ID: d.ID, Date: d.Date, FromAddr: d.FromAddr, FromName: d.FromName, Subject: d.Subject,
		Account: d.Account, AccountName: d.AccountName, Folder: d.Folder, FolderPath: d.FolderPath,
		MessageID: d.MessageID, ThreadID: d.ThreadID, Read: d.Read, Flagged: d.Flagged, Replied: d.Replied,
		HasAttachments: d.HasAttachments, AttachmentCount: d.AttachmentCount,
		SizeBytes: d.SizeBytes, Flags: tbFlags(d),
	}
}

// tbFlags renders U(nread) F(lagged) R(eplied) A(ttachments).
func tbFlags(d tbMessageDoc) string {
	var b strings.Builder
	if !d.Read {
		b.WriteByte('U')
	}
	if d.Flagged {
		b.WriteByte('F')
	}
	if d.Replied {
		b.WriteByte('R')
	}
	if d.HasAttachments {
		b.WriteByte('A')
	}
	return b.String()
}

// tbStoreFor returns a nil store when the empty result was already emitted.
func tbStoreFor(cmd *cobra.Command, flags *rootFlags, resource string) (*store.Store, error) {
	db, err := tbOpenStore(cmd)
	if err != nil || db == nil {
		if err == nil {
			err = tbEmitEmpty(cmd, flags)
		}
		return nil, err
	}
	hintIfUnsynced(cmd, db, resource)
	hintIfStale(cmd, db, resource, flags.maxAge)
	return db, nil
}

// tbQueryMessages returns message docs without body_text.
func tbQueryMessages(db *store.Store, where string, args []any, order string, limit int) ([]tbMessageDoc, error) {
	return tbQueryMessagesBody(db, where, args, order, limit, false)
}

func tbQueryMessagesBody(db *store.Store, where string, args []any, order string, limit int, withBody bool) ([]tbMessageDoc, error) {
	sel := `json_remove(data,'$.body_text')`
	if withBody {
		sel = `data`
	}
	q := `SELECT ` + sel + ` FROM resources WHERE resource_type = 'messages'`
	if where != "" {
		q += " AND " + where
	}
	if order != "" {
		q += " ORDER BY " + order
	}
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	return tbScanDocs[tbMessageDoc](db, q, args...)
}

func tbScanDocs[T any](db *store.Store, q string, args ...any) ([]T, error) {
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]T, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// tbGetMessage finds a message by store id or Message-ID, preferring a non-sent copy.
func tbGetMessage(db *store.Store, ref string) (*tbMessageDoc, error) {
	ref = strings.TrimSpace(ref)
	var raw string
	err := db.DB().QueryRow(`SELECT data FROM resources WHERE resource_type = 'messages' AND id = ?`, ref).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) && strings.Contains(ref, "@") {
		copies, qerr := tbQueryMessagesBody(db, `json_extract(data,'$.message_id') = ?`, []any{tbprofile.NormalizeMessageID(ref)}, "id", 0, true)
		if qerr != nil {
			return nil, qerr
		}
		if len(copies) > 0 {
			mb, merr := tbLoadMailbox(db)
			if merr != nil {
				return nil, merr
			}
			for i := range copies {
				if !mb.sentByMe(copies[i]) {
					return &copies[i], nil
				}
			}
			return &copies[0], nil
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFoundErr(fmt.Errorf("message %q not found in the local store (ids come from: %s messages list)", ref, tbCLIName))
	}
	if err != nil {
		return nil, err
	}
	var d tbMessageDoc
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func tbReadRaw(d *tbMessageDoc) ([]byte, error) {
	if d.MboxPath == "" || d.Length <= 0 {
		return nil, fmt.Errorf("message %s has no mbox location; run: %s sync --full", d.ID, tbCLIName)
	}
	raw, err := tbprofile.ReadMessageAt(d.MboxPath, d.Offset, d.Length, d.MessageID)
	if errors.Is(err, tbprofile.ErrMessageMoved) {
		return nil, fmt.Errorf("message %s moved inside its mbox (folder compacted since the last sync); run: %s sync", d.ID, tbCLIName)
	}
	if err != nil {
		return nil, fmt.Errorf("reading message %s from its mbox: %w; the store may be stale, run: %s sync", d.ID, err, tbCLIName)
	}
	return raw, nil
}

func tbParseTimeBound(s string, now time.Time, future bool) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	d, err := cliutil.ParseDurationLoose(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: use a duration like 7d/2w/36h or a date like 2025-01-31", s)
	}
	if future {
		return now.Add(d), nil
	}
	return now.Add(-d), nil
}

func tbShortDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04")
}

// tbSafe keeps mail-borne terminal control characters (C0 but \n \t, DEL, C1) out of human output.
func tbSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' || r >= 0x7f && r <= 0x9f {
			return utf8.RuneError
		}
		return r
	}, s)
}

type tbSafeWriter struct{ w io.Writer }

// Write reports len(p) so fmt and tabwriter see no short write after mapping.
func (s tbSafeWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(s.w, tbSafe(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// tbHumanOut is the human-mode stdout: everything printed through it goes through tbSafe.
func tbHumanOut(cmd *cobra.Command) io.Writer { return tbSafeWriter{cmd.OutOrStdout()} }

func tbTrunc(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func tbSender(name, addr string) string {
	if name != "" {
		return name
	}
	return addr
}

// tbFindChild returns the direct child command named name.
func tbFindChild(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func tbPrintMessageRows(cmd *cobra.Command, flags *rootFlags, rows []tbMessageRow, withDirection bool) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	tw := newTabWriter(tbHumanOut(cmd))
	if withDirection {
		fmt.Fprintln(tw, "ID\tDATE\tDIR\tFROM\tSUBJECT\tFOLDER\tFLAGS")
	} else {
		fmt.Fprintln(tw, "ID\tDATE\tFROM\tSUBJECT\tFOLDER\tFLAGS")
	}
	for _, r := range rows {
		if withDirection {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, tbShortDate(r.Date), r.Direction, tbTrunc(tbSender(r.FromName, r.FromAddr), 28), tbTrunc(r.Subject, 60), r.FolderPath, r.Flags)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, tbShortDate(r.Date), tbTrunc(tbSender(r.FromName, r.FromAddr), 28), tbTrunc(r.Subject, 60), r.FolderPath, r.Flags)
		}
	}
	return tw.Flush()
}
