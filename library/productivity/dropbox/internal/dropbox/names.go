package dropbox

import (
	"regexp"
	"strings"
	"unicode"
)

var conflictedCopyRE = regexp.MustCompile(`(?i)^(.*?) \((?:(.+?)'s )?conflicted copy(?: (\d{4}-\d{2}-\d{2}))?(?: \(\d+\))?\)(\.[^./]+)?$`)
var selectiveSyncConflictRE = regexp.MustCompile(`(?i)^(.*?) \(selective sync conflict(?: \d+)?\)(\.[^./]+)?$`)
var caseConflictRE = regexp.MustCompile(`(?i)^(.*?) \(case conflict(?: \d+)?\)(\.[^./]+)?$`)
var invalidFilesRE = regexp.MustCompile(`(?i)^(.*?) \((.+?)'s invalid files\)(\.[^./]+)?$`)
var numberedNameRE = regexp.MustCompile(`(?i) \(1\)$`)
var trailingNumberRE = regexp.MustCompile(`(?i)\s+2$`)
var junkNumberRE = regexp.MustCompile(`(?i) \(\d+\)(?:\.[^./]+)?$`)

func ConflictedCopy(name string) (original, owner, date string, ok bool) {
	original, kind, owner, date, ok := ConflictMarker(name)
	if !ok || kind != "conflicted_copy" {
		return "", "", "", false
	}
	return original, owner, date, true
}

// ConflictMarker removes all trailing Dropbox conflict markers, preserving a
// file extension after the marker. The kind and metadata describe the outer
// marker, which is the copy represented by name.
func ConflictMarker(name string) (original, kind, owner, date string, ok bool) {
	original, kind, owner, date, _, ok = ConflictMarkerWithDepth(name)
	return original, kind, owner, date, ok
}

// ConflictMarkerWithDepth also reports how many markers were removed.
func ConflictMarkerWithDepth(name string) (original, kind, owner, date string, depth int, ok bool) {
	original = name
	for {
		base, nextKind, nextOwner, nextDate, found := stripConflictMarker(original)
		if !found {
			break
		}
		if depth == 0 {
			kind, owner, date = nextKind, nextOwner, nextDate
		}
		original = base
		depth++
	}
	if depth == 0 {
		return "", "", "", "", 0, false
	}
	return original, kind, owner, date, depth, true
}

func stripConflictMarker(name string) (original, kind, owner, date string, ok bool) {
	if m := conflictedCopyRE.FindStringSubmatch(name); m != nil && strings.TrimSpace(m[1]) != "" {
		return m[1] + m[4], "conflicted_copy", m[2], m[3], true
	}
	if m := selectiveSyncConflictRE.FindStringSubmatch(name); m != nil && strings.TrimSpace(m[1]) != "" {
		return m[1] + m[2], "selective_sync_conflict", "", "", true
	}
	if m := caseConflictRE.FindStringSubmatch(name); m != nil && strings.TrimSpace(m[1]) != "" {
		return m[1] + m[2], "case_conflict", "", "", true
	}
	if m := invalidFilesRE.FindStringSubmatch(name); m != nil && strings.TrimSpace(m[1]) != "" {
		return m[1] + m[3], "invalid_files", m[2], "", true
	}
	return "", "", "", "", false
}

func NormalizeFolderName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	for {
		old := s
		s = numberedNameRE.ReplaceAllString(s, "")
		s = trailingNumberRE.ReplaceAllString(s, "")
		s = strings.TrimSuffix(s, "-copy")
		s = strings.TrimSuffix(s, " copy")
		s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
		if s == old {
			return s
		}
	}
}

func JunkName(name string) (kind string, ok bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(lower, "copy of "):
		return "copy_of", true
	case strings.HasPrefix(lower, "untitled"):
		return "untitled", true
	case strings.HasPrefix(lower, "new folder"):
		return "new_folder", true
	case junkNumberRE.MatchString(lower):
		return "numbered_copy", true
	default:
		stem := lower
		if i := strings.LastIndex(stem, "."); i > 0 {
			stem = stem[:i]
		}
		if strings.HasSuffix(stem, " copy") || strings.HasSuffix(stem, "-copy") {
			return "copy", true
		}
	}
	return "", false
}
