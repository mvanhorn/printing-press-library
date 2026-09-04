package cli

import (
	"regexp"
	"strings"
	"testing"
)

func TestHappyArgsAnnotationsWellFormed(t *testing.T) {
	root := RootCmd()
	tests := []struct {
		path []string
		want string
	}{
		{[]string{"fairness", "nudge"}, "<friend>=Example Friend"},
		{[]string{"ledger"}, "<group>=Example Group"},
		{[]string{"settle-up"}, "<group-or-friend>=Example Group"},
		{[]string{"split"}, "<group>=Example Group;--amount=84"},
	}
	tokenRE := regexp.MustCompile(`^(?:<[a-z-]+>=\S.*|--[a-z-]+(?:=.+)?)$`)
	for _, tc := range tests {
		cmd, _, err := root.Find(tc.path)
		if err != nil {
			t.Fatalf("find %v: %v", tc.path, err)
		}
		happy := cmd.Annotations["pp:happy-args"]
		if happy != tc.want {
			t.Fatalf("%v happy args=%q want %q", tc.path, happy, tc.want)
		}
		for _, token := range strings.Split(happy, ";") {
			if !tokenRE.MatchString(token) {
				t.Errorf("%v malformed token %q", tc.path, token)
			}
			if strings.HasPrefix(token, "--") {
				name := strings.TrimPrefix(strings.SplitN(token, "=", 2)[0], "--")
				if cmd.Flags().Lookup(name) == nil {
					t.Errorf("%v unknown flag --%s", tc.path, name)
				}
			} else {
				label := strings.SplitN(token, "=", 2)[0]
				if !strings.Contains(cmd.Use, label) {
					t.Errorf("%v Use %q lacks %q", tc.path, cmd.Use, label)
				}
			}
		}
		if !strings.Contains(strings.Split(cmd.Annotations["pp:typed-exit-codes"], ",")[1], "3") {
			t.Errorf("%v lacks typed exit code 3", tc.path)
		}
	}
}
