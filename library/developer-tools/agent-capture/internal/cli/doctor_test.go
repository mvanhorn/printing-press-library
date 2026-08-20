package cli

import (
	"context"
	"reflect"
	"testing"
)

func TestRemotionProbeCommandUsesVersions(t *testing.T) {
	cmd := remotionProbeCommand(context.Background(), "/usr/bin/npx")

	want := []string{"/usr/bin/npx", "remotion", "versions"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("remotion probe argv = %#v, want %#v", cmd.Args, want)
	}
}
