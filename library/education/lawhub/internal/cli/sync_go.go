package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
)

type historyResp struct {
	Items       []historyItem `json:"items"`
	RecordCount int           `json:"recordCount"`
}
type historyItem struct {
	ModuleID, ModuleName, TestInstanceID, StartDate, EndDate, ModuleType string
	ExamMode, IsCompleted                                                bool
	SectionResults                                                       []sectionResult `json:"sectionResults"`
	TotalScore, ConvertedScore                                           int
}
type sectionResult struct {
	SectionID, SectionName                           string
	TotalCorrect, TotalAnswered, TotalQuestions      int
	StartTime, EndTime                               string
	IsCompleted, IsVariableSection, IsSectionExposed bool
}
type reportRow struct {
	SectionIndex     int    `json:"section_index"`
	QuestionNumber   int    `json:"question_number"`
	FlaggedText      string `json:"flagged_text"`
	ChosenAnswer     string `json:"chosen_answer"`
	AnswerStatus     string `json:"answer_status"`
	CorrectAnswer    string `json:"correct_answer"`
	QuestionType     string `json:"question_type"`
	Difficulty       int    `json:"difficulty"`
	TimeText         string `json:"time_text"`
	IsCorrect        any    `json:"is_correct"`
	Flagged          int    `json:"flagged"`
	TimeSpentSeconds int    `json:"time_spent_seconds"`
}

func currentPageURL(p *rod.Page) string {
	out := ""
	func() { defer func() { _ = recover() }(); out = p.MustEval(`() => location.href`).String() }()
	return out
}

func pageLooksUnauthenticated(p *rod.Page) bool {
	href := currentPageURL(p)
	if strings.Contains(href, "auth.lawhub.org") || strings.Contains(href, "login") || strings.Contains(href, "oauth2") {
		return true
	}
	text := ""
	func() {
		defer func() { _ = recover() }()
		text = p.MustEval(`() => (document.body && document.body.innerText || '').slice(0,1000)`).String()
	}()
	low := strings.ToLower(text)
	return strings.Contains(low, "sign in / create account") || strings.Contains(low, "we can't sign you in") || strings.Contains(low, "forgot your password")
}

func waitForAuthSettled(p *rod.Page, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if pageLooksUnauthenticated(p) {
			return fmt.Errorf("LawHub session expired or unauthenticated; run `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222`")
		}
		href := currentPageURL(p)
		if strings.Contains(href, "app.lawhub.org") {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	if pageLooksUnauthenticated(p) {
		return fmt.Errorf("LawHub session expired or unauthenticated; run `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222`")
	}
	return nil
}

func shouldDisableChromeSandbox() bool {
	if os.Geteuid() == 0 {
		return true
	}
	for _, path := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	if raw, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		text := string(raw)
		return strings.Contains(text, "docker") || strings.Contains(text, "kubepods") || strings.Contains(text, "containerd") || strings.Contains(text, "podman")
	}
	return false
}

