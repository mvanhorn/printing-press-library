package cli

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func atom(kind string, payload []byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, uint32(len(payload)+8))
	b.WriteString(kind)
	b.Write(payload)
	return b.Bytes()
}

func writeSyntheticMemo(t *testing.T, transcriptJSON string) string {
	t.Helper()
	payload := atom("moov", atom("trak", atom("udta", atom("tsrp", []byte(transcriptJSON)))))
	path := filepath.Join(t.TempDir(), "memo.m4a")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractTranscriptFromSyntheticM4A(t *testing.T) {
	path := writeSyntheticMemo(t, `{"attributedString":["Hello world.",{"timeRange":[1.25,0.5]},"Second sentence.",{"timeRange":[3.5,0.5]}]}`)
	text, segments, err := extractTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 2 {
		t.Fatalf("segments=%d want=2", len(segments))
	}
	if !strings.Contains(text, "[0:01] Hello world. Second sentence.") || segments[1].Time != 3.5 {
		t.Fatalf("unexpected transcript: %q", text)
	}
}

func TestReadTranscriptRejectsOversizedAtom(t *testing.T) {
	large := bytes.Repeat([]byte{'x'}, maxTranscriptAtomBytes+1)
	payload := atom("moov", atom("trak", atom("udta", atom("tsrp", large))))
	path := filepath.Join(t.TempDir(), "oversized.m4a")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := readTranscriptJSON(path)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error=%v want transcript-too-large error", err)
	}
}

func createTestDB(t *testing.T, schema string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "CloudRecordings.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenDBIsReadOnly(t *testing.T) {
	path := createTestDB(t, `CREATE TABLE ZCLOUDRECORDING (Z_PK INTEGER PRIMARY KEY, ZDATE FLOAT, ZPATH TEXT, ZDURATION FLOAT, ZLOCALDURATION FLOAT, ZUNIQUEID TEXT, ZENCRYPTEDTITLE TEXT, ZCUSTOMLABEL TEXT);`)
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE SHOULD_NOT_EXIST (id INTEGER)"); err == nil {
		t.Fatal("read-only database accepted a write")
	}
}

func TestValidateSchemaRejectsMissingColumns(t *testing.T) {
	path := createTestDB(t, `CREATE TABLE ZCLOUDRECORDING (Z_PK INTEGER PRIMARY KEY, ZDATE FLOAT);`)
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	err = validateVoiceMemosSchema(db)
	if err == nil || !strings.Contains(err.Error(), "unsupported Voice Memos schema") {
		t.Fatalf("error=%v", err)
	}
}

func TestValidateSchemaAcceptsRequiredColumns(t *testing.T) {
	path := createTestDB(t, `CREATE TABLE ZCLOUDRECORDING (Z_PK INTEGER PRIMARY KEY, ZDATE FLOAT, ZPATH TEXT, ZDURATION FLOAT, ZLOCALDURATION FLOAT, ZUNIQUEID TEXT, ZENCRYPTEDTITLE TEXT, ZCUSTOMLABEL TEXT);`)
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := validateVoiceMemosSchema(db); err != nil {
		t.Fatal(err)
	}
}

func TestQueryMemosUsesSyntheticStoreAndTranscript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CloudRecordings.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	schema := `CREATE TABLE ZCLOUDRECORDING (Z_PK INTEGER PRIMARY KEY, ZDATE FLOAT, ZPATH TEXT, ZDURATION FLOAT, ZLOCALDURATION FLOAT, ZUNIQUEID TEXT, ZENCRYPTEDTITLE TEXT, ZCUSTOMLABEL TEXT);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ZCLOUDRECORDING (Z_PK,ZDATE,ZPATH,ZDURATION,ZLOCALDURATION,ZUNIQUEID,ZCUSTOMLABEL) VALUES (?,?,?,?,?,?,?)`, 7, 800000000.0, "memo.m4a", 65.0, 65.0, "synthetic-uuid", "Synthetic memo"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	memoPath := writeSyntheticMemo(t, `{"attributedString":["Synthetic transcript.",{"timeRange":[0.5,0.5]}]}`)
	data, err := os.ReadFile(memoPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo.m4a"), data, 0600); err != nil {
		t.Fatal(err)
	}

	oldCfg := cfg
	cfg.DBPath = path
	cfg.RecordingsDir = dir
	t.Cleanup(func() { cfg = oldCfg })
	memos, err := queryMemos(10, 0, "Synthetic", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(memos) != 1 || memos[0].ID != 7 || !memos[0].HasTranscript || !memos[0].Exists {
		t.Fatalf("unexpected memos: %+v", memos)
	}
}

func TestCopyFileCreatesPrivateOutput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.m4a")
	destination := filepath.Join(dir, "destination.m4a")
	if err := os.WriteFile(source, []byte("synthetic audio"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(source, destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("mode=%#o want=0600", got)
	}
}

func TestCopyFileRejectsSymlinkDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.m4a")
	target := filepath.Join(dir, "target.m4a")
	destination := filepath.Join(dir, "destination.m4a")
	if err := os.WriteFile(source, []byte("synthetic audio"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("do not replace"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, destination); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(source, destination); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error=%v want symlink rejection", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "do not replace" {
		t.Fatalf("symlink target was modified: %q", data)
	}
}

func TestOpenDBHandlesLiteralURICharacters(t *testing.T) {
	dir := t.TempDir()
	safePath := filepath.Join(dir, "store.db")
	path := filepath.Join(dir, "voice?memo#store.db")
	db, err := sql.Open("sqlite", safePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE test (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(safePath, path); err != nil {
		t.Fatal(err)
	}
	ro, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ro.Close() }()
	if _, err := ro.Exec(`INSERT INTO test VALUES (1)`); err == nil {
		t.Fatal("database with URI characters was writable")
	}
}

func TestResolveRecordingPathRejectsEscapes(t *testing.T) {
	base := t.TempDir()
	if _, err := resolveRecordingPath(base, "../../outside.m4a"); err == nil {
		t.Fatal("accepted traversal path")
	}
	if _, err := resolveRecordingPath(base, "/tmp/outside.m4a"); err == nil {
		t.Fatal("accepted absolute path")
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "memo.m4a"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveRecordingPath(base, filepath.Join("linked", "memo.m4a")); err == nil {
		t.Fatal("accepted escaping symlink")
	}
}

func TestTranscriptCanAppearInSecondTrack(t *testing.T) {
	transcript := []byte(`{"attributedString":["Second track.",{"timeRange":[1,1]}]}`)
	firstTrack := atom("trak", atom("mdia", []byte("synthetic")))
	secondTrack := atom("trak", atom("udta", atom("tsrp", transcript)))
	payload := atom("moov", append(firstTrack, secondTrack...))
	path := filepath.Join(t.TempDir(), "multi-track.m4a")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	text, _, err := extractTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Second track.") {
		t.Fatalf("text=%q", text)
	}
}

func TestZeroSizedFinalTranscriptAtom(t *testing.T) {
	transcript := []byte(`{"attributedString":["Final atom.",{"timeRange":[1,1]}]}`)
	zeroTSRP := append([]byte{0, 0, 0, 0, 't', 's', 'r', 'p'}, transcript...)
	payload := atom("moov", atom("trak", atom("udta", zeroTSRP)))
	path := filepath.Join(t.TempDir(), "zero-size.m4a")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	text, _, err := extractTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Final atom.") {
		t.Fatalf("text=%q", text)
	}
}
