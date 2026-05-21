package cli

import (
	"fmt"
	"strings"
	"time"
)

// Czech locale uses DD.MM.YYYY everywhere on the wire. Users want
// to type ISO YYYY-MM-DD or relative tokens like "today" / "yesterday"
// / "-3". These helpers handle the round-trip.

// parseFlexDate accepts:
//   - "today" or empty
//   - "yesterday"
//   - "-N" (N days ago)
//   - DD.MM.YYYY
//   - YYYY-MM-DD
//
// Returns DD.MM.YYYY which the API wants on the wire.
func parseFlexDate(in string) (string, error) {
	in = strings.TrimSpace(in)
	now := time.Now().Local()
	switch in {
	case "", "today":
		return now.Format("02.01.2006"), nil
	case "yesterday":
		return now.AddDate(0, 0, -1).Format("02.01.2006"), nil
	}
	if strings.HasPrefix(in, "-") {
		var days int
		if _, err := fmt.Sscanf(in, "-%d", &days); err == nil {
			return now.AddDate(0, 0, -days).Format("02.01.2006"), nil
		}
	}
	if t, err := time.Parse("02.01.2006", in); err == nil {
		return t.Format("02.01.2006"), nil
	}
	if t, err := time.Parse("2006-01-02", in); err == nil {
		return t.Format("02.01.2006"), nil
	}
	return "", fmt.Errorf("unrecognized date %q (try YYYY-MM-DD, DD.MM.YYYY, today, yesterday, or -N)", in)
}

// ddmmToISO converts API "01.05.2026" or epoch ms to ISO YYYY-MM-DD for output.
func ddmmToISO(in string) string {
	if t, err := time.Parse("02.01.2006", in); err == nil {
		return t.Format("2006-01-02")
	}
	return in
}

// mealSlotID maps English meal aliases to the Kalorické Tabulky time GUID.
// Czech name        | English alias      | Time GUID
// Snídaně           | breakfast          | 1
// Svačina (dop.)    | morning-snack      | 2
// Oběd              | lunch              | 3
// Svačina (odp.)    | afternoon-snack    | 4
// Večeře            | dinner             | 5
// 2. svačina        | evening-snack      | 6 (when configured)
func mealSlotID(in string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "1", "breakfast", "snidane", "snídaně":
		return "1", nil
	case "2", "morning-snack", "morning_snack", "svacina-am", "svacina":
		return "2", nil
	case "3", "lunch", "obed", "oběd":
		return "3", nil
	case "4", "afternoon-snack", "afternoon_snack", "svacina-pm":
		return "4", nil
	case "5", "dinner", "vecere", "večeře":
		return "5", nil
	case "6", "evening-snack", "evening_snack":
		return "6", nil
	}
	return "", fmt.Errorf("unknown meal slot %q (use breakfast|morning-snack|lunch|afternoon-snack|dinner|evening-snack or the Czech equivalent)", in)
}

// mealSlotLabel returns the English label for a time GUID.
func mealSlotLabel(id string) string {
	switch strings.TrimSpace(id) {
	case "1":
		return "breakfast"
	case "2":
		return "morning-snack"
	case "3":
		return "lunch"
	case "4":
		return "afternoon-snack"
	case "5":
		return "dinner"
	case "6":
		return "evening-snack"
	}
	return "meal-" + id
}
