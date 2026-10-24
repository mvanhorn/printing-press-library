// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strconv"
	"strings"
	"testing"
)

func TestReviewDigestHelpWires(t *testing.T) {
	r := s3aRun(t, "review-digest", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "review-digest <app> [flags]") || !strings.Contains(r.stdout, "--since-version") {
		t.Fatalf("review-digest --help: exit %d\n%s", r.code, r.stdout)
	}
}

// s3aExpectedDigest recomputes the expected counts straight from the
// fixture rows, independent of the command.
func s3aExpectedDigest(storefronts map[string]bool, sinceVersion string) (total int, stars map[string]int, byVersion map[string]int, sum int) {
	stars, byVersion = s3aStarMap(), map[string]int{}
	for _, r := range s3aChatGPTReviews {
		if !storefronts[r.storefront] {
			continue
		}
		if sinceVersion != "" && (r.version == "" || s3aCompareVersions(r.version, sinceVersion) < 0) {
			continue
		}
		total++
		sum += r.rating
		stars[strconv.Itoa(r.rating)]++
		v := r.version
		if v == "" {
			v = s3aUnknownVersion
		}
		byVersion[v]++
	}
	return total, stars, byVersion, sum
}

func TestReviewDigestCountsMatchInsertedRows(t *testing.T) {
	db := s3aSeededDB(t)
	var v reviewDigestView
	s3aDecode(t, s3aRun(t, "review-digest", "6448311069", "--db", db, "--json"), &v)
	total, stars, byVersion, sum := s3aExpectedDigest(map[string]bool{"us": true}, "")
	if v.Total != total || total != 10 {
		t.Fatalf("total = %d, want %d (fixture us rows)", v.Total, total)
	}
	for k, n := range stars {
		if v.Stars[k] != n {
			t.Errorf("stars[%s] = %d, want %d", k, v.Stars[k], n)
		}
	}
	if v.AverageRating == nil || *v.AverageRating != s3aRound2(float64(sum)/float64(total)) || *v.AverageRating != 3.2 {
		t.Fatalf("averageRating = %v, want 3.2", v.AverageRating)
	}
	if len(v.ByVersion) != len(byVersion) || v.ByVersion[0].Version != "1.2026.290" || v.ByVersion[1].Version != "1.2026.280" {
		t.Fatalf("byVersion = %+v (newest first)", v.ByVersion)
	}
	for _, ver := range v.ByVersion {
		if ver.Reviews != byVersion[ver.Version] {
			t.Errorf("version %s reviews = %d, want %d", ver.Version, ver.Reviews, byVersion[ver.Version])
		}
		histTotal := 0
		for _, n := range ver.Stars {
			histTotal += n
		}
		if histTotal != ver.Reviews {
			t.Errorf("version %s histogram sums to %d, not %d", ver.Version, histTotal, ver.Reviews)
		}
	}
	if v.ByVersion[0].AverageRating != 3.0 || v.ByVersion[1].AverageRating != 3.5 {
		t.Fatalf("per-version means = %v / %v", v.ByVersion[0].AverageRating, v.ByVersion[1].AverageRating)
	}
	if len(v.ByStorefront) != 1 || v.ByStorefront[0].Storefront != "us" || v.ByStorefront[0].Reviews != 10 {
		t.Fatalf("byStorefront = %+v", v.ByStorefront)
	}
	if v.LowStarReviews != 4 {
		t.Fatalf("lowStarReviews = %d, want 4", v.LowStarReviews)
	}
	terms := []string{}
	for _, c := range v.ComplaintTerms {
		terms = append(terms, c.Term+"="+strconv.Itoa(c.Reviews))
	}
	if strings.Join(terms, ",") != "crash=3,login=3" || v.ComplaintTerms[0].Share != 0.75 {
		t.Fatalf("complaint terms = %v (stopwords and single mentions must be dropped)", terms)
	}
	if v.Note != "" {
		t.Fatalf("unexpected note %q", v.Note)
	}
}

