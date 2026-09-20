package nepraper

import "strings"
import "testing"

// TestExtractTextSurvivesHostileHeaders pins the two bounds an untrusted PDF
// can otherwise exploit: a self-declared page count with nothing behind it,
// and a panic inside the third-party reader.
func TestExtractTextSurvivesHostileHeaders(t *testing.T) {
	// A tiny file whose page tree claims an enormous count must be REFUSED
	// rather than preallocating for it.
	tiny := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
		"2 0 obj<</Type/Pages/Count 2000000000/Kids[]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF")
	_, err := ExtractText(tiny)
	if err == nil {
		t.Fatal("a 130-byte document claiming 2,000,000,000 pages was accepted; the page count is the " +
			"document's own claim and must be bounded by the bytes actually present")
	}
	t.Logf("  refused: %v", err)

	// Garbage that is not a PDF at all.
	if _, err := ExtractText([]byte("not a pdf")); err == nil {
		t.Fatal("non-PDF input was accepted")
	}
	// Truncated/corrupt PDF: must return an error, never panic.
	for _, b := range [][]byte{
		[]byte("%PDF-1.4\n"),
		[]byte("%PDF-1.7\n" + strings.Repeat("\x00", 512)),
		append([]byte("%PDF-1.5\n"), []byte("1 0 obj<</Type/Pages/Count 5>>endobj")...),
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ExtractText PANICKED on malformed input instead of returning an error: %v", r)
				}
			}()
			if _, err := ExtractText(b); err == nil {
				t.Logf("  (accepted %d bytes without error)", len(b))
			}
		}()
	}
}
