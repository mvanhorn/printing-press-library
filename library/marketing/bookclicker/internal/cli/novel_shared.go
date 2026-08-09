// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the novel (transcendence) commands.
// Kept in its own file so `generate --force` preserves it.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/internal/store"
)

// novelReservation is one booked promotion, as mirrored locally.
//
// Bookclicker exposes no JSON index for reservations. Rows arrive from each
// book's launch page, which embeds the full reservation set as HTML-escaped
// JSON in a `data-promos` attribute; `reservations pull` reads it over plain
// HTTP.
type novelReservation struct {
	ID          string `json:"id"`
	BookID      int64  `json:"book_id"`
	BookTitle   string `json:"book_title,omitempty"`
	ListID      int64  `json:"list_id"`
	ListName    string `json:"list_name"`
	ListSize    int64  `json:"list_size"`
	Date        string `json:"date"`
	InvType     string `json:"inv_type"`
	Status      string `json:"status"`
	IsSwap      bool   `json:"is_swap"`
	Price       *int64 `json:"price"`
	SwapPairID  int64  `json:"swap_reservation_id,omitempty"`
	Counterpar  string `json:"counterparty,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	AcceptedAt  string `json:"seller_accepted_at,omitempty"`
	DeclinedAt  string `json:"seller_declined_at,omitempty"`
	CancelledAt string `json:"cancelled_at,omitempty"`
}

// openMirror resolves the db path and opens the local store. It returns
// handled=true when the mirror is absent and the caller has already been given
// an honest empty result, so the caller should return nil immediately.
func openMirror(ctx context.Context, dbPath string, out io.Writer, errOut io.Writer, flags *rootFlags, empty any) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("bookclicker-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(errOut, "no local mirror at %s\nrun: bookclicker-pp-cli sync --db %s\n", dbPath, dbPath)
		if !wantsHumanTable(out, flags) {
			return nil, true, printJSONFiltered(out, empty, flags)
		}
		return nil, true, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, true, fmt.Errorf("opening database: %w", err)
	}
	return db, false, nil
}

// loadReservations reads mirrored reservations, optionally filtered by book.
func loadReservations(ctx context.Context, db *store.Store, bookID int64) ([]novelReservation, error) {
	if err := store.EnsureBookclickerTables(ctx, db); err != nil {
		return nil, err
	}
	q := `SELECT id, book_id, book_title, list_id, list_name, list_size, date,
	             inv_type, status, is_swap, price, swap_reservation_id,
	             counterparty, created_at, seller_accepted_at, seller_declined_at,
	             buyer_cancelled_at
	      FROM reservation_mirror`
	args := []any{}
	if bookID > 0 {
		q += " WHERE book_id = ?"
		args = append(args, bookID)
	}
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying reservations: %w", err)
	}
	out := make([]novelReservation, 0)
	for rows.Next() {
		var (
			id, bookTitle, listName, date, invType, status     sql.NullString
			counter, created, accepted, declined, cancelled    sql.NullString
			bookIDv, listID, listSize, isSwap, price, swapPair sql.NullInt64
		)
		if err := rows.Scan(&id, &bookIDv, &bookTitle, &listID, &listName, &listSize,
			&date, &invType, &status, &isSwap, &price, &swapPair,
			&counter, &created, &accepted, &declined, &cancelled); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning reservation: %w", err)
		}
		r := novelReservation{
			ID: id.String, BookID: bookIDv.Int64, BookTitle: bookTitle.String,
			ListID: listID.Int64, ListName: listName.String, ListSize: listSize.Int64,
			Date: date.String, InvType: invType.String, Status: status.String,
			IsSwap: isSwap.Int64 == 1, SwapPairID: swapPair.Int64,
			Counterpar: counter.String, CreatedAt: created.String,
			AcceptedAt: accepted.String, DeclinedAt: declined.String,
			CancelledAt: cancelled.String,
		}
		if price.Valid {
			p := price.Int64
			r.Price = &p
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating reservations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing rows: %w", err)
	}
	return out, nil
}

// reservationsEmptyNote explains the one way this table gets populated, so an
// empty result never reads as "you have no bookings".
const reservationsEmptyNote = "no mirrored reservations yet - run 'bookclicker-pp-cli reservations pull --all' to ingest them from your launch pages"

// parseLooseDate accepts the date shapes Bookclicker renders.
func parseLooseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z07:00", "January 2nd 2006", "January 2 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseOrdinalDate turns "September 5th" (as rendered on launch pages) into a
// date, inferring the year from a reference point.
func parseOrdinalDate(s string, ref time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, suf := range []string{"st", "nd", "rd", "th"} {
		s = strings.ReplaceAll(s, suf+" ", " ")
		if strings.HasSuffix(s, suf) {
			s = strings.TrimSuffix(s, suf)
		}
	}
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(fields[1])
	if err != nil {
		return time.Time{}, false
	}
	var month time.Month
	for m := time.January; m <= time.December; m++ {
		if strings.EqualFold(m.String(), fields[0]) {
			month = m
			break
		}
	}
	if month == 0 {
		return time.Time{}, false
	}
	year := ref.Year()
	if len(fields) > 2 {
		if y, err := strconv.Atoi(fields[2]); err == nil {
			year = y
		}
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), true
}

// truncPad shortens a string for table output.
func truncPad(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
