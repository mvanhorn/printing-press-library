package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func tripReplay(t *testing.T) *replay {
	listing, home, detail := readFixture(t, "ginza-bars-unknown.html"), readFixture(t, "english-home.html"), readFixture(t, "sushi-detail.html")
	return newReplay(t, func(req *http.Request) response {
		if req.URL.Path == "/en/" {
			return response{body: home}
		}
		if strings.Contains(req.URL.Path, "rstLst") {
			return response{body: listing}
		}
		if req.URL.Path == "/en/tokyo/A1301/A130103/13294162/" {
			return response{body: detail}
		}
		return response{status: 404, body: []byte("unexpected trip request")}
	})
}

func fetchTripCandidates(t *testing.T, w *workspace) {
	t.Helper()
	mustSucceed(t, w.run(t, "find", "--area", ginzaURL, "--cuisine", "bar", "--meal", "dinner", "--budget-max", "5000", "--agent"))
}

func TestTripNotebookSurvivesProcessRestartsWithNotesAndOfflineComparison(t *testing.T) {
	r := tripReplay(t)
	w := newWorkspace(t, r)
	fetchTripCandidates(t, w)
	requests := r.count()
	for _, c := range []struct{ id, note string }{{"13005012", "Anchor for our Ginza evening"}, {"13002248", "Backup before dinner"}, {"13060180", "Larger group option"}} {
		mustSucceed(t, w.run(t, "lists", "add", "japan", c.id, "--note", c.note, "--agent"))
	}
	p := mustSucceed(t, w.run(t, "lists", "show", "japan", "--agent"))
	values := items(t, p)
	equal(t, len(values), 3)
	equal(t, values[0]["id"], "13005012")
	equal(t, values[1]["id"], "13002248")
	anchor := restaurant(t, values, "13005012")
	equal(t, anchor["note"], "Anchor for our Ginza evening")
	equal(t, anchor["rating"], 3.79)
	assertBudget(t, anchor, "dinner_budget", "JPY 3,000 - JPY 3,999", float64(3000), float64(3999))
	if anchor["fetched_at"] == nil || anchor["source_surface"] != "listing" || anchor["age_seconds"] == nil {
		t.Fatal("saved freshness/provenance fields missing")
	}
	names := mustSucceed(t, w.run(t, "lists", "show", "--agent"))
	if !bytes.Contains(mustJSON(names), []byte("japan")) {
		t.Fatal("lists show omitted notebook")
	}
	mustSucceed(t, w.run(t, "lists", "note", "japan", "13002248", "--note", "Backup after train arrival", "--agent"))
	comparison := mustSucceed(t, w.run(t, "lists", "compare", "japan", "13005012", "13002248", "--agent"))
	equal(t, len(items(t, comparison)), 2)
	equal(t, restaurant(t, items(t, comparison), "13002248")["note"], "Backup after train arrival")
	equal(t, restaurant(t, items(t, comparison), "13005012")["fetched_at"], anchor["fetched_at"])
	equal(t, r.count(), requests)
	mustSucceed(t, w.run(t, "lists", "remove", "japan", "13060180", "--agent"))
	after := mustSucceed(t, w.run(t, "lists", "show", "japan", "--data-source", "local", "--agent"))
	equal(t, len(items(t, after)), 2)
	mustFail(t, w.run(t, "lists", "compare", "japan", "13060180", "--agent"))
	equal(t, r.count(), requests)
}

func mustJSON(value any) []byte { b, _ := json.Marshal(value); return b }

