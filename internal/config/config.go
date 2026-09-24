// Package config loads exchange configuration from the environment.
package config

import (
	"os"
	"time"
	"flag"
	"log"
	"github.com/ilyakaznacheev/cleanenv"
)

type HttpServer struct {
	Addr string `yaml:"address" env:"HOST" env-default:"0.0.0.0"`
	Port string `yaml:"port" env:"PORT" env-default:"8080"`
}
//TODO: complete config struct with all the required fields,and validation
type Config struct {
	Env string `yaml:"env" env-required:"true"`
	DBPath string `yaml:"dbPath" env:"DB_PATH" env-required:"true"`
	HttpServer `yaml:"httpServer"`

	LogLevel        string
	LogPretty       bool
	WALPath         string
	WALHMACKey      string
	Symbols         []string
	ShutdownTimeout time.Duration
}

func (srv *HttpServer) GetAddr() string {
	return srv.Addr+":"+srv.Port
}

func (cfg *Config) checkEnv(){
	if(cfg.Env=="dev" && cfg.HttpServer.Addr=="localhost"){
		log.Printf("develepoment Environment, addr: %s",cfg.HttpServer.GetAddr())
	}else if(cfg.Env=="prod"){
		if(cfg.HttpServer.Addr != "0.0.0.0"){
			cfg.HttpServer.Addr = "0.0.0.0"
			log.Printf("setting address to %s",cfg.HttpServer.Addr)
		}
		log.Println("Production Environment")
	}else{
		log.Fatalf("invalid Env value: %s",cfg.Env)
	}
}

func MustLoad() *Config {
	var configPath string

	configPath = os.Getenv("CONFIG_PATH")

	if configPath == "" {
		flags := flag.String("config", "","path to config file")
		flag.Parse()

		configPath = *flags

		if configPath == ""{
			log.Fatal("config path not set")
		}
	}
	
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var config Config
	err := cleanenv.ReadConfig(configPath,&config)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	config.checkEnv()

	return &config
}

