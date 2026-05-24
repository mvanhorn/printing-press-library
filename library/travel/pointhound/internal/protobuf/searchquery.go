// Package protobuf encodes Pointhound search queries into the URL-safe
// base64 protobuf shape that the /flights?q=<...> endpoint accepts.
package protobuf

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	wireTypeVarint = 0
	wireTypeBytes  = 2
)

// CabinClass codes match the protobuf wire enum.
const (
	CabinEconomy        = 1
	CabinPremiumEconomy = 2
	CabinBusiness       = 3
	CabinFirst          = 4
)

// SearchQuery captures the user's search intent. Date is ISO YYYY-MM-DD.
type SearchQuery struct {
	OriginCode      string
	OriginName      string
	DestinationCode string
	DestinationName string
	Date            string
	Cabin           int
	Passengers      int
}

// Encode returns the URL-safe base64 protobuf for sq, suitable for the
// /flights?q=<value> URL parameter.
func (sq SearchQuery) Encode() (string, error) {
	if sq.OriginCode == "" || sq.DestinationCode == "" {
		return "", fmt.Errorf("origin and destination codes are required")
	}
	if sq.Date == "" {
		return "", fmt.Errorf("date (YYYY-MM-DD) is required")
	}
	if sq.Cabin < 1 || sq.Cabin > 4 {
		return "", fmt.Errorf("cabin must be 1-4 (1=economy, 2=premium_economy, 3=business, 4=first)")
	}
	if sq.Passengers < 1 || sq.Passengers > 9 {
		return "", fmt.Errorf("passengers must be 1-9")
	}

	var buf []byte
	origin := encodeOrigin(sq.OriginCode, sq.OriginName)
	buf = appendTag(buf, 1, wireTypeBytes)
	buf = appendVarint(buf, uint64(len(origin)))
	buf = append(buf, origin...)

	dest := encodeOrigin(sq.DestinationCode, sq.DestinationName)
	buf = appendTag(buf, 2, wireTypeBytes)
	buf = appendVarint(buf, uint64(len(dest)))
	buf = append(buf, dest...)

	buf = appendTag(buf, 3, wireTypeBytes)
	buf = appendVarint(buf, uint64(len(sq.Date)))
	buf = append(buf, []byte(sq.Date)...)

	buf = appendTag(buf, 5, wireTypeVarint)
	buf = appendVarint(buf, uint64(sq.Cabin))

	buf = appendTag(buf, 6, wireTypeVarint)
	buf = appendVarint(buf, uint64(sq.Passengers))

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func encodeOrigin(code, name string) []byte {
	var buf []byte
	if code != "" {
		buf = appendTag(buf, 1, wireTypeBytes)
		buf = appendVarint(buf, uint64(len(code)))
		buf = append(buf, []byte(code)...)
	}
	if name != "" {
		buf = appendTag(buf, 2, wireTypeBytes)
		buf = appendVarint(buf, uint64(len(name)))
		buf = append(buf, []byte(name)...)
	}
	return buf
}

func appendTag(buf []byte, fieldNum, wireType int) []byte {
	return appendVarint(buf, uint64(fieldNum)<<3|uint64(wireType))
}

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

// CabinForString maps the user-friendly cabin string to the wire enum value.
func CabinForString(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "economy":
		return CabinEconomy, nil
	case "premium", "premium_economy", "premium-economy":
		return CabinPremiumEconomy, nil
	case "business":
		return CabinBusiness, nil
	case "first":
		return CabinFirst, nil
	}
	return 0, fmt.Errorf("unknown cabin %q (expected economy/premium_economy/business/first)", s)
}
