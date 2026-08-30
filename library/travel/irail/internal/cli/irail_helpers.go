// Hand-authored helpers shared by the irail novel commands.
//
// Lives beside the generated files so `generate --force` preserves it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embed the tz database so Europe/Brussels resolves anywhere

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/irailref"
)

// irailFetch performs a GET against the iRail API and decodes the envelope.
// pp:client-call
func irailFetch(ctx context.Context, c *client.Client, path string, params map[string]string) (map[string]any, error) {
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing iRail response from %s: %w", path, err)
	}
	return m, nil
}

// belgiumTZ is the reference clock for every date/time the API accepts.
// iRail interprets bare hhmm/ddmmyy values as Belgian local time.
func belgiumTZ() *time.Location {
	if loc, err := time.LoadLocation("Europe/Brussels"); err == nil {
		return loc
	}
	return time.Local
}

func nowInBelgium() time.Time { return time.Now().In(belgiumTZ()) }

// weekdayAliases maps English, Dutch and French weekday names (and common
// abbreviations) onto time.Weekday. commandtrein accepts Dutch only.
var weekdayAliases = map[string]time.Weekday{
	"monday": time.Monday, "mon": time.Monday, "maandag": time.Monday, "ma": time.Monday, "lundi": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "dinsdag": time.Tuesday, "di": time.Tuesday, "mardi": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday, "woensdag": time.Wednesday, "wo": time.Wednesday, "mercredi": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "donderdag": time.Thursday, "do": time.Thursday, "jeudi": time.Thursday,
	"friday": time.Friday, "fri": time.Friday, "vrijdag": time.Friday, "vr": time.Friday, "vendredi": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday, "zaterdag": time.Saturday, "za": time.Saturday, "samedi": time.Saturday,
	"sunday": time.Sunday, "sun": time.Sunday, "zondag": time.Sunday, "zo": time.Sunday, "dimanche": time.Sunday,
}

// todayAliases and tomorrowAliases cover the four languages the API serves.
var todayAliases = map[string]bool{"today": true, "vandaag": true, "aujourdhui": true, "heute": true}
var tomorrowAliases = map[string]bool{"tomorrow": true, "morgen": true, "demain": true}
var dayAfterAliases = map[string]bool{"overmorgen": true, "dayaftertomorrow": true, "apresdemain": true, "ubermorgen": true}

// parseHumanDate converts a human date into iRail's ddmmyy wire format.
// Accepts: empty (meaning "unset"), today/tomorrow/overmorgen and their Dutch,
// French and German equivalents, weekday names (next occurrence), ISO
// 2026-07-25, 25/07/2026, relative +3d, and raw ddmmyy passthrough.
func parseHumanDate(in string, now time.Time) (string, error) {
	s := strings.ToLower(strings.TrimSpace(in))
	if s == "" {
		return "", nil
	}
	key := irailref.Fold(s)

	switch {
	case todayAliases[key]:
		return now.Format("020106"), nil
	case tomorrowAliases[key]:
		return now.AddDate(0, 0, 1).Format("020106"), nil
	case dayAfterAliases[key]:
		return now.AddDate(0, 0, 2).Format("020106"), nil
	}

	if wd, ok := weekdayAliases[key]; ok {
		// Next occurrence, where today's weekday means a week out only if the
		// caller explicitly named today's day.
		delta := (int(wd) - int(now.Weekday()) + 7) % 7
		if delta == 0 {
			delta = 7
		}
		return now.AddDate(0, 0, delta).Format("020106"), nil
	}

	// Relative: +3d / +2w
	if strings.HasPrefix(s, "+") && len(s) > 2 {
		unit := s[len(s)-1]
		if n, err := strconv.Atoi(s[1 : len(s)-1]); err == nil {
			switch unit {
			case 'd':
				return now.AddDate(0, 0, n).Format("020106"), nil
			case 'w':
				return now.AddDate(0, 0, n*7).Format("020106"), nil
			}
		}
	}

	for _, layout := range []string{"2006-01-02", "02/01/2006", "02-01-2006", "2006/01/02"} {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t.Format("020106"), nil
		}
	}

	// Raw ddmmyy passthrough, validated so a typo cannot reach the API as a 500.
	if len(s) == 6 {
		if _, err := time.ParseInLocation("020106", s, now.Location()); err == nil {
			return s, nil
		}
	}

	return "", fmt.Errorf("unrecognised date %q: use tomorrow, monday, 2026-07-25, +2d or ddmmyy", in)
}

