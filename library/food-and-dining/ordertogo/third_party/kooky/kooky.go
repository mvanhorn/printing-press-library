package kooky

// PATCH: Minimal local replacement for the kooky API surface used by the
// generated CLI in this offline build environment.

import (
	"net/http"
	"strings"
	"time"
)

type Cookie struct {
	http.Cookie
	Creation time.Time
}

type Filter interface {
	Filter(*Cookie) bool
}

type FilterFunc func(*Cookie) bool

func (f FilterFunc) Filter(c *Cookie) bool {
	return f != nil && f(c)
}

var Valid Filter = FilterFunc(func(cookie *Cookie) bool {
	return cookie != nil && (cookie.Expires.IsZero() || cookie.Expires.After(time.Now()))
})

func DomainHasSuffix(suffix string) Filter {
	return FilterFunc(func(cookie *Cookie) bool {
		return cookie != nil && strings.HasSuffix(strings.TrimPrefix(cookie.Domain, "."), strings.TrimPrefix(suffix, "."))
	})
}

func FilterCookie(cookie *Cookie, filters ...Filter) bool {
	for _, filter := range filters {
		if filter != nil && !filter.Filter(cookie) {
			return false
		}
	}
	return true
}
