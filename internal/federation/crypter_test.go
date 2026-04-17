package federation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptNoKey_PassesThrough(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "")
	out, err := encrypt("-----BEGIN CERTIFICATE-----\nfoo\n-----END CERTIFICATE-----")
	require.NoError(t, err)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nfoo\n-----END CERTIFICATE-----", out,
		"without HIVE_MASTER_KEY, encrypt is a no-op for backwards compat")
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "test-master-key-32bytes-something")
	plain := "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----"
	enc, err := encrypt(plain)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(enc, cryptPrefix), "envelope should carry version tag")
	assert.NotContains(t, enc, "PRIVATE KEY", "plaintext must not leak through")

	dec, err := decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, plain, dec)
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "") // decrypt still accepts plaintext if no prefix
	out, err := decrypt("just a plaintext pem")
	require.NoError(t, err)
	assert.Equal(t, "just a plaintext pem", out)
}

func TestDecryptEncryptedWithoutKey_Errors(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "a-key")
	enc, err := encrypt("secret")
	require.NoError(t, err)

	// Pretend the operator forgot to set the key on a different node.
	t.Setenv("HIVE_MASTER_KEY", "")
	_, err = decrypt(enc)
	assert.Error(t, err, "encrypted value without key must surface an error, not silent garbage")
}
