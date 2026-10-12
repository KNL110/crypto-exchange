package config

import "testing"

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
