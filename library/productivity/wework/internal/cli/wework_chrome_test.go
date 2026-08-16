package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

func jwtWithExp(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	return header + "." + payload + ".sig"
}

func TestReadChromeSessionFromLevelDBBindsOriginAndCompleteBundle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "leveldb")
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	put := func(key, value string) {
		t.Helper()
		if err := db.Put([]byte(key), append([]byte{1}, []byte(value)...), nil); err != nil {
			t.Fatal(err)
		}
	}
	wework := jwtWithClaims(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":2000000000}`)
	robinhood := jwtWithClaims(`{"iss":"https://api.robinhood.com","exp":2100000000,"user_origin":"US","service_records":[]}`)
	weworkCache, _ := json.Marshal(map[string]any{"body": map[string]any{
		"access_token": wework, "refresh_token": "refresh-wework", "scope": "openid",
	}})
	robinhoodCache, _ := json.Marshal(map[string]any{
		"access_token": robinhood, "refresh_token": "refresh-robinhood",
	})
	put("_https://members.wework.com\x00\x01@@auth0spajs@@::client::audience::scope", string(weworkCache))
	put("_https://members.wework.com\x00\x01CurrentAccountUUID", `"account-1"`)
	put("_https://members.wework.com\x00\x01WWMemberType", `"1"`)
	put("_https://robinhood.com\x00\x01oauth", string(robinhoodCache))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	got, scanned, err := readChromeSessionFromDirs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if scanned != 1 {
		t.Fatalf("profiles scanned = %d, want 1", scanned)
	}
	if got.Token != wework || got.Refresh != "refresh-wework" || got.UUID != "account-1" || got.Member != "1" {
		t.Fatalf("session bundle did not stay bound to the WeWork origin: %#v", got)
	}
}

func TestDecodeChromeLocalStorageValue(t *testing.T) {
	got, err := decodeChromeLocalStorageValue(append([]byte{1}, []byte(`{"body":true}`)...))
	if err != nil || got != `{"body":true}` {
		t.Fatalf("single-byte value = %q, %v", got, err)
	}
	got, err = decodeChromeLocalStorageValue([]byte{0, '"', 0, '1', 0, '"', 0})
	if err != nil || chromeLocalStorageScalar(got) != "1" {
		t.Fatalf("UTF-16 scalar = %q, %v", got, err)
	}
}

func TestReadChromeProfileSessionRetriesIncoherentSnapshot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "leveldb")
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	key := "_https://members.wework.com\x00\x01CurrentAccountUUID"
	if err := db.Put([]byte(key), append([]byte{1}, []byte(`"account-1"`)...), nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	attempts := 0
	flakySnapshotter := func(source string) (string, func(), error) {
		attempts++
		if attempts == 1 {
			return "", func() {}, errors.New("simulated compaction race")
		}
		return snapshotChromeLevelDB(source)
	}
	_, uuid, _, originSeen, err := readChromeProfileSessionWithSnapshotter(dir, flakySnapshotter)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || !originSeen || uuid != "account-1" {
		t.Fatalf("retry result: attempts=%d origin=%t uuid=%q", attempts, originSeen, uuid)
	}
}

func TestPickFreshestChromeSessionKeepsMatchingRefresh(t *testing.T) {
	older := jwtWithExp(1000000000)
	newer := jwtWithExp(2000000000)
	got := pickFreshestChromeSession([]chromeDiskSession{
		{Token: newer, Refresh: "refresh-new"},
		{Token: older, Refresh: "refresh-old"},
	})
	if got.Token != newer || got.Refresh != "refresh-new" {
		t.Fatalf("picked session did not keep the matching rotating refresh token")
	}
}

func jwtWithClaims(claimsJSON string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(claimsJSON))
	return header + "." + payload + ".sig"
}

func TestTokenIsWeWork(t *testing.T) {
	wework := jwtWithClaims(`{"exp":2000000000,"iss":"https://idp.wework.com/","azp":"public-client"}`)
	if !tokenIsWeWork(wework) {
		t.Fatal("expected WeWork token to be recognized")
	}
	other := jwtWithClaims(`{"exp":2100000000,"iss":"https://api.robinhood.com","user_origin":"US","service_records":[]}`)
	if tokenIsWeWork(other) {
		t.Fatal("expected non-WeWork token to be rejected")
	}
	if tokenIsWeWork("not.a.jwt") || tokenIsWeWork("") {
		t.Fatal("malformed tokens must be rejected")
	}
}

func TestItoaScan(t *testing.T) {
	cases := map[int]string{0: "0", 3: "3", 42: "42"}
	for in, want := range cases {
		if got := itoaScan(in); got != want {
			t.Errorf("itoaScan(%d) = %q, want %q", in, got, want)
		}
	}
}
