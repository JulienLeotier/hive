package federation

import (
	"github.com/JulienLeotier/hive/internal/secretstore"
)

// The envelope encryption for PEM material stored in federation_links is now
// provided by the shared internal/secretstore package. A3 hardening originally
// introduced it here for CA + client cert + key; it has since been extracted
// so webhook URLs and other sensitive DB fields can reuse the same envelope.
//
// These thin wrappers preserve the federation-facing API; callers in this
// package need not care where the bytes actually go.

// HasMasterKey reports whether HIVE_MASTER_KEY is configured.
func HasMasterKey() bool { return secretstore.HasMasterKey() }

// IsEncrypted reports whether a stored value carries the envelope tag.
func IsEncrypted(stored string) bool { return secretstore.IsEncrypted(stored) }

func encrypt(plain string) (string, error)  { return secretstore.Encrypt(plain) }
func decrypt(stored string) (string, error) { return secretstore.Decrypt(stored) }
