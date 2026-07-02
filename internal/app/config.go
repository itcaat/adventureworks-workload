package app

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DSN             string
	Server          string
	Database        string
	User            string
	Password        string
	Trusted         bool
	Encrypt         string
	TrustServerCert bool

	Users          int
	Duration       time.Duration
	Ramp           time.Duration
	Profile        string
	WriteMode      string
	Seed           int64
	ThinkMin       time.Duration
	ThinkMax       time.Duration
	RequestTimeout time.Duration
	ReportDir      string
	ReportName     string
	ProgressEvery  time.Duration

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	LogLevel slog.Level
	TUI      bool
}

func ParseConfig(args []string, getenv func(string) string) (Config, error) {
	cfg := Config{
		DSN:             getenv("AWLOAD_DSN"),
		Server:          firstNonEmpty(getenv("AWLOAD_SERVER"), "localhost:1433"),
		Database:        firstNonEmpty(getenv("AWLOAD_DATABASE"), "AdventureWorks2022"),
		User:            getenv("AWLOAD_USER"),
		Password:        getenv("AWLOAD_PASSWORD"),
		Encrypt:         firstNonEmpty(getenv("AWLOAD_ENCRYPT"), "true"),
		Users:           intFromEnv(getenv, "AWLOAD_USERS", 10),
		Duration:        durationFromEnv(getenv, "AWLOAD_DURATION", 5*time.Minute),
		Ramp:            durationFromEnv(getenv, "AWLOAD_RAMP", 15*time.Second),
		Profile:         firstNonEmpty(getenv("AWLOAD_PROFILE"), "mixed"),
		WriteMode:       firstNonEmpty(getenv("AWLOAD_WRITE_MODE"), "off"),
		Seed:            time.Now().UnixNano(),
		ThinkMin:        durationFromEnv(getenv, "AWLOAD_THINK_MIN", 50*time.Millisecond),
		ThinkMax:        durationFromEnv(getenv, "AWLOAD_THINK_MAX", 750*time.Millisecond),
		RequestTimeout:  durationFromEnv(getenv, "AWLOAD_REQUEST_TIMEOUT", 30*time.Second),
		ReportDir:       firstNonEmpty(getenv("AWLOAD_REPORT_DIR"), "reports"),
		ProgressEvery:   durationFromEnv(getenv, "AWLOAD_PROGRESS_EVERY", 5*time.Second),
		ConnMaxLifetime: durationFromEnv(getenv, "AWLOAD_CONN_MAX_LIFETIME", 10*time.Minute),
	}

	fs := flag.NewFlagSet("awload", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.DSN, "dsn", cfg.DSN, "full go-mssqldb connection string; overrides server/user/password flags")
	fs.StringVar(&cfg.Server, "server", cfg.Server, "SQL Server host:port; use -dsn for named instances")
	fs.StringVar(&cfg.Database, "database", cfg.Database, "database name")
	fs.StringVar(&cfg.User, "user", cfg.User, "SQL login user")
	fs.StringVar(&cfg.Password, "password", cfg.Password, "SQL login password")
	fs.BoolVar(&cfg.Trusted, "trusted", false, "use trusted_connection=true in the generated DSN")
	fs.StringVar(&cfg.Encrypt, "encrypt", cfg.Encrypt, "go-mssqldb encrypt value: true, false, disable, strict")
	fs.BoolVar(&cfg.TrustServerCert, "trust-server-cert", boolFromEnv(getenv, "AWLOAD_TRUST_SERVER_CERT", false), "trust SQL Server certificate")
	fs.IntVar(&cfg.Users, "users", cfg.Users, "number of concurrent virtual users")
	fs.DurationVar(&cfg.Duration, "duration", cfg.Duration, "workload duration, for example 10m or 1h")
	fs.DurationVar(&cfg.Ramp, "ramp", cfg.Ramp, "time window used to gradually start users")
	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "workload profile: mixed, read-heavy, reporting, write-light")
	fs.StringVar(&cfg.WriteMode, "write-mode", cfg.WriteMode, "write mode: off or cart")
	fs.Int64Var(&cfg.Seed, "seed", cfg.Seed, "random seed for repeatable user behavior")
	fs.DurationVar(&cfg.ThinkMin, "think-min", cfg.ThinkMin, "minimum user think time between operations")
	fs.DurationVar(&cfg.ThinkMax, "think-max", cfg.ThinkMax, "maximum user think time between operations")
	fs.DurationVar(&cfg.RequestTimeout, "request-timeout", cfg.RequestTimeout, "per-operation timeout")
	fs.StringVar(&cfg.ReportDir, "report-dir", cfg.ReportDir, "directory for final markdown and json reports")
	fs.StringVar(&cfg.ReportName, "report-name", cfg.ReportName, "report filename prefix; run timestamp is appended automatically")
	fs.DurationVar(&cfg.ProgressEvery, "progress-every", cfg.ProgressEvery, "live progress interval")
	fs.IntVar(&cfg.MaxOpenConns, "max-open-conns", intFromEnv(getenv, "AWLOAD_MAX_OPEN_CONNS", 0), "database/sql max open conns; 0 derives from users")
	fs.IntVar(&cfg.MaxIdleConns, "max-idle-conns", intFromEnv(getenv, "AWLOAD_MAX_IDLE_CONNS", 0), "database/sql max idle conns; 0 derives from users")
	fs.DurationVar(&cfg.ConnMaxLifetime, "conn-max-lifetime", cfg.ConnMaxLifetime, "database/sql connection max lifetime")
	fs.BoolVar(&cfg.TUI, "tui", boolFromEnv(getenv, "AWLOAD_TUI", true), "show live terminal dashboard during the run (use -tui=false to disable)")
	logLevel := fs.String("log-level", firstNonEmpty(getenv("AWLOAD_LOG_LEVEL"), "info"), "log level: debug, info, warn, error")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	progressEverySet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "progress-every" {
			progressEverySet = true
		}
	})
	if cfg.TUI && !progressEverySet {
		cfg.ProgressEvery = 250 * time.Millisecond
	}

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return cfg, err
	}
	cfg.LogLevel = level

	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Users < 1 {
		return errors.New("users must be >= 1")
	}
	if c.Duration <= 0 {
		return errors.New("duration must be positive")
	}
	if c.ThinkMin < 0 || c.ThinkMax < 0 || c.ThinkMin > c.ThinkMax {
		return errors.New("think time must be non-negative and think-min <= think-max")
	}
	if _, ok := profileWeights(c.Profile); !ok {
		return fmt.Errorf("unsupported profile %q", c.Profile)
	}
	if c.WriteMode != "off" && c.WriteMode != "cart" {
		return fmt.Errorf("unsupported write mode %q", c.WriteMode)
	}
	if c.DSN == "" && !c.Trusted && c.User == "" {
		return errors.New("provide -dsn, -trusted, or -user/-password")
	}
	return nil
}

