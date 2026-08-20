package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func main() {
	dataDir, _ := auth.ChromeDataDir()
	profileDir := dataDir + "/Default"

	// Hunt for JWTs across ALL records in localStorage LevelDB (not just the
	// Superhuman origin prefix). The earlier byte-grep showed 5 eyJ sequences
	// somewhere in the .ldb files; let's find which keys they sit under.
	srcDir := filepath.Join(profileDir, "Local Storage", "leveldb")
	snap, err := snapshotCopy(srcDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snapshot: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(snap)

	db, err := leveldb.OpenFile(snap, &opt.Options{ReadOnly: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	jwtsFound := 0
	totalKeys := 0
	originSummary := make(map[string]int)
	for iter.Next() {
		totalKeys++
		k := iter.Key()
		v := iter.Value()

		// Categorize the origin from the key
		origin := "?"
		if len(k) > 1 && k[0] == '_' {
			// Pattern: _<origin>\x00\x01<key>
			nul := -1
			for i := 1; i < len(k); i++ {
				if k[i] == 0 {
					nul = i
					break
				}
			}
			if nul > 1 {
				origin = string(k[1:nul])
			}
		} else if len(k) > 5 && string(k[:5]) == "META:" {
			origin = "META:" + originAfterMeta(k[5:])
		} else if len(k) >= 7 && string(k[:7]) == "VERSION" {
			origin = "(version-marker)"
		}
		originSummary[origin]++

		// Try both decodings on the value
		one, two := decodeBoth(v)
		isJWT := func(s string) bool { return strings.HasPrefix(s, "eyJ") && len(s) > 100 }

		if isJWT(one) || isJWT(two) {
			jwtsFound++
			fmt.Fprintf(os.Stderr, "=== JWT match #%d ===\n", jwtsFound)
			fmt.Fprintf(os.Stderr, "  key (len=%d): origin=%q  hex-tail=%x\n", len(k), origin, k[max(0, len(k)-50):])
			// Decode the key bytes after the prefix
			if origin != "?" && len(k) > len(origin)+3 {
				keyTail := k[len(origin)+3:]
				ot, tt := decodeBoth(keyTail)
				fmt.Fprintf(os.Stderr, "  key tail one-byte: %q\n", truncate(ot, 80))
				fmt.Fprintf(os.Stderr, "  key tail utf-16:  %q\n", truncate(tt, 80))
			}
			if isJWT(one) {
				fmt.Fprintf(os.Stderr, "  value (one-byte JWT): %s... (len=%d)\n", one[:50], len(one))
			}
			if isJWT(two) {
				fmt.Fprintf(os.Stderr, "  value (utf-16 JWT):   %s... (len=%d)\n", two[:50], len(two))
			}
		}
	}

	fmt.Fprintf(os.Stderr, "\n=== Total keys: %d ===\n", totalKeys)
	fmt.Fprintf(os.Stderr, "=== JWTs found: %d ===\n", jwtsFound)
	fmt.Fprintf(os.Stderr, "\n=== Origins in localStorage ===\n")
	for o, n := range originSummary {
		if strings.Contains(o, "superhuman") || strings.Contains(o, "google") || n > 5 {
			fmt.Fprintf(os.Stderr, "  %s: %d keys\n", o, n)
		}
	}
}

func snapshotCopy(srcDir string) (string, error) {
	dst, err := os.MkdirTemp("", "sh-probe-*")
	if err != nil {
		return "", err
	}
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		if e.IsDir() || e.Name() == "LOCK" {
			continue
		}
		s, err := os.Open(filepath.Join(srcDir, e.Name()))
		if err != nil {
			continue
		}
		d, _ := os.Create(filepath.Join(dst, e.Name()))
		_, _ = io.Copy(d, s)
		s.Close()
		d.Close()
	}
	return dst, nil
}

func decodeBoth(b []byte) (oneByte, utf16LE string) {
	if len(b) == 0 {
		return "", ""
	}
	rest := b
	if b[0] == 0x00 || b[0] == 0x01 {
		rest = b[1:]
	}
	one := string(rest)
	// UTF-16 LE
	if len(rest)%2 != 0 {
		rest = rest[:len(rest)-1]
	}
	if len(rest) == 0 {
		return one, ""
	}
	u16 := make([]uint16, len(rest)/2)
	for i := range u16 {
		u16[i] = uint16(rest[2*i]) | uint16(rest[2*i+1])<<8
	}
	// Manual UTF-16 LE -> Go string
	out := make([]byte, 0, len(u16))
	for _, c := range u16 {
		if c < 0x80 {
			out = append(out, byte(c))
		} else {
			// just placeholder for non-ASCII
			out = append(out, '?')
		}
	}
	return one, string(out)
}

func originAfterMeta(b []byte) string {
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func truncate(s string, n int) string {
	clean := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 0x20 && r <= 0x7e {
			clean = append(clean, r)
		} else {
			clean = append(clean, '·')
		}
	}
	if len(clean) > n {
		return string(clean[:n]) + "..."
	}
	return string(clean)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
