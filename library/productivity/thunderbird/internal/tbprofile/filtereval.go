package tbprofile

import (
	"strconv"
	"strings"
	"time"
)

// FilterSubject is the stored view of a message that filter terms are
// evaluated against.
type FilterSubject struct {
	FromAddr       string
	FromName       string
	To             []string
	Cc             []string
	Subject        string
	Body           string
	ListID         string
	Date           time.Time
	SizeBytes      int64
	Read           bool
	Replied        bool
	Flagged        bool
	Forwarded      bool
	HasAttachments bool
}

type addrUnit struct{ addr, name string }

func (u addrUnit) candidates() []string {
	out := []string{u.addr}
	if u.name != "" {
		out = append(out, u.name, u.name+" <"+u.addr+">")
	}
	return out
}

func addrUnits(list []string) []addrUnit {
	out := make([]addrUnit, 0, len(list))
	for _, a := range list {
		out = append(out, addrUnit{addr: a})
	}
	return out
}

func textPositive(op, have, want string) (bool, bool) {
	have, want = strings.ToLower(have), strings.ToLower(want)
	switch op {
	case "contains", "doesn't contain":
		return strings.Contains(have, want), true
	case "is", "isn't":
		return have == want, true
	case "begins with":
		return strings.HasPrefix(have, want), true
	case "ends with":
		return strings.HasSuffix(have, want), true
	}
	return false, false
}

func negativeOp(op string) bool { return op == "doesn't contain" || op == "isn't" }

func evalText(op, value string, candidates []string) (bool, bool) {
	if _, ok := textPositive(op, "", ""); !ok {
		return false, false
	}
	pos := false
	for _, c := range candidates {
		if m, _ := textPositive(op, c, value); m {
			pos = true
			break
		}
	}
	if negativeOp(op) {
		return !pos, true
	}
	return pos, true
}

func evalAddrs(op, value string, units []addrUnit) (bool, bool) {
	var cands []string
	for _, u := range units {
		cands = append(cands, u.candidates()...)
	}
	return evalText(op, value, cands)
}

func parseFilterDate(v string) (time.Time, bool) {
	for _, layout := range []string{"02-Jan-2006", "2-Jan-2006", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(v), time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func evalNumber(op string, have, want int64) (bool, bool) {
	switch op {
	case "is greater than":
		return have > want, true
	case "is less than":
		return have < want, true
	case "is":
		return have == want, true
	}
	return false, false
}

// EvalTerm evaluates one filter term against a stored message. ok is false
// when the field, operator or value cannot be evaluated from stored data;
// such a term must be reported as unevaluated, never guessed.
func EvalTerm(t FilterTerm, m FilterSubject, now time.Time) (match, ok bool) {
	field := strings.ToLower(strings.Trim(strings.TrimSpace(t.Field), `"`))
	op := strings.ToLower(strings.TrimSpace(t.Op))
	from := addrUnit{addr: m.FromAddr, name: m.FromName}
	switch field {
	case "from":
		return evalAddrs(op, t.Value, []addrUnit{from})
	case "to":
		return evalAddrs(op, t.Value, addrUnits(m.To))
	case "cc":
		return evalAddrs(op, t.Value, addrUnits(m.Cc))
	case "to or cc":
		return evalAddrs(op, t.Value, addrUnits(append(append([]string{}, m.To...), m.Cc...)))
	case "subject":
		return evalText(op, t.Value, []string{m.Subject})
	case "body":
		return evalText(op, t.Value, []string{m.Body})
	case "list-id":
		return evalText(op, t.Value, []string{m.ListID})
	case "date":
		day, ok := parseFilterDate(t.Value)
		if !ok {
			return false, false
		}
		next := day.AddDate(0, 0, 1)
		d := m.Date.In(time.Local)
		switch op {
		case "is before":
			return !m.Date.IsZero() && d.Before(day), true
		case "is after":
			return !m.Date.IsZero() && !d.Before(next), true
		case "is":
			return !m.Date.IsZero() && !d.Before(day) && d.Before(next), true
		case "isn't":
			return m.Date.IsZero() || d.Before(day) || !d.Before(next), true
		}
		return false, false
	case "age in days":
		n, err := strconv.ParseInt(strings.TrimSpace(t.Value), 10, 64)
		if err != nil {
			return false, false
		}
		age := int64(now.Sub(m.Date) / (24 * time.Hour))
		match, ok := evalNumber(op, age, n)
		return ok && match && !m.Date.IsZero(), ok
	case "size":
		n, err := strconv.ParseInt(strings.TrimSpace(t.Value), 10, 64)
		if err != nil || op == "is" {
			return false, false
		}
		return evalNumber(op, m.SizeBytes/1024, n)
	case "status":
		var have bool
		switch strings.ToLower(strings.TrimSpace(t.Value)) {
		case "read":
			have = m.Read
		case "replied":
			have = m.Replied
		case "flagged":
			have = m.Flagged
		case "forwarded":
			have = m.Forwarded
		default:
			return false, false
		}
		return evalBool(op, have)
	case "has attachment status":
		var want bool
		switch strings.ToLower(strings.TrimSpace(t.Value)) {
		case "true", "yes":
			want = true
		case "false", "no":
		default:
			return false, false
		}
		match, ok := evalBool(op, m.HasAttachments == want)
		return match, ok
	}
	return false, false
}

func evalBool(op string, have bool) (bool, bool) {
	switch op {
	case "is":
		return have, true
	case "isn't":
		return !have, true
	}
	return false, false
}

// TermSupported reports whether EvalTerm can evaluate the term.
func TermSupported(t FilterTerm) bool {
	_, ok := EvalTerm(t, FilterSubject{}, time.Time{})
	return ok
}

// EvalFilter evaluates a whole condition (AND, OR or ALL). ok is false when
// any term is unsupported.
func EvalFilter(matchType string, terms []FilterTerm, m FilterSubject, now time.Time) (match, ok bool) {
	if strings.EqualFold(matchType, "ALL") {
		return true, true
	}
	or := strings.EqualFold(matchType, "OR")
	result := !or
	for _, t := range terms {
		tm, tok := EvalTerm(t, m, now)
		if !tok {
			return false, false
		}
		if or {
			result = result || tm
		} else {
			result = result && tm
		}
	}
	return result && len(terms) > 0, true
}
