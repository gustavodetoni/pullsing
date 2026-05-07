package config

import (
	"io"
	"os"
	"time"
)

const (
	defaultHTTPAddr         = ":8080"
	defaultShutdownTimeout  = 10 * time.Second
	defaultReadTimeout      = 5 * time.Second
	defaultReadHeaderTimout = 3 * time.Second
	defaultWriteTimeout     = 10 * time.Second
	defaultIdleTimeout      = 30 * time.Second
)

type Config struct {
	AppName           string
	Environment       string
	HTTPAddr          string
	ShutdownTimeout   time.Duration
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	PostgresURL       string
	RedisAddr         string
}

func Load() Config {
	return Config{
		AppName:           getEnv("PULLSING_APP_NAME", "pullsing-server"),
		Environment:       getEnv("PULLSING_ENV", "development"),
		HTTPAddr:          getEnv("PULLSING_HTTP_ADDR", defaultHTTPAddr),
		ShutdownTimeout:   getEnvDuration("PULLSING_SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
		ReadTimeout:       getEnvDuration("PULLSING_HTTP_READ_TIMEOUT", defaultReadTimeout),
		ReadHeaderTimeout: getEnvDuration("PULLSING_HTTP_READ_HEADER_TIMEOUT", defaultReadHeaderTimout),
		WriteTimeout:      getEnvDuration("PULLSING_HTTP_WRITE_TIMEOUT", defaultWriteTimeout),
		IdleTimeout:       getEnvDuration("PULLSING_HTTP_IDLE_TIMEOUT", defaultIdleTimeout),
		PostgresURL:       getEnv("PULLSING_POSTGRES_URL", "postgres://pullsing:pullsing@postgres:5432/pullsing?sslmode=disable"),
		RedisAddr:         getEnv("PULLSING_REDIS_ADDR", "redis:6379"),
	}
}

func (c Config) LogOutput() io.Writer {
	return os.Stdout
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}

	return fallback
}
