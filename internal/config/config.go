package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration for Hive — a single-user
// BMAD product factory. Most knobs are environment variables (see
// /settings in the dashboard) ; this struct only covers what an
// operator might want to pin in hive.yaml : storage backend, TLS,
// retention, tracing.
type Config struct {
	LogLevel      string              `yaml:"log_level"`
	DataDir       string              `yaml:"data_dir"`
	Port          int                 `yaml:"port"`
	Storage       string              `yaml:"storage"`      // "sqlite" (default) or "postgres"
	PostgresURL   string              `yaml:"postgres_url"` // used when storage=postgres
	TLS           *TLSBlock           `yaml:"tls,omitempty"`
	Retention     *RetentionBlock     `yaml:"retention,omitempty"`
	Observability *ObservabilityBlock `yaml:"observability,omitempty"`
}

// ObservabilityBlock wires traces out to an OTLP collector.
// OTEL_EXPORTER_OTLP_ENDPOINT is respected when the endpoint field is
// empty so standard OTel tooling keeps working.
type ObservabilityBlock struct {
	Traces *TracesBlock `yaml:"traces,omitempty"`
}

// TracesBlock configures the trace exporter. Protocol defaults to grpc.
// SampleRatio defaults to 1.0 (every trace exported).
type TracesBlock struct {
	Enabled     bool    `yaml:"enabled,omitempty"`
	Endpoint    string  `yaml:"endpoint,omitempty"`
	Protocol    string  `yaml:"protocol,omitempty"`
	SampleRatio float64 `yaml:"sample_ratio,omitempty"`
	Version     string  `yaml:"service_version,omitempty"`
}

// TLSBlock enables HTTPS when CertFile + KeyFile are both set.
type TLSBlock struct {
	CertFile string `yaml:"cert_file,omitempty"`
	KeyFile  string `yaml:"key_file,omitempty"`
}

// Enabled reports whether TLS is configured with a usable cert/key pair.
func (t *TLSBlock) Enabled() bool {
	return t != nil && t.CertFile != "" && t.KeyFile != ""
}

// RetentionBlock caps growth on the append-only tables. Zero or
// negative values disable the janitor for that table.
type RetentionBlock struct {
	EventsMaxAgeDays int `yaml:"events_max_age_days,omitempty"` // default 90
	AuditMaxAgeDays  int `yaml:"audit_max_age_days,omitempty"`  // default 365
	IntervalMinutes  int `yaml:"interval_minutes,omitempty"`    // default 60
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

// Load reads configuration from a YAML file, then applies environment
// variable overrides with the HIVE_ prefix. Missing file is not an
// error — defaults are used.
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

	if strings.HasPrefix(cfg.DataDir, "~/") {
		home, _ := os.UserHomeDir()
		cfg.DataDir = filepath.Join(home, cfg.DataDir[2:])
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Validate rejects configurations that would start but misbehave silently.
func (c Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d (must be 1-65535)", c.Port)
	}
	if c.Retention != nil && c.Retention.IntervalMinutes < 0 {
		return fmt.Errorf("retention.interval_minutes must not be negative, got %d", c.Retention.IntervalMinutes)
	}
	return nil
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
