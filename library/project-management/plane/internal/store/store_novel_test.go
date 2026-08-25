package store

import "testing"

// TestShouldIndexSearchString_DateClassifierRange is a regression guard for the
// isoDatePattern character class. The time portion was written `[0-9:.+-Zz]`,
// where `+-Z` is an unintended ASCII range (0x2B..0x5A) that swallows
// `, / ; < = > ? @ A-Z` and misclassifies non-date strings as dates, wrongly
// excluding them from the FTS content index. The fix moves `-` to the end of
// the class (`[0-9:.+Zz-]`) so it is a literal.
func TestShouldIndexSearchString_DateClassifierRange(t *testing.T) {
	const key = "summary" // non-identifier key so classification is reached

	// Real ISO dates/timestamps must stay OUT of the FTS content index.
	dates := []string{
		"2024-01-15",
		"2024-01-15T10:30:00Z",
		"2024-01-15 10:30:00",
		"2024-01-15T10:30:00.123+05:30",
	}
	for _, s := range dates {
		if shouldIndexSearchString(key, s) {
			t.Errorf("expected date-like %q to be excluded from FTS indexing", s)
		}
	}

	// These are NOT dates; the old `+-Z` range wrongly matched them and dropped
	// them from the index. They must now be indexed.
	nonDates := []string{
		"2024-01-15 A?/=@",   // uppercase + punctuation caught by the bad range
		"2024-01-15 PLANNED", // date-prefixed status word
		"2024-01-15/backup",  // date-prefixed slug
	}
	for _, s := range nonDates {
		if !shouldIndexSearchString(key, s) {
			t.Errorf("expected non-date %q to be indexed (regex range regression)", s)
		}
	}
}
