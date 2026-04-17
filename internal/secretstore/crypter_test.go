package secretstore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptNoKeyReturnsPlaintext(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "")
	out, err := Encrypt("hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

func TestEncryptRoundTrip(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "unit-test-key")
	ct, err := Encrypt("s3cr3t-bearer-token")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(ct, "enc:v1:"))
	assert.NotContains(t, ct, "s3cr3t")

	plain, err := Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t-bearer-token", plain)
}

func TestDecryptPassesThroughLegacyPlaintext(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "unit-test-key")
	out, err := Decrypt("https://example.com/hook?token=abc")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/hook?token=abc", out)
}

func TestDecryptWithoutKeyFailsLoudly(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "k1")
	ct, err := Encrypt("x")
	require.NoError(t, err)
	t.Setenv("HIVE_MASTER_KEY", "")
	_, err = Decrypt(ct)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot decrypt secret")
}

func TestEmptyStringStaysEmpty(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "k")
	ct, err := Encrypt("")
	require.NoError(t, err)
	assert.Empty(t, ct)
	plain, err := Decrypt("")
	require.NoError(t, err)
	assert.Empty(t, plain)
}

func TestHasMasterKey(t *testing.T) {
	t.Setenv("HIVE_MASTER_KEY", "")
	assert.False(t, HasMasterKey())
	t.Setenv("HIVE_MASTER_KEY", "x")
	assert.True(t, HasMasterKey())
}
