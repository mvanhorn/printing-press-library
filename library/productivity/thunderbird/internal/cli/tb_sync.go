// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

// tbResourceTypes are the store resource types sync writes, in sync order.
var tbResourceTypes = []string{"accounts", "identities", "folders", "messages", "attachments", "contacts", "filters", "events"}

const tbSyncBatchSize = 500

// tbDogfoodMaxNewMessages bounds one sync run under the dogfood harness.
const tbDogfoodMaxNewMessages = 2000

type tbSyncOptions struct {
	Resources      map[string]bool
	Full           bool
	MaxNewMessages int
	ReadDir        tbprofile.DirReader
	OnUpgrade      func()
}

// tbStoreFormat 2 added attachment inline flags and inline-free attachment counts; 3 renders HTML quotes as "> " lines.
const tbStoreFormat = 3

type tbSyncSummary struct {
	Profile         string         `json:"profile"`
	DBPath          string         `json:"db_path"`
	Full            bool           `json:"full"`
	Resources       map[string]int `json:"resources"`
	NewMessages     int            `json:"new_messages"`
	PrunedMessages  int            `json:"pruned_messages"`
	FoldersScanned  int            `json:"folders_scanned"`
	FoldersSkipped  int            `json:"folders_unchanged"`
	FlagFolders     int            `json:"flag_refresh_folders"`
	FlagsUpdated    int            `json:"flags_updated"`
	Capped          bool           `json:"capped"`
	FormatUpgrade   bool           `json:"format_upgrade,omitempty"`
	UpgradePending  bool           `json:"upgrade_pending,omitempty"`
	Warnings        []string       `json:"warnings"`
	ElapsedMS       int64          `json:"elapsed_ms"`
	ProfileNotFound bool           `json:"profile_not_found,omitempty"`
}

// tbMessageDoc is the stored JSON shape of a message.
type tbMessageDoc struct {
	ID              string   `json:"id"`
	Account         string   `json:"account"`
	AccountName     string   `json:"account_name"`
	Folder          string   `json:"folder"`
	FolderPath      string   `json:"folder_path"`
	FolderKey       string   `json:"folder_key"`
	Date            string   `json:"date"`
	FromAddr        string   `json:"from_addr"`
	FromName        string   `json:"from_name"`
	To              []string `json:"to"`
	Cc              []string `json:"cc"`
	Subject         string   `json:"subject"`
	MessageID       string   `json:"message_id"`
	InReplyTo       string   `json:"in_reply_to"`
	References      []string `json:"references"`
	ThreadID        string   `json:"thread_id"`
	ThreadRoot      string   `json:"thread_root"`
	Read            bool     `json:"read"`
	Replied         bool     `json:"replied"`
	Flagged         bool     `json:"flagged"`
	Forwarded       bool     `json:"forwarded"`
	Outgoing        bool     `json:"outgoing"`
	SizeBytes       int64    `json:"size_bytes"`
	HasAttachments  bool     `json:"has_attachments"`
	AttachmentCount int      `json:"attachment_count"`
	ListID          string   `json:"list_id"`
	ListUnsubscribe string   `json:"list_unsubscribe"`
	Automated       bool     `json:"automated"`
	AuthResults     string   `json:"auth_results"`
	BodyText        string   `json:"body_text"`
	MboxPath        string   `json:"mbox_path"`
	Offset          int64    `json:"offset"`
	Length          int64    `json:"length"`
}

// tbAttachmentDoc is the stored JSON shape of an attachment.
type tbAttachmentDoc struct {
	ID          string `json:"id"`
	MessageID   string `json:"message_id"`
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Account     string `json:"account"`
	Folder      string `json:"folder"`
	FolderPath  string `json:"folder_path"`
	FolderKey   string `json:"folder_key"`
	Date        string `json:"date"`
	FromAddr    string `json:"from_addr"`
	Subject     string `json:"subject"`
	Inline      *bool  `json:"inline,omitempty"`
}

