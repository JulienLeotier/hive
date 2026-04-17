package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	LogLevel    string           `yaml:"log_level"`
	DataDir     string           `yaml:"data_dir"`
	Port        int              `yaml:"port"`
	Storage     string           `yaml:"storage"`      // "sqlite" (default) or "postgres"
	PostgresURL string           `yaml:"postgres_url"` // used when storage=postgres
	TLS         *TLSBlock        `yaml:"tls,omitempty"`
	OIDC        *OIDCBlock       `yaml:"oidc,omitempty"`
	Federation  *FederationBlock `yaml:"federation,omitempty"`
	Checkpoint  *CheckpointBlock `yaml:"checkpoint,omitempty"`
	Breaker     *BreakerBlock    `yaml:"breaker,omitempty"`
	Retry       *RetryBlock      `yaml:"retry,omitempty"`
	Knowledge   *KnowledgeBlock  `yaml:"knowledge,omitempty"`
	EventBus    *EventBusBlock   `yaml:"event_bus,omitempty"`
	Cluster     *ClusterBlock    `yaml:"cluster,omitempty"`
	Retention   *RetentionBlock  `yaml:"retention,omitempty"`
}

// TLSBlock enables HTTPS when CertFile + KeyFile are both set. Leaving either
// empty keeps the server on plaintext HTTP, which is only appropriate behind a
// TLS-terminating proxy in a trusted network.
type TLSBlock struct {
	CertFile string `yaml:"cert_file,omitempty"`
	KeyFile  string `yaml:"key_file,omitempty"`
}

// Enabled reports whether TLS is configured with a usable cert/key pair.
func (t *TLSBlock) Enabled() bool {
	return t != nil && t.CertFile != "" && t.KeyFile != ""
}

// CheckpointBlock tunes the background checkpoint supervisor. Story 2.6.
type CheckpointBlock struct {
	IntervalSeconds int `yaml:"interval_seconds,omitempty"` // default 30
	MaxAgeSeconds   int `yaml:"max_age_seconds,omitempty"`  // default 300 (5m)
}

// BreakerBlock tunes circuit breaker thresholds. Story 5.1.
type BreakerBlock struct {
	Threshold           int `yaml:"threshold,omitempty"`             // default 3
	ResetTimeoutSeconds int `yaml:"reset_timeout_seconds,omitempty"` // default 30
}

// RetryBlock tunes the default retry policy. Story 5.5.
type RetryBlock struct {
	MaxAttempts   int     `yaml:"max_attempts,omitempty"`    // default 3
	InitialWaitMs int     `yaml:"initial_wait_ms,omitempty"` // default 200
	MaxWaitMs     int     `yaml:"max_wait_ms,omitempty"`     // default 2000
	Multiplier    float64 `yaml:"multiplier,omitempty"`      // default 2.0
	Jitter        float64 `yaml:"jitter,omitempty"`          // default 0.2
}

// KnowledgeBlock tunes knowledge-layer lifecycle. Story 10.3.
type KnowledgeBlock struct {
	MaxAgeDays int `yaml:"max_age_days,omitempty"` // default 90
}

// RetentionBlock caps growth on the big append-only tables. Zero or negative
// values disable the janitor for that table. Defaults chosen to keep a year
// of cost data for billing review, a month of completed tasks for debugging,
// and 90 days of events for audit trails.
type RetentionBlock struct {
	EventsMaxAgeDays         int `yaml:"events_max_age_days,omitempty"`          // default 90
	CompletedTasksMaxAgeDays int `yaml:"completed_tasks_max_age_days,omitempty"` // default 30
	CostsMaxAgeDays          int `yaml:"costs_max_age_days,omitempty"`           // default 365
	AuditMaxAgeDays          int `yaml:"audit_max_age_days,omitempty"`           // default 365
	IntervalMinutes          int `yaml:"interval_minutes,omitempty"`             // default 60
}

// OIDCBlock holds OIDC SSO settings. Story 21.1.
type OIDCBlock struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes,omitempty"`
}

// FederationBlock controls which capabilities this hive exposes to federated
// peers. Story 19.2.
type FederationBlock struct {
	Share []string `yaml:"share,omitempty"` // empty = expose every capability
}

// EventBusBlock tunes the distributed event bus. Story 15.2/22.2.
type EventBusBlock struct {
	Backend string `yaml:"backend,omitempty"` // "sqlite" (default) or "nats"
	NATSURL string `yaml:"nats_url,omitempty"`
	Subject string `yaml:"subject,omitempty"` // default "hive.events"
}

// ClusterBlock configures this node's identity and routing preferences in a
// multi-node deployment. Story 22.3.
type ClusterBlock struct {
	NodeID  string `yaml:"node_id,omitempty"`
	Routing string `yaml:"routing,omitempty"` // "local-first" (default) or "best-fit"
}

// Default returns a Config with sensible defaults.
func Default() Config {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/tmp"
	}
	return Config{
		LogLevel: "info",
		DataDir:  filepath.Join(home, ".hive", "data"),
		Port:     8233,
	}
}

// Load reads configuration from a YAML file, then applies environment variable
// overrides with the HIVE_ prefix. Missing file is not an error — defaults are used.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parsing config %s: %w", path, err)
		}
	}

	applyEnvOverrides(&cfg)

	// Expand ~ in data dir
	if strings.HasPrefix(cfg.DataDir, "~/") {
		home, _ := os.UserHomeDir()
		cfg.DataDir = filepath.Join(home, cfg.DataDir[2:])
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("HIVE_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("HIVE_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("HIVE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("HIVE_STORAGE"); v != "" {
		cfg.Storage = v
	}
	if v := os.Getenv("HIVE_POSTGRES_URL"); v != "" {
		cfg.PostgresURL = v
	}
	if cert, key := os.Getenv("HIVE_TLS_CERT"), os.Getenv("HIVE_TLS_KEY"); cert != "" || key != "" {
		if cfg.TLS == nil {
			cfg.TLS = &TLSBlock{}
		}
		if cert != "" {
			cfg.TLS.CertFile = cert
		}
		if key != "" {
			cfg.TLS.KeyFile = key
		}
	}
}
