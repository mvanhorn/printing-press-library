package cli

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

func TestEtsyListingCSVForm(t *testing.T) {
	for _, command := range []string{"update", "create-draft"} {
		for _, tc := range []struct{ name, tags, materials, wantTags, wantMaterials string }{
			{"csv", "a,b", "x,y", "a,b", "x,y"},
			{"legacy", `["a","b"]`, `["x","y"]`, "a,b", "x,y"},
			{"empty", "", "", "", ""},
		} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				testenv.Isolate(t, cliutil.StateDir)
				srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
					if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
						t.Errorf("Content-Type = %q", got)
					}
					return false
				})
				args := []string{"listings", command, "9000001"}
				method, path := "PATCH", "/shops/9000001/listings/1000000001"
				if command == "update" {
					args = append(args, "1000000001")
				} else {
					method, path = "POST", "/shops/9000001/listings"
					args = append(args, "--title", "Synthetic listing", "--description", "Synthetic description", "--price", "1", "--quantity", "1", "--taxonomy-id", "1", "--who-made", "i_did", "--when-made", "made_to_order")
				}
				args = append(args, "--tags", tc.tags, "--materials", tc.materials, "--no-cache", "--rate-limit", "1000", "--no-learn", "--json", "--yes")
				_, stderr, err := s2Run(t, args...)
				if err != nil {
					t.Fatalf("command: %v; stderr: %s", err, stderr)
				}
				reqs := srv.all()
				if len(reqs) != 1 {
					t.Fatalf("requests = %+v", reqs)
				}
				req := reqs[0]
				if req.Method != method || req.Path != path {
					t.Fatalf("request = %s %s", req.Method, req.Path)
				}
				fields, err := url.ParseQuery(req.Body)
				if err != nil {
					t.Fatal(err)
				}
				for key, want := range map[string]string{"tags": tc.wantTags, "materials": tc.wantMaterials} {
					got, present := fields[key]
					if !present || len(got) != 1 || got[0] != want {
						t.Errorf("%s = %q (present %v), want %q; body %s", key, got, present, want, req.Body)
					}
				}
			})
		}
	}
}

func TestEtsyCSVFieldValue(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		wantErr   bool
	}{
		{"a,b", "a,b", false},
		{" a , b ,, c ", "a,b,c", false},
		{`["a","b"]`, "a,b", false},
		{`["a",1]`, "", true},
		{"", "", false},
		{`["a",null]`, "", true},
		{`[`, "", true},
		{`[]`, "", false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := etsyCSVFieldValue(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid JSON array of strings") {
				t.Fatalf("error = %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