func browserPage() (browser *rod.Browser, page *rod.Page, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not launch/connect browser for LawHub session: %v", r)
		}
	}()
	session := sessionPath()
	if _, err := os.Stat(session); err != nil {
		return nil, nil, fmt.Errorf("missing LawHub storage state; run `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222` or `lawhub-pp-cli auth import-file <storage-state.json>`")
	}
	browserExe := resolveBrowserPath("")
	launch := launcher.New().Bin(browserExe).Headless(true)
	if shouldDisableChromeSandbox() {
		launch = launch.NoSandbox(true)
	}
	u := launch.MustLaunch()
	b := rod.New().ControlURL(u).MustConnect()
	p := b.MustPage("about:blank")
	var st storageState
	if raw, err := os.ReadFile(session); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	if len(st.Cookies) == 0 && len(st.Origins) == 0 {
		b.MustClose()
		return nil, nil, fmt.Errorf("saved LawHub session is a browser-profile marker, not portable storage state; run `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222` or `lawhub-pp-cli auth import-file <storage-state.json>`")
	}
	cookieParams := make([]*proto.NetworkCookieParam, 0, len(st.Cookies))
	for _, c := range st.Cookies {
		cookieParams = append(cookieParams, &proto.NetworkCookieParam{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, HTTPOnly: c.HTTPOnly, Secure: c.Secure})
	}
	if len(cookieParams) > 0 {
		if err := p.SetCookies(cookieParams); err != nil {
			b.MustClose()
			return nil, nil, fmt.Errorf("failed to restore LawHub session cookies: %w", err)
		}
	}
	restoreLocalStorage(p, st.Origins)
	p.MustNavigate(lawhubURL).MustWaitLoad()
	if err := waitForAuthSettled(p, 12*time.Second); err != nil {
		b.MustClose()
		return nil, nil, err
	}
	return b, p, nil
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }
func slug(s string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return strings.Trim(strings.ToLower(re.ReplaceAllString(s, "-")), "-")
}
func moduleIDFromName(name string) string {
	m := regexp.MustCompile(`(?i)PrepTest\s+(\d+)`).FindStringSubmatch(name)
	if len(m) > 1 {
		return "LSAC" + m[1]
	}
	return ""
}
func sectionType(id, fallback string) string {
	u := strings.ToUpper(id)
	switch {
	case strings.HasPrefix(u, "LR"):
		return "Logical Reasoning"
	case strings.HasPrefix(u, "RC"):
		return "Reading Comprehension"
	case strings.HasPrefix(u, "LG") || strings.HasPrefix(u, "AR"):
		return "Analytical Reasoning"
	}
	return fallback
}

func syncBrowserCmd() *cobra.Command {
	var wait int
	c := &cobra.Command{Use: "browser", RunE: func(cmd *cobra.Command, args []string) error {
		b, p, err := browserPage()
		if err != nil {
			return err
		}
		defer b.MustClose()
		time.Sleep(time.Duration(wait) * time.Millisecond)
		body := p.MustElement("body").MustText()
		tests := []map[string]any{}
		db, dbPath, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		ts := nowISO()
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "The Official LSAT") || strings.HasPrefix(line, "LSAT Argumentative Writing") {
				t := map[string]any{"id": slug(line), "name": line, "type": "fulltest", "available": true, "source_url": lawhubURL}
				if !strings.Contains(line, "PrepTest") {
					t["type"] = "writing"
				}
				tests = append(tests, t)
				_, _ = db.Exec(`INSERT INTO tests(id,name,type,available,source_url,synced_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,type=excluded.type,available=excluded.available,source_url=excluded.source_url,synced_at=excluded.synced_at`, t["id"], t["name"], t["type"], 1, lawhubURL, ts)
			}
		}
		_, _ = db.Exec(`INSERT INTO sync_log(synced_at,source,item_count,status,message) VALUES(?,?,?,?,?)`, ts, "browser-go", len(tests), "ok", "visible library sync")
		return emit(map[string]any{"tests_synced": len(tests), "tests": tests, "db": dbPath})
	}}
	c.Flags().IntVar(&wait, "wait-ms", 8000, "wait after page load")
	return c
}

func syncHistoryCmd() *cobra.Command {
	var module string
	var pageSize int
	c := &cobra.Command{Use: "history", RunE: func(cmd *cobra.Command, args []string) error {
		db, dbPath, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		mods := []string{}
		if module != "" {
			mods = []string{module}
		} else {
			rows, _ := db.Query(`SELECT name FROM tests ORDER BY name`)
			if rows != nil {
				defer rows.Close()
				for rows.Next() {
					var n string
					_ = rows.Scan(&n)
					if m := moduleIDFromName(n); m != "" {
						mods = append(mods, m)
					}
				}
			}
		}
		seenMods := map[string]bool{}
		uniqMods := []string{}
		for _, m := range mods {
			if m != "" && !seenMods[m] {
				seenMods[m] = true
				uniqMods = append(uniqMods, m)
			}
		}
		mods = uniqMods
		if len(mods) == 0 {
			mods = []string{"LSAC140"}
		}
		b, p, err := browserPage()
		if err != nil {
			return err
		}
		defer b.MustClose()
		ts := nowISO()
		imported := 0
		userID, userSource, err := resolveUserID(p)
		if err != nil {
			return err
		}
		details := map[string]any{"user": map[string]any{"id": userID, "source": userSource}}
		for _, m := range mods {
			moduleImported, moduleDetails := syncHistoryModule(db, p, userID, m, pageSize, ts)
			imported += moduleImported
			details[m] = moduleDetails
		}
		_, _ = db.Exec(`INSERT INTO sync_log(synced_at,source,item_count,status,message) VALUES(?,?,?,?,?)`, ts, "history-go", imported, "ok", mustJSON(details))
		return emit(map[string]any{"attempts_synced": imported, "modules": details, "db": dbPath})
	}}
	c.Flags().StringVar(&module, "module", "", "module id, e.g. LSAC140")
	c.Flags().IntVar(&pageSize, "page-size", 25, "page size")
	return c
}

