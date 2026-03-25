package config

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Global-ish variable within the package to hold the instance
var (
	cfg *Config
	mu  sync.RWMutex
)

type Env string

const (
	EnvDev   Env = "development"
	EnvStage Env = "staging"
	EnvProd  Env = "production"
)

type Config struct {
	AppEnv        Env                  `mapstructure:"APP_ENV"`
	LogLevel      string               `mapstructure:"LOG_LEVEL"`
	JwtSecretKey  string               `mapstructure:"JWT_SECRET_KEY"`
	Grpc          string               `mapstructure:"GRPC_ADDR"`
	Database      DatabaseConfig       `mapstructure:",squash"` // squash flattens the fields
	Redis         RedisConfig          `mapstructure:",squash"`
	Security      SecurityConfig       `mapstructure:",squash"`
	CircuitBreak  CircuitBreakerConfig `mapstructure:",squash"`
	Timeout       TimeoutConfig        `mapstructure:",squash"`
	Bruteforce    BruteforceConfig     `mapstructure:",squash"`
	ShadowLimiter ShadowLimiterConfig  `mapstructure:",squash"`
}

type DatabaseConfig struct {
	User            string `mapstructure:"DB_USER"`
	Password        string `mapstructure:"DB_PASSWORD"`
	Host            string `mapstructure:"DB_HOST"`
	Port            string `mapstructure:"DB_PORT"`
	Name            string `mapstructure:"DB_NAME"`
	Params          string `mapstructure:"DB_APP_PARAMS"`
	MigrationParams string `mapstructure:"DB_MIGRATE_PARAMS"`
	DSN             string // Will be computed
}

type SecurityConfig struct {
	BcryptCost      int           `mapstructure:"BCRYPT_COST"`
	AccessTokenTTL  time.Duration `mapstructure:"ACCESS_TOKEN_TTL"`
	RefreshTokenTTL time.Duration `mapstructure:"REFRESH_TOKEN_TTL"`
}

type RedisConfig struct {
	RedisAddr string `mapstructure:"REDIS_ADDR"`
	RedisPass string `mapstructure:"REDIS_PASSWORD"`
}

type CircuitBreakerConfig struct {
	TimeoutSeconds     time.Duration // Will be computed
	Timeout            string        `mapstructure:"DB_TIMEOUT"`
	Interval           time.Duration `mapstructure:"DB_INTERVAL"`
	MaxRequests        int           `mapstructure:"DB_MAX_REQUESTS"`
	FailureRatio       float64       `mapstructure:"DB_FAILURE_RATIO"`
	ConsecutiveFailure int           `mapstructure:"DB_CONSECUTIVE_FAILURE"`
}

type TimeoutConfig struct {
	Auth     time.Duration `mapstructure:"TIMEOUT_AUTH"`
	Standard time.Duration `mapstructure:"TIMEOUT_STANDARD"`
	Long     time.Duration `mapstructure:"TIMEOUT_LONG"`
}

type BruteforceConfig struct {
	MaxAttempts int           `mapstructure:"RATELIMITER_MAX_ATTEMPTS"`
	BanDuration time.Duration `mapstructure:"RATELIMITER_BANDURATION"`
}

type ShadowLimiterConfig struct {
	MaxAttempts int           `mapstructure:"SHADOWLIMITER_MAX_ATTEMPTS"`
	Window      time.Duration `mapstructure:"SHADOWLIMITER_WINDOW"`
}

func (e Env) IsProd() bool { return e == EnvProd }

// To be commented when in prod mode but still in http mode
// func (e Env) IsSecure() bool { return e == EnvProd || e == EnvStage }

// Uncommnent if isSecure is commented
func (e Env) IsSecure() bool { return false }

func initLogger(cfg *Config) {
	// Setup Logger based on Config
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler

	if cfg.AppEnv.IsProd() {
		// Production: Optimized JSON for log aggregators
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Development: Readable text with colors (if using a library) or standard text
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler).With(
		slog.String("env", string(cfg.AppEnv)),
		slog.String("version", "1.0.0"),
	)
	slog.SetDefault(logger)
}

// Helper to avoid duplicating code in Load and Watch
func unmarshalAndCompute(v *viper.Viper, c *Config) {
	if err := v.Unmarshal(c); err != nil {
		slog.Error("Unable to decode into struct", "err", err)
		return
	}

	for _, key := range v.AllKeys() {
		slog.Debug("Viper key found", "key", key, "value", v.Get(key))
	}

	// Re-compute fields
	c.Database.DSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		c.Database.User, c.Database.Password, c.Database.Host,
		c.Database.Port, c.Database.Name, c.Database.MigrationParams,
	)

	timeoutInt := v.GetInt("DB_TIMEOUT")
	if timeoutInt == 0 {
		timeoutInt = 30
	}
	c.CircuitBreak.TimeoutSeconds = time.Duration(timeoutInt) * time.Second
}

