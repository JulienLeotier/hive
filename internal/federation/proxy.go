package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Proxy forwards tasks to federated peer hives. Story 19.3 closes the loop:
// Router.FindCapableAgent returns agentID="federation:<peer>", the engine
// hands the task to Proxy.Invoke which makes an HTTP call to the peer and
// streams the result back.
type Proxy struct {
	store *Store

	mu      sync.RWMutex
	clients map[string]*http.Client // name → cached client (plain or mTLS)
}

// NewProxy builds a proxy backed by a federation store. mTLS config is pulled
// from the store per-peer so clients pick up rotated certs on the next call.
func NewProxy(store *Store) *Proxy {
	return &Proxy{store: store, clients: map[string]*http.Client{}}
}

// ProxyRequest is the wire format we send to a peer hive. Kept small so v0.x
// can evolve the schema without breaking existing deployments.
type ProxyRequest struct {
	TaskID string `json:"task_id"`
	Type   string `json:"type"`
	Input  any    `json:"input"`
}

// ProxyResponse mirrors the local task result shape.
type ProxyResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Invoke proxies a task to the named peer. Returns the peer's response or an
// error if the link is down, the peer rejects the task, or the proxy call
// times out.
func (p *Proxy) Invoke(ctx context.Context, peer string, req ProxyRequest) (*ProxyResponse, error) {
	client, baseURL, err := p.clientFor(ctx, peer)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal proxy request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, "POST", baseURL+"/api/v1/tasks/proxied", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("proxy to %s failed: %w", peer, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("peer %s returned %d: %s", peer, resp.StatusCode, string(data))
	}

	var out ProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode proxy response: %w", err)
	}
	return &out, nil
}

// clientFor returns (cached) http.Client wired for this peer's mTLS material.
func (p *Proxy) clientFor(ctx context.Context, peer string) (*http.Client, string, error) {
	p.mu.RLock()
	client, ok := p.clients[peer]
	p.mu.RUnlock()

	links, err := p.store.List(ctx)
	if err != nil {
		return nil, "", err
	}
	var url string
	for _, l := range links {
		if l.Name == peer {
			url = l.URL
			break
		}
	}
	if url == "" {
		return nil, "", fmt.Errorf("peer %q not registered", peer)
	}

	if ok {
		return client, url, nil
	}

	client, err = p.store.BuildClient(ctx, peer)
	if err != nil {
		return nil, "", err
	}
	p.mu.Lock()
	p.clients[peer] = client
	p.mu.Unlock()
	return client, url, nil
}

// NewResolver builds a router.FederationResolver + a live Proxy wired to the
// same Store. The resolver reports whether any peer offers the requested
// capability; the proxy is returned so callers can use it (or hold it alive).
func NewResolver(ctx context.Context, store *Store) (Resolver, *Proxy) {
	proxy := NewProxy(store)
	resolver := func(inner context.Context, taskType string) (hiveName, hiveURL string, ok bool) {
		links, err := store.List(inner)
		if err != nil {
			slog.Warn("federation resolver: listing links failed", "error", err)
			return "", "", false
		}
		for _, link := range links {
			if link.Status != "active" {
				continue
			}
			for _, c := range link.SharedCaps {
				if c == taskType {
					return link.Name, link.URL, true
				}
			}
		}
		return "", "", false
	}
	return resolver, proxy
}

// Resolver matches task.FederationResolver. Defined locally so task package
// doesn't need to import federation.
type Resolver func(ctx context.Context, taskType string) (hiveName, hiveURL string, ok bool)
