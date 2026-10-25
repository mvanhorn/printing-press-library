//go:build !windows

package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func TestCreditGuardSharedBudgetAndUnknown(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "budget.json")
	costs := filepath.Join(dir, "costs.json")
	if err := os.WriteFile(budget, []byte(`{"run_id":"test","limit":2}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(costs, []byte(`{"GET https://public-api.apptweak.com/known":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(budget)
	st := info.Sys().(*syscall.Stat_t)
	t.Setenv(creditRunIDEnv, "test")
	t.Setenv(creditBudgetIdentityEnv, fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	t.Setenv(creditBudgetEnv, budget)
	costData, _ := os.ReadFile(costs)
	sum := sha256.Sum256(costData)
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(sum[:]))
	t.Setenv(creditCostsEnv, costs)
	req, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known", nil)
	var wg sync.WaitGroup
	results := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- reserveAppTweakCredits(req) }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 2 {
		t.Fatalf("success=%d want 2", success)
	}
	unknown, _ := http.NewRequest("GET", "https://public-api.apptweak.com/unknown", nil)
	if err := reserveAppTweakCredits(unknown); err == nil {
		t.Fatal("unknown endpoint was allowed")
	}
	data, err := os.ReadFile(budget)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"total":2`) {
		t.Fatalf("journal missing total 2: %s", data)
	}
}

func TestCreditGuardSubprocessSharesBudget(t *testing.T) {
	if os.Getenv("APPTWEAK_GUARD_CHILD") == "1" {
		req, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known", nil)
		if err := reserveAppTweakCredits(req); err != nil {
			t.Fatal(err)
		}
		return
	}
	dir := t.TempDir()
	budget := filepath.Join(dir, "budget.json")
	costs := filepath.Join(dir, "costs.json")
	if err := os.WriteFile(budget, []byte(`{"run_id":"test","limit":1}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(costs, []byte(`{"GET https://public-api.apptweak.com/known":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(budget)
	st := info.Sys().(*syscall.Stat_t)
	identity := fmt.Sprintf("%d:%d", st.Dev, st.Ino)
	costData, _ := os.ReadFile(costs)
	costSum := sha256.Sum256(costData)
	costHash := hex.EncodeToString(costSum[:])
	run := func() error {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCreditGuardSubprocessSharesBudget$")
		cmd.Env = append(os.Environ(), creditBudgetEnv+"="+budget, creditCostsEnv+"="+costs, creditBudgetIdentityEnv+"="+identity, creditRunIDEnv+"=test", creditCostsHashEnv+"="+costHash, "APPTWEAK_GUARD_CHILD=1")
		return cmd.Run()
	}
	if err := run(); err != nil {
		t.Fatalf("first child: %v", err)
	}
	if err := run(); err == nil {
		t.Fatal("second child exceeded shared budget")
	}
}

func TestCreditGuardRejectsPartialAndReset(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "journal")
	costs := filepath.Join(dir, "costs")
	if err := os.WriteFile(budget, []byte("{\"run_id\":\"test\",\"limit\":2}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(costs, []byte(`{"GET https://public-api.apptweak.com/known":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(budget)
	st := info.Sys().(*syscall.Stat_t)
	t.Setenv(creditBudgetEnv, budget)
	costData, _ := os.ReadFile(costs)
	sum := sha256.Sum256(costData)
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(sum[:]))
	t.Setenv(creditCostsEnv, costs)
	t.Setenv(creditRunIDEnv, "test")
	t.Setenv(creditBudgetIdentityEnv, fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	req, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known", nil)
	if err := reserveAppTweakCredits(req); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(budget, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"seq":2`)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := reserveAppTweakCredits(req); err == nil {
		t.Fatal("partial journal accepted")
	}
	if err := os.WriteFile(budget, []byte("{\"run_id\":\"test\",\"limit\":2}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// An owner rewrite of the same inode is outside the guard's trust boundary.
}

func TestCreditGuardRejectsQueryVariationAndReplacement(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "journal")
	costs := filepath.Join(dir, "costs")
	if err := os.WriteFile(budget, []byte("{\"run_id\":\"test\",\"limit\":2}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(costs, []byte(`{"GET https://public-api.apptweak.com/known?days=1":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(budget)
	st := info.Sys().(*syscall.Stat_t)
	t.Setenv(creditBudgetEnv, budget)
	costData, _ := os.ReadFile(costs)
	sum := sha256.Sum256(costData)
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(sum[:]))
	t.Setenv(creditCostsEnv, costs)
	t.Setenv(creditRunIDEnv, "test")
	t.Setenv(creditBudgetIdentityEnv, fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	changed, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known?days=2", nil)
	if err := reserveAppTweakCredits(changed); err == nil {
		t.Fatal("query variation accepted")
	}
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(replacement, []byte("{\"run_id\":\"test\",\"limit\":2}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, budget); err != nil {
		t.Fatal(err)
	}
	known, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known?days=1", nil)
	if err := reserveAppTweakCredits(known); err == nil {
		t.Fatal("replaced budget accepted")
	}
}

func TestCreditGuardStopsRedirectHop(t *testing.T) {
	t.Setenv(creditBudgetEnv, "/tmp/enabled-for-redirect-test")
	var second int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/first" {
			http.Redirect(w, r, "/second", http.StatusFound)
			return
		}
		second++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	resp, err := GuardedHTTPClient(server.Client()).Get(server.URL + "/first")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || second != 0 {
		t.Fatalf("redirect followed: status=%d second=%d", resp.StatusCode, second)
	}
}

func TestCreditGuardLimitExtensionPreservesReservations(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "journal")
	costs := filepath.Join(dir, "costs")
	if err := os.WriteFile(budget, []byte("{\"run_id\":\"test\",\"limit\":2}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(costs, []byte(`{"GET https://public-api.apptweak.com/known":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(budget)
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	t.Setenv(creditBudgetEnv, budget)
	t.Setenv(creditCostsEnv, costs)
	t.Setenv(creditBudgetIdentityEnv, fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	t.Setenv(creditRunIDEnv, "test")
	costData, _ := os.ReadFile(costs)
	sum := sha256.Sum256(costData)
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(sum[:]))
	req, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known", nil)
	for i := 0; i < 2; i++ {
		if err := reserveAppTweakCredits(req); err != nil {
			t.Fatal(err)
		}
	}
	if err := reserveAppTweakCredits(req); err == nil {
		t.Fatal("original limit exceeded")
	}
	// Model an operator-approved append while retaining the original inode and
	// hash chain. The extension consumes no credits and cannot lower the total.
	r1 := creditRecord{Seq: 1, Total: 1}
	r1.Hash = recordHash(r1)
	r2 := creditRecord{Seq: 2, Total: 2, Previous: r1.Hash}
	r2.Hash = recordHash(r2)
	ext := creditRecord{Seq: 3, Total: 2, Limit: 4, CostsHash: hex.EncodeToString(sum[:]), Previous: r2.Hash}
	ext.Hash = recordHash(ext)
	b, _ := json.Marshal(ext)
	f, err := os.OpenFile(budget, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := reserveAppTweakCredits(req); err != nil {
			t.Fatal(err)
		}
	}
	if err := reserveAppTweakCredits(req); err == nil {
		t.Fatal("extended limit exceeded")
	}
	changedCosts := filepath.Join(dir, "changed-costs")
	changedData := []byte(`{"GET https://public-api.apptweak.com/known":1,"GET https://public-api.apptweak.com/other":1}`)
	if err := os.WriteFile(changedCosts, changedData, 0600); err != nil {
		t.Fatal(err)
	}
	changedHash := sha256.Sum256(changedData)
	t.Setenv(creditCostsEnv, changedCosts)
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(changedHash[:]))
	if err := reserveAppTweakCredits(req); err == nil || !strings.Contains(err.Error(), "journal manifest hash mismatch") {
		t.Fatalf("unrecorded manifest transition accepted: %v", err)
	}
	data, err := os.ReadFile(budget)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"total":4`) || !strings.Contains(string(data), `"limit":4`) {
		t.Fatalf("extension or total missing: %s", data)
	}
}

func TestCreditGuardBoundedResponseKeywords(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "journal")
	costs := filepath.Join(dir, "costs")
	if err := os.WriteFile(budget, []byte("{\"run_id\":\"test\",\"limit\":51}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pattern := "GET https://public-api.apptweak.com/api/public/store/apps/keywords-rankings/current.json?apps=544007664&country=us&device=iphone&keywords=%2A&metrics=rank"
	costData, _ := json.Marshal(map[string]int{pattern: 51})
	if err := os.WriteFile(costs, costData, 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(budget)
	st := info.Sys().(*syscall.Stat_t)
	t.Setenv(creditBudgetEnv, budget)
	t.Setenv(creditCostsEnv, costs)
	t.Setenv(creditBudgetIdentityEnv, fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	t.Setenv(creditRunIDEnv, "test")
	sum := sha256.Sum256(costData)
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(sum[:]))
	base := "https://public-api.apptweak.com/api/public/store/apps/keywords-rankings/current.json?apps=544007664&country=us&device=iphone&metrics=rank&keywords="
	for _, suffix := range []string{"a%2Cb%2Cc%2Cd%2Ce%2Cf", "a&limit=999", "", "a%2C%20"} {
		req, _ := http.NewRequest("GET", base+suffix, nil)
		if err := reserveAppTweakCredits(req); err == nil {
			t.Fatalf("unbounded or changed shape accepted: %s", suffix)
		}
	}
	req, _ := http.NewRequest("GET", base+"a%2Cb", nil)
	if err := reserveAppTweakCredits(req); err != nil {
		t.Fatal(err)
	}
	if err := reserveAppTweakCredits(req); err == nil {
		t.Fatal("second wildcard request exceeded conservative 51-credit reservation")
	}
}

func TestCreditGuardRejectsPublicUnixJournal(t *testing.T) {
	dir := t.TempDir()
	budget, costs := filepath.Join(dir, "journal"), filepath.Join(dir, "costs")
	if err := os.WriteFile(budget, []byte("{\"run_id\":\"test\",\"limit\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(budget, 0644); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"GET https://public-api.apptweak.com/known":1}`)
	if err := os.WriteFile(costs, data, 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(budget)
	st := info.Sys().(*syscall.Stat_t)
	hash := sha256.Sum256(data)
	t.Setenv(creditBudgetEnv, budget)
	t.Setenv(creditCostsEnv, costs)
	t.Setenv(creditBudgetIdentityEnv, fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	t.Setenv(creditRunIDEnv, "test")
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(hash[:]))
	req, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known", nil)
	if err := reserveAppTweakCredits(req); err == nil || !strings.Contains(err.Error(), "not private") {
		t.Fatalf("public journal accepted: %v", err)
	}
}

func TestCreditGuardRejectsSymlinkAndMissingJournal(t *testing.T) {
	dir := t.TempDir()
	budget, link, costs := filepath.Join(dir, "journal"), filepath.Join(dir, "link"), filepath.Join(dir, "costs")
	if err := os.WriteFile(budget, []byte("{\"run_id\":\"test\",\"limit\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(budget, link); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"GET https://public-api.apptweak.com/known":1}`)
	if err := os.WriteFile(costs, data, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(budget)
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	hash := sha256.Sum256(data)
	t.Setenv(creditBudgetEnv, link)
	t.Setenv(creditCostsEnv, costs)
	t.Setenv(creditBudgetIdentityEnv, fmt.Sprintf("%d:%d", st.Dev, st.Ino))
	t.Setenv(creditRunIDEnv, "test")
	t.Setenv(creditCostsHashEnv, hex.EncodeToString(hash[:]))
	req, _ := http.NewRequest("GET", "https://public-api.apptweak.com/known", nil)
	if err := reserveAppTweakCredits(req); err == nil {
		t.Fatal("symlink journal accepted")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := reserveAppTweakCredits(req); err == nil {
		t.Fatal("missing journal accepted")
	}
}
