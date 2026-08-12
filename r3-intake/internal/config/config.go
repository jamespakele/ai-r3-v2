// Package config holds the runtime configuration for the R3 Intake app,
// including the (currently inactive) at-rest encryption seam.
package config

import (
	"os"
	"strings"
)

// Config is the single source of truth for runtime configuration.
type Config struct {
	// HTTPAddr is the public Go server listen address.
	HTTPAddr string
	// PBInternalAddr is the in-process PocketBase listen address. Never exposed
	// on the public port unless --admin proxies /_/ through.
	PBInternalAddr string
	// ExposePBAdmin mirrors the --admin flag / R3_ADMIN env var. When false,
	// requests to /_/ return 404 from the public port.
	ExposePBAdmin bool
	// PBRootDir is the PocketBase root dir (holds pb_data/ and migrations/).
	// When using embedded migrations, this is used only for the pb_data path.
	PBRootDir string

	// DataDir is the runtime data directory. Defaults to PBRootDir so local dev
	// keeps pb_data under ./pocketbase. In production this is overridden via
	// R3_DATA_DIR to /srv/go-apps/<name>/data.
	DataDir string

	// PBAdminEmail / PBAdminPassword are the PocketBase superuser credentials
	// used to (a) log into the PB admin UI at /_/ under --admin, and (b) seed
	// the in-process _superusers record on startup. They are NOT the app login.
	PBAdminEmail    string
	PBAdminPassword string

	// Seed app users created on first run so "one command" really is the whole
	// setup. Override via env for production.
	SeedAdminEmail    string
	SeedAdminPassword string
	SeedAdminName     string
	SeedCMEmail       string
	SeedCMPassword    string
	SeedCMName        string

	// DisableSeed skips creating the default superuser and seed users on first
	// run. Useful in production when accounts are created manually.
	DisableSeed bool

	// SessionKey is the HMAC key for the Go session cookie.
	SessionKey string

	// Encryption is the at-rest encryption seam. Defaults to disabled
	// (plaintext). Switching on is a config flip + migration 002.
	Encryption EncryptionConfig

	// MCP is the optional Model Context Protocol server configuration.
	// Disabled by default (empty Token); set R3_MCP_TOKEN to enable.
	MCP MCPConfig
}

// MCPConfig controls the read-only Model Context Protocol server.
// Token is the static bearer token required by both stdio and HTTP transports.
// When Token is empty, MCP is disabled entirely. BaseURL is used to build the
// edit_url returned with every intake record.
type MCPConfig struct {
	Token   string
	BaseURL string
}

// EncryptionConfig describes the (future) at-rest encryption of sensitive
// fields. All repository save/load paths call Cipher.Encrypt/Decrypt on the
// SensitiveFields regardless of Enabled — they do not know whether encryption
// is on.
type EncryptionConfig struct {
	Enabled         bool
	Algorithm       string   // default "aes-gcm"
	KeyEnv          string   // default "R3_ENCRYPTION_KEY"
	SensitiveFields []string // default ["ssn","dob","participantSigDataUrl","casemanagerSigDataUrl"]
}

// Default returns the production-default config. Flags / env vars override.
func Default() Config {
	return Config{
		HTTPAddr:        ":8090",
		PBInternalAddr:  "127.0.0.1:8091",
		ExposePBAdmin:   false,
		PBRootDir:       "pocketbase",
		DataDir:         "pocketbase",
		PBAdminEmail:    "admin@r3.local",
		PBAdminPassword: "r3admin123",
		SeedAdminEmail:  "admin@r3.local",
		SeedAdminPassword: "admin123",
		SeedAdminName:   "R3 Admin",
		SeedCMEmail:     "cm@r3.local",
		SeedCMPassword:  "cm123456",
		SeedCMName:      "Demo Case Manager",
		DisableSeed:     false,
		SessionKey:      "r3-session-change-me",
		Encryption: EncryptionConfig{
			Enabled:         false,
			Algorithm:       "aes-gcm",
			KeyEnv:          "R3_ENCRYPTION_KEY",
			SensitiveFields: []string{"ssn", "dob", "participantSigDataUrl", "casemanagerSigDataUrl"},
		},
		MCP: MCPConfig{
			Token:   "",
			BaseURL: "http://localhost:8090",
		},
	}
}

// FromEnv mutates in by overriding fields from R3_* environment variables.
// This is the single place env vars are read so both subcommands behave
// identically.
func FromEnv(in *Config) {
	if v := os.Getenv("R3_ADMIN"); v == "1" || v == "true" {
		in.ExposePBAdmin = true
	}
	if v := os.Getenv("R3_HTTP_ADDR"); v != "" {
		in.HTTPAddr = v
	}
	if v := os.Getenv("R3_PB_INTERNAL_ADDR"); v != "" {
		in.PBInternalAddr = v
	}
	if v := os.Getenv("R3_DATA_DIR"); v != "" {
		in.DataDir = strings.TrimSuffix(v, "/")
	}
	if v := os.Getenv("R3_PB_ROOT_DIR"); v != "" {
		in.PBRootDir = strings.TrimSuffix(v, "/")
	}
	if v := os.Getenv("R3_PB_ADMIN_EMAIL"); v != "" {
		in.PBAdminEmail = v
	}
	if v := os.Getenv("R3_PB_ADMIN_PASSWORD"); v != "" {
		in.PBAdminPassword = v
	}
	if v := os.Getenv("R3_SEED_ADMIN_EMAIL"); v != "" {
		in.SeedAdminEmail = v
	}
	if v := os.Getenv("R3_SEED_ADMIN_PASSWORD"); v != "" {
		in.SeedAdminPassword = v
	}
	if v := os.Getenv("R3_SEED_ADMIN_NAME"); v != "" {
		in.SeedAdminName = v
	}
	if v := os.Getenv("R3_SEED_CM_EMAIL"); v != "" {
		in.SeedCMEmail = v
	}
	if v := os.Getenv("R3_SEED_CM_PASSWORD"); v != "" {
		in.SeedCMPassword = v
	}
	if v := os.Getenv("R3_SEED_CM_NAME"); v != "" {
		in.SeedCMName = v
	}
	if v := os.Getenv("R3_DISABLE_SEED"); v == "1" || v == "true" {
		in.DisableSeed = true
	}
	if v := os.Getenv("R3_SESSION_KEY"); v != "" {
		in.SessionKey = v
	}
	if v := os.Getenv("R3_ENCRYPTION_KEY"); v != "" {
		// Encryption key presence alone does not enable encryption; R3_ENCRYPTION_ENABLED
		// is the explicit switch. The key env name is stored for later use.
		in.Encryption.KeyEnv = "R3_ENCRYPTION_KEY"
	}
	if v := os.Getenv("R3_ENCRYPTION_ENABLED"); v == "1" || v == "true" {
		in.Encryption.Enabled = true
	}
	if v := os.Getenv("R3_MCP_TOKEN"); v != "" {
		in.MCP.Token = v
	}
	if v := os.Getenv("R3_MCP_BASE_URL"); v != "" {
		in.MCP.BaseURL = strings.TrimSuffix(v, "/")
	}
}