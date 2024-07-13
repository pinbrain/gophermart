package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type ServerConf struct {
	ServerAddress  string `env:"RUN_ADDRESS"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DSN            string `env:"DATABASE_URI"`
	LogLevel       string `env:"LOG_LEVEL"`
}

func InitConfig() (ServerConf, error) {
	serverConf := ServerConf{}

	err := godotenv.Load()
	if err != nil {
		return serverConf, err
	}
	err = env.Parse(&serverConf)
	if err != nil {
		return serverConf, err
	}

	return serverConf, nil
}
