package federation

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Store persists federation links across restarts.
type Store struct {
	db *sql.DB
}

// NewStore builds a federation store.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Add inserts or upserts a federation link, optionally with mTLS material.
// When HIVE_MASTER_KEY is set the cert/key PEMs are envelope-encrypted at
// rest (A3 mitigation). Without the env var, values are stored as plaintext
// for backwards compatibility with existing dev databases.
func (s *Store) Add(ctx context.Context, link *Link, caCert, clientCert, clientKey string) error {
	caps, _ := json.Marshal(link.SharedCaps)
	encCA, err := encrypt(caCert)
	if err != nil {
		return fmt.Errorf("encrypting ca_cert: %w", err)
	}
	encCert, err := encrypt(clientCert)
	if err != nil {
		return fmt.Errorf("encrypting client_cert: %w", err)
	}
	encKey, err := encrypt(clientKey)
	if err != nil {
		return fmt.Errorf("encrypting client_key: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO federation_links (id, name, url, status, shared_caps, ca_cert, client_cert, client_key, last_heartbeat)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(name) DO UPDATE SET
		    url = excluded.url,
		    status = excluded.status,
		    shared_caps = excluded.shared_caps,
		    ca_cert = excluded.ca_cert,
		    client_cert = excluded.client_cert,
		    client_key = excluded.client_key`,
		link.Name, link.Name, link.URL, link.Status, string(caps), encCA, encCert, encKey)
	return err
}

// List returns every federation link.
func (s *Store) List(ctx context.Context) ([]*Link, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, url, status, shared_caps, COALESCE(last_heartbeat, '') FROM federation_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Link
	for rows.Next() {
		l := &Link{}
		var capsJSON, heartbeat string
		if err := rows.Scan(&l.ID, &l.Name, &l.URL, &l.Status, &capsJSON, &heartbeat); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(capsJSON), &l.SharedCaps)
		if t, err := time.Parse("2006-01-02 15:04:05", heartbeat); err == nil {
			l.LastHeartbeat = t
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Remove deletes a link by name.
func (s *Store) Remove(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM federation_links WHERE name = ?`, name)
	return err
}

// TLSConfigFor builds a *tls.Config for a stored link. Story 19.1.
func (s *Store) TLSConfigFor(ctx context.Context, name string) (*tls.Config, error) {
	var caStored, certStored, keyStored string
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(ca_cert,''), COALESCE(client_cert,''), COALESCE(client_key,'')
		 FROM federation_links WHERE name = ?`, name,
	).Scan(&caStored, &certStored, &keyStored)
	if err != nil {
		return nil, err
	}
	caPEM, err := decrypt(caStored)
	if err != nil {
		return nil, fmt.Errorf("decrypt ca_cert for %s: %w", name, err)
	}
	certPEM, err := decrypt(certStored)
	if err != nil {
		return nil, fmt.Errorf("decrypt client_cert for %s: %w", name, err)
	}
	keyPEM, err := decrypt(keyStored)
	if err != nil {
		return nil, fmt.Errorf("decrypt client_key for %s: %w", name, err)
	}
	if caPEM == "" && certPEM == "" {
		return nil, nil // no mTLS configured
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caPEM != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caPEM)) {
			return nil, fmt.Errorf("invalid CA cert")
		}
		cfg.RootCAs = pool
	}
	if certPEM != "" && keyPEM != "" {
		pair, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return nil, fmt.Errorf("loading client cert: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

// BuildClient returns an *http.Client configured for the link's TLS material.
func (s *Store) BuildClient(ctx context.Context, name string) (*http.Client, error) {
	tlsCfg, err := s.TLSConfigFor(ctx, name)
	if err != nil {
		return nil, err
	}
	if tlsCfg == nil {
		return &http.Client{Timeout: 10 * time.Second}, nil
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}

// Hydrate loads persisted links into an in-memory Manager.
func (s *Store) Hydrate(ctx context.Context, m *Manager) error {
	links, err := s.List(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range links {
		m.links[l.Name] = l
	}
	return nil
}

// AuditEncryptionAtRest scans federation_links for cert material that isn't
// wrapped in the enc:v1: envelope and logs a warning for each. Called once
// at startup when HIVE_MASTER_KEY is set — encourages the operator to
// rotate plaintext rows (e.g. via a delete+re-add cycle). When the key is
// not set, we skip silently since plaintext is expected.
func (s *Store) AuditEncryptionAtRest(ctx context.Context) (plaintextRows int, _ error) {
	if !HasMasterKey() {
		return 0, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, COALESCE(ca_cert,''), COALESCE(client_cert,''), COALESCE(client_key,'')
		 FROM federation_links`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, ca, cert, key string
		if err := rows.Scan(&name, &ca, &cert, &key); err != nil {
			continue
		}
		// A blob counts as "plaintext" only when it's non-empty AND not
		// tagged — purely empty fields (mTLS not configured) are fine.
		if plaintextField(ca) || plaintextField(cert) || plaintextField(key) {
			plaintextRows++
			slog.Warn("federation link has plaintext TLS material despite HIVE_MASTER_KEY being set — rotate with `hive federation re-encrypt`",
				"peer", name)
		}
	}
	return plaintextRows, rows.Err()
}

func plaintextField(v string) bool {
	return v != "" && !IsEncrypted(v)
}
