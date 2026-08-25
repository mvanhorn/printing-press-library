package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
)

func TestWordPressDBPathResolution(t *testing.T) {
	t.Setenv("WORDPRESS_DATA_DIR", t.TempDir())
	explicit := filepath.Join(t.TempDir(), "explicit.db")
	originalArgs := os.Args
	os.Args = []string{"wordpress-pp-cli", "sync", "--db", explicit}
	t.Cleanup(func() { os.Args = originalArgs })
	if got := wordpressDBPath(&rootFlags{}); got != explicit {
		t.Fatalf("explicit path = %q, want %q", got, explicit)
	}
	os.Args = []string{"wordpress-pp-cli"}

	configPath := filepath.Join(t.TempDir(), "config.toml")
	registry, err := config.LoadWordPressSites(configPath)
	if err != nil {
		t.Fatal(err)
	}
	registry.Active = "Client Site"
	registry.Sites["Client Site"] = config.WordPressSite{
		Name: "Client Site", Namespaces: make([]string, 0), AddedAt: time.Unix(1, 0).UTC(),
	}
	if err := config.SaveWordPressSites(registry); err != nil {
		t.Fatal(err)
	}
	perSitePath := wordpressSiteDBPath("client-site")
	if err := os.MkdirAll(filepath.Dir(perSitePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(perSitePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := wordpressDBPath(&rootFlags{configPath: configPath}), wordpressSiteDBPath("client-site"); got != want {
		t.Fatalf("active-site path = %q, want %q", got, want)
	}
	if got := wordpressSiteDBPath("other-site"); got == defaultDBPath("wordpress-pp-cli") {
		t.Fatalf("site path %q is not isolated from the generic store", got)
	}
}

func TestExplicitWordPressDBPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "separate value", args: []string{"sync", "--db", "/tmp/client.db"}, want: "/tmp/client.db"},
		{name: "equals value", args: []string{"sync", "--db=/tmp/client.db"}, want: "/tmp/client.db"},
		{name: "last wins", args: []string{"--db", "/tmp/a.db", "--db=/tmp/b.db"}, want: "/tmp/b.db"},
		{name: "missing", args: []string{"sync"}, want: ""},
		{name: "empty", args: []string{"--db="}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := explicitWordPressDBPath(tt.args); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
