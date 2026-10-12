package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ilyakaznacheev/cleanenv"
)

// checkEnv calls log.Fatalf on a bad env, which would exit the test binary.
func TestCheckEnvAcceptsDevOnAnyAddress(t *testing.T) {
	for _, addr := range []string{"localhost", "0.0.0.0", "127.0.0.1"} {
		cfg := Config{Env: "dev", HttpServer: HttpServer{Addr: addr, Port: "8080"}}
		cfg.checkEnv()
		if cfg.HttpServer.Addr != addr {
			t.Errorf("dev addr changed from %q to %q", addr, cfg.HttpServer.Addr)
		}
	}
}

func TestCheckEnvProdBindsAllInterfaces(t *testing.T) {
	cfg := Config{Env: "prod", HttpServer: HttpServer{Addr: "localhost", Port: "8080"}}
	cfg.checkEnv()
	if cfg.HttpServer.Addr != "0.0.0.0" {
		t.Fatalf("prod addr = %q, want 0.0.0.0", cfg.HttpServer.Addr)
	}
}

func TestLoadDotEnvRealEnvironmentWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("ZZ_FROM_FILE=file\nZZ_BOTH=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZZ_BOTH", "real")
	t.Setenv("ZZ_FROM_FILE", "") // registered for cleanup, then unset so the file can fill it
	os.Unsetenv("ZZ_FROM_FILE")

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("ZZ_FROM_FILE"); got != "file" {
		t.Errorf("ZZ_FROM_FILE = %q, want %q from .env", got, "file")
	}
	if got := os.Getenv("ZZ_BOTH"); got != "real" {
		t.Errorf("ZZ_BOTH = %q, want the real environment's %q", got, "real")
	}
}

func TestLoadDotEnvMissingFileIsFine(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatalf("loadDotEnv(missing): %v", err)
	}
}

// Secrets are read from the environment only: one written into the YAML is ignored.
func TestSecretsAreNotReadFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "env: dev\ndbPath: yaml-db\nwalHmacKey: yaml-key\nganacheServer:\n    privateKey: yaml-pk\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_PATH", "env-db")
	t.Setenv("OPERATOR_PRIVATE_KEY", "env-pk")
	t.Setenv("WAL_HMAC_KEY", "env-key")

	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if cfg.DBPath != "env-db" || cfg.GanacheServer.PrivateKey != "env-pk" || cfg.WALHMACKey != "env-key" {
		t.Fatalf("got db=%q pk=%q hmac=%q, want the env values", cfg.DBPath, cfg.GanacheServer.PrivateKey, cfg.WALHMACKey)
	}

	os.Unsetenv("OPERATOR_PRIVATE_KEY")
	if err := cleanenv.ReadConfig(path, &Config{}); err == nil {
		t.Fatalf("ReadConfig without OPERATOR_PRIVATE_KEY succeeded using the YAML value; want it required from env")
	}
}
