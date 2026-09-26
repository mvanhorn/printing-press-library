package tbprofile

import (
	"regexp"
	"strings"
)

var (
	attribEndRE   = regexp.MustCompile(`(?i)(ha scritto|wrote|a écrit|schrieb[^:]*)\s*:\s*$`)
	attribStartRE = regexp.MustCompile(`(?i)^(il|on|le|am)\s`)
	origMsgRE     = regexp.MustCompile(`(?i)^-{2,}\s*(original message|messaggio originale|ursprüngliche nachricht|message d'origine)\s*-{2,}$`)
	hdrFromRE     = regexp.MustCompile(`(?i)^\*?(da|from|de|von)\s*:\*?\s`)
	hdrSentRE     = regexp.MustCompile(`(?i)^\*?(inviato|sent|data|date|envoyé|gesendet)\s*:`)
	hdrSubjectRE  = regexp.MustCompile(`(?i)^\*?(oggetto|subject|objet|betreff)\s*:`)
	ruleLineRE    = regexp.MustCompile(`^[_\-=]{5,}$`)
	inlineNameRE  = regexp.MustCompile(`(?i)^image\d+\.(png|jpe?g|gif)$|logo|facebook|linkedin|twitter|instagram|firma|signature`)
)

// StripQuoted returns only the new text of a message body: ">" quoted lines and their attribution are
// removed, and everything from an Outlook-style header block or "Original Message" separator is cut.
func StripQuoted(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	drop := make([]bool, len(lines))
	end := len(lines)
	for i := 0; i < end; i++ {
		t := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(lines[i], ">"):
			drop[i] = true
		case attribEndRE.MatchString(t):
			start := i
			if !attribStartRE.MatchString(t) && i > 0 && attribStartRE.MatchString(strings.TrimSpace(lines[i-1])) {
				start = i - 1
			}
			if !attribStartRE.MatchString(strings.TrimSpace(lines[start])) {
				continue
			}
			if nextQuoted(lines, i+1) {
				for j := start; j <= i; j++ {
					drop[j] = true
				}
			} else {
				end = start
			}
		case origMsgRE.MatchString(t), hdrFromRE.MatchString(t) && outlookBlock(lines, i):
			end = i
		}
	}
	kept := make([]string, 0, end)
	gap := false
	for i := 0; i < end; i++ {
		if drop[i] {
			gap = true
			continue
		}
		blank := strings.TrimSpace(lines[i]) == ""
		if blank && gap && len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
			continue
		}
		if !blank {
			gap = false
		}
		kept = append(kept, lines[i])
	}
	for len(kept) > 0 && (strings.TrimSpace(kept[len(kept)-1]) == "" || end < len(lines) && ruleLineRE.MatchString(strings.TrimSpace(kept[len(kept)-1]))) {
		kept = kept[:len(kept)-1]
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func nextQuoted(lines []string, from int) bool {
	for j := from; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) != "" {
			return strings.HasPrefix(lines[j], ">")
		}
	}
	return false
}

// outlookBlock needs a sent/date and a subject line under the From line so a sentence starting "Da: " is not a cut.
func outlookBlock(lines []string, from int) bool {
	sent, subject := false, false
	for j := from + 1; j < len(lines) && j <= from+6; j++ {
		t := strings.TrimSpace(lines[j])
		sent = sent || hdrSentRE.MatchString(t)
		subject = subject || hdrSubjectRE.MatchString(t)
	}
	return sent && subject
}

// LikelyInline guesses from stored metadata whether an image is a signature/logo embedded in the body.
func LikelyInline(filename, contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "image/") && inlineNameRE.MatchString(filename)
}
