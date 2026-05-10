package chrome

// PATCH: Minimal Chrome cookie reader backing auth login --chrome.

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/browserutils/kooky"
	_ "modernc.org/sqlite"
)

func ReadCookies(ctx context.Context, filename string, filters ...kooky.Filter) ([]*kooky.Cookie, error) {
	db, err := sql.Open("sqlite", "file:"+filename+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `SELECT host_key, name, value, path, expires_utc, is_secure, is_httponly FROM cookies`)
	if err != nil {
		return nil, fmt.Errorf("query cookies: %w", err)
	}
	defer rows.Close()

	var cookies []*kooky.Cookie
	for rows.Next() {
		var domain, name, value, path string
		var expiresUTC int64
		var secure, httpOnly int
		if err := rows.Scan(&domain, &name, &value, &path, &expiresUTC, &secure, &httpOnly); err != nil {
			return nil, err
		}
		cookie := &kooky.Cookie{Cookie: http.Cookie{
			Name:     name,
			Value:    value,
			Domain:   domain,
			Path:     path,
			Secure:   secure != 0,
			HttpOnly: httpOnly != 0,
			Expires:  chromeTime(expiresUTC),
		}}
		if kooky.FilterCookie(cookie, filters...) {
			cookies = append(cookies, cookie)
		}
	}
	return cookies, rows.Err()
}

func chromeTime(microseconds int64) time.Time {
	if microseconds <= 0 {
		return time.Time{}
	}
	return time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(microseconds) * time.Microsecond)
}
