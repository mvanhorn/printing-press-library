// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package shield

import (
	"strings"
	"testing"
)

func TestDetectFindsDeterministicPII(t *testing.T) {
	text := "Cathryn Lavery used cathryn@example.com, 4111 1111 1111 1111, 192.168.1.1, and 123-45-6789."
	entities := Detect(text)
	kinds := map[string]bool{}
	for _, e := range entities {
		kinds[e.Kind] = true
	}
	for _, want := range []string{"PERSON", "EMAIL", "CARD", "IP", "SSN"} {
		if !kinds[want] {
			t.Fatalf("missing %s in %#v", want, entities)
		}
	}
	if got := RiskScore(entities); got != 10 {
		t.Fatalf("RiskScore() = %d, want 10", got)
	}
}

func TestDetectLikelyNamesSkipsCommonNonPersonPhrases(t *testing.T) {
	text := "Trips to New York and San Francisco used Google Cloud logs. Alice Johnson still owns the account."
	entities := Detect(text)
	persons := map[string]bool{}
	for _, entity := range entities {
		if entity.Kind == "PERSON" {
			persons[entity.Value] = true
		}
	}
	for _, falsePositive := range []string{"New York", "San Francisco", "Google Cloud"} {
		if persons[falsePositive] {
			t.Fatalf("detected %q as PERSON in %#v", falsePositive, entities)
		}
	}
	if !persons["Alice Johnson"] {
		t.Fatalf("missing Alice Johnson PERSON detection in %#v", entities)
	}
}

func TestDetectPrefersHigherRiskOverlap(t *testing.T) {
	entities := Detect("Alice Bob@example.com")
	if len(entities) != 1 {
		t.Fatalf("entities = %#v, want one EMAIL entity", entities)
	}
	if entities[0].Kind != "EMAIL" || entities[0].Value != "Bob@example.com" {
		t.Fatalf("entity = %#v, want Bob@example.com EMAIL", entities[0])
	}
}

func TestRestructureCoarsensWithoutBucketingDates(t *testing.T) {
	input := "name,email,amount,date\nCathryn Lavery,cathryn@example.com,1234,2026-06-24\n"
	got, err := Restructure(input, RestructureOptions{BucketNumerics: true, CoarsenDates: "quarter", DropColumns: []string{"email"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "example.com") {
		t.Fatalf("email column not dropped: %s", got)
	}
	if !strings.Contains(got, "1000-9999") || !strings.Contains(got, "2026-Q2") {
		t.Fatalf("unexpected restructure output: %s", got)
	}
}
