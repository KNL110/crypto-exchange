// Package config loads exchange configuration.
//
// Non-secret settings (ports, hosts, log level, paths) live in the YAML
// config file and can be overridden by environment variables. Secrets (the
// database URL, the operator private key, the WAL HMAC key) are read from the
// environment only, never from the YAML file, so they can't end up in it.
// For local development a .env file in the working directory is loaded into
// the environment first.
package config

import (
	"errors"
	"flag"
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type HttpServer struct {
	Addr string `yaml:"address" env:"HOST" env-default:"0.0.0.0"`
	Port string `yaml:"port" env:"PORT" env-default:"8080"`
}

type GanacheServer struct {
	Address string `yaml:"address" env:"GANACHE_HOST" env-default:"localhost"`
	Port    string `yaml:"port" env:"GANACHE_PORT" env-default:"8545"`
	// PrivateKey is the exchange's own operator key (hex), never a user's key. Secret: env only.
	PrivateKey string `yaml:"-" env:"OPERATOR_PRIVATE_KEY" env-required:"true"`
}

// TODO: complete config struct with all the required fields,and validation
type Config struct {
	Env           string `yaml:"env" env-required:"true"`
	DBPath        string `yaml:"-" env:"DB_PATH" env-required:"true"` // contains the DB password. Secret: env only.
	HttpServer    `yaml:"httpServer"`
	GanacheServer `yaml:"ganacheServer"`

	LogLevel        string `yaml:"logLevel" env:"LOG_LEVEL" env-default:"info"`    // debug, info, warn, error
	LogPretty       bool   `yaml:"logPretty" env:"LOG_PRETTY" env-default:"false"` // human-readable console logs instead of JSON
	WALPath         string `yaml:"walPath" env:"WAL_PATH" env-default:"./data/wal.log"`
	WALHMACKey      string `yaml:"-" env:"WAL_HMAC_KEY"` // empty disables WAL tamper detection. Secret: env only.
	Symbols         []string
	ShutdownTimeout time.Duration
}

func (srv *HttpServer) GetAddr() string {
	return srv.Addr + ":" + srv.Port
}

func (srv *GanacheServer) GetAddr() string {
	return srv.Address + ":" + srv.Port
}

// TODO: check for what to do with ganache server
func (cfg *Config) checkEnv() {
	switch cfg.Env {
	case "dev":
		log.Printf("development Environment, addr: %s", cfg.HttpServer.GetAddr())
	case "prod":
		if cfg.HttpServer.Addr != "0.0.0.0" {
			cfg.HttpServer.Addr = "0.0.0.0"
			log.Printf("setting address to %s", cfg.HttpServer.Addr)
		}
		log.Println("Production Environment")
	default:
		log.Fatalf("invalid Env value: %s", cfg.Env)
	}
}

// loadDotEnv loads KEY=VALUE lines from path into the environment. Variables
// already set in the real environment win. A missing file is fine.
func loadDotEnv(path string) error {
	if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func MustLoad() *Config {
	if err := loadDotEnv(".env"); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	var configPath string

	configPath = os.Getenv("CONFIG_PATH")

	if configPath == "" {
		flags := flag.String("config", "", "path to config file")
		flag.Parse()

		configPath = *flags

		if configPath == "" {
			log.Fatal("config path not set")
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var config Config
	err := cleanenv.ReadConfig(configPath, &config)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	config.checkEnv()

	return &config
}