// parseHumanTime converts a human time into iRail's hhmm wire format.
// Accepts: empty, now, 08:12, 8:12, 0812, and relative +45m / +2h.
func parseHumanTime(in string, now time.Time) (string, error) {
	s := strings.ToLower(strings.TrimSpace(in))
	if s == "" {
		return "", nil
	}
	if s == "now" {
		return now.Format("1504"), nil
	}

	if strings.HasPrefix(s, "+") && len(s) > 2 {
		unit := s[len(s)-1]
		if n, err := strconv.Atoi(s[1 : len(s)-1]); err == nil {
			switch unit {
			case 'm':
				return now.Add(time.Duration(n) * time.Minute).Format("1504"), nil
			case 'h':
				return now.Add(time.Duration(n) * time.Hour).Format("1504"), nil
			}
		}
	}

	for _, layout := range []string{"15:04", "1504", "3:04pm", "3pm"} {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t.Format("1504"), nil
		}
	}

	return "", fmt.Errorf("unrecognised time %q: use 08:12, 0812, now or +30m", in)
}

// resolveWhen turns user-supplied date and time into iRail wire values.
//
// It fixes a bug clirail documents in its own README: when only a time is
// given and that time has already passed today, treating it as today silently
// answers the wrong question (planning a 07:00 trip at 23:00). Here the date
// rolls forward to tomorrow and the caller is told, via rolled, so it can say
// so rather than quietly changing the user's intent.
func resolveWhen(dateIn, timeIn string, now time.Time) (date, hhmm string, rolled bool, err error) {
	date, err = parseHumanDate(dateIn, now)
	if err != nil {
		return "", "", false, err
	}
	hhmm, err = parseHumanTime(timeIn, now)
	if err != nil {
		return "", "", false, err
	}

	if hhmm != "" && date == "" {
		asked, perr := time.ParseInLocation("1504", hhmm, now.Location())
		if perr == nil {
			today := time.Date(now.Year(), now.Month(), now.Day(), asked.Hour(), asked.Minute(), 0, 0, now.Location())
			if today.Before(now) {
				date = now.AddDate(0, 0, 1).Format("020106")
				rolled = true
			}
		}
	}
	return date, hhmm, rolled, nil
}

// --- typed coercion -------------------------------------------------------
//
// iRail returns every scalar as a JSON string ("delay":"0"). These helpers
// convert to real Go types so novel commands emit numbers as numbers and
// booleans as booleans. See iRail issue "All data is returned as strings".

func irailInt(v any) int {
	switch t := v.(type) {
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0
		}
		return n
	case float64:
		return int(t)
	case int:
		return t
	}
	return 0
}

func irailInt64(v any) int64 {
	switch t := v.(type) {
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return 0
		}
		return n
	case float64:
		return int64(t)
	case int64:
		return t
	}
	return 0
}

// irailBool reads iRail's "0"/"1" string booleans.
func irailBool(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) == "1" || strings.EqualFold(strings.TrimSpace(t), "true")
	case bool:
		return t
	case float64:
		return t != 0
	}
	return false
}

func irailString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// mapAt walks a nested map by key path, returning nil when any hop is absent.
func mapAt(m map[string]any, path ...string) map[string]any {
	cur := m
	for _, k := range path {
		if cur == nil {
			return nil
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

// sliceAt returns a nested array, tolerating iRail's habit of collapsing a
// single-element array into a bare object.
func sliceAt(m map[string]any, path ...string) []any {
	if len(path) == 0 {
		return nil
	}
	parent := m
	if len(path) > 1 {
		parent = mapAt(m, path[:len(path)-1]...)
	}
	if parent == nil {
		return nil
	}
	switch v := parent[path[len(path)-1]].(type) {
	case []any:
		return v
	case map[string]any:
		return []any{v}
	}
	return nil
}

// unixToLocal formats an iRail unix-second string as RFC3339 in Belgian time.
func unixToLocal(v any) string {
	sec := irailInt64(v)
	if sec == 0 {
		return ""
	}
	return time.Unix(sec, 0).In(belgiumTZ()).Format(time.RFC3339)
}

// resolveStationName maps a user-supplied alias, telegraph code or id onto a
// name the API accepts. Unknown input is passed through unchanged so the API
// keeps the final say and new stations still work before the dataset refreshes.
func resolveStationName(in string) string {
	if st, ok := irailref.Lookup(in); ok {
		if st.Name != "" {
			return st.Name
		}
	}
	return in
}

// clockOf extracts HH:MM from an RFC3339 timestamp for human output.
//
// unixToLocal returns "" when iRail omits a time field, which its own issue
// tracker shows does happen on the connections endpoint. Slicing such a string
// at a fixed offset panics, so every human-facing render goes through here.
func clockOf(rfc3339 string) string {
	const start, end = 11, 16
	if len(rfc3339) < end {
		return "--:--"
	}
	return rfc3339[start:end]
}

// humanDuration renders seconds as a compact "1h04" / "23m" string.
func humanDuration(sec int) string {
	if sec <= 0 {
		return "0m"
	}
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02d", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