func syncHistoryModule(db *sql.DB, p *rod.Page, userID, module string, pageSize int, ts string) (int, map[string]any) {
	if pageSize <= 0 {
		pageSize = 25
	}
	details := map[string]any{"pages": []map[string]any{}, "recordCount": 0, "items": 0}
	imported := 0
	seenAttempts := map[string]bool{}
	for pageNumber := 1; ; pageNumber++ {
		api := historyAPIURL(userID, module, pageNumber, pageSize)
		raw, err := fetchJSONString(p, api)
		if err != nil {
			details["error"] = err.Error()
			break
		}
		var hr historyResp
		if err := json.Unmarshal([]byte(raw), &hr); err != nil {
			details["error"] = err.Error()
			break
		}
		details["recordCount"] = hr.RecordCount
		details["items"] = asInt(details["items"]) + int64(len(hr.Items))
		details["pages"] = append(details["pages"].([]map[string]any), map[string]any{"page": pageNumber, "items": len(hr.Items)})
		for _, it := range hr.Items {
			if !it.IsCompleted || it.TestInstanceID == "" || seenAttempts[it.TestInstanceID] {
				continue
			}
			seenAttempts[it.TestInstanceID] = true
			upsertHistoryAttempt(db, it, ts)
			imported++
		}
		if len(hr.Items) < pageSize || pageNumber*pageSize >= hr.RecordCount {
			break
		}
	}
	details["attempts_synced"] = imported
	return imported, details
}

func historyAPIURL(userID, module string, pageNumber, pageSize int) string {
	return fmt.Sprintf("https://app.lawhub.org/api/request/v2/api/user/%s/history/%s?PageNumber=%d&SortOrder=desc&SortField=startDate&PageSize=%d", url.PathEscape(userID), url.PathEscape(module), pageNumber, pageSize)
}

func upsertHistoryAttempt(db *sql.DB, it historyItem, ts string) {
	correct, total := 0, 0
	for _, sr := range it.SectionResults {
		if !sr.IsVariableSection {
			correct += sr.TotalCorrect
			total += sr.TotalQuestions
		}
	}
	mode := "Self-Paced Mode"
	if it.ExamMode {
		mode = "Exam Mode"
	}
	_, _ = db.Exec(`INSERT INTO tests(id,name,type,available,source_url,synced_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,type=excluded.type,available=excluded.available,source_url=excluded.source_url,synced_at=excluded.synced_at`, it.ModuleID, it.ModuleName, "fulltest", 1, "https://app.lawhub.org/history/"+it.ModuleID, ts)
	_, _ = db.Exec(`INSERT INTO attempts(id,test_id,test_name,mode,started_at,completed_at,scaled_score,raw_score,total_questions,correct_count,source_url,synced_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET test_id=excluded.test_id,test_name=excluded.test_name,mode=excluded.mode,started_at=excluded.started_at,completed_at=excluded.completed_at,scaled_score=excluded.scaled_score,raw_score=excluded.raw_score,total_questions=excluded.total_questions,correct_count=excluded.correct_count,source_url=excluded.source_url,synced_at=excluded.synced_at`, it.TestInstanceID, it.ModuleID, it.ModuleName, mode, it.StartDate, it.EndDate, it.ConvertedScore, it.TotalScore, total, correct, "https://app.lawhub.org/scoreReport/"+it.TestInstanceID, ts)
	for idx, sr := range it.SectionResults {
		stype := sectionType(sr.SectionID, sr.SectionName)
		if sr.IsVariableSection {
			stype += " (Variable)"
		}
		_, _ = db.Exec(`INSERT INTO sections(id,attempt_id,section_index,section_type,correct_count,total_questions,time_limit_seconds,time_spent_seconds) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET section_type=excluded.section_type,correct_count=excluded.correct_count,total_questions=excluded.total_questions`, sr.SectionID, it.TestInstanceID, idx+1, stype, sr.TotalCorrect, sr.TotalQuestions, nil, nil)
	}
}

