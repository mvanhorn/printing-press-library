// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"

	"github.com/spf13/cobra"
)

// helpDuration finds a duration ("24 h", "15 minutes", "2 hours") in help text.
var helpDuration = regexp.MustCompile(`(?i)\b(\d+)\s*(hours?|h|minutes?|mins?)\b`)

// TestHelpTextNeverClaimsALongerCacheWindowThanTheClient: every sentence of a
// command's Short or Long that talks about caching may only name durations up
// to appstore.MaxCacheTTL (15 minutes since 06e5ce7). keyword-rank's help
// still said Search API answers are "cached up to 24 h", which told users a
// re-run within a day records nothing new.
func TestHelpTextNeverClaimsALongerCacheWindowThanTheClient(t *testing.T) {
	var walk func(c *cobra.Command)
	checked := 0
	walk = func(c *cobra.Command) {
		for _, text := range []string{c.Short, c.Long} {
			for _, sentence := range strings.Split(text, ".") {
				if !strings.Contains(strings.ToLower(sentence), "cach") {
					continue
				}
				for _, m := range helpDuration.FindAllStringSubmatch(sentence, -1) {
					n, _ := strconv.Atoi(m[1])
					unit := time.Minute
					if strings.HasPrefix(strings.ToLower(m[2]), "h") {
						unit = time.Hour
					}
					checked++
					if d := time.Duration(n) * unit; d > appstore.MaxCacheTTL {
						t.Errorf("%s help claims a %s cache window, longer than the client's %s: %q",
							c.CommandPath(), d, appstore.MaxCacheTTL, strings.Join(strings.Fields(sentence), " "))
					}
				}
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(RootCmd())
	if checked == 0 {
		t.Fatal("no help sentence names a cache window; the check matched nothing")
	}
	kr, _, err := RootCmd().Find([]string{"keyword-rank"})
	if err != nil || kr.Name() != "keyword-rank" {
		t.Fatalf("keyword-rank not found: %v", err)
	}
	want := fmt.Sprintf("%d minutes", int(appstore.MaxCacheTTL/time.Minute))
	if !strings.Contains(strings.Join(strings.Fields(kr.Long), " "), want) {
		t.Fatalf("keyword-rank help does not state the %s cache window:\n%s", want, kr.Long)
	}
}
