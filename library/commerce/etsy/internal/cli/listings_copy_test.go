package cli

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestListingsCopyDryRunPrintsStrippedDraft(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	out, _, err := s2Run(t, "listings", "copy", "1000000001", "--dry-run", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	if srv.count("POST", "/shops/9000001/listings") != 0 {
		t.Fatal("default run created a listing")
	}
	var res struct {
		Mode            string         `json:"mode"`
		Draft           map[string]any `json:"draft"`
		Stripped        []string       `json:"stripped_fields"`
		MissingRequired []string       `json:"missing_required"`
	}
	s2Decode(t, out, &res)
	if res.Mode != "preview" || len(res.MissingRequired) != 0 {
		t.Fatalf("copy = %s", out)
	}
	for _, banned := range []string{"listing_id", "shop_id", "state", "url", "views", "num_favorers", "created_timestamp"} {
		if _, present := res.Draft[banned]; present {
			t.Fatalf("draft still carries %q: %s", banned, out)
		}
	}
	if res.Draft["title"] != "Hand-glazed ceramic mug" || res.Draft["price"] != 24.0 || res.Draft["quantity"] != 4.0 {
		t.Fatalf("draft values = %v", res.Draft)
	}
	if strings.Join(res.Stripped, ",") != "created_timestamp,listing_id,num_favorers,shop_id,state,url,views" {
		t.Fatalf("stripped = %v", res.Stripped)
	}
}

func TestListingsCopyYesPostsCreateDraft(t *testing.T) {
	srv := s2NewServer(t, func(w_ http.ResponseWriter, r *http.Request, body string) bool {
		if r.Method == "POST" {
			w_.WriteHeader(201)
			w_.Write([]byte(`{"listing_id":1000000099,"state":"draft"}`))
			return true
		}
		return s2ShopHandler(nil)(w_, r, body)
	})
	out, _, err := s2Run(t, "listings", "copy", "1000000001", "--title", "Hand-glazed mug, moss green", "--yes", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	var post *s2Req
	for _, r := range srv.all() {
		r := r
		if r.Method == "POST" {
			post = &r
		}
	}
	if post == nil || post.Path != "/shops/9000001/listings" {
		t.Fatalf("no createDraftListing POST: %+v", srv.all())
	}
	form, err := url.ParseQuery(post.Body)
	s2Must(t, err)
	if form.Get("title") != "Hand-glazed mug, moss green" || form.Get("price") != "24" || form.Get("tags") != "mug,ceramic,handmade" || form.Get("quantity") != "4" || form.Get("who_made") != "i_did" {
		t.Fatalf("form = %v", form)
	}
	for _, banned := range []string{"listing_id", "state", "url", "views", "shop_id"} {
		if form.Has(banned) {
			t.Fatalf("POST carried %q", banned)
		}
	}
	if !strings.Contains(out, `"mode": "created"`) || !strings.Contains(out, "1000000099") {
		t.Fatalf("output = %s", out)
	}
}

func TestListingsCopyRefusesIncompleteDraft(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	// listing 2 has no description/who_made/when_made/taxonomy_id
	out, _, err := s2Run(t, "listings", "copy", "1000000002", "--yes", "--json", "--no-cache", "--rate-limit", "1000")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "description") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "missing_required") || srv.count("POST", "/shops/9000001/listings") != 0 {
		t.Fatalf("out=%s posts=%d", out, srv.count("POST", "/shops/9000001/listings"))
	}
}

func TestListingsCopyArgErrors(t *testing.T) {
	srv := s2NewServer(t, nil)
	if _, _, err := s2Run(t, "listings", "copy", "abc"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("bad id: %v", err)
	}
	if _, _, err := s2Run(t, "listings", "copy", "--title", "x"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("missing id: %v", err)
	}
	if len(srv.all()) != 0 {
		t.Fatal("argument errors hit the API")
	}
}
