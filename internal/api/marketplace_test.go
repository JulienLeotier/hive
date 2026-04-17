package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogExcludesNonPublishable(t *testing.T) {
	srv := setupServer(t)

	// One publishable healthy agent with real capabilities; one hidden
	// agent that should NOT show up in the catalog.
	_, err := srv.db().Exec(`
		INSERT INTO agents (id, name, type, config, capabilities, health_status, publishable) VALUES
		('a1', 'public', 'http', '{}', ?, 'healthy', 1),
		('a2', 'secret', 'http', '{}', ?, 'healthy', 0)`,
		`{"task_types":["code-review","summarize"],"cost_per_run":0.12}`,
		`{"task_types":["internal"]}`,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/federation/catalog", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []CatalogAgent `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Data, 1, "only publishable agents leak to peers")
	assert.Equal(t, "public", resp.Data[0].Name)
	assert.ElementsMatch(t, []string{"code-review", "summarize"}, resp.Data[0].TaskTypes)
	assert.InDelta(t, 0.12, resp.Data[0].CostPerRun, 0.001)
}

func TestCatalogSkipsUnhealthy(t *testing.T) {
	srv := setupServer(t)
	_, err := srv.db().Exec(`
		INSERT INTO agents (id, name, type, config, capabilities, health_status, publishable) VALUES
		('a1', 'down', 'http', '{}', ?, 'unavailable', 1)`,
		`{"task_types":["t"]}`,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/federation/catalog", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []CatalogAgent `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Empty(t, resp.Data, "unhealthy agents must not be published")
}

func TestMarketplaceAggregatesPeers(t *testing.T) {
	// Fake peer that speaks the catalog protocol.
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/federation/catalog" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []CatalogAgent{
				{Name: "remote-a", Type: "http", Version: "2.1.0", TaskTypes: []string{"trans"}, CostPerRun: 0.05},
			},
		})
	}))
	defer peer.Close()

	srv := setupServer(t)
	_, err := srv.db().Exec(
		`INSERT INTO federation_links (id, name, url, status) VALUES (?, ?, ?, 'active')`,
		"fed-1", "peer-alpha", peer.URL,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/marketplace", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Data []MarketplacePeer `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "peer-alpha", resp.Data[0].PeerName)
	assert.Equal(t, peerStatusOK, resp.Data[0].Status)
	require.Len(t, resp.Data[0].Agents, 1)
	assert.Equal(t, "remote-a", resp.Data[0].Agents[0].Name)
}

func TestMarketplaceReportsUnreachablePeer(t *testing.T) {
	srv := setupServer(t)
	// Point the link at a dead port so the fetch fails fast.
	_, err := srv.db().Exec(
		`INSERT INTO federation_links (id, name, url, status) VALUES (?, ?, ?, 'active')`,
		"fed-1", "peer-dead", "http://127.0.0.1:1",
	)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/marketplace", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []MarketplacePeer `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, peerStatusUnreachable, resp.Data[0].Status, "dead peer surfaces as unreachable, not hidden")
	assert.NotEmpty(t, resp.Data[0].Error)
}
