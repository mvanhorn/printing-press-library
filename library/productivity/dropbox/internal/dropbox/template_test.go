package dropbox

import (
	"testing"
	"time"
)

func TestExpandFolderTemplate(t *testing.T) {
	modified := time.Date(2019, 3, 4, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ name, want string }{{"Image.JPG", "/Photos/2019/03/04/jpg"}, {"README", "/Photos/2019/03/04/none"}} {
		got, err := ExpandFolderTemplate("/Photos/{year}/{month}/{day}/{ext}", tc.name, modified)
		if err != nil || got != tc.want {
			t.Fatalf("%q = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}
