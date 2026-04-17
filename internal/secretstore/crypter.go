// Package secretstore provides AES-GCM envelope encryption for sensitive
// values stored in the database. A single master key (HIVE_MASTER_KEY) gates
// the entire deployment; each write generates a fresh 12-byte nonce.
//
// Values encrypted by this package carry an "enc:v1:" prefix. Reads accept
// both prefixed (ciphertext) and bare (legacy plaintext) values so rolling
// out encryption on an existing database doesn't need a migration step.
//
// When HIVE_MASTER_KEY is not set, Encrypt is a no-op and Decrypt passes
// through plaintext. This keeps dev/tests frictionless while letting
// operators opt into encryption by setting the env var in prod.
//
// This is not a replacement for a KMS/HSM, but it closes the "DB dump =
// secrets" attack surface, and the envelope format leaves room for a v2
// that wraps the key itself with a remote KMS.
package secretstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

const cryptPrefix = "enc:v1:"

// errNoKey is returned when decryption is requested but HIVE_MASTER_KEY isn't
// set. Callers should treat this as "fall back to plaintext".
var errNoKey = errors.New("HIVE_MASTER_KEY not configured")

// HasMasterKey reports whether HIVE_MASTER_KEY is configured.
func HasMasterKey() bool {
	_, err := derivedKey()
	return err == nil
}

// IsEncrypted reports whether a stored value carries the "enc:v1:" envelope
// tag. Lets callers scan tables for plaintext rows that should be rotated.
func IsEncrypted(stored string) bool {
	return strings.HasPrefix(stored, cryptPrefix)
}

// derivedKey reads HIVE_MASTER_KEY and returns its SHA-256 digest.
func derivedKey() ([]byte, error) {
	k := os.Getenv("HIVE_MASTER_KEY")
	if k == "" {
		return nil, errNoKey
	}
	sum := sha256.Sum256([]byte(k))
	return sum[:], nil
}

// Encrypt wraps plaintext with AES-GCM and the "enc:v1:" tag. When no master
// key is configured, returns the input unchanged so plaintext flows stay
// backwards-compatible.
func Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key, err := derivedKey()
	if err == errNoKey {
		return plain, nil
	}
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return cryptPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. Values missing the prefix are returned as-is
// (legacy plaintext). If the prefix is present but no key is configured,
// returns an explicit error so the caller can surface a config problem
// instead of silently running with garbage bytes.
func Decrypt(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, cryptPrefix) {
		return stored, nil
	}
	key, err := derivedKey()
	if err != nil {
		return "", fmt.Errorf("cannot decrypt secret: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(stored[len(cryptPrefix):])
	if err != nil {
		return "", fmt.Errorf("base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	if len(ct) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, body := ct[:gcm.NonceSize()], ct[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}
	return string(plain), nil
}
