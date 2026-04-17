package federation

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

// Envelope encryption for the PEM material we store in federation_links.
// The adversarial review (A3) flagged that CA + client cert + key were
// persisted in plaintext, so an operator with DB read access could exfiltrate
// every peer's mTLS material.
//
// Strategy:
//   - Derive a 256-bit key by SHA-256(HIVE_MASTER_KEY).
//   - Encrypt with AES-GCM. A random 12-byte nonce is prepended to the
//     ciphertext. The final value is base64-encoded and tagged with the
//     "enc:v1:" prefix so loaders can distinguish encrypted from legacy
//     plaintext values written before this migration.
//   - If HIVE_MASTER_KEY is unset, encryption is a no-op and reads accept
//     plaintext. This keeps dev/tests frictionless; an operator opts in by
//     setting the env var in production.
//
// Not a KMS, not a HSM — but a material improvement over plaintext at rest,
// and the envelope format leaves room for a v2 that wraps with a real KMS.

const cryptPrefix = "enc:v1:"

// errNoKey is returned when decryption is requested but HIVE_MASTER_KEY isn't
// set. Callers should treat this as "fall back to plaintext".
var errNoKey = errors.New("HIVE_MASTER_KEY not configured")

// derivedKey reads HIVE_MASTER_KEY and returns its SHA-256 digest, or an
// error if the env var is empty.
func derivedKey() ([]byte, error) {
	k := os.Getenv("HIVE_MASTER_KEY")
	if k == "" {
		return nil, errNoKey
	}
	sum := sha256.Sum256([]byte(k))
	return sum[:], nil
}

// encrypt wraps plaintext with AES-GCM and the "enc:v1:" tag. When no master
// key is configured, returns the input unchanged so plaintext flows stay
// backwards-compatible.
func encrypt(plain string) (string, error) {
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

// decrypt reverses encrypt. Values missing the "enc:v1:" prefix are returned
// as-is (legacy plaintext). If the prefix is present but no key is
// configured, returns an explicit error so the caller can surface a real
// configuration problem instead of silently running with garbage TLS
// material.
func decrypt(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if len(stored) < len(cryptPrefix) || stored[:len(cryptPrefix)] != cryptPrefix {
		return stored, nil // legacy plaintext
	}
	key, err := derivedKey()
	if err != nil {
		return "", fmt.Errorf("cannot decrypt federation material: %w", err)
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
