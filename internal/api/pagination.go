package api

import (
	"net/http"
	"strconv"
)

// Pagination shared across list endpoints. We avoid cursor pagination for
// now — every list endpoint orders by created_at DESC or id DESC, so offset
// is good enough up to a few hundred thousand rows. When that hurts we'll
// switch to keyset pagination (last_seen_id/created_at in the WHERE), but
// that's premature for the current scale.
//
// Contract:
//   - ?limit clamped to [1, maxLimit]; missing = defaultLimit.
//   - ?offset clamped to [0, +inf); missing = 0. Large offsets are permitted
//     but slow — that's on the caller to notice.
//   - Negative / non-numeric inputs fall back to the default instead of
//     rejecting, because a dashboard query string typo shouldn't 400.

// parseLimit reads ?limit and clamps it. max=0 means "no cap" (so a caller
// can opt out by passing max=0 — currently no handler needs this).
func parseLimit(r *http.Request, dflt, max int) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return dflt
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return dflt
	}
	if max > 0 && n > max {
		return max
	}
	return n
}

// parseOffset reads ?offset. Negative/non-numeric → 0.
func parseOffset(r *http.Request) int {
	raw := r.URL.Query().Get("offset")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