type tbFolderDoc struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	AccountName string `json:"account_name"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	MboxPath    string `json:"mbox_path"`
	Offline     bool   `json:"offline"`
	Total       int    `json:"total"`
	Unread      int    `json:"unread"`
	Flagged     int    `json:"flagged"`
	SizeBytes   int64  `json:"size_bytes"`
	SentFolder  bool   `json:"sent_folder"`
}

func tbFolderKey(account, path string) string { return account + ":" + path }

func tbFormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

type tbBatcher struct {
	db    *store.Store
	items map[string][]json.RawMessage
	count map[string]int
}

func newTBBatcher(db *store.Store) *tbBatcher {
	return &tbBatcher{db: db, items: map[string][]json.RawMessage{}, count: map[string]int{}}
}

func (b *tbBatcher) add(resource string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b.items[resource] = append(b.items[resource], raw)
	if len(b.items[resource]) >= tbSyncBatchSize {
		return b.flush(resource)
	}
	return nil
}

func (b *tbBatcher) flush(resource string) error {
	items := b.items[resource]
	if len(items) == 0 {
		return nil
	}
	n, _, err := b.db.UpsertBatch(resource, items)
	if err != nil {
		return fmt.Errorf("storing %s: %w", resource, err)
	}
	b.count[resource] += n
	b.items[resource] = items[:0]
	return nil
}

func (b *tbBatcher) flushAll() error {
	for _, r := range tbResourceTypes {
		if err := b.flush(r); err != nil {
			return err
		}
	}
	return nil
}

var errTBSyncCap = errors.New("sync message cap reached")

type tbDiscoveredFolder struct {
	account tbprofile.Account
	folder  tbprofile.Folder
}

// tbSyncRun: protected holds folder-key prefixes of unreadable sources, whose rows must not be pruned.
type tbSyncRun struct {
	ctx       context.Context
	db        *store.Store
	b         *tbBatcher
	opts      tbSyncOptions
	sum       *tbSyncSummary
	synced    map[string]bool
	protected []string
	// unread is set when an account, subtree or folder could not be read, so its old rows were kept as they were.
	unread bool
}

func (s *tbSyncRun) warn(format string, a ...any) {
	s.sum.Warnings = append(s.sum.Warnings, fmt.Sprintf(format, a...))
}

func (s *tbSyncRun) isProtected(folderKey string) bool {
	for _, p := range s.protected {
		if strings.HasPrefix(folderKey, p) {
			return true
		}
	}
	return false
}

// replace flushes resource and prunes the stored rows not in keep.
func (s *tbSyncRun) replace(resource string, keep map[string]bool) error {
	if err := s.b.flush(resource); err != nil {
		return err
	}
	if _, err := s.db.PruneResourcesNotIn(resource, keep); err != nil {
		return err
	}
	s.synced[resource] = true
	return nil
}

// keepExisting adds to keep the stored ids of resource matching where.
func (s *tbSyncRun) keepExisting(resource, where string, keep map[string]bool, args ...any) error {
	rows, err := s.db.DB().Query(`SELECT id FROM resources WHERE resource_type = ? AND `+where, append([]any{resource}, args...)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		keep[id] = true
	}
	return rows.Err()
}

func (s *tbSyncRun) keepPrefix(resource, prefix string, keep map[string]bool) error {
	return s.keepExisting(resource, `instr(id, ?) = 1`, keep, prefix)
}

// tbStoreFormatOld reports a store whose messages were parsed by an older format; an empty store needs no upgrade.
func tbStoreFormatOld(db *store.Store) (bool, error) {
	v, _, err := db.GetTBMeta("store_format")
	if err != nil {
		return false, err
	}
	if n, _ := strconv.Atoi(v); n >= tbStoreFormat {
		return false, nil
	}
	states, err := db.ListMboxStates()
	return len(states) > 0, err
}

// runTBSync ingests the profile into db. It never writes to the profile.
func runTBSync(ctx context.Context, db *store.Store, profileDir string, opts tbSyncOptions) (*tbSyncSummary, error) {
	start := time.Now()
	sum := &tbSyncSummary{Profile: profileDir, DBPath: db.Path(), Full: opts.Full, Resources: map[string]int{}, Warnings: []string{}}
	want := func(r string) bool { return len(opts.Resources) == 0 || opts.Resources[r] }
	if err := db.EnsureThunderbirdTables(ctx); err != nil {
		return nil, err
	}
	prefs, err := tbprofile.ParsePrefs(filepath.Join(profileDir, "prefs.js"))
	if err != nil {
		return nil, fmt.Errorf("reading prefs.js: %w", err)
	}
	accounts := prefs.Accounts(profileDir)
	parseMail := want("messages") || want("attachments")
	old := false
	if parseMail {
		if old, err = tbStoreFormatOld(db); err != nil {
			return nil, err
		}
		if old && !opts.Full {
			opts.Full, sum.FormatUpgrade = true, true
			if opts.OnUpgrade != nil {
				opts.OnUpgrade()
			}
		}
	}
	s := &tbSyncRun{ctx: ctx, db: db, b: newTBBatcher(db), opts: opts, sum: sum, synced: map[string]bool{}}

	if want("accounts") || want("identities") {
		if err := s.syncAccounts(accounts, want); err != nil {
			return nil, err
		}
	}
	var folders []tbDiscoveredFolder
	if want("folders") || want("messages") || want("attachments") {
		folders = s.discoverFolders(accounts)
	}
	if want("messages") || want("attachments") {
		if err := s.syncMessages(accounts, folders); err != nil {
			return nil, err
		}
	}
	if want("folders") {
		if err := s.syncFolders(prefs, accounts, folders); err != nil {
			return nil, err
		}
	}
	if want("contacts") {
		if err := s.syncContacts(profileDir); err != nil {
			return nil, err
		}
	}
	if want("filters") {
		if err := s.syncFilters(accounts); err != nil {
			return nil, err
		}
	}
	if want("events") {
		if err := s.syncEvents(profileDir); err != nil {
			return nil, err
		}
	}

	if err := s.b.flushAll(); err != nil {
		return nil, err
	}
	// A capped upgrade or one that skipped unreadable mail leaves old rows behind, so the next sync must re-parse again.
	sum.UpgradePending = old && (sum.Capped || s.unread)
	if parseMail && !sum.UpgradePending {
		if err := db.SetTBMeta("store_format", strconv.Itoa(tbStoreFormat)); err != nil {
			return nil, err
		}
	}
	for _, r := range tbResourceTypes {
		if !s.synced[r] {
			continue
		}
		n, err := db.Count(r)
		if err != nil {
			return nil, err
		}
		sum.Resources[r] = n
		if err := db.SaveSyncState(r, "", n); err != nil {
			return nil, err
		}
	}
	sum.ElapsedMS = time.Since(start).Milliseconds()
	return sum, nil
}

func (s *tbSyncRun) syncAccounts(accounts []tbprofile.Account, want func(string) bool) error {
	accKeep, idKeep := map[string]bool{}, map[string]bool{}
	if want("identities") {
		for _, id := range tbIdentityDocsFromPrefs(accounts) {
			idKeep[id.ID] = true
			if err := s.b.add("identities", id); err != nil {
				return err
			}
		}
	}
	if want("accounts") {
		for _, a := range accounts {
			accKeep[a.Key] = true
			row := tbAccountRowFromPrefs(a)
			row.Source = ""
			if err := s.b.add("accounts", row); err != nil {
				return err
			}
		}
	}
	for _, r := range []string{"accounts", "identities"} {
		if !want(r) {
			continue
		}
		keep := accKeep
		if r == "identities" {
			keep = idKeep
		}
		if err := s.replace(r, keep); err != nil {
			return err
		}
	}
	return nil
}

func (s *tbSyncRun) discoverFolders(accounts []tbprofile.Account) []tbDiscoveredFolder {
	var out []tbDiscoveredFolder
	for _, a := range accounts {
		fs, skipped, err := tbprofile.DiscoverFolders(a.Server.Directory, a.Key, s.opts.ReadDir)
		if err != nil {
			s.warn("account %s: %v", a.Key, err)
			s.protected, s.unread = append(s.protected, a.Key+":"), true
			continue
		}
		for _, sk := range skipped {
			s.warn("account %s folder %s: %v", a.Key, sk.Prefix, sk.Err)
			s.protected, s.unread = append(s.protected, tbFolderKey(a.Key, sk.Prefix)+"/"), true
		}
		for _, f := range fs {
			out = append(out, tbDiscoveredFolder{a, f})
		}
	}
	return out
}

func (s *tbSyncRun) syncMessages(accounts []tbprofile.Account, folders []tbDiscoveredFolder) error {
	identityEmails := tbOwnAddresses(tbIdentityDocsFromPrefs(accounts))
	livePaths, unfinished := map[string]bool{}, map[string]bool{}
	newCount, freshCount := 0, 0
	for _, d := range folders {
		if !d.folder.Offline || d.folder.MboxPath == "" {
			continue
		}
		livePaths[d.folder.MboxPath] = true
		if s.sum.Capped {
			unfinished[tbFolderKey(d.account.Key, d.folder.Path)] = true
			continue
		}
		res, err := s.ingestFolder(d.account, d.folder, identityEmails, freshCount)
		if s.sum.Capped {
			unfinished[tbFolderKey(d.account.Key, d.folder.Path)] = true
		}
		newCount += res.New
		freshCount += res.Fresh
		s.sum.PrunedMessages += res.Pruned
		if res.Scanned {
			s.sum.FoldersScanned++
		} else {
			s.sum.FoldersSkipped++
		}
		if err != nil {
			return err
		}
	}
	s.sum.NewMessages = newCount
	states, err := s.db.ListMboxStates()
	if err != nil {
		return err
	}
	for _, st := range states {
		// Rows of an unfinished folder may still carry its old mbox path.
		if livePaths[st.Path] || unfinished[st.FolderKey] || s.isProtected(st.FolderKey) {
			continue
		}
		n, err := s.pruneMsgs(st.FolderKey, tbWhereMboxPath, st.Path, nil, nil)
		if err != nil {
			return err
		}
		s.sum.PrunedMessages += n
		if err := s.db.DeleteMboxState(st.Path); err != nil {
			return err
		}
	}
	if s.sum.FoldersScanned > 0 || s.sum.PrunedMessages > 0 {
		if err := tbResolveThreadRoots(s.db); err != nil {
			return err
		}
	}
	s.synced["messages"], s.synced["attachments"] = true, true
	return nil
}

const tbWhereMboxPath = `json_extract(data,'$.mbox_path') = ?`

// pruneMsgs deletes the messages of folder key matching where (one arg), except keepMsgs, and their attachments except keepAtts.
func (s *tbSyncRun) pruneMsgs(key, where string, arg any, keepMsgs, keepAtts map[string]bool) (int, error) {
	matched := map[string]bool{}
	if err := s.keepExisting("messages", `json_extract(data,'$.folder_key') = ? AND `+where, matched, key, arg); err != nil {
		return 0, err
	}
	return s.dropMsgs(key, matched, keepMsgs, keepAtts)
}

// dropMsgs deletes the matched messages of folder key, except keepMsgs, and their attachments except keepAtts.
func (s *tbSyncRun) dropMsgs(key string, matched, keepMsgs, keepAtts map[string]bool) (int, error) {
	var doomed []string
	for id := range matched {
		if !keepMsgs[id] {
			doomed = append(doomed, id)
		}
	}
	rows, err := s.db.DB().Query(`SELECT id, COALESCE(json_extract(data,'$.message_id'),'') FROM resources
		WHERE resource_type = 'attachments' AND json_extract(data,'$.folder_key') = ?`, key)
	if err != nil {
		return 0, err
	}
	var doomedAtts []string
	for rows.Next() {
		var id, msg string
		if err := rows.Scan(&id, &msg); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if matched[msg] && !keepAtts[id] {
			doomedAtts = append(doomedAtts, id)
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n, err := s.db.DeleteResourceIDs("messages", doomed)
	if err != nil {
		return n, err
	}
	_, err = s.db.DeleteResourceIDs("attachments", doomedAtts)
	return n, err
}

func (s *tbSyncRun) syncFolders(prefs tbprofile.Prefs, accounts []tbprofile.Account, folders []tbDiscoveredFolder) error {
	stats, err := tbFolderMessageStats(s.db)
	if err != nil {
		return err
	}
	fcc := map[string]bool{}
	for _, uri := range prefs.SentFolderURIs(accounts) {
		if acc, path, ok := tbprofile.MatchFolderURI(uri, accounts); ok {
			fcc[strings.ToLower(tbFolderKey(acc, path))] = true
		}
	}
	keep := map[string]bool{}
	for _, p := range s.protected {
		if err := s.keepPrefix("folders", p, keep); err != nil {
			return err
		}
	}
	for _, d := range folders {
		key := tbFolderKey(d.account.Key, d.folder.Path)
		keep[key] = true
		st := stats[key]
		doc := tbFolderDoc{
			ID: key, Account: d.account.Key, AccountName: d.account.Name(), Name: d.folder.Name, Path: d.folder.Path,
			MboxPath: d.folder.MboxPath, Offline: d.folder.Offline, Total: st[0], Unread: st[1], Flagged: st[2], SizeBytes: d.folder.SizeBytes,
			SentFolder: fcc[strings.ToLower(key)] || tbprofile.IsSentFolderName(d.folder.Path),
		}
		if err := s.b.add("folders", doc); err != nil {
			return err
		}
	}
	return s.replace("folders", keep)
}

func (s *tbSyncRun) syncContacts(profileDir string) error {
	keep := map[string]bool{}
	for _, path := range tbprofile.AddressBookFiles(profileDir) {
		cards, err := tbprofile.ReadAddressBook(path)
		if err != nil {
			s.warn("address book %s: %v", filepath.Base(path), err)
			if err := s.keepExisting("contacts", `json_extract(data,'$.book') = ?`, keep, tbprofile.BookName(path)); err != nil {
				return err
			}
			continue
		}
		for _, c := range cards {
			keep[c.ID] = true
			primary := ""
			if len(c.Emails) > 0 {
				primary = c.Emails[0]
			}
			doc := map[string]any{
				"id": c.ID, "book": c.Book, "collected": c.Book == "history", "display_name": c.DisplayName,
				"first_name": c.FirstName, "last_name": c.LastName, "nickname": c.NickName, "company": c.Company,
				"emails": c.Emails, "primary_email": primary,
			}
			if err := s.b.add("contacts", doc); err != nil {
				return err
			}
		}
	}
	return s.replace("contacts", keep)
}

func (s *tbSyncRun) syncFilters(accounts []tbprofile.Account) error {
	keep := map[string]bool{}
	for _, a := range accounts {
		if a.Server.Directory == "" {
			continue
		}
		rules, err := tbprofile.ParseFilterRules(filepath.Join(a.Server.Directory, "msgFilterRules.dat"))
		if err != nil {
			s.warn("filters of %s: %v", a.Key, err)
			if err := s.keepPrefix("filters", a.Key+":", keep); err != nil {
				return err
			}
			continue
		}
		for _, f := range rules {
			id := a.Key + ":" + strconv.Itoa(f.Index)
			keep[id] = true
			action, actionValue := "", ""
			if len(f.Actions) > 0 {
				action, actionValue = f.Actions[0].Type, f.Actions[0].Value
			}
			doc := map[string]any{
				"id": id, "account": a.Key, "account_name": a.Name(), "index": f.Index, "name": f.Name, "enabled": f.Enabled,
				"type": f.Type, "actions": f.Actions, "action": action, "action_value": actionValue,
				"conditions": f.Terms, "condition": f.Condition, "match_type": f.MatchType, "raw": f.Raw,
			}
			if err := s.b.add("filters", doc); err != nil {
				return err
			}
		}
	}
	return s.replace("filters", keep)
}

func (s *tbSyncRun) syncEvents(profileDir string) error {
	evs, err := tbprofile.ReadCalendarEvents(profileDir)
	if err != nil {
		s.warn("calendar: %v", err)
		return nil
	}
	keep := map[string]bool{}
	for _, e := range evs {
		keep[e.ID] = true
		doc := map[string]any{
			"id": e.ID, "calendar_id": e.CalendarID, "title": e.Title, "start": tbFormatTime(e.Start),
			"end": tbFormatTime(e.End), "location": e.Location,
		}
		if err := s.b.add("events", doc); err != nil {
			return err
		}
	}
	return s.replace("events", keep)
}

func tbLastStoredMessage(db *store.Store, folderKey string) (*tbMessageDoc, error) {
	docs, err := tbQueryMessages(db, `json_extract(data,'$.folder_key') = ?`, []any{folderKey}, `json_extract(data,'$.offset') DESC`, 1)
	if err != nil || len(docs) == 0 {
		return nil, err
	}
	return &docs[0], nil
}

// tbResumeValid detects compaction that kept or grew the file size.
func tbResumeValid(path string, offset int64, last *tbMessageDoc) bool {
	if ok, err := tbprofile.BoundaryAt(path, offset); err != nil || !ok {
		return false
	}
	if last == nil {
		return true
	}
	_, err := tbprofile.ReadMessageAt(path, last.Offset, last.Length, last.MessageID)
	return err == nil
}

type tbFolderResult struct {
	New     int
	Fresh   int
	Pruned  int
	Scanned bool
}

// errTBReparse stops an incremental scan whose re-read message was expunged: its row may belong to an earlier copy.
var errTBReparse = errors.New("re-read message expunged")

func (s *tbSyncRun) ingestFolder(acc tbprofile.Account, f tbprofile.Folder, identityEmails map[string]bool, alreadyFresh int) (tbFolderResult, error) {
	var res tbFolderResult
	info, err := os.Stat(f.MboxPath)
	if err != nil {
		s.warn("folder %s/%s: %v", acc.Key, f.Path, err)
		s.unread = true
		return res, nil
	}
	key := tbFolderKey(acc.Key, f.Path)
	st, ok, err := s.db.GetMboxState(f.MboxPath)
	if err != nil {
		return res, err
	}
	if ok && st.FolderKey != key {
		if res.Pruned, err = s.pruneMsgs(st.FolderKey, tbWhereMboxPath, f.MboxPath, nil, nil); err != nil {
			return res, err
		}
		ok = false
	}
	reparse := s.opts.Full || !ok || info.Size() < st.Size || info.Size() < st.LastOffset
	// MTime 0 marks a checkpoint left by a capped or cancelled scan.
	if !reparse && st.MTime != 0 && info.Size() == st.Size && info.ModTime().Unix() == st.MTime {
		return res, nil
	}
	res.Scanned = true
	var last *tbMessageDoc
	if !reparse {
		if last, err = tbLastStoredMessage(s.db, key); err != nil {
			return res, err
		}
		if !tbResumeValid(f.MboxPath, st.LastOffset, last) {
			reparse, last = true, nil
		}
	}
	// Thunderbird rewrites X-Mozilla-Status in place, so a changed mtime can mean changed flags at the same size.
	if !reparse && info.ModTime().Unix() != st.MTime {
		valid, pruned, err := s.refreshFlags(key, f.MboxPath, st.LastOffset)
		res.Pruned += pruned
		if err != nil {
			return res, err
		}
		if !valid {
			reparse, last = true, nil
		}
	}
	var (
		startAt, end, resume, lastStart int64
		lastID                          string
		keepMsgs, keepAtts, stored      map[string]bool
		scanErr                         error
	)
	for {
		startAt, lastID = 0, ""
		if !reparse {
			startAt = st.LastOffset
			if last != nil && last.Offset > startAt {
				lastID = last.ID
			}
		} else if stored == nil && s.opts.MaxNewMessages > 0 {
			// Rows already stored do not count against the cap, so a capped re-parse advances on every run.
			stored = map[string]bool{}
			if err := s.keepExisting("messages", `json_extract(data,'$.folder_key') = ?`, stored, key); err != nil {
				return res, err
			}
		}
		keepMsgs, keepAtts, res.New, res.Fresh = map[string]bool{}, map[string]bool{}, 0, 0
		resume, lastStart = startAt, -1
		end, scanErr = s.scanFolder(acc, f, key, startAt, identityEmails, func(raw tbprofile.RawMessage, m *tbprofile.Message, doc tbMessageDoc) error {
			reread := lastID != "" && doc.ID == lastID
			if reread && m.Expunged {
				return errTBReparse
			}
			fresh := !reread && !stored[doc.ID]
			if fresh && s.opts.MaxNewMessages > 0 && alreadyFresh+res.Fresh >= s.opts.MaxNewMessages {
				return errTBSyncCap
			}
			resume, lastStart = raw.Offset+raw.Length, raw.Start
			if m.Expunged {
				return nil
			}
			keepMsgs[doc.ID] = true
			if !reread {
				res.New++
			}
			if fresh {
				res.Fresh++
			}
			return s.addMessage(acc, f, key, m, doc, keepAtts)
		})
		if !errors.Is(scanErr, errTBReparse) {
			break
		}
		reparse = true
	}
	capped := errors.Is(scanErr, errTBSyncCap)
	cancelled := errors.Is(scanErr, context.Canceled) || errors.Is(scanErr, context.DeadlineExceeded)
	if scanErr != nil && !capped && !cancelled {
		return res, fmt.Errorf("parsing %s/%s: %w", acc.Key, f.Path, scanErr)
	}
	if err := s.b.flush("messages"); err != nil {
		return res, err
	}
	if err := s.b.flush("attachments"); err != nil {
		return res, err
	}
	if lastID != "" && lastStart >= 0 && !keepMsgs[lastID] {
		n, err := s.pruneMsgs(key, "id = ?", lastID, keepMsgs, keepAtts)
		res.Pruned += n
		if err != nil {
			return res, err
		}
	}
	state := store.MboxState{Path: f.MboxPath, FolderKey: key, Size: info.Size(), MTime: info.ModTime().Unix(), LastOffset: end}
	switch {
	case (capped || cancelled) && reparse && ok:
		// The rows a partial re-parse of a known folder would prune are still unknown: force a re-parse next run.
		state.Size, state.MTime, state.LastOffset = math.MaxInt64, 0, 0
	case capped || cancelled:
		state.LastOffset, state.MTime = resume, 0
	default:
		if reparse {
			n, err := s.pruneMsgs(key, tbWhereMboxPath, f.MboxPath, keepMsgs, keepAtts)
			res.Pruned += n
			if err != nil {
				return res, err
			}
		}
		if lastStart >= 0 {
			// Resume at the last message so a copy caught mid-write is read again.
			state.LastOffset = lastStart
		}
	}
	s.sum.Capped = s.sum.Capped || capped
	if err := s.db.SaveMboxState(state); err != nil {
		return res, err
	}
	if cancelled {
		return res, scanErr
	}
	return res, nil
}

const tbFlagHeaderBytes = 8 << 10

// refreshFlags re-reads the status header of the stored messages of key before end; valid is false when a row no longer matches its offset.
func (s *tbSyncRun) refreshFlags(key, mboxPath string, end int64) (valid bool, pruned int, err error) {
	type row struct {
		id, mid     string
		off, length int64
		flags       [4]bool
	}
	q, err := s.db.DB().Query(`SELECT id, json_extract(data,'$.offset','$.length','$.message_id','$.read','$.replied','$.flagged','$.forwarded')
		FROM resources WHERE resource_type = 'messages' AND json_extract(data,'$.folder_key') = ? AND json_extract(data,'$.mbox_path') = ?
		AND json_extract(data,'$.offset') < ? ORDER BY json_extract(data,'$.offset')`, key, mboxPath, end)
	if err != nil {
		return false, 0, err
	}
	var rows []row
	for q.Next() {
		var id, vals string
		if err := q.Scan(&id, &vals); err != nil {
			_ = q.Close()
			return false, 0, err
		}
		var arr []any
		if err := json.Unmarshal([]byte(vals), &arr); err != nil || len(arr) != 7 {
			_ = q.Close()
			return false, 0, fmt.Errorf("stored message %s: bad fields", id)
		}
		r := row{id: id}
		off, _ := arr[0].(float64)
		length, _ := arr[1].(float64)
		r.off, r.length = int64(off), int64(length)
		r.mid, _ = arr[2].(string)
		for i := range r.flags {
			r.flags[i], _ = arr[3+i].(bool)
		}
		rows = append(rows, r)
	}
	_ = q.Close()
	if err := q.Err(); err != nil {
		return false, 0, err
	}
	s.sum.FlagFolders++
	f, err := os.Open(filepath.Clean(mboxPath))
	if err != nil {
		return false, 0, nil
	}
	defer f.Close()
	buf := make([]byte, tbFlagHeaderBytes)
	changed := map[string][4]bool{}
	expunged := map[string]bool{}
	for _, r := range rows {
		hs, err := tbprofile.ReadHeaderStatus(f, r.off, r.length, buf)
		if err != nil || hs.MessageID != "" && r.mid != "" && hs.MessageID != r.mid {
			return false, 0, nil
		}
		if !hs.HasStatus {
			continue
		}
		if hs.Status&tbprofile.StatusExpunged != 0 {
			expunged[r.id] = true
			continue
		}
		nf := [4]bool{hs.Status&tbprofile.StatusRead != 0, hs.Status&tbprofile.StatusReplied != 0,
			hs.Status&tbprofile.StatusFlagged != 0, hs.Status&tbprofile.StatusForwarded != 0}
		if nf != r.flags {
			changed[r.id] = nf
		}
	}
	batch := make([]json.RawMessage, 0, len(changed))
	for id, nf := range changed {
		raw, err := s.db.Get("messages", id)
		if err != nil {
			return false, 0, err
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return false, 0, err
		}
		doc["read"], doc["replied"], doc["flagged"], doc["forwarded"] = nf[0], nf[1], nf[2], nf[3]
		out, err := json.Marshal(doc)
		if err != nil {
			return false, 0, err
		}
		batch = append(batch, out)
	}
	if len(batch) > 0 {
		if _, _, err := s.db.UpsertBatch("messages", batch); err != nil {
			return false, 0, err
		}
	}
	s.sum.FlagsUpdated += len(batch)
	pruned, err = s.dropMsgs(key, expunged, nil, nil)
	return true, pruned, err
}

func (s *tbSyncRun) scanFolder(acc tbprofile.Account, f tbprofile.Folder, key string, startAt int64, identityEmails map[string]bool,
	fn func(raw tbprofile.RawMessage, m *tbprofile.Message, doc tbMessageDoc) error) (int64, error) {
	return tbprofile.ScanMbox(f.MboxPath, startAt, func(raw tbprofile.RawMessage) error {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		m := tbprofile.ParseMessage(raw.Data)
		return fn(raw, m, tbBuildMessageDoc(acc, f, key, raw, m, identityEmails))
	})
}

func (s *tbSyncRun) addMessage(acc tbprofile.Account, f tbprofile.Folder, key string, m *tbprofile.Message, doc tbMessageDoc, keepAtts map[string]bool) error {
	if err := s.b.add("messages", doc); err != nil {
		return err
	}
	for _, a := range m.Attachments {
		ad := tbAttachmentDoc{
			ID: doc.ID + ":" + strconv.Itoa(a.Index), MessageID: doc.ID, Index: a.Index, Filename: a.Filename,
			ContentType: a.ContentType, SizeBytes: a.SizeBytes, Account: acc.Key, Folder: f.Name, FolderPath: f.Path,
			FolderKey: key, Date: doc.Date, FromAddr: doc.FromAddr, Subject: doc.Subject, Inline: &a.Inline,
		}
		keepAtts[ad.ID] = true
		if err := s.b.add("attachments", ad); err != nil {
			return err
		}
	}
	return nil
}

func tbBuildMessageDoc(acc tbprofile.Account, f tbprofile.Folder, key string, raw tbprofile.RawMessage, m *tbprofile.Message, identityEmails map[string]bool) tbMessageDoc {
	root := tbprofile.ThreadRoot(m.MessageID, m.InReplyTo, m.References)
	id := tbprofile.MessageKey(acc.Key, f.Path, m.MessageID, raw.Offset)
	threadID := id
	if root != "" {
		threadID = tbprofile.ThreadID(root)
	}
	attCount := 0
	for _, a := range m.Attachments {
		if !a.Inline {
			attCount++
		}
	}
	return tbMessageDoc{
		ID: id, Account: acc.Key, AccountName: acc.Name(), Folder: f.Name, FolderPath: f.Path, FolderKey: key,
		Date: tbFormatTime(m.Date), FromAddr: m.FromAddr, FromName: m.FromName, To: m.To, Cc: m.Cc, Subject: m.Subject,
		MessageID: m.MessageID, InReplyTo: m.InReplyTo, References: m.References, ThreadID: threadID, ThreadRoot: root,
		Read: m.Read, Replied: m.Replied, Flagged: m.Flagged, Forwarded: m.Forwarded, Outgoing: identityEmails[m.FromAddr],
		SizeBytes: raw.Length, HasAttachments: attCount > 0, AttachmentCount: attCount,
		ListID: m.ListID, ListUnsubscribe: m.ListUnsubscribe, Automated: m.Automated, AuthResults: m.AuthResults, BodyText: m.BodyText,
		MboxPath: f.MboxPath, Offset: raw.Offset, Length: raw.Length,
	}
}

// tbResolveThreadRoots joins replies whose parent had no References to the parent's root.
func tbResolveThreadRoots(db *store.Store) error {
	rows, err := db.DB().Query(`SELECT id, COALESCE(json_extract(data,'$.message_id'),''), COALESCE(json_extract(data,'$.thread_root'),'')
		FROM resources WHERE resource_type = 'messages'`)
	if err != nil {
		return err
	}
	type entry struct{ id, mid, root string }
	var all []entry
	rootOf := map[string]string{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.mid, &e.root); err != nil {
			_ = rows.Close()
			return err
		}
		all = append(all, e)
		if e.mid != "" && e.root != "" && (rootOf[e.mid] == "" || rootOf[e.mid] == e.mid) {
			rootOf[e.mid] = e.root
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	resolve := func(r string) string {
		seen := map[string]bool{}
		for !seen[r] {
			seen[r] = true
			next := rootOf[r]
			if next == "" || next == r {
				return r
			}
			r = next
		}
		return r
	}
	var batch []json.RawMessage
	for _, e := range all {
		if e.root == "" {
			continue
		}
		final := resolve(e.root)
		if final == e.root {
			continue
		}
		raw, err := db.Get("messages", e.id)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		doc["thread_root"], doc["thread_id"] = final, tbprofile.ThreadID(final)
		out, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		batch = append(batch, out)
	}
	if len(batch) == 0 {
		return nil
	}
	_, _, err = db.UpsertBatch("messages", batch)
	return err
}

// tbFolderMessageStats returns folder_key -> [total, unread, flagged].
func tbFolderMessageStats(db *store.Store) (map[string][3]int, error) {
	rows, err := db.DB().Query(`SELECT COALESCE(json_extract(data,'$.folder_key'),''),
		COUNT(*),
		SUM(CASE WHEN json_extract(data,'$.read') THEN 0 ELSE 1 END),
		SUM(CASE WHEN json_extract(data,'$.flagged') THEN 1 ELSE 0 END)
		FROM resources WHERE resource_type = 'messages' GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][3]int{}
	for rows.Next() {
		var key string
		var total, unread, flagged int
		if err := rows.Scan(&key, &total, &unread, &flagged); err != nil {
			return nil, err
		}
		out[key] = [3]int{total, unread, flagged}
	}
	return out, rows.Err()
}

