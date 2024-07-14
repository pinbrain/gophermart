package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type ServerConf struct {
	ServerAddress  string `env:"RUN_ADDRESS" envDefault:":8080"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DSN            string `env:"DATABASE_URI"`
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"`
}

func validateConf(cfg ServerConf) error {
	invalidParams := []string{}

	if cfg.AccrualAddress == "" {
		invalidParams = append(invalidParams, "accrual address")
	}
	if cfg.DSN == "" {
		invalidParams = append(invalidParams, "database uri")
	}

	if len(invalidParams) > 0 {
		return fmt.Errorf("invalid config params: %s", strings.Join(invalidParams, "; "))
	}
	return nil
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

	if err = validateConf(serverConf); err != nil {
		return serverConf, err
	}

	return serverConf, nil
}
