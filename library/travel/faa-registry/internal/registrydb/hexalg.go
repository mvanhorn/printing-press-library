// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package registrydb

import (
	"fmt"
	"strconv"
	"strings"
)

// US N-number ↔ ICAO 24-bit address conversion. The FAA assigns the block
// A00001–ADF7C7 sequentially: a00001 = N1 ... adf7c7 = N99999. Registrations
// use digits plus a suffix drawn from the alphabet without I and O.
//
// Port of the reference algorithm (guillaumemichel/icao-nnumber_converter).

const (
	hexCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ" // 24 letters, no I/O
	hexDigits  = "0123456789"
)

var (
	suffixSize  = 1 + len(hexCharset)*(1+len(hexCharset)) // 601: "", "A".."Z", "AA".."ZZ"
	bucket4Size = 1 + len(hexCharset) + len(hexDigits)    // 35
	bucket3Size = len(hexDigits)*bucket4Size + suffixSize // 951
	bucket2Size = len(hexDigits)*bucket3Size + suffixSize // 10111
	bucket1Size = len(hexDigits)*bucket2Size + suffixSize // 101711
)

const (
	icaoFirst = 0xA00001
	icaoLast  = 0xADF7C7
)

// suffix returns the 0..600 offset's letter suffix ("", "A".."Z", "AA".."ZZ").
func suffix(offset int) string {
	if offset == 0 {
		return ""
	}
	char0 := hexCharset[(offset-1)/(len(hexCharset)+1)]
	rem := (offset - 1) % (len(hexCharset) + 1)
	if rem == 0 {
		return string(char0)
	}
	return string(char0) + string(hexCharset[rem-1])
}

// suffixOffset is the inverse of suffix.
func suffixOffset(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	if len(s) > 2 {
		return 0, fmt.Errorf("invalid suffix %q", s)
	}
	i0 := strings.IndexByte(hexCharset, s[0])
	if i0 < 0 {
		return 0, fmt.Errorf("invalid suffix character %q", s[0])
	}
	count := i0*(len(hexCharset)+1) + 1
	if len(s) == 2 {
		i1 := strings.IndexByte(hexCharset, s[1])
		if i1 < 0 {
			return 0, fmt.Errorf("invalid suffix character %q", s[1])
		}
		count += i1 + 1
	}
	return count, nil
}

// IcaoToTail converts a US ICAO 24-bit hex address (e.g. "A008C5") to its
// N-number ("N101DQ"). Returns an error for addresses outside the US block.
func IcaoToTail(icao string) (string, error) {
	h := strings.ToLower(strings.TrimSpace(icao))
	h = strings.TrimPrefix(h, "0x")
	v, err := strconv.ParseInt(h, 16, 64)
	if err != nil {
		return "", fmt.Errorf("invalid hex address %q", icao)
	}
	if v < icaoFirst || v > icaoLast {
		return "", fmt.Errorf("hex address %s is outside the US civil block (A00001-ADF7C7)", strings.ToUpper(h))
	}
	i := int(v) - icaoFirst

	out := strings.Builder{}
	out.WriteByte('N')

	d1 := i/bucket1Size + 1
	rem := i % bucket1Size
	out.WriteString(strconv.Itoa(d1))
	if rem < suffixSize {
		return out.String() + suffix(rem), nil
	}
	rem -= suffixSize

	d2 := rem / bucket2Size
	rem = rem % bucket2Size
	out.WriteString(strconv.Itoa(d2))
	if rem < suffixSize {
		return out.String() + suffix(rem), nil
	}
	rem -= suffixSize

	d3 := rem / bucket3Size
	rem = rem % bucket3Size
	out.WriteString(strconv.Itoa(d3))
	if rem < suffixSize {
		return out.String() + suffix(rem), nil
	}
	rem -= suffixSize

	d4 := rem / bucket4Size
	rem = rem % bucket4Size
	out.WriteString(strconv.Itoa(d4))
	if rem == 0 {
		return out.String(), nil
	}
	return out.String() + string((hexCharset + hexDigits)[rem-1]), nil
}

// ValidTail checks that a string is a well-formed US N-number: after the
// optional leading N, 1-5 characters, starting with a digit 1-9, digits
// optionally followed by at most two trailing letters (I and O excluded).
func ValidTail(tail string) error {
	t := NormalizeTail(tail)
	if t == "" || len(t) > 5 {
		return fmt.Errorf("N-number %q must be 1-5 characters after the N", tail)
	}
	if t[0] < '1' || t[0] > '9' {
		return fmt.Errorf("N-number %q must start with a digit 1-9", tail)
	}
	letters := 0
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch {
		case c >= '0' && c <= '9':
			if letters > 0 {
				return fmt.Errorf("N-number %q is invalid: digits cannot follow letters", tail)
			}
		case strings.IndexByte(hexCharset, c) >= 0:
			letters++
			if letters > 2 {
				return fmt.Errorf("N-number %q is invalid: at most two trailing letters", tail)
			}
		default:
			return fmt.Errorf("N-number %q contains invalid character %q (I and O are not used)", tail, c)
		}
	}
	return nil
}

// TailToIcao converts a US N-number (with or without the leading N) to its
// ICAO 24-bit hex address, uppercase without prefix (e.g. "A008C5").
func TailToIcao(tail string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(tail))
	t = strings.TrimPrefix(t, "N")
	if t == "" || len(t) > 5 {
		return "", fmt.Errorf("invalid N-number %q", tail)
	}
	if t[0] < '1' || t[0] > '9' {
		return "", fmt.Errorf("N-number %q must start with a digit 1-9", tail)
	}
	for _, r := range t {
		if !strings.ContainsRune(hexCharset+hexDigits, r) {
			return "", fmt.Errorf("N-number %q contains invalid character %q (I and O are not used)", tail, r)
		}
	}

	v := icaoFirst
	d1 := int(t[0] - '0')
	v += (d1 - 1) * bucket1Size
	rest := t[1:]

	buckets := []int{bucket2Size, bucket3Size, bucket4Size}
	for level := 0; ; level++ {
		if rest == "" {
			break
		}
		if rest[0] >= '0' && rest[0] <= '9' {
			if level == 3 {
				// 5th char after 4 digits: single letter position
				off := strings.IndexByte(hexCharset+hexDigits, rest[0])
				v += off + 1
				rest = rest[1:]
				if rest != "" {
					return "", fmt.Errorf("invalid N-number %q", tail)
				}
				break
			}
			v += suffixSize + int(rest[0]-'0')*buckets[level]
			rest = rest[1:]
			continue
		}
		// letter suffix terminates the number
		if level == 3 {
			off := strings.IndexByte(hexCharset+hexDigits, rest[0])
			if off < 0 || len(rest) > 1 {
				return "", fmt.Errorf("invalid N-number %q", tail)
			}
			v += off + 1
			break
		}
		off, err := suffixOffset(rest)
		if err != nil {
			return "", err
		}
		v += off
		rest = ""
	}
	if v > icaoLast {
		return "", fmt.Errorf("N-number %q maps outside the US block", tail)
	}
	return strings.ToUpper(strconv.FormatInt(int64(v), 16)), nil
}
