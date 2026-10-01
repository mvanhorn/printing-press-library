package cli

import "testing"

func TestLegacyNestedCommandAliasesResolve(t *testing.T) {
	for _, tc := range []struct {
		parent string
		legacy string
		want   string
	}{
		{parent: "messages", legacy: "post_message", want: "post-message"},
		{parent: "users", legacy: "lookup_by_email", want: "lookup-by-email"},
	} {
		t.Run(tc.legacy, func(t *testing.T) {
			cmd, remaining, err := RootCmd().Find([]string{tc.parent, tc.legacy})
			if err != nil {
				t.Fatal(err)
			}
			if cmd.Name() != tc.want || len(remaining) != 0 {
				t.Fatalf("resolved %s to %s with remaining args %v", tc.legacy, cmd.Name(), remaining)
			}
		})
	}
}
