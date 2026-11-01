//go:build !windows

package voice

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"os"
	"path/filepath"
	"syscall"
)

// A private leaf directory plus trusted ancestors prevents a different OS user
// from replacing a SQLite pathname between validation and the driver's open.
func verifyPrivateDirectory(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	for p := resolved; ; p = filepath.Dir(p) {
		fi, err := os.Lstat(p)
		if err != nil {
			return err
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe Voice directory: %s", p)
		}
		uid, owner := int64(os.Geteuid()), int64(st.Uid)
		if owner != uid && owner != 0 {
			return fmt.Errorf("Voice directory ancestor has an untrusted owner: %s", p)
		}
		if p == resolved && (owner != uid || fi.Mode().Perm()&0077 != 0) {
			return fmt.Errorf("Voice directory must be owned by the current user with mode 0700: %s", p)
		}
		stickyRoot := st.Uid == 0 && fi.Mode()&os.ModeSticky != 0
		if fi.Mode().Perm()&0022 != 0 && !stickyRoot {
			return fmt.Errorf("Voice directory ancestor is writable by another user: %s", p)
		}
		if err = cliutil.CheckDarwinPrivateFileACL(p, "Voice directory"); err != nil {
			return err
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