func TestReviewDigestAllStorefrontsAndSinceVersion(t *testing.T) {
	db := s3aSeededDB(t)
	var v reviewDigestView
	s3aDecode(t, s3aRun(t, "review-digest", "6448311069", "--countries", "all", "--db", db, "--json"), &v)
	total, stars, _, _ := s3aExpectedDigest(map[string]bool{"us": true, "gb": true}, "")
	if v.Total != total || total != 14 || strings.Join(v.Storefronts, ",") != "gb,us" || v.Stars["1"] != stars["1"] {
		t.Fatalf("all storefronts: total %d storefronts %v stars %v", v.Total, v.Storefronts, v.Stars)
	}
	if last := v.ByVersion[len(v.ByVersion)-1]; last.Version != s3aUnknownVersion || last.Reviews != 1 {
		t.Fatalf("unknown version bucket = %+v", last)
	}
	if v.ComplaintTerms[0].Term != "crash" || v.ComplaintTerms[0].Reviews != 4 {
		t.Fatalf("terms = %+v", v.ComplaintTerms)
	}

	v = reviewDigestView{}
	s3aDecode(t, s3aRun(t, "review-digest", "6448311069", "--countries", "us,gb", "--since-version", "1.2026.290", "--db", db, "--json"), &v)
	total, _, byVersion, _ := s3aExpectedDigest(map[string]bool{"us": true, "gb": true}, "1.2026.290")
	if v.Total != total || total != 9 || len(v.ByVersion) != 1 || v.ByVersion[0].Reviews != byVersion["1.2026.290"] {
		t.Fatalf("--since-version: total %d byVersion %+v", v.Total, v.ByVersion)
	}
	if v.ExcludedUnknownVersion != 1 || !strings.Contains(v.Note, "without a version were left out") {
		t.Fatalf("unknown-version accounting = %d %q", v.ExcludedUnknownVersion, v.Note)
	}

	v = reviewDigestView{}
	s3aDecode(t, s3aRun(t, "review-digest", "6448311069", "--since-version", "2.0", "--db", db, "--json"), &v)
	if v.Total != 0 || v.AverageRating != nil || !strings.Contains(v.Note, "none of the 10 stored reviews") {
		t.Fatalf("future version = %+v", v)
	}
	v = reviewDigestView{}
	s3aDecode(t, s3aRun(t, "review-digest", "6448311069", "--terms", "1", "--db", db, "--json"), &v)
	if len(v.ComplaintTerms) != 1 {
		t.Fatalf("--terms 1 = %+v", v.ComplaintTerms)
	}
	// A store URL selects its storefront unless --countries is given.
	v = reviewDigestView{}
	s3aDecode(t, s3aRun(t, "review-digest", "https://apps.apple.com/gb/app/chatgpt/id6448311069", "--db", db, "--json"), &v)
	if v.Total != 4 || strings.Join(v.Storefronts, ",") != "gb" {
		t.Fatalf("URL storefront digest = %d reviews in %v, want 4 in gb", v.Total, v.Storefronts)
	}
}

func TestReviewDigestEmptyAndErrors(t *testing.T) {
	r := s3aRun(t, "review-digest", "6448311069", "--db", t.TempDir()+"/absent.db", "--json")
	var v reviewDigestView
	s3aDecode(t, r, &v)
	if v.Total != 0 || v.AverageRating != nil || len(v.ByVersion) != 0 || len(v.ComplaintTerms) != 0 || !strings.Contains(v.Note, "no local reviews") {
		t.Fatalf("missing mirror = %+v", v)
	}
	for _, want := range []string{`"byVersion": []`, `"complaintTerms": []`, `"averageRating": null`, `"5": 0`} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("empty digest JSON missing %s:\n%s", want, r.stdout)
		}
	}
	v = reviewDigestView{}
	s3aDecode(t, s3aRun(t, "review-digest", "324684580", "--db", s3aSeededDB(t), "--json"), &v)
	if v.Total != 0 || !strings.Contains(v.Note, "no stored reviews of app 324684580 in us") {
		t.Fatalf("app without reviews = %+v", v)
	}
	for _, args := range [][]string{
		{"review-digest", "--json"},
		{"review-digest", "__printing_press_invalid__"},
		{"review-digest", "6448311069", "--countries", "us,zz"},
		{"review-digest", "6448311069", "--terms", "-1"},
	} {
		if r := s3aRun(t, args...); r.code != 2 {
			t.Errorf("%v: exit %d, want 2 (%v)", args, r.code, r.err)
		}
	}
}

func TestDigestTermsAndVersions(t *testing.T) {
	got := s3aTerms("The app CRASHES after the update; please fix! Can't log in, can't log in. 2026 ads")
	if strings.Join(got, ",") != "crashes,update,fix,log,ads" {
		t.Fatalf("terms = %v", got)
	}
	cases := []struct {
		a, b string
		want int
	}{
		{"9.1.10", "9.1.9", 1}, {"1.0", "1.0.0", 0}, {"1.2026.280", "1.2026.290", -1}, {"2", "10", -1}, {"1.0b", "1.0a", 1},
	}
	for _, c := range cases {
		if got := s3aCompareVersions(c.a, c.b); got != c.want {
			t.Errorf("compare(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
