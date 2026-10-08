package dropbox

import "testing"

func TestDropboxPathHelpers(t *testing.T) {
	for _, tc := range []struct {
		path, parent string
		within       bool
	}{
		{"/Docs/a.txt", "/Docs", true},
		{"/Docs2/a.txt", "/Docs", false},
		{"/Fotos/Café/a.jpg", "/Fotos/Café", true},
		{"/anywhere", "", true},
	} {
		if got := PathWithin(tc.path, tc.parent); got != tc.within {
			t.Fatalf("PathWithin(%q,%q)=%t", tc.path, tc.parent, got)
		}
	}
	parent, base := ParentBase("/Fotos/Café")
	if parent != "/Fotos" || base != "Café" {
		t.Fatalf("ParentBase=%q,%q", parent, base)
	}
	if IndexRoot("/Fotos/Café/a.jpg") != "/fotos" || IndexRoot("/top.txt") != "" {
		t.Fatal("wrong index root")
	}
}
