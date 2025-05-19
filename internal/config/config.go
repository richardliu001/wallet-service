package config

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

// Config top-level struct
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Postgres  PostgresConfig  `yaml:"postgres"`
	Redis     RedisConfig     `yaml:"redis"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	RateLimit RateLimitConfig `yaml:"ratelimit"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
}

type RateLimitConfig struct {
	RPS   int `yaml:"rps"`
	Burst int `yaml:"burst"`
}

// Load reads yaml file
func Load(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	// override DSN password from env if present
	if pw := os.Getenv("POSTGRES_PASSWORD"); pw != "" {
		cfg.Postgres.DSN = cfg.Postgres.DSN + " password=" + pw
	}
	return &cfg, nil
}