func fetchJSONString(p *rod.Page, api string) (raw string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fetch LawHub API failed: %v", r)
		}
	}()
	raw = p.MustEval(`async u => {
		const response = await fetch(u);
		const text = await response.text();
		if (!response.ok) throw new Error('HTTP ' + response.status + ': ' + text.slice(0, 200));
		try { JSON.parse(text); } catch (e) { throw new Error('invalid JSON: ' + text.slice(0, 200)); }
		return text;
	}`, api).String()
	if raw == "" {
		return "", fmt.Errorf("fetch LawHub API returned empty response")
	}
	return raw, nil
}

func attemptIDs(db *sql.DB, attempt string) []string {
	if attempt != "" {
		return []string{attempt}
	}
	ids := []string{}
	rows, _ := db.Query(`SELECT id FROM attempts ORDER BY COALESCE(completed_at, started_at, synced_at) DESC`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			ids = append(ids, id)
		}
	}
	return ids
}

func upsertReportQuestions(db *sql.DB, attemptID string, rows []reportRow) int {
	updated := 0
	for _, r := range rows {
		res, _ := db.Exec(`UPDATE questions SET question_type=?, chosen_answer=?, correct_answer=?, is_correct=?, time_spent_seconds=COALESCE(?,time_spent_seconds), flagged=?, source_url=?, answered=?, difficulty=? WHERE attempt_id=? AND section_index=? AND question_number=?`,
			r.QuestionType, r.ChosenAnswer, nullIfEmpty(r.CorrectAnswer), r.IsCorrect, nullIfZero(r.TimeSpentSeconds), r.Flagged, fmt.Sprintf("https://app.lawhub.org/question/%s/Section%%20%d", attemptID, r.SectionIndex), boolInt(r.ChosenAnswer != ""), nullIfZero(r.Difficulty), attemptID, r.SectionIndex, r.QuestionNumber)
		n, _ := res.RowsAffected()
		if n > 0 {
			updated += int(n)
			continue
		}
		sid := fmt.Sprintf("%s:section:%d", attemptID, r.SectionIndex)
		qid := fmt.Sprintf("%s:%s:q:%d", attemptID, sid, r.QuestionNumber)
		res, _ = db.Exec(`INSERT INTO questions(id,attempt_id,section_id,section_index,question_number,question_type,chosen_answer,correct_answer,is_correct,time_spent_seconds,flagged,source_url,answered,difficulty)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET question_type=excluded.question_type,chosen_answer=excluded.chosen_answer,correct_answer=excluded.correct_answer,is_correct=excluded.is_correct,time_spent_seconds=excluded.time_spent_seconds,flagged=excluded.flagged,source_url=excluded.source_url,answered=excluded.answered,difficulty=excluded.difficulty`,
			qid, attemptID, sid, r.SectionIndex, r.QuestionNumber, r.QuestionType, r.ChosenAnswer, nullIfEmpty(r.CorrectAnswer), r.IsCorrect, r.TimeSpentSeconds, r.Flagged, fmt.Sprintf("https://app.lawhub.org/question/%s/Section%%20%d", attemptID, r.SectionIndex), boolInt(r.ChosenAnswer != ""), nullIfZero(r.Difficulty))
		n, _ = res.RowsAffected()
		updated += int(n)
	}
	return updated
}