func runTBSyncCommand(cmd *cobra.Command, flags *rootFlags, resourcesCSV string, full bool, dbPath string) error {
	selected, err := parseTBResources(resourcesCSV)
	if err != nil {
		return usageErr(err)
	}
	profileDir, err := resolveTBProfile(flags)
	if dbPath == "" {
		dbPath = defaultDBPath("thunderbird-pp-cli")
		if err == nil {
			dbPath = tbSyncDBPath(profileDir)
		}
	}
	if errors.Is(err, tbprofile.ErrNoProfile) {
		fmt.Fprintln(cmd.ErrOrStderr(), "no Thunderbird profile found; pass --profile <dir|name> or set THUNDERBIRD_PROFILE")
		sum := &tbSyncSummary{DBPath: dbPath, Full: full, Resources: map[string]int{}, Warnings: []string{}, ProfileNotFound: true}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printJSONFiltered(cmd.OutOrStdout(), sum, flags)
		}
		return nil
	}
	if err != nil {
		return err
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()
	opts := tbSyncOptions{Resources: selected, Full: full, OnUpgrade: func() {
		fmt.Fprintln(cmd.ErrOrStderr(), "store format upgrade: re-parsing every folder once (inline attachments, quoted HTML)")
	}}
	if cliutil.IsDogfoodEnv() {
		opts.MaxNewMessages = tbDogfoodMaxNewMessages
	}
	sum, err := runTBSync(cmd.Context(), db, profileDir, opts)
	if err != nil {
		return err
	}
	for _, w := range sum.Warnings {
		fmt.Fprintln(tbSafeWriter{cmd.ErrOrStderr()}, "warning:", w)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), sum, flags)
	}
	w := tbHumanOut(cmd)
	fmt.Fprintf(w, "Synced %s\n", sum.Profile)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "RESOURCE\tROWS")
	for _, r := range tbResourceTypes {
		if n, ok := sum.Resources[r]; ok {
			fmt.Fprintf(tw, "%s\t%d\n", r, n)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "%d messages parsed (%d folders scanned, %d unchanged, %d pruned) in %.1fs\n",
		sum.NewMessages, sum.FoldersScanned, sum.FoldersSkipped, sum.PrunedMessages, float64(sum.ElapsedMS)/1000)
	if sum.Capped {
		fmt.Fprintf(w, "stopped after %d messages for this run; run sync again to continue\n", tbDogfoodMaxNewMessages)
	}
	return nil
}

func parseTBResources(csv string) (map[string]bool, error) {
	out := map[string]bool{}
	valid := map[string]bool{}
	for _, r := range tbResourceTypes {
		valid[r] = true
	}
	for _, r := range strings.Split(csv, ",") {
		r = strings.ToLower(strings.TrimSpace(r))
		if r == "" {
			continue
		}
		if !valid[r] {
			names := append([]string(nil), tbResourceTypes...)
			sort.Strings(names)
			return nil, fmt.Errorf("unknown resource %q (valid: %s)", r, strings.Join(names, ", "))
		}
		out[r] = true
	}
	return out, nil
}
