package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCategoryKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"Sans Serif", "sansserif"},
		{"sans-serif", "sansserif"},
		{"sans_serif", "sansserif"},
		{"sans serif", "sansserif"},
		{"  SANS-SERIF  ", "sansserif"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := categoryKey(tc.in); got != tc.want {
			t.Errorf("categoryKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCategoriesMatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		stored string
		query  string
		want   bool
	}{
		{name: "hyphen slug matches stored display name", stored: "Sans Serif", query: "sans-serif", want: true},
		{name: "stored display name still matches", stored: "Sans Serif", query: "Sans Serif", want: true},
		{name: "space form matches stored display name", stored: "Sans Serif", query: "sans serif", want: true},
		{name: "underscore form matches stored display name", stored: "Sans Serif", query: "sans_serif", want: true},
		{name: "unrelated category does not match", stored: "Sans Serif", query: "serif", want: false},
		{name: "display single-token still matches", stored: "Display", query: "display", want: true},
		{name: "empty query does not match", stored: "Sans Serif", query: "", want: false},
		{name: "whitespace query does not match", stored: "Sans Serif", query: "   ", want: false},
		{name: "unknown slug does not match", stored: "Sans Serif", query: "not-a-category", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := categoriesMatch(tc.stored, tc.query); got != tc.want {
				t.Fatalf("categoriesMatch(%q, %q) = %v, want %v", tc.stored, tc.query, got, tc.want)
			}
		})
	}
}

func TestFilterByCategory(t *testing.T) {
	t.Parallel()
	meta := &FontMetadata{
		FamilyMetadataList: []Font{
			{Family: "Inter", Category: "Sans Serif"},
			{Family: "Playfair Display", Category: "Serif"},
			{Family: "Press Start 2P", Category: "Display"},
		},
	}
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "hyphen slug matches fonts with Category Sans Serif", query: "sans-serif", want: []string{"Inter"}},
		{name: "stored display name still matches", query: "Sans Serif", want: []string{"Inter"}},
		{name: "space form matches", query: "sans serif", want: []string{"Inter"}},
		{name: "unrelated category does not match", query: "handwriting", want: nil},
		{name: "empty slug returns empty", query: "", want: nil},
		{name: "unknown slug returns empty", query: "not-a-category", want: nil},
		{name: "single-token display still matches", query: "display", want: []string{"Press Start 2P"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterByCategory(meta, tc.query)
			if len(got) != len(tc.want) {
				t.Fatalf("filterByCategory(%q) returned %d fonts %v, want %v", tc.query, len(got), families(got), tc.want)
			}
			for i, family := range tc.want {
				if got[i].Family != family {
					t.Fatalf("filterByCategory(%q)[%d] = %q, want %q", tc.query, i, got[i].Family, family)
				}
			}
		})
	}
}

func families(fonts []Font) []string {
	out := make([]string, len(fonts))
	for i, f := range fonts {
		out[i] = f.Family
	}
	return out
}

func TestWriteMetadataCacheReplacesSymlinkWithoutFollowingIt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, "cache", "metadata.json")
	if err := os.Mkdir(filepath.Dir(cache), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, cache); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := writeMetadataCache(cache, []byte(`{"familyMetadataList":[{}]}`)); err != nil {
		t.Fatalf("writeMetadataCache: %v", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "keep" {
		t.Fatalf("symlink target changed: data=%q err=%v", got, err)
	}
	if info, err := os.Lstat(cache); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("cache was not replaced with a regular file: info=%v err=%v", info, err)
	}
}

func TestWriteFontFileReportsCreationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "font.ttf")
	if err := writeFontFile(path, []byte("font")); err == nil {
		t.Fatal("writeFontFile unexpectedly succeeded for a missing parent")
	}
}

func TestDownloadCommandFailures(t *testing.T) {
	if endpoint := os.Getenv("GFONTS_TEST_ENDPOINT"); endpoint != "" {
		target, err := url.Parse(endpoint)
		if err != nil {
			t.Fatal(err)
		}
		http.DefaultTransport = fontTestTransport{target: target, base: http.DefaultTransport}
		cacheFile = filepath.Join(os.Getenv("GFONTS_TEST_CACHE"), "metadata.json")
		root := NewRootCommand()
		root.SetArgs([]string{"download", "Inter", "--output", os.Getenv("GFONTS_TEST_OUTPUT")})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		return
	}

	for _, scenario := range []string{"failed_read", "oversized", "failed_file"} {
		t.Run(scenario, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/metadata/fonts":
					_, _ = io.WriteString(w, `{"familyMetadataList":[{"family":"Inter","fonts":{"regular":{}}}]}`)
				case "/css2":
					_, _ = fmt.Fprintf(w, "@font-face { src: url(%s/file); font-weight: 400; font-style: normal; }", server.URL)
				case "/file":
					switch scenario {
					case "failed_read":
						w.Header().Set("Content-Length", "10")
						_, _ = io.WriteString(w, "short")
					case "oversized":
						_, _ = io.CopyN(w, fontTestBytes{}, (32<<20)+1)
					default:
						_, _ = io.WriteString(w, "font")
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			output := filepath.Join(t.TempDir(), "fonts")
			if err := os.Mkdir(output, 0700); err != nil {
				t.Fatal(err)
			}
			if scenario == "failed_file" {
				if err := os.Mkdir(filepath.Join(output, "Inter-regular.ttf"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(os.Args[0], "-test.run=^TestDownloadCommandFailures$")
			command.Env = append(os.Environ(),
				"GFONTS_TEST_ENDPOINT="+server.URL,
				"GFONTS_TEST_CACHE="+t.TempDir(),
				"GFONTS_TEST_OUTPUT="+output,
			)
			combined, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(combined), "1 download(s) failed") {
				t.Fatalf("download command err=%v output=%q, want nonzero exit with failure summary", err, combined)
			}
		})
	}
}

func TestFontOutputStemStaysWithinSelectedDirectory(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"Inter", "Inter"},
		{"Noto Sans", "Noto-Sans"},
		{"../../outside", "outside"},
		{`..\\outside`, "outside"},
		{"../", "font"},
	} {
		if got := fontOutputStem(test.input); got != test.want {
			t.Errorf("fontOutputStem(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

type fontTestTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (transport fontTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copyReq := req.Clone(req.Context())
	copyURL := *req.URL
	copyURL.Scheme, copyURL.Host = transport.target.Scheme, transport.target.Host
	copyReq.URL, copyReq.Host = &copyURL, transport.target.Host
	return transport.base.RoundTrip(copyReq)
}

type fontTestBytes struct{}

func (fontTestBytes) Read(buf []byte) (int, error) {
	for i := range buf {
		buf[i] = 'f'
	}
	return len(buf), nil
}
