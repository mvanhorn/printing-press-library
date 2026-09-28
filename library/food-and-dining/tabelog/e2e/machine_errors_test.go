package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func assertMachineDiagnostic(t *testing.T, res result, code int) {
	t.Helper()
	equal(t, res.code, code)
	var diagnostic map[string]any
	if err := json.Unmarshal(res.stderr, &diagnostic); err != nil {
		t.Fatalf("diagnostic must be one JSON object, without duplicate Cobra prose: %q", res.stderr)
	}
	equal(t, diagnostic["code"], float64(code))
	message, ok := diagnostic["error"].(string)
	if !ok || strings.TrimSpace(message) == "" {
		t.Fatal("diagnostic has no actionable message")
	}
}

func TestMachineValidationDiagnosticsHaveStableCodesWithoutHTTP(t *testing.T) {
	for _, mode := range []string{"--agent", "--json"} {
		for name, scenario := range map[string]struct {
			args []string
			code int
		}{
			"invalid-amount":        {[]string{"find", "--area", "tokyo", "--meal", "dinner", "--budget-max", "1500"}, 2},
			"missing-show-identity": {[]string{"show"}, 2},
			"missing-notebook-id":   {[]string{"lists", "add", "trip"}, 1},
		} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				r := newReplay(t, func(*http.Request) response { return response{status: 503} })
				w := newWorkspace(t, r)
				res := w.run(t, append(scenario.args, mode)...)
				assertMachineDiagnostic(t, res, scenario.code)
				equal(t, len(res.stdout), 0)
				equal(t, r.count(), 0)
			})
		}
	}
}

func TestMachineSourceFailureHasDiagnosticAndNonSuccessCode(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{status: 503, body: []byte("upstream unavailable")} })
	w := newWorkspace(t, r)
	res := w.run(t, "find", "--area", tokyoURL, "--agent")
	assertMachineDiagnostic(t, res, 5)
	equal(t, len(res.stdout), 0)
	equal(t, r.count(), 1)
}

func TestHumanValidationErrorKeepsConciseCobraText(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{status: 503} })
	w := newWorkspace(t, r)
	res := w.run(t, "find", "--area", "tokyo", "--meal", "dinner", "--budget-max", "1500")
	equal(t, res.code, 2)
	if !strings.HasPrefix(string(res.stderr), "Error: unsupported budget threshold") {
		t.Fatalf("human error was replaced with machine output: %q", res.stderr)
	}
	equal(t, len(res.stdout), 0)
	equal(t, r.count(), 0)
}

func TestMachinePartialRefreshKeepsResultsBesideDiagnostic(t *testing.T) {
	r := tripReplay(t)
	w := newWorkspace(t, r)
	fetchTripCandidates(t, w)
	mustSucceed(t, w.run(t, "show", sushiURL, "--agent"))
	for _, id := range []string{"13005012", "13294162"} {
		mustSucceed(t, w.run(t, "lists", "add", "trip", id, "--agent"))
	}
	detail := readFixture(t, "sushi-detail.html")
	r.serve(func(req *http.Request) response {
		if req.URL.Path == "/en/tokyo/A1301/A130103/13294162/" {
			return response{body: detail}
		}
		return response{status: 503, body: []byte("upstream unavailable")}
	})
	before := r.count()
	res := w.run(t, "lists", "refresh", "trip", "13005012", "13294162", "--agent")
	assertMachineDiagnostic(t, res, 1)
	if res.payload == nil {
		t.Fatal("partial refresh lost usable structured stdout")
	}
	equal(t, len(items(t, res.payload)), 2)
	equal(t, object(t, res.payload["meta"])["partial_failure"], true)
	equal(t, object(t, res.payload["meta"])["failed"], float64(1))
	equal(t, r.count()-before, 2)
}
