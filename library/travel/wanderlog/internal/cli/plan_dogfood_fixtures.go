// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// These are runner inputs, not command defaults. They neither create fixtures
// nor alter execution/validation; the live runner still calls the real API.
// Boolean flags stay in Examples/runner dry-run injection: v4.31.1 expands
// boolean annotation values to a stray positional "true".
func applyWanderlogDogfoodFixtures(root *cobra.Command) {
	configureWanderlogDogfoodFixtures(root, os.Getenv, time.Now().UTC())
}

func configureWanderlogDogfoodFixtures(root *cobra.Command, getenv func(string) string, now time.Time) {
	set := func(path string, values ...string) {
		current := root
		for _, part := range strings.Fields(path) {
			var child *cobra.Command
			for _, candidate := range current.Commands() {
				if candidate.Name() == part {
					child = candidate
					break
				}
			}
			if child == nil {
				return
			}
			current = child
		}
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		for i := range values {
			values[i] = strings.ReplaceAll(values[i], ";", `\;`)
		}
		current.Annotations["pp:happy-args"] = strings.Join(values, ";")
	}
	// Calendar-relative dates prevent archived examples becoming past stays.
	// Never assign these dates to the actual search flags.
	set("lodging search", "--start-date="+now.AddDate(0, 0, 30).Format("2006-01-02"), "--end-date="+now.AddDate(0, 0, 37).Format("2006-01-02"))
	key := strings.TrimSpace(getenv("WANDERLOG_DOGFOOD_PLAN_KEY"))
	if key == "" {
		return
	}
	if _, err := validateResolvedPlanKey(key); err != nil {
		return
	}
	target := "--target-key=" + key
	mutator := func(path string, args ...string) {
		args = append([]string{target}, args...)
		set(path, args...)
	}
	set("plan overview", target)
	set("plan days", target, "--days=1,2")
	if file := strings.TrimSpace(getenv("WANDERLOG_DOGFOOD_BLOCKS_FILE")); file != "" {
		mutator("plan block add-batch", "--blocks-file="+file)
	}
	mutator("plan block attachment remove", "--day=1", "--block-index=0", "--attachment-index=0")
	mutator("plan block rename", "--day=1", "--block-index=1", "--name=Fixture place label")
	mutator("plan budget expense edit", "--expense-index=0", "--description=Fixture expense preview")
	mutator("plan budget expense remove", "--expense-index=0")
	mutator("plan budget payment remove", "--payment-index=0")
	mutator("plan checklist item add", "--day=1", "--block-index=2", "--text=Fixture checklist preview")
	mutator("plan checklist item check", "--day=1", "--block-index=2", "--item-index=0")
	mutator("plan checklist item remove", "--day=1", "--block-index=2", "--item-index=0")
	mutator("plan place replace", "--day=1", "--block-index=1", "--place-id=ChIJLU7jZClu5kcR4PcOOO6p3I0")
	mutator("plan reservation edit", "--day=2", "--block-index=0", "--kind=flight", "--field=confirmationNumber", "--value=FIXTURE-PREVIEW")
	mutator("plan reservation remove", "--day=2", "--block-index=0", "--kind=flight")
	mutator("plan section delete", "--day=8")
	if id, err := strconv.Atoi(strings.TrimSpace(getenv("WANDERLOG_DOGFOOD_NOTE_BLOCK_ID"))); err == nil && id > 0 {
		set("plan block get", target, "--block-id="+strconv.Itoa(id))
	}
	if file := strings.TrimSpace(getenv("WANDERLOG_DOGFOOD_CHANGES_FILE")); file != "" {
		mutator("plan edit", "--changes-file="+file)
	}
}
