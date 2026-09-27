package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
	"strings"
	"testing"
)

func TestPublicSourceExamplesHaveSupportedExplicitInputs(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	paths := []string{"hotel get-51870.html", "hotel get-facilities-policies", "yado list-e.html", "keyword", "hotelinfo"}
	for _, path := range paths {
		cmd, remaining, err := root.Find(strings.Fields(path))
		if err != nil || len(remaining) != 0 {
			t.Fatalf("missing source route %s: %v %v", path, remaining, err)
		}
		if !strings.Contains(cmd.Example, root.Name()+" "+path+" ") {
			t.Fatalf("%s lacks an explicit example: %q", path, cmd.Example)
		}
		positional := map[string]bool{}
		for _, name := range strings.Fields(cmd.Use)[1:] {
			positional[strings.Trim(name, "<>[]")] = true
		}
		happy := cmd.Annotations["pp:happy-args"]
		if happy == "" {
			t.Fatalf("%s has no source sample inputs", path)
		}
		for _, token := range strings.Split(happy, ";") {
			parts := strings.SplitN(token, "=", 2)
			if len(parts) != 2 || parts[1] == "" {
				t.Fatalf("%s malformed sample token %q", path, token)
			}
			if strings.HasPrefix(parts[0], "--") {
				if cmd.Flags().Lookup(strings.TrimPrefix(parts[0], "--")) == nil {
					t.Fatalf("%s sample invents flag %s", path, parts[0])
				}
			} else if !positional[parts[0]] {
				t.Fatalf("%s sample invents positional %s", path, parts[0])
			}
		}
		out, _, err := runPublicTravelCLI(append(strings.Fields(path), "--help")...)
		if err != nil || !strings.Contains(out, "Examples:") || !strings.Contains(out, strings.TrimSpace(cmd.Example)) {
			t.Fatalf("%s help misses source example: %v\n%s", path, err, out)
		}
	}
	keyword, _, _ := root.Find([]string{"keyword"})
	if !strings.Contains(keyword.Annotations["pp:happy-args"], "--query=品川") || !strings.Contains(keyword.Annotations["pp:happy-args"], "--page-size=3") {
		t.Fatalf("keyword sample can issue an empty or unbounded query: %v", keyword.Annotations)
	}
	for _, name := range []string{"hotel", "yado", "keyword", "hotelinfo"} {
		cmd, _, _ := root.Find([]string{name})
		if !cmd.Hidden {
			t.Fatalf("raw %s source route became visible in root help", name)
		}
	}
}

func TestPublicAreaAndHotelParentHelpIncludesExamples(t *testing.T) {
	testenv.Isolate(t)
	for _, path := range []string{"areas", "hotels"} {
		out, _, err := runPublicTravelCLI(path, "--help")
		if err != nil || !strings.Contains(out, "Examples:") || !strings.Contains(out, "rakuten-travel-pp-cli "+path+" ") {
			t.Fatalf("%s parent help lacks examples: %v\n%s", path, err, out)
		}
	}
}
