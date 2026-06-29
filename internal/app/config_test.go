package app

import (
	"database/sql"
	"log/slog"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

func TestParseConfigAppliesFlagsAndEnv(t *testing.T) {
	getenv := func(name string) string {
		switch name {
		case "AWLOAD_USERS":
			return "25"
		case "AWLOAD_PROFILE":
			return "read-heavy"
		default:
			return ""
		}
	}

	cfg, err := ParseConfig([]string{
		"-dsn", "sqlserver://example",
		"-duration", "90s",
		"-ramp", "20s",
		"-write-mode", "cart",
		"-report-name", "stress",
		"-log-level", "warn",
	}, getenv)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if cfg.Users != 25 {
		t.Fatalf("Users = %d, want 25", cfg.Users)
	}
	if cfg.Duration != 90*time.Second {
		t.Fatalf("Duration = %s, want 90s", cfg.Duration)
	}
	if cfg.Ramp != 20*time.Second {
		t.Fatalf("Ramp = %s, want 20s", cfg.Ramp)
	}
	if cfg.Profile != "read-heavy" {
		t.Fatalf("Profile = %q, want read-heavy", cfg.Profile)
	}
	if cfg.WriteMode != "cart" {
		t.Fatalf("WriteMode = %q, want cart", cfg.WriteMode)
	}
	if cfg.ReportName != "stress" {
		t.Fatalf("ReportName = %q, want stress", cfg.ReportName)
	}
	if cfg.LogLevel != slog.LevelWarn {
		t.Fatalf("LogLevel = %v, want warn", cfg.LogLevel)
	}
}

func TestConfigConnectionStringPrefersDSN(t *testing.T) {
	cfg := Config{
		DSN:      "sqlserver://custom-dsn",
		Server:   "ignored:1433",
		User:     "ignored",
		Password: "ignored",
	}
	if got := cfg.ConnectionString(); got != "sqlserver://custom-dsn" {
		t.Fatalf("ConnectionString() = %q, want custom dsn", got)
	}
}

func TestConfigConnectionStringBuildsFromParts(t *testing.T) {
	cfg := Config{
		Server:          "db.example:1433",
		Database:        "AdventureWorks2022",
		User:            "awload",
		Password:        "secret",
		Encrypt:         "true",
		TrustServerCert: true,
	}
	got := cfg.ConnectionString()
	for _, want := range []string{
		"sqlserver://",
		"db.example:1433",
		"database=AdventureWorks2022",
		"encrypt=true",
		"TrustServerCertificate=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ConnectionString() = %q, missing %q", got, want)
		}
	}
}

func TestApplyPoolSettingsDerivesFromUsers(t *testing.T) {
	cfg := Config{Users: 100}
	db, err := sql.Open("sqlserver", "sqlserver://example")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	cfg.ApplyPoolSettings(db)

	if got := db.Stats().MaxOpenConnections; got != 155 {
		t.Fatalf("MaxOpenConnections = %d, want 155", got)
	}
}

func TestValidateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "unsupported profile",
			cfg:  Config{DSN: "x", Users: 1, Duration: time.Second, Profile: "invalid"},
		},
		{
			name: "unsupported write mode",
			cfg:  Config{DSN: "x", Users: 1, Duration: time.Second, Profile: "mixed", WriteMode: "bulk"},
		},
		{
			name: "think min greater than max",
			cfg:  Config{DSN: "x", Users: 1, Duration: time.Second, Profile: "mixed", ThinkMin: time.Second, ThinkMax: time.Millisecond},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