func TestSavedAlternativesEchoConstraintsAndDistinguishUnknownCandidates(t *testing.T) {
	r := tripReplay(t)
	w := newWorkspace(t, r)
	fetchTripCandidates(t, w)
	requests := r.count()
	for _, id := range []string{"13005012", "13002248", "13060180", "13120361", "13224720"} {
		mustSucceed(t, w.run(t, "lists", "add", "backups", id, "--note", "saved "+id, "--agent"))
	}
	p := mustSucceed(t, w.run(t, "lists", "alternatives", "backups", "--for", "13005012", "--match", "area,category", "--agent"))
	values := items(t, p)
	equal(t, len(values), 2)
	equal(t, values[0]["id"], "13002248")
	equal(t, values[1]["id"], "13224720")
	criteria := object(t, object(t, p["meta"])["criteria"])
	equal(t, criteria["matched"], float64(2))
	equal(t, criteria["unmatched"], float64(1))
	equal(t, criteria["unevaluable"], float64(1))
	for _, value := range values {
		if value["id"] == "13005012" {
			t.Fatal("anchor appeared as alternative")
		}
		if len(value["reasons"].([]any)) == 0 {
			t.Fatal("alternative match evidence missing")
		}
	}
	bounded := mustSucceed(t, w.run(t, "lists", "alternatives", "backups", "--for", "13005012", "--match", "area,category", "--meal", "dinner", "--budget-max", "5000", "--agent"))
	boundedItems := items(t, bounded)
	equal(t, len(boundedItems), 1)
	equal(t, boundedItems[0]["id"], "13002248")
	criteria = object(t, object(t, bounded["meta"])["criteria"])
	equal(t, criteria["matched"], float64(1))
	equal(t, criteria["unmatched"], float64(1))
	equal(t, criteria["unevaluable"], float64(2))
	if criteria["budget_scope"] == nil {
		t.Fatal("cached bracket scope missing")
	}
	areaOnly := mustSucceed(t, w.run(t, "lists", "alternatives", "backups", "--for", "13005012", "--match", "area", "--agent"))
	equal(t, len(items(t, areaOnly)), 4)
	equal(t, r.count(), requests)
}