func (c Config) ConnectionString() string {
	if c.DSN != "" {
		return c.DSN
	}

	u := url.URL{Scheme: "sqlserver", Host: c.Server}
	if !c.Trusted {
		u.User = url.UserPassword(c.User, c.Password)
	}

	q := u.Query()
	q.Set("database", c.Database)
	q.Set("encrypt", c.Encrypt)
	if c.Trusted {
		q.Set("trusted_connection", "true")
	}
	if c.TrustServerCert {
		q.Set("TrustServerCertificate", "true")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c Config) ApplyPoolSettings(db *sql.DB) {
	maxOpen := c.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = c.Users + c.Users/2 + 5
	}
	maxIdle := c.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = min(maxOpen, max(2, c.Users/2))
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(c.ConnMaxLifetime)
}

func (c Config) RunID() string {
	return time.Now().UTC().Format("20060102T150405Z")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func intFromEnv(getenv func(string) string, name string, fallback int) int {
	raw := getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func boolFromEnv(getenv func(string) string, name string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(getenv(name)))
	if raw == "" {
		return fallback
	}
	return raw == "1" || raw == "true" || raw == "yes" || raw == "y"
}

func durationFromEnv(getenv func(string) string, name string, fallback time.Duration) time.Duration {
	raw := getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", raw)
	}
}
