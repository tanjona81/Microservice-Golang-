package config

import (
	"example/hello/internal/utils"
	"fmt"
	"time"
)

type OldConfig struct {
	LogLevel      string
	Database      DatabaseOldConfig
	Redis         RedisOldConfig
	Security      SecurityOldConfig
	CircuitBreak  CircuitBreakerOldConfig
	Timeout       TimeoutOldConfig
	Bruteforece   BruteforceOldConfig
	ShadowLimiter ShadowLimiterOldConfig
}

type DatabaseOldConfig struct {
	User            string
	Password        string
	Host            string
	Port            string
	Name            string
	Params          string
	MigrationParams string
	DSN             string
}

type SecurityOldConfig struct {
	BcryptCost      int
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type RedisOldConfig struct {
	RedisAddr string
}

type CircuitBreakerOldConfig struct {
	TimeoutSeconds time.Duration
	Timeout        string
	Interval       time.Duration
	MaxRequests    int
}

type TimeoutOldConfig struct {
	Auth     time.Duration
	Standard time.Duration
	Long     time.Duration
}

type BruteforceOldConfig struct {
	MaxAttempts int
	BanDuration time.Duration
}

type ShadowLimiterOldConfig struct {
	MaxAttempts int
	Window      time.Duration
}

func LoadOldConfig() *OldConfig {
	return &OldConfig{
		LogLevel: utils.GetEnv("LOG_LEVEL", "info"),
		Database: DatabaseOldConfig{
			User:            utils.GetEnv("DB_USER", "root"),
			Password:        utils.GetEnv("DB_PASSWORD", "root"),
			Host:            utils.GetEnv("DB_HOST", "127.0.0.1"),
			Port:            utils.GetEnv("DB_PORT", "3306"),
			Name:            utils.GetEnv("DB_NAME", "user"),
			Params:          utils.GetEnv("DB_APP_PARAMS", "parseTime=true"),
			MigrationParams: utils.GetEnv("DB_MIGRATE_PARAMS", "parseTime=true&multiStatements=true"),
			DSN: fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
				utils.GetEnv("DB_USER", "root"),
				utils.GetEnv("DB_PASSWORD", "root"),
				utils.GetEnv("DB_HOST", "127.0.0.1"),
				utils.GetEnv("DB_PORT", "3306"),
				utils.GetEnv("DB_NAME", "user"),
				utils.GetEnv("DB_MIGRATE_PARAMS", "parseTime=true&multiStatements=true"), // Use the safe version
			),
		},
		Redis: RedisOldConfig{
			RedisAddr: utils.GetEnv("REDIS_ADDR", "127.0.0.1:6379"),
		},
		Security: SecurityOldConfig{
			BcryptCost:      utils.GetEnvAsInt("BCRYPT_COST", 12),
			AccessTokenTTL:  utils.GetEnvAsDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL: utils.GetEnvAsDuration("REFRESH_TOKEN_TTL", 168*time.Hour),
		},
		CircuitBreak: CircuitBreakerOldConfig{
			TimeoutSeconds: time.Duration(utils.GetEnvAsInt("DB_TIMEOUT", 30)) * time.Second,
			Timeout:        utils.GetEnv("DB_TIMEOUT", "30"),
			Interval:       utils.GetEnvAsDuration("DB_INTERVAL", 10*time.Second),
			MaxRequests:    utils.GetEnvAsInt("DB_MAX_REQUESTS", 5),
		},
		Timeout: TimeoutOldConfig{
			Auth:     utils.GetEnvAsDuration("TIMEOUT_AUTH", 3*time.Second),
			Standard: utils.GetEnvAsDuration("TIMEOUT_STANDARD", 1*time.Second),
			Long:     utils.GetEnvAsDuration("TIMEOUT_LONG", 5*time.Second),
		},
		Bruteforece: BruteforceOldConfig{
			MaxAttempts: utils.GetEnvAsInt("RATELIMITER_MAX_ATTEMPTS", 5),
			BanDuration: utils.GetEnvAsDuration("RATELIMITER_BANDURATION", 10*time.Minute),
		},
		ShadowLimiter: ShadowLimiterOldConfig{
			MaxAttempts: utils.GetEnvAsInt("SHADOWLIMITER_MAX_ATTEMPTS", 3),
			Window:      utils.GetEnvAsDuration("SHADOWLIMITER_WINDOW", 1*time.Minute),
		},
	}
}