func syncReportMetadataCmd() *cobra.Command {
	var attempt string
	var wait int
	c := &cobra.Command{Use: "report-metadata", RunE: func(cmd *cobra.Command, args []string) error {
		db, dbPath, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		ids := attemptIDs(db, attempt)
		b, p, err := browserPage()
		if err != nil {
			return err
		}
		defer b.MustClose()
		details := map[string]any{}
		total := 0
		errs := []string{}
		for _, id := range ids {
			rows, err := extractReportRows(p, id, wait)
			if err != nil {
				details[id] = map[string]any{"error": err.Error()}
				errs = append(errs, fmt.Sprintf("%s: %s", id, err.Error()))
				continue
			}
			updated := updateReportRows(db, id, rows)
			total += updated
			details[id] = map[string]any{"report_rows": len(rows), "updated_questions": updated}
		}
		ts := nowISO()
		status := "ok"
		if len(errs) > 0 {
			status = "error"
		}
		_, _ = db.Exec(`INSERT INTO sync_log(synced_at,source,item_count,status,message) VALUES(?,?,?,?,?)`, ts, "report-metadata-go", total, status, mustJSON(details))
		out := map[string]any{"attempts_synced": len(ids), "questions_updated": total, "details": details, "db": dbPath}
		if len(errs) > 0 {
			out["errors"] = errs
			_ = emit(out)
			return fmt.Errorf("report metadata sync failed for %d/%d attempt(s)", len(errs), len(ids))
		}
		return emit(out)
	}}
	c.Flags().StringVar(&attempt, "attempt", "", "attempt id")
	c.Flags().IntVar(&wait, "wait-ms", 20000, "wait ms")
	return c
}

