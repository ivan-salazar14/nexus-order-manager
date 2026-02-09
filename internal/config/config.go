package config

import (
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	App      AppConfig      `yaml:"app"`
	Database DatabaseConfig `yaml:"database"`
	Binance  BinanceConfig  `yaml:"binance"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Outbox   OutboxConfig   `yaml:"outbox"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// AppConfig holds application settings
type AppConfig struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
}

// DatabaseConfig holds PostgreSQL connection settings
type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Name         string `yaml:"name"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	SSLMode      string `yaml:"ssl_mode"`
}

// DSN returns the PostgreSQL connection string
func (d *DatabaseConfig) DSN() string {
	return "host=" + d.Host +
		" port=" + itoa(d.Port) +
		" user=" + d.Username +
		" password=" + d.Password +
		" dbname=" + d.Name +
		" sslmode=" + d.SSLMode
}

// BinanceConfig holds Binance API settings
type BinanceConfig struct {
	Testnet BinanceTestnetConfig `yaml:"testnet"`
}

// BinanceTestnetConfig holds testnet-specific settings
type BinanceTestnetConfig struct {
	Enabled   bool   `yaml:"enabled"`
	APIKey    string `yaml:"api_key"`
	APISecret string `yaml:"api_secret"`
	BaseURL   string `yaml:"base_url"`
}

// KafkaConfig holds Kafka connection settings
type KafkaConfig struct {
	Brokers       []string          `yaml:"brokers"`
	ConsumerGroup string            `yaml:"consumer_group"`
	Topics        KafkaTopicsConfig `yaml:"topics"`
}

// KafkaTopicsConfig holds Kafka topic names
type KafkaTopicsConfig struct {
	Orders string `yaml:"orders"`
	Events string `yaml:"events"`
}

// OutboxConfig holds Outbox Relay settings
type OutboxConfig struct {
	PollIntervalMs int `yaml:"poll_interval_ms"`
	BatchSize      int `yaml:"batch_size"`
}

// PollInterval returns the poll interval as a time.Duration
func (o *OutboxConfig) PollInterval() time.Duration {
	return time.Duration(o.PollIntervalMs) * time.Millisecond
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Expand environment variables
	expandEnvVars(&cfg)

	return &cfg, nil
}

// expandEnvVars expands environment variables in config values
func expandEnvVars(cfg *Config) {
	cfg.Binance.Testnet.APIKey = expandEnvVar(cfg.Binance.Testnet.APIKey)
	cfg.Binance.Testnet.APISecret = expandEnvVar(cfg.Binance.Testnet.APISecret)
	cfg.Database.Password = expandEnvVar(cfg.Database.Password)
}

// expandEnvVar expands a single environment variable
func expandEnvVar(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return os.Getenv(value[2 : len(value)-1])
	}
	return value
}

// itoa converts an integer to string (helper function)
func itoa(i int) string {
	return string(rune('0'+i/1000%10)) + string(rune('0'+i/100%10)) + string(rune('0'+i/10%10)) + string(rune('0'+i%10))
}
