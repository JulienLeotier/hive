package workflow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager() (*TriggerManager, *int32) {
	var fires int32
	tm := NewTriggerManager(func(ctx context.Context, cfg *Config, p TriggerPayload) error {
		atomic.AddInt32(&fires, 1)
		return nil
	})
	return tm, &fires
}

func TestWebhookHandlerFiresRegisteredPath(t *testing.T) {
	tm, fires := newTestManager()
	require.NoError(t, tm.Register(context.Background(), &Config{
		Name:    "wf",
		Tasks:   []TaskDef{{Name: "t", Type: "x"}},
		Trigger: &TriggerDef{Type: "webhook", Webhook: "/hooks/deploy"},
	}))

	srv := httptest.NewServer(WebhookHandler(tm))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/hooks/deploy", "application/json", strings.NewReader(`{"ref":"main"}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(fires))
}

func TestWebhookHandler404OnUnknownPath(t *testing.T) {
	tm, _ := newTestManager()
	srv := httptest.NewServer(WebhookHandler(tm))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/hooks/nope", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestWebhookHandlerRequiresSignatureWhenSecretSet(t *testing.T) {
	tm, fires := newTestManager()
	require.NoError(t, tm.Register(context.Background(), &Config{
		Name:    "wf",
		Tasks:   []TaskDef{{Name: "t", Type: "x"}},
		Trigger: &TriggerDef{Type: "webhook", Webhook: "/hooks/secure", Secret: "s3cr3t"},
	}))
	srv := httptest.NewServer(WebhookHandler(tm))
	defer srv.Close()

	// Unsigned request rejected
	resp, err := http.Post(srv.URL+"/hooks/secure", "application/json", strings.NewReader(`{"a":1}`))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "body=%s", string(body))
	assert.Equal(t, int32(0), atomic.LoadInt32(fires))

	// Valid signature accepted
	payload := []byte(`{"a":1}`)
	mac := hmac.New(sha256.New, []byte("s3cr3t"))
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/hooks/secure", strings.NewReader(string(payload)))
	req.Header.Set("X-Hive-Signature", sig)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(fires))
}

func TestWebhookHandlerRejectsNonPost(t *testing.T) {
	tm, _ := newTestManager()
	srv := httptest.NewServer(WebhookHandler(tm))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hooks/anything")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}
