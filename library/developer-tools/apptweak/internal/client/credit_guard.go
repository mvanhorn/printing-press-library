package client

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The caller creates a private journal with one header line and pins its inode.
// Each attempt appends and syncs a reservation before network I/O.
const creditBudgetEnv = "APPTWEAK_CREDIT_BUDGET_FILE"
const creditCostsEnv = "APPTWEAK_CREDIT_COSTS_FILE"
const creditBudgetIdentityEnv = "APPTWEAK_CREDIT_BUDGET_ID"
const creditRunIDEnv = "APPTWEAK_CREDIT_RUN_ID"
const creditCostsHashEnv = "APPTWEAK_CREDIT_COSTS_SHA256"

type creditHeader struct {
	RunID string `json:"run_id"`
	Limit int64  `json:"limit"`
}
type creditRecord struct {
	Seq       int64  `json:"seq"`
	Total     int64  `json:"total"`
	Limit     int64  `json:"limit,omitempty"`
	CostsHash string `json:"costs_hash,omitempty"`
	Previous  string `json:"previous"`
	Hash      string `json:"hash"`
}

func recordHash(r creditRecord) string {
	payload := fmt.Sprintf("%d:%d:%s", r.Seq, r.Total, r.Previous)
	if r.Limit != 0 {
		payload += fmt.Sprintf(":limit:%d", r.Limit)
	}
	if r.CostsHash != "" {
		payload += ":costs:" + r.CostsHash
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func reserveAppTweakCredits(req *http.Request) error {
	budgetPath, costsPath := os.Getenv(creditBudgetEnv), os.Getenv(creditCostsEnv)
	if budgetPath == "" && costsPath == "" {
		return nil
	}
	if budgetPath == "" || costsPath == "" || os.Getenv(creditBudgetIdentityEnv) == "" || os.Getenv(creditRunIDEnv) == "" || os.Getenv(creditCostsHashEnv) == "" {
		return errors.New("AppTweak credit guard configuration incomplete")
	}
	if !filepath.IsAbs(budgetPath) || !filepath.IsAbs(costsPath) {
		return errors.New("AppTweak credit guard paths must be absolute")
	}
	costsData, err := os.ReadFile(costsPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(costsData)
	if hex.EncodeToString(sum[:]) != os.Getenv(creditCostsHashEnv) {
		return errors.New("AppTweak costs manifest changed")
	}
	var costs map[string]int64
	if err := json.Unmarshal(costsData, &costs); err != nil {
		return err
	}
	key := strings.ToUpper(req.Method) + " " + req.URL.String()
	cost, ok := costs[key]
	if !ok && req.Method == http.MethodGet && req.URL.Path == "/api/public/store/apps/keywords-rankings/current.json" {
		// A paid-keyword response supplies the organic lookup terms. The
		// manifest can preapprove only this exact bounded request shape;
		// reserve the verified five-keyword maximum before sending.
		q := req.URL.Query()
		values := q["keywords"]
		if len(values) == 1 {
			parts := strings.Split(values[0], ",")
			valid := len(parts) > 0 && len(parts) <= 5
			for _, part := range parts {
				valid = valid && strings.TrimSpace(part) != ""
			}
			if valid {
				q.Set("keywords", "*")
				candidate := *req.URL
				candidate.RawQuery = q.Encode()
				cost, ok = costs["GET "+candidate.String()]
				if ok && cost < 51 {
					return errors.New("AppTweak bounded rank price below verified maximum")
				}
			}
		}
	}
	freeBalance := ok && cost == 0 && key == "GET https://public-api.apptweak.com/api/public/apptweak/usage/credits"
	if !freeBalance && (!ok || cost <= 0) {
		return fmt.Errorf("AppTweak credit cost unknown for %s", key)
	}
	f, err := os.OpenFile(budgetPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	unlock, err := lockCreditJournal(f)
	if err != nil {
		return err
	}
	defer unlock()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(budgetPath)
	if err != nil || !info.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, pathInfo) {
		return errors.New("AppTweak credit journal path or mode changed")
	}
	if err := creditJournalPrivate(f, info); err != nil {
		return fmt.Errorf("AppTweak credit journal access rules are not private: %w", err)
	}
	identity, err := creditJournalIdentity(f)
	if err != nil || identity != os.Getenv(creditBudgetIdentityEnv) {
		return errors.New("AppTweak credit journal identity changed")
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 1<<20)
	if !scan.Scan() {
		return errors.New("empty AppTweak credit journal")
	}
	var header creditHeader
	if err := json.Unmarshal(scan.Bytes(), &header); err != nil {
		return err
	}
	if header.Limit <= 0 || header.RunID != os.Getenv(creditRunIDEnv) {
		return errors.New("AppTweak credit journal header mismatch")
	}
	var last creditRecord
	effectiveLimit := header.Limit
	effectiveCostsHash := ""
	for scan.Scan() {
		var r creditRecord
		if err := json.Unmarshal(scan.Bytes(), &r); err != nil {
			return fmt.Errorf("corrupt AppTweak credit journal: %w", err)
		}
		if r.Seq != last.Seq+1 || r.Previous != last.Hash || r.Hash != recordHash(r) {
			return errors.New("invalid AppTweak credit journal chain")
		}
		if r.Limit != 0 || r.CostsHash != "" {
			// A transition retains the same inode and hash chain and cannot
			// reclaim a prior reservation.
			if r.Total != last.Total {
				return errors.New("invalid AppTweak credit journal transition")
			}
			if r.Limit != 0 {
				if r.Limit <= effectiveLimit {
					return errors.New("invalid AppTweak credit journal limit extension")
				}
				effectiveLimit = r.Limit
			}
			if r.CostsHash != "" {
				if len(r.CostsHash) != 64 || r.CostsHash == effectiveCostsHash {
					return errors.New("invalid AppTweak credit journal manifest transition")
				}
				effectiveCostsHash = r.CostsHash
			}
		} else if r.Total <= last.Total || r.Total > effectiveLimit {
			return errors.New("invalid AppTweak credit journal reservation")
		}
		last = r
	}
	if err := scan.Err(); err != nil {
		return err
	}
	if effectiveCostsHash != "" && effectiveCostsHash != os.Getenv(creditCostsHashEnv) {
		return errors.New("AppTweak credit journal manifest hash mismatch")
	}
	if info.Size() > 0 {
		tail := make([]byte, 1)
		if _, err := f.ReadAt(tail, info.Size()-1); err != nil || tail[0] != '\n' {
			return errors.New("partial AppTweak credit journal record")
		}
	}
	if freeBalance {
		return nil
	}
	if cost > effectiveLimit-last.Total {
		return errors.New("AppTweak credit budget exhausted")
	}
	next := creditRecord{Seq: last.Seq + 1, Total: last.Total + cost, Previous: last.Hash}
	next.Hash = recordHash(next)
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, 2); err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
func ReserveAppTweakCredits(req *http.Request) error { return reserveAppTweakCredits(req) }

// CreditGuardActive reports whether any guard configuration was supplied.
func CreditGuardActive() bool {
	return os.Getenv(creditBudgetEnv) != "" || os.Getenv(creditCostsEnv) != ""
}

func GuardedHTTPClient(base *http.Client) *http.Client {
	if !CreditGuardActive() {
		return base
	}
	copy := *base
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}