func extractReportRows(p *rod.Page, id string, wait int) ([]reportRow, error) {
	func() {
		defer func() { _ = recover() }()
		p.MustNavigate("https://app.lawhub.org/scoreReport/" + id).MustWaitLoad()
	}()
	if err := waitForAuthSettled(p, 12*time.Second); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(time.Duration(wait) * time.Millisecond)
	seenTable := false
	for time.Now().Before(deadline) {
		if pageLooksUnauthenticated(p) {
			return nil, fmt.Errorf("LawHub session expired or unauthenticated while loading score report; run `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222`")
		}
		func() {
			defer func() { _ = recover() }()
			if p.MustEval(`() => document.querySelectorAll('table.testResults tr').length`).Int() > 1 {
				seenTable = true
			}
		}()
		if seenTable {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !seenTable {
		if pageLooksUnauthenticated(p) {
			return nil, fmt.Errorf("LawHub session expired or unauthenticated while loading score report; run `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222`")
		}
		return nil, fmt.Errorf("score report table not found for attempt %s after %dms", id, wait)
	}
	js := reportRowsJS()
	raw := ""
	func() {
		defer func() { _ = recover() }()
		raw = p.MustEval("async code => JSON.stringify(await eval(code)())", js).String()
	}()
	if raw == "" {
		return nil, fmt.Errorf("score report extraction returned empty payload for attempt %s", id)
	}
	rows, err := ParseReportRowsJSON(raw)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func normalizeReportRows(rows []reportRow) []reportRow {
	for i := range rows {
		if rows[i].AnswerStatus == "correct" {
			rows[i].IsCorrect = 1
		} else if rows[i].AnswerStatus == "incorrect" {
			rows[i].IsCorrect = 0
		}
		if rows[i].AnswerStatus == "incorrect" && rows[i].ChosenAnswer != "" && rows[i].ChosenAnswer == rows[i].CorrectAnswer {
			rows[i].CorrectAnswer = ""
		}
		rows[i].Flagged = parseFlagged(rows[i].FlaggedText)
		rows[i].TimeSpentSeconds = parseDuration(rows[i].TimeText)
	}
	return rows
}

func reportRowsJS() string {
	return `async () => { function clean(s){return (s||'').trim().replace(/\s+/g,' ')}; function diff(s){let m=clean(s).match(/(\d+)/); return m?Number(m[1]):null}; function parse(cells){ if(cells.length<8)return null; let q=Number(clean(cells[0].innerText)); if(!q)return null; return {question_number:q,flagged_text:clean(cells[1].innerText),chosen_answer:clean(cells[2].innerText),answer_status:clean(cells[3].innerText).toLowerCase(),correct_answer:clean(cells[4].innerText),question_type:clean(cells[5].innerText),difficulty:diff(cells[6].innerText),time_text:clean(cells[7].innerText)} } async function sleep(ms){return new Promise(r=>setTimeout(r,ms))}; let out=[]; let n=Math.max(document.querySelectorAll('.answers-panel-section-title,.answers-panel-section-title-unselected').length,1); for(let i=0;i<n;i++){let tabs=Array.from(document.querySelectorAll('.answers-panel-section-title,.answers-panel-section-title-unselected')); if(tabs[i]){tabs[i].click(); await sleep(450)}; for(const tr of Array.from(document.querySelectorAll('table.testResults tr'))){let r=parse(Array.from(tr.querySelectorAll('td'))); if(r){r.section_index=i+1; out.push(r)}}}; return out }`
}

func ParseReportRowsJSON(raw string) ([]reportRow, error) {
	var rows []reportRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	rows = normalizeReportRows(rows)
	if err := validateReportRows(rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func validateReportRows(rows []reportRow) error {
	if len(rows) == 0 {
		return errors.New("score report contained zero parseable rows")
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.SectionIndex <= 0 || r.QuestionNumber <= 0 {
			return fmt.Errorf("invalid report row section=%d question=%d", r.SectionIndex, r.QuestionNumber)
		}
		if r.AnswerStatus != "correct" && r.AnswerStatus != "incorrect" && r.AnswerStatus != "unanswered" && r.AnswerStatus != "" {
			return fmt.Errorf("unexpected answer status %q for section %d question %d", r.AnswerStatus, r.SectionIndex, r.QuestionNumber)
		}
		key := fmt.Sprintf("%d:%d", r.SectionIndex, r.QuestionNumber)
		if seen[key] {
			return fmt.Errorf("duplicate score report row for section/question %s", key)
		}
		seen[key] = true
	}
	return nil
}

func updateReportRows(db *sql.DB, id string, rows []reportRow) int {
	updated := 0
	for _, r := range rows {
		res, _ := db.Exec(`UPDATE questions SET question_type=?, chosen_answer=?, correct_answer=?, is_correct=?, time_spent_seconds=COALESCE(?,time_spent_seconds), flagged=?, difficulty=?, answered=? WHERE attempt_id=? AND section_index=? AND question_number=?`, r.QuestionType, r.ChosenAnswer, nullIfEmpty(r.CorrectAnswer), r.IsCorrect, nullIfZero(r.TimeSpentSeconds), r.Flagged, nullIfZero(r.Difficulty), boolInt(r.ChosenAnswer != ""), id, r.SectionIndex, r.QuestionNumber)
		n, _ := res.RowsAffected()
		if n == 0 {
			updated += upsertReportQuestions(db, id, []reportRow{r})
		} else {
			updated += int(n)
		}
	}
	return updated
}

func parseFlagged(s string) int {
	text := strings.TrimSpace(strings.ToLower(s))
	if text == "flagged" || text == "yes" || text == "true" {
		return 1
	}
	return 0
}

func parseDuration(s string) int {
	re := regexp.MustCompile(`(\d+)\s*([hms])\b`)
	m := re.FindAllStringSubmatch(strings.ToLower(s), -1)
	total := 0
	for _, x := range m {
		n, _ := strconv.Atoi(x[1])
		switch x[2] {
		case "h":
			total += n * 3600
		case "m":
			total += n * 60
		case "s":
			total += n
		}
	}
	return total
}
func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullIfZero(i int) any {
	if i == 0 {
		return nil
	}
	return i
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
