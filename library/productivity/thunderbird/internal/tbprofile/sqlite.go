package tbprofile

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// withSnapshot copies a live SQLite database (and its -wal) to a temp dir
// and opens the copy read-only, so Thunderbird's files are never touched.
func withSnapshot(dbPath string, fn func(*sql.DB) error) error {
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "tbsnap-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	dst := filepath.Join(tmp, filepath.Base(dbPath))
	if err := copyFile(dbPath, dst); err != nil {
		return err
	}
	// -shm is rebuilt from the WAL; copying a live one could be inconsistent.
	if _, err := os.Stat(dbPath + "-wal"); err == nil {
		if err := copyFile(dbPath+"-wal", dst+"-wal"); err != nil {
			return err
		}
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dst)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	return fn(db)
}

func copyFile(src, dst string) error {
	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(filepath.Clean(dst))
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// Contact is one address book card.
type Contact struct {
	ID          string   `json:"id"`
	Book        string   `json:"book"`
	DisplayName string   `json:"display_name"`
	FirstName   string   `json:"first_name"`
	LastName    string   `json:"last_name"`
	NickName    string   `json:"nickname"`
	Company     string   `json:"company"`
	Emails      []string `json:"emails"`
}

// BookName is the Contact.Book of the address book file at path.
func BookName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// ReadAddressBook reads the EAV properties table of an abook sqlite file.
func ReadAddressBook(path string) ([]Contact, error) {
	book := BookName(path)
	out := make([]Contact, 0)
	err := withSnapshot(path, func(db *sql.DB) error {
		rows, err := db.Query(`SELECT card, name, value FROM properties ORDER BY card`)
		if err != nil {
			return err
		}
		defer rows.Close()
		byCard := map[string]*Contact{}
		var order []string
		for rows.Next() {
			var card, name string
			var value sql.NullString
			if err := rows.Scan(&card, &name, &value); err != nil {
				return err
			}
			c := byCard[card]
			if c == nil {
				c = &Contact{ID: card, Book: book, Emails: []string{}}
				byCard[card] = c
				order = append(order, card)
			}
			v := strings.TrimSpace(value.String)
			switch name {
			case "DisplayName":
				c.DisplayName = v
			case "FirstName":
				c.FirstName = v
			case "LastName":
				c.LastName = v
			case "NickName":
				c.NickName = v
			case "Company":
				c.Company = v
			case "PrimaryEmail", "SecondEmail":
				if v != "" {
					if name == "PrimaryEmail" {
						c.Emails = append([]string{strings.ToLower(v)}, c.Emails...)
					} else {
						c.Emails = append(c.Emails, strings.ToLower(v))
					}
				}
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range order {
			out = append(out, *byCard[id])
		}
		return nil
	})
	return out, err
}

// AddressBookFiles lists abook.sqlite, abook-N.sqlite and history.sqlite.
func AddressBookFiles(profileDir string) []string {
	var out []string
	for _, pat := range []string{"abook.sqlite", "abook-*.sqlite", "history.sqlite"} {
		m, _ := filepath.Glob(filepath.Join(profileDir, pat))
		sort.Strings(m)
		out = append(out, m...)
	}
	return out
}

// Event is one calendar-data/local.sqlite event.
type Event struct {
	ID         string    `json:"id"`
	CalendarID string    `json:"calendar_id"`
	Title      string    `json:"title"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Location   string    `json:"location,omitempty"`
}

// ReadCalendarEvents reads events from profileDir/calendar-data/local.sqlite.
func ReadCalendarEvents(profileDir string) ([]Event, error) {
	path := filepath.Join(profileDir, "calendar-data", "local.sqlite")
	out := make([]Event, 0)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return out, nil
	}
	err := withSnapshot(path, func(db *sql.DB) error {
		rows, err := db.Query(`SELECT cal_id, id, COALESCE(title,''), COALESCE(event_start,0), COALESCE(event_end,0) FROM cal_events ORDER BY event_start`)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				return nil
			}
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e Event
			var start, end int64
			if err := rows.Scan(&e.CalendarID, &e.ID, &e.Title, &start, &end); err != nil {
				return err
			}
			e.Start = microsToTime(start)
			e.End = microsToTime(end)
			out = append(out, e)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return readEventLocations(db, out)
	})
	return out, err
}

func readEventLocations(db *sql.DB, events []Event) error {
	rows, err := db.Query(`SELECT item_id, value FROM cal_properties WHERE key = 'LOCATION'`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	loc := map[string]string{}
	for rows.Next() {
		var id string
		var v sql.NullString
		if err := rows.Scan(&id, &v); err != nil {
			return fmt.Errorf("reading event locations: %w", err)
		}
		loc[id] = v.String
	}
	for i := range events {
		events[i].Location = loc[events[i].ID]
	}
	return rows.Err()
}

func microsToTime(us int64) time.Time {
	if us == 0 {
		return time.Time{}
	}
	return time.UnixMicro(us).UTC()
}
