// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ingest reservation history from server-rendered launch pages.
//
// Bookclicker exposes no JSON index for reservations - the /api/reservations
// routes are all POST/PUT actions on a single id. The only place booked
// promotions appear is the HTML launch page, so this command scrapes them into
// the local mirror that swap-balance, partner-roi, stale, confirm-due and
// launch health all read.
// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/internal/store"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		resCmd, _, err := root.Find([]string{"reservations"})
		if err == nil && resCmd != nil {
			addNovelCommandIfAbsent(resCmd, newReservationsPullCmd(flags))
		}
	})
}

type pullResult struct {
	Books    int      `json:"books_scanned"`
	Found    int      `json:"reservations_found"`
	Inserted int      `json:"inserted"`
	Updated  int      `json:"updated"`
	BookIDs  []int64  `json:"book_ids"`
	Skipped  []string `json:"skipped,omitempty"`
	DryRun   bool     `json:"dry_run,omitempty"`
	Note     string   `json:"note,omitempty"`
}

// reDataPromos captures the launch page's embedded reservation payload. The
// page renders its table client-side, so the served HTML carries no <tr> rows;
// the data lives in an HTML-escaped JSON array on a data-promos attribute.
var reDataPromos = regexp.MustCompile(`data-promos="([^"]*)"`)

func newReservationsPullCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		flagBook int64
		flagAll  bool
		flagMax  int
	)

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Ingest booked promotions from your launch pages into the local mirror",
		Long: "Bookclicker renders reservations only as HTML on each book's launch page;\n" +
			"there is no JSON index for them. This reads those pages with your session\n" +
			"and mirrors the rows locally so the history-based commands have data.\n\n" +
			"Read-only against Bookclicker: it issues GETs and never books or cancels.",
		Example:     "  bookclicker-pp-cli reservations pull --all",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reservations pull")
			}
			if flagBook <= 0 && !flagAll {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("pass --book <id> or --all"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			cfg, err := config.Load("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if strings.TrimSpace(cfg.BookclickerSession) == "" {
				return &cliError{code: 4, err: fmt.Errorf(
					"no Bookclicker session stored; run 'bookclicker-pp-cli auth login' or set BOOKCLICKER_SESSION")}
			}

			if dbPath == "" {
				dbPath = defaultDBPath("bookclicker-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureBookclickerTables(ctx, db); err != nil {
				return err
			}

			books, err := pullTargetBooks(ctx, db, flagBook)
			if err != nil {
				return err
			}
			if len(books) == 0 {
				res := pullResult{Note: "no books in the local mirror; run 'bookclicker-pp-cli sync' first", BookIDs: []int64{}}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			if flagMax > 0 && len(books) > flagMax {
				books = books[:flagMax]
			}

			hc := &http.Client{Timeout: 30 * time.Second}
			res := pullResult{BookIDs: make([]int64, 0, len(books)), Skipped: make([]string, 0)}
			for _, b := range books {
				res.Books++
				res.BookIDs = append(res.BookIDs, b.id)
				page, err := fetchLaunchPage(ctx, hc, cfg, b.id)
				if err != nil {
					res.Skipped = append(res.Skipped, fmt.Sprintf("book %d: %v", b.id, err))
					continue
				}
				rows := parseLaunchReservations(page, b.id, b.title)
				res.Found += len(rows)
				for _, r := range rows {
					ins, err := upsertReservation(ctx, db, r)
					if err != nil {
						return err
					}
					if ins {
						res.Inserted++
					} else {
						res.Updated++
					}
				}
				// Human-paced: these are ordinary page loads on a small site.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(700 * time.Millisecond):
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Scanned %d launch page(s): %d reservation(s) found, %d new, %d updated.\n",
				res.Books, res.Found, res.Inserted, res.Updated)
			for _, s := range res.Skipped {
				fmt.Fprintf(w, "  skipped %s\n", s)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().Int64Var(&flagBook, "book", 0, "Pull reservations for a single book id")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Pull reservations for every mirrored book")
	cmd.Flags().IntVar(&flagMax, "max-books", 0, "Stop after this many books (0 = no limit)")
	return cmd
}

type pullBook struct {
	id    int64
	title string
}

func pullTargetBooks(ctx context.Context, db *store.Store, only int64) ([]pullBook, error) {
	q := `SELECT CAST(id AS INTEGER), title FROM books WHERE deleted IS NULL OR deleted = 0`
	args := []any{}
	if only > 0 {
		q = `SELECT CAST(id AS INTEGER), title FROM books WHERE CAST(id AS INTEGER) = ?`
		args = append(args, only)
	}
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying books: %w", err)
	}
	out := make([]pullBook, 0)
	for rows.Next() {
		var id sql.NullInt64
		var title sql.NullString
		if err := rows.Scan(&id, &title); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning book: %w", err)
		}
		if id.Valid && id.Int64 > 0 {
			out = append(out, pullBook{id: id.Int64, title: title.String})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating books: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing rows: %w", err)
	}
	return out, nil
}

func fetchLaunchPage(ctx context.Context, hc *http.Client, cfg *config.Config, bookID int64) (string, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://www.bookclicker.com"
	}
	url := fmt.Sprintf("%s/my_books/%d/launch", base, bookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", "_bookclicker_session="+strings.TrimSpace(cfg.BookclickerSession))
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "bookclicker-pp-cli")
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("session rejected (HTTP %d); run 'auth login'", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	html := string(body)
	if strings.Contains(html, `href="/login"`) && !strings.Contains(html, `href="/sign_out"`) {
		return "", fmt.Errorf("redirected to login; session expired")
	}
	return html, nil
}

// launchPromo mirrors one entry of the launch page's data-promos array. Only
// the fields the local mirror stores are declared; the upstream object carries
// roughly forty, including the full accept/decline/cancel lifecycle.
type launchPromo struct {
	ID                 int64  `json:"id"`
	ListID             int64  `json:"list_id"`
	BookID             int64  `json:"book_id"`
	ListName           string `json:"list_name"`
	RecordedListName   string `json:"recorded_list_name"`
	ListSize           int64  `json:"list_size"`
	Date               string `json:"date"`
	InvType            string `json:"inv_type"`
	Status             string `json:"status"`
	Price              *int64 `json:"price"`
	SwapOffer          int    `json:"swap_offer"`
	PaymentOffer       int    `json:"payment_offer"`
	SwapReservationID  int64  `json:"swap_reservation_id"`
	CreatedAt          string `json:"created_at"`
	SellerAcceptedAt   string `json:"seller_accepted_at"`
	SellerDeclinedAt   string `json:"seller_declined_at"`
	BuyerCancelledAt   string `json:"buyer_cancelled_at"`
	SellerCancelledAt  string `json:"seller_cancelled_at"`
	ConfirmRequestedAt string `json:"confirmation_requested_at"`
}

// parseLaunchReservations reads the reservation set a launch page embeds in its
// data-promos attribute. The attribute is HTML-escaped JSON, so it needs
// unescaping before it will parse.
func parseLaunchReservations(page string, bookID int64, bookTitle string) []novelReservation {
	out := make([]novelReservation, 0)
	m := reDataPromos.FindStringSubmatch(page)
	if m == nil {
		return out
	}
	var promos []launchPromo
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &promos); err != nil {
		return out
	}
	for _, p := range promos {
		name := strings.TrimSpace(p.ListName)
		if name == "" {
			name = strings.TrimSpace(p.RecordedListName)
		}
		book := p.BookID
		if book == 0 {
			book = bookID
		}
		out = append(out, novelReservation{
			ID:          strconv.FormatInt(p.ID, 10),
			BookID:      book,
			BookTitle:   bookTitle,
			ListID:      p.ListID,
			ListName:    name,
			ListSize:    p.ListSize,
			Date:        strings.TrimSpace(p.Date),
			InvType:     strings.ToLower(strings.TrimSpace(p.InvType)),
			Status:      strings.ToLower(strings.TrimSpace(p.Status)),
			IsSwap:      p.SwapOffer == 1,
			Price:       p.Price,
			SwapPairID:  p.SwapReservationID,
			Counterpar:  name,
			CreatedAt:   p.CreatedAt,
			AcceptedAt:  p.SellerAcceptedAt,
			DeclinedAt:  p.SellerDeclinedAt,
			CancelledAt: firstNonEmpty(p.BuyerCancelledAt, p.SellerCancelledAt),
		})
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// upsertReservation returns true when the row was newly inserted.
func upsertReservation(ctx context.Context, db *store.Store, r novelReservation) (bool, error) {
	var existing sql.NullString
	err := db.DB().QueryRowContext(ctx, `SELECT id FROM reservation_mirror WHERE id = ?`, r.ID).Scan(&existing)
	isNew := err == sql.ErrNoRows
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("checking reservation: %w", err)
	}
	swap := 0
	if r.IsSwap {
		swap = 1
	}
	_, err = db.DB().ExecContext(ctx, `
		INSERT INTO reservation_mirror
			(id, book_id, book_title, list_id, list_name, list_size, date, inv_type,
			 status, is_swap, price, swap_reservation_id, counterparty, created_at,
			 seller_accepted_at, seller_declined_at, buyer_cancelled_at, source, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'launch-page', CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			list_size = excluded.list_size,
			is_swap = excluded.is_swap,
			price = excluded.price,
			seller_accepted_at = excluded.seller_accepted_at,
			seller_declined_at = excluded.seller_declined_at,
			buyer_cancelled_at = excluded.buyer_cancelled_at,
			synced_at = CURRENT_TIMESTAMP`,
		r.ID, r.BookID, r.BookTitle, r.ListID, r.ListName, r.ListSize, r.Date, r.InvType,
		r.Status, swap, r.Price, r.SwapPairID, r.Counterpar, r.CreatedAt,
		r.AcceptedAt, r.DeclinedAt, r.CancelledAt)
	if err != nil {
		return false, fmt.Errorf("upserting reservation: %w", err)
	}
	return isNew, nil
}
