package dropbox

import "testing"

func TestDevDirKind(t *testing.T) {
	for _, tc := range []struct {
		path, kind string
	}{
		{"/p/node_modules/a/b.js", "node_modules"},
		{"/p/node_modules", "node_modules"},
		{"/notes/node_modules_guide.txt", ""},
		{"/P/.Git/HEAD", ".git"},
		{"/x/venv/lib/site-packages/a.py", "venv"},
		{"/x/Pods/a", "Pods"},
		{"/x/DerivedData/a", "DerivedData"},
	} {
		got, ok := DevDirKind(tc.path)
		if got != tc.kind || ok != (tc.kind != "") {
			t.Errorf("DevDirKind(%q) = %q, %t; want %q", tc.path, got, ok, tc.kind)
		}
	}
}
