// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

// Package milanote talks to Milanote's private web API: JSON REST under
// /api for reads, and the web app's socket.io channel (Engine.IO 4 over HTTP
// long-polling) for writes.
package milanote

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"
)

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Base62 encodes a non-negative integer with Milanote's id alphabet.
func Base62(n int64) string {
	if n <= 0 {
		return "0"
	}
	var out []byte
	for n > 0 {
		out = append([]byte{base62Alphabet[n%62]}, out...)
		n /= 62
	}
	return string(out)
}

// RandomBase62 returns n random characters from the base62 alphabet.
func RandomBase62(n int) string {
	b := make([]byte, n)
	max := big.NewInt(62)
	for i := range b {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			b[i] = base62Alphabet[time.Now().UnixNano()%62]
			continue
		}
		b[i] = base62Alphabet[v.Int64()]
	}
	return string(b)
}

// IDGen mints element ids the way the web app does:
// base62(unix seconds) + clientId + base62(tick).
type IDGen struct {
	mu       sync.Mutex
	clientID string
	tick     int64
	now      func() time.Time
}

// NewIDGen starts a generator for the clientId and tick Milanote returned
// from /api/users/me.
func NewIDGen(clientID string, tick int64) *IDGen {
	if clientID == "" {
		clientID = RandomBase62(6)
	}
	return &IDGen{clientID: clientID, tick: tick, now: time.Now}
}

// Next returns a fresh element id.
func (g *IDGen) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tick++
	return Base62(g.now().Unix()) + g.clientID + Base62(g.tick)
}

// Tick reports the last tick handed out; actions carry it in user.clientTick.
func (g *IDGen) Tick() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.tick
}

// ClientID is the clientId embedded in every id.
func (g *IDGen) ClientID() string { return g.clientID }

// NewTaskID returns a to-do item id such as task-UyDA675rUgS8q.
func NewTaskID() string { return "task-" + RandomBase62(13) }

// nodeKey is the short random key the editor stamps on block nodes.
func nodeKey() string { return RandomBase62(5) }
