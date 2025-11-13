package config

import (
	"flag"
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string       `yaml:"env" env-default:"local"`
	DatabaseUrl string       `yaml:"database_url" env-required:"false"`
	Server      ServerConfig `yaml:"rest" env-required:"false"`
	JWT         JWTSecret    `yaml:"jwt" env-required:"false"`
	Bot         BotConfig    `yaml:"bot" env-required:"true"`
}

type BotConfig struct {
	Token string `yaml:"token" env-required:"true"`
}

type ServerConfig struct {
	Port string `yaml:"port" env-default:":8080"`
}
type JWTSecret struct {
	Secret string `yaml:"secret" env-required:"true"`
}

func MustLoad() *Config {
	path := fetchConfigPath()

	if path == "" {
		panic("Config file not found in path")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("Config file not found in path")
	}

	var config Config
	log.Printf("Loading config from %s", path)
	if err := cleanenv.ReadConfig(path, &config); err != nil {
		panic(err)
	}
	return &config

}

func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "./config/local.yaml", "config path")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
