// Hand-authored tests for batch claim-file parsing. Not generated.
package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseClaimLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"blank and comment only", "\n   \n# a comment\n#another\n", nil},
		{
			"mixed",
			"# header comment\nvitamin D respiratory\n\n  statins cardiovascular  \n# trailing comment\n",
			[]string{"vitamin D respiratory", "statins cardiovascular"},
		},
		{"trims whitespace", "  spaced claim  \n", []string{"spaced claim"}},
		{"hash mid-line is not a comment", "claim # with hash\n", []string{"claim # with hash"}},
		{"no trailing newline", "only line", []string{"only line"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseClaimLines(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseClaimLines(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestReadBatchClaims(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	t.Run("single file skips blanks and comments", func(t *testing.T) {
		p := writeFile("claims.txt", "# c\nalpha\n\n  beta\n")
		got, err := readBatchClaims([]string{p})
		if err != nil {
			t.Fatalf("readBatchClaims: %v", err)
		}
		want := []string{"alpha", "beta"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("claims = %#v, want %#v", got, want)
		}
	})

	t.Run("glob expands and dedupes files", func(t *testing.T) {
		a := writeFile("a.txt", "one\n")
		_ = writeFile("b.txt", "two\n")
		// Pass the glob and one explicit path that overlaps it; the file must
		// be read once, not twice.
		got, err := readBatchClaims([]string{filepath.Join(dir, "*.txt"), a})
		if err != nil {
			t.Fatalf("readBatchClaims: %v", err)
		}
		count := map[string]int{}
		for _, c := range got {
			count[c]++
		}
		if count["one"] != 1 {
			t.Errorf("claim \"one\" appeared %d times, want 1 (dedup failed)", count["one"])
		}
		if count["two"] != 1 {
			t.Errorf("claim \"two\" appeared %d times, want 1", count["two"])
		}
	})

	t.Run("missing file is an error", func(t *testing.T) {
		if _, err := readBatchClaims([]string{filepath.Join(dir, "does-not-exist.txt")}); err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})
}