func findings(t *testing.T, item map[string]any) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, raw := range item["findings"].([]any) {
		f := object(t, raw)
		field := f["field"].(string)
		out[field] = append(out[field], f["code"].(string))
	}
	return out
}
func hasCode(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

func TestAuditSeparatesUnfetchedUnknownAndOldEvidenceWithoutHTTP(t *testing.T) {
	r := tripReplay(t)
	w := newWorkspace(t, r)
	fetchTripCandidates(t, w)
	for _, id := range []string{"13005012", "13224720"} {
		mustSucceed(t, w.run(t, "lists", "add", "audit", id, "--agent"))
	}
	requests := r.count()
	p := mustSucceed(t, w.run(t, "lists", "audit", "audit", "--require", "hours,dinner_budget", "--max-age", "0", "--agent"))
	anchor := findings(t, restaurant(t, items(t, p), "13005012"))
	if !hasCode(anchor["hours"], "detail_not_fetched") {
		t.Fatal("listing hours incorrectly became source unknown/known")
	}
	unknown := findings(t, restaurant(t, items(t, p), "13224720"))
	if !hasCode(unknown["dinner_budget"], "source_unknown") {
		t.Fatal("explicit missing dinner price did not remain source_unknown")
	}
	for _, codes := range unknown {
		if hasCode(codes, "older_than_threshold") {
			t.Fatal("max-age0 did not disable age findings")
		}
	}
	old := mustSucceed(t, w.run(t, "lists", "audit", "audit", "--require", "hours", "--max-age", "1ns", "--agent"))
	if !bytes.Contains(mustJSON(old), []byte("older_than_threshold")) {
		t.Fatal("old snapshot warning missing")
	}
	equal(t, r.count(), requests)
}

func TestRefreshPartialFailureRetainsNotesAndPriorSnapshots(t *testing.T) {
	r := tripReplay(t)
	w := newWorkspace(t, r)
	fetchTripCandidates(t, w)
	mustSucceed(t, w.run(t, "show", sushiURL, "--agent"))
	for _, id := range []string{"13294162", "13005012"} {
		mustSucceed(t, w.run(t, "lists", "add", "refresh", id, "--note", "keep note "+id, "--agent"))
	}
	before := mustSucceed(t, w.run(t, "lists", "show", "refresh", "--agent"))
	prior := restaurant(t, items(t, before), "13005012")
	updated := readFixture(t, "sushi-detail.html")
	updated = bytes.ReplaceAll(updated, []byte("3.47"), []byte("3.60"))
	updated = bytes.ReplaceAll(updated, []byte("JPY 10,000 - JPY 14,999"), []byte("JPY 20,000 - JPY 29,999"))
	// Source-current payment evidence becomes unknown; a successful refresh
	// must not carry the previous accepted-card text forward as a current fact.
	payment := regexp.MustCompile(`(?s)(<th>\s*Payment methods\s*</th>\s*<td[^>]*>).*?(</td>)`)
	updated = payment.ReplaceAll(updated, []byte("${1}-${2}"))
	r.serve(func(req *http.Request) response {
		if req.URL.Path == "/en/tokyo/A1301/A130103/13294162/" {
			return response{body: updated}
		}
		return response{status: 503, body: []byte("deliberate partial refresh failure")}
	})
	requests := r.count()
	res := w.run(t, "lists", "refresh", "refresh", "13294162", "13294162", "13005012", "--agent")
	mustFail(t, res)
	if res.payload == nil {
		t.Fatal("partial refresh did not return usable JSON results")
	}
	meta := object(t, res.payload["meta"])
	equal(t, meta["partial_failure"], true)
	equal(t, meta["failed"], float64(1))
	equal(t, r.count()-requests, 2)
	refreshed := restaurant(t, items(t, res.payload), "13294162")
	if len(refreshed["changes"].([]any)) == 0 {
		t.Fatal("same-field refresh changes missing")
	}
	after := mustSucceed(t, w.run(t, "lists", "show", "refresh", "--data-source", "local", "--agent"))
	values := items(t, after)
	failed := restaurant(t, values, "13005012")
	if !reflect.DeepEqual(stableSaved(failed), stableSaved(prior)) {
		t.Fatal("failed refresh changed prior snapshot, timestamps, or note")
	}
	current := restaurant(t, values, "13294162")
	equal(t, current["rating"], 3.6)
	equal(t, current["note"], "keep note 13294162")
	assertBudget(t, current, "dinner_budget", "JPY 20,000 - JPY 29,999", float64(20000), float64(29999))
	state := object(t, current["evidence"])
	equal(t, state["payment"], "source_unknown")
	if current["payment"] != nil && current["payment"] != "" && current["payment"] != "-" {
		t.Fatal("old payment fact carried forward through successful unknown refresh")
	}
	equal(t, r.count()-requests, 2)
}

func TestFindAfterShowCannotDowngradeSavedDetailEvidence(t *testing.T) {
	detailBody, listing, home := readFixture(t, "samboa-detail.html"), readFixture(t, "ginza-bars.html"), readFixture(t, "english-home.html")
	r := newReplay(t, func(req *http.Request) response {
		if req.URL.Path == "/en/tokyo/A1301/A130101/13005012/" {
			return response{body: detailBody}
		}
		if req.URL.Path == "/en/" {
			return response{body: home}
		}
		return response{body: listing}
	})
	w := newWorkspace(t, r)
	detail := mustSucceed(t, w.run(t, "show", "https://tabelog.com/en/tokyo/A1301/A130101/13005012/", "--agent"))
	original := restaurant(t, items(t, detail), "13005012")
	mustSucceed(t, w.run(t, "lists", "add", "detail", "13005012", "--note", "keep detailed evidence", "--agent"))
	mustSucceed(t, w.run(t, "find", "--area", ginzaURL, "--cuisine", "bar", "--meal", "dinner", "--budget-max", "5000", "--data-source", "live", "--agent"))
	local := mustSucceed(t, w.run(t, "show", "13005012", "--data-source", "local", "--agent"))
	saved := restaurant(t, items(t, local), "13005012")
	for _, field := range []string{"source_surface", "fetched_at", "hours", "payment", "rating", "review_count"} {
		equal(t, saved[field], original[field])
	}
	notebook := mustSucceed(t, w.run(t, "lists", "show", "detail", "--agent"))
	entry := restaurant(t, items(t, notebook), "13005012")
	for _, field := range []string{"source_surface", "fetched_at", "hours", "payment", "rating", "review_count"} {
		equal(t, entry[field], original[field])
	}
	equal(t, entry["note"], "keep detailed evidence")
}

func stableSaved(item map[string]any) map[string]any {
	copy := make(map[string]any, len(item))
	for key, value := range item {
		if key != "age_seconds" && key != "stale" {
			copy[key] = value
		}
	}
	return copy
}