func (c *Config) Validate() error {
	var errs []string

	// Security Checks
	if c.Bruteforce.MaxAttempts <= 0 {
		errs = append(errs, "RATELIMITER_MAX_ATTEMPTS must be > 0 (found: %d)", strconv.Itoa(c.Bruteforce.MaxAttempts))
	}
	if c.Security.BcryptCost < 10 {
		errs = append(errs, "BCRYPT_COST is too low for production security (minimum 10)")
	}

	// Database Checks
	if c.Database.Password == "" {
		errs = append(errs, "DB_PASSWORD cannot be empty")
	}
	if c.Database.Host == "" {
		errs = append(errs, "DB_HOST is required")
	}

	// Infrastructure Checks
	if c.Redis.RedisAddr == "" {
		errs = append(errs, "REDIS_ADDR is required for Brute Force protection")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n - %s", strings.Join(errs, "\n - "))
	}

	return nil
}

func bindConfig(v *viper.Viper) {
	requiredKeys := map[string]string{
		"APP_ENV":                    "AppEnv",
		"LOG_LEVEL":                  "LogLevel",
		"GRPC_ADDR":                  "Grpc",
		"DB_USER":                    "Database.User",
		"DB_PASSWORD":                "Database.Password",
		"DB_HOST":                    "Database.Host",
		"DB_PORT":                    "Database.Port",
		"DB_NAME":                    "Database.Name",
		"DB_APP_PARAMS":              "Database.Params",
		"DB_MIGRATE_PARAMS":          "Database.MigrationParams",
		"JWT_SECRET_KEY":             "JwtSecretKey",
		"REDIS_ADDR":                 "Redis.RedisAddr",
		"REDIS_PASSWORD":             "Redis.RedisPass",
		"BCRYPT_COST":                "Security.BcryptCost",
		"ACCESS_TOKEN_TTL":           "Security.AccessTokenTTL",
		"REFRESH_TOKEN_TTL":          "Security.RefreshTokenTTL",
		"DB_MAX_REQUESTS":            "CircuitBreaker.MaxRequests",
		"DB_INTERVAL":                "CircuitBreaker.Interval",
		"DB_TIMEOUT":                 "CircuitBreaker.Timeout",
		"DB_FAILURE_RATIO":           "CircuitBreaker.FailureRatio",
		"DB_CONSECUTIVE_FAILURE":     "CircuitBreaker.ConsecutiveFailure",
		"TIMEOUT_AUTH":               "Timeout.Auth",
		"TIMEOUT_STANDARD":           "Timeout.Standard",
		"TIMEOUT_LONG":               "Timeout.Long",
		"RATELIMITER_MAX_ATTEMPTS":   "Bruteforce.MaxAttempts",
		"RATELIMITER_BANDURATION":    "Bruteforce.BanDuration",
		"SHADOWLIMITER_MAX_ATTEMPTS": "ShadowLimiter.MaxAttempts",
		"SHADOWLIMITER_WINDOW":       "ShadowLimiter.Window",
	}

	for envVar, viperKey := range requiredKeys {
		v.SetDefault(viperKey, "")
		if err := v.BindEnv(viperKey, envVar); err != nil {
			fmt.Printf("BindEnv error: %s", envVar)
		}

		if !v.IsSet(viperKey) {
			fmt.Printf("missing required env: %s", envVar)
		}
	}
}

func LoadConfig() *Config {
	v := viper.New()
	// Tell Viper where to find the file
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	// Map environment variables (e.g., DB_USER -> Database.User)
	// v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// Enable Environment Variable overrides
	v.AutomaticEnv()

	bindConfig(v)

	// v.BindEnv("DB_USER")
	// v.BindEnv("DB_PASSWORD")
	// v.BindEnv("REDIS_ADDR")

	// Set Defaults (Architect's Fail-safe)
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", "3306")
	v.SetDefault("RATELIMITER_MAX_ATTEMPTS", 5)
	v.SetDefault("RATELIMITER_BANDURATION", "15m")
	v.SetDefault("BCRYPT_COST", 12)

	// Read the file
	if err := v.ReadInConfig(); err != nil {
		log.Printf("No .env file found, using environment variables")
	}

	// Initial load
	cfg = &Config{}

	unmarshalAndCompute(v, cfg)

	// Set up the Watcher
	v.OnConfigChange(func(e fsnotify.Event) {
		slog.Info("Config file changed", "file", e.Name)

		// LOCK for writing to prevent crashes during web requests
		mu.Lock()
		defer mu.Unlock()

		unmarshalAndCompute(v, cfg)
	})

	initLogger(cfg)

	v.WatchConfig()

	return cfg
}
