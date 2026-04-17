package api

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLimit(t *testing.T) {
	tests := []struct {
		raw          string
		dflt, max, w int
	}{
		{"", 100, 500, 100},          // unset → default
		{"0", 100, 500, 100},         // zero → default
		{"-5", 100, 500, 100},        // negative → default
		{"abc", 100, 500, 100},       // non-numeric → default
		{"50", 100, 500, 50},         // valid
		{"10000", 100, 500, 500},     // clamped to max
		{"100", 100, 0, 100},         // max=0 disables cap (current callers don't use this)
	}
	for _, tc := range tests {
		req := httptest.NewRequest("GET", "/?limit="+tc.raw, nil)
		got := parseLimit(req, tc.dflt, tc.max)
		assert.Equal(t, tc.w, got, "limit=%q → %d (dflt=%d max=%d)", tc.raw, got, tc.dflt, tc.max)
	}
}

func TestParseOffset(t *testing.T) {
	tests := []struct {
		raw string
		w   int
	}{
		{"", 0},
		{"0", 0},
		{"-100", 0}, // negative → 0 (don't send OFFSET -100 to SQLite)
		{"foo", 0},
		{"250", 250},
	}
	for _, tc := range tests {
		req := httptest.NewRequest("GET", "/?offset="+tc.raw, nil)
		assert.Equal(t, tc.w, parseOffset(req), "offset=%q", tc.raw)
	}
}
