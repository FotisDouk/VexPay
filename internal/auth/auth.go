// Package auth handles API-key authentication for the REST API.
//
// API keys are high-entropy random tokens. They are stored only as SHA-256
// hashes and compared in constant time. Because the tokens carry full entropy,
// a fast hash is appropriate here (unlike user passwords, which need a slow KDF).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"sync"
)

// Principal is the authenticated caller behind an API key.
type Principal struct {
	MerchantID string
	Sandbox    bool
}

// Key is a stored API key record (the raw secret is never retained).
type Key struct {
	Hash       string
	MerchantID string
	Sandbox    bool
}

// GenerateKey returns a new raw API key and its record. The raw value is shown
// to the user once and never stored.
func GenerateKey(merchantID string, sandbox bool) (raw string, key Key) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("vexpay: crypto/rand unavailable: " + err.Error())
	}
	prefix := "vpk_live_"
	if sandbox {
		prefix = "vpk_test_"
	}
	raw = prefix + hex.EncodeToString(b[:])
	return raw, Key{Hash: hashKey(raw), MerchantID: merchantID, Sandbox: sandbox}
}

// NewKey builds a stored record for an externally supplied raw key (e.g. one
// configured via environment). The raw value is hashed and not retained.
func NewKey(raw, merchantID string, sandbox bool) Key {
	return Key{Hash: hashKey(raw), MerchantID: merchantID, Sandbox: sandbox}
}

// Store authenticates raw API keys against stored records.
type Store struct {
	mu   sync.RWMutex
	keys map[string]Key // hash -> key
}

// NewStore returns an empty key store.
func NewStore() *Store {
	return &Store{keys: make(map[string]Key)}
}

// Add stores a key record.
func (s *Store) Add(k Key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[k.Hash] = k
}

// Authenticate resolves a raw key to its Principal.
func (s *Store) Authenticate(raw string) (Principal, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Principal{}, false
	}
	h := hashKey(raw)
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Range with constant-time compare so lookup timing doesn't leak which
	// prefix matched.
	for hash, k := range s.keys {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(h)) == 1 {
			return Principal{MerchantID: k.MerchantID, Sandbox: k.Sandbox}, true
		}
	}
	return Principal{}, false
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
