//go:build darwin

package voice

import (
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDarwinPrivateWritesRejectInheritedACLBeforeSecretBytes(t *testing.T) {
	restore, e := cliutil.SetHomeOverride(privateTestDir(t))
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	t.Setenv("GOOGLE_VOICE_PROFILE", "acl-test")
	session, _, e := Paths()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(session), 0700); e != nil {
		t.Fatal(e)
	}
	if b, e := exec.Command("chmod", "+a", "everyone allow read,file_inherit", filepath.Dir(session)).CombinedOutput(); e != nil {
		t.Fatalf("ACL setup failed: %s %v", b, e)
	}
	if e = SaveSession(fakeSession()); e == nil {
		t.Fatal("unsafe inherited credential ACL accepted")
	}
	if _, e = os.Stat(session); !os.IsNotExist(e) {
		t.Fatal("credential bytes were published before privacy verification")
	}
	path := filepath.Join(privateTestDir(t), "mirror.sqlite3")
	m, e := OpenMirror(path, "account-id", true)
	if e != nil {
		t.Fatal(e)
	}
	m.Close()
	if b, e := exec.Command("chmod", "+a", "everyone allow read", path).CombinedOutput(); e != nil {
		t.Fatalf("ACL setup failed: %s %v", b, e)
	}
	if db, e := OpenMirror(path, "account-id", false); e == nil {
		db.Close()
		t.Fatal("public effective ACL accepted for message database")
	}
}

func TestMirrorRefusesForeignWritableDirectoryACL(t *testing.T) {
	dir := privateTestDir(t)
	m, e := OpenMirror(filepath.Join(dir, "messages.sqlite3"), "account", true)
	if e != nil {
		t.Fatal(e)
	}
	m.Close()
	if b, e := exec.Command("chmod", "+a", "everyone allow add_file,delete_child,search", dir).CombinedOutput(); e != nil {
		t.Fatalf("ACL: %v %s", e, b)
	}
	if m, e = OpenMirror(filepath.Join(dir, "messages.sqlite3"), "account", true); e == nil {
		m.Close()
		t.Fatal("accepted foreign writable directory ACL")
	}
}
