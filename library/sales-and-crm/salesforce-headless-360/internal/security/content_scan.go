package security

import (
	"context"
	"regexp"
	"strings"
)

type ContentScanFilter struct{}

var (
	emailRE = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	ssnRE   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	phoneRE = regexp.MustCompile(`(?m)(?:\+?[1-9]\d{0,2}[\s.-]?)?(?:\(?\d{3}\)?[\s.-]?)\d{3}[\s.-]?\d{4}\b`)
	cardRE  = regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)
)

func (ContentScanFilter) Apply(_ context.Context, record *Record) *Record {
	if record == nil {
		return record
	}
	for key, value := range record.Fields {
		text, ok := value.(string)
		if !ok || !scanCandidate(key, text) {
			continue
		}
		replaced, changed := replacePII(text)
		if changed {
			record.Fields[key] = replaced
			ProvenanceCounter(record.Provenance, ReasonContentScan)
		}
	}
	return record
}

func scanCandidate(field, text string) bool {
	name := strings.ToLower(field)
	if strings.Contains(name, "description") || strings.Contains(name, "subject") ||
		strings.Contains(name, "body") || strings.Contains(name, "comment") ||
		strings.Contains(name, "note") || strings.Contains(name, "long") ||
		len(text) > 255 {
		return true
	}
	return false
}

func replacePII(input string) (string, bool) {
	changed := false
	out := emailRE.ReplaceAllStringFunc(input, func(string) string {
		changed = true
		return "{{PII:email}}"
	})
	out = ssnRE.ReplaceAllStringFunc(out, func(string) string {
		changed = true
		return "{{PII:ssn}}"
	})
	out = cardRE.ReplaceAllStringFunc(out, func(match string) string {
		if luhnValid(digitsOnly(match)) {
			changed = true
			return "{{PII:credit_card}}"
		}
		return match
	})
	out = phoneRE.ReplaceAllStringFunc(out, func(match string) string {
		digits := digitsOnly(match)
		if len(digits) < 10 || len(digits) > 15 || luhnValid(digits) {
			return match
		}
		changed = true
		return "{{PII:phone}}"
	})
	return out, changed
}

func digitsOnly(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func luhnValid(digits string) bool {
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}
