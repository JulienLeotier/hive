package workflow

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// WebhookHandler returns an http.Handler that dispatches incoming webhook
// requests to workflow triggers. The request path (including the `/hooks/`
// prefix) is used as the lookup key, so workflow YAML must declare its
// webhook path in the same form the HTTP transport will receive (e.g.
// `webhook: /hooks/deploy`).
//
// When a workflow declares a secret, the request must include a matching
// `X-Hive-Signature: sha256=<hex>` header computed over the raw body.
//
// Responses:
//   - 200 with `{"ok": true}` on successful dispatch
//   - 401 when the signature is missing or invalid
//   - 404 when no workflow is bound at the path
//   - 500 when the workflow itself returns an error
func WebhookHandler(tm *TriggerManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := r.URL.Path
		if path == "" || path == "/" {
			http.Error(w, "missing webhook path", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB cap
		if err != nil {
			http.Error(w, "reading body: "+err.Error(), http.StatusBadRequest)
			return
		}

		secret := tm.WebhookSecret(path)
		if secret != "" {
			sig := r.Header.Get("X-Hive-Signature")
			if !verifySignature(secret, body, sig) {
				slog.Warn("webhook signature rejected", "path", path)
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Forward the parsed JSON body as the trigger payload. Non-JSON or
		// empty bodies still fire the workflow with a nil payload — this
		// mirrors GitHub/Stripe webhook semantics where the workflow author
		// opts into richer parsing downstream.
		var parsed map[string]any
		if len(body) > 0 {
			if err := json.Unmarshal(body, &parsed); err != nil {
				// Non-JSON body: still forward as `raw` so the workflow can see it.
				parsed = map[string]any{"raw": string(body)}
			}
		}

		if err := tm.FireWebhook(r.Context(), path, parsed); err != nil {
			if strings.Contains(err.Error(), "no webhook trigger registered") {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			slog.Error("webhook firing failed", "path", path, "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// verifySignature checks that header matches HMAC-SHA256(secret, body).
// Accepts both raw hex and the `sha256=<hex>` form used by GitHub-style hooks.
func verifySignature(secret string, body []byte, header string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	header = strings.TrimPrefix(header, "sha256=")
	expected, err := hex.DecodeString(header)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), expected)
}

