package config

import (
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type ServerConf struct {
	ServerAddress  string `env:"RUN_ADDRESS"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DSN            string `env:"DATABASE_URI"`
	LogLevel       string `env:"LOG_LEVEL"`
}

func validateConf(cfg ServerConf) error {
	invalidParams := []string{}

	if cfg.AccrualAddress == "" {
		invalidParams = append(invalidParams, "accrual address")
	} else {
		if err := validateBaseURL(cfg.AccrualAddress); err != nil {
			invalidParams = append(invalidParams, "accrual address")
		}
	}
	if cfg.DSN == "" {
		invalidParams = append(invalidParams, "database uri")
	}

	if len(invalidParams) > 0 {
		return fmt.Errorf("invalid config params: %s", strings.Join(invalidParams, "; "))
	}
	return nil
}

func loadFlags(cfg *ServerConf) error {
	flag.StringVar(&cfg.ServerAddress, "a", ":8080", "Адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.LogLevel, "l", "info", "Уровень логирования")
	flag.StringVar(&cfg.DSN, "d", "", "Строка с адресом подключения к БД")
	flag.StringVar(&cfg.AccrualAddress, "r", "", "Адрес системы расчёта начислений")
	flag.Parse()

	return nil
}

func loadEnvs(cfg *ServerConf) error {
	err := godotenv.Load()
	if err != nil {
		return nil
	}
	err = env.Parse(cfg)
	if err != nil {
		return err
	}

	return nil
}

func validateBaseURL(baseURL string) error {
	_, err := url.ParseRequestURI(baseURL)
	return err
}

func InitConfig() (ServerConf, error) {
	serverConf := ServerConf{}

	if err := loadFlags(&serverConf); err != nil {
		return serverConf, err
	}
	if err := loadEnvs(&serverConf); err != nil {
		return serverConf, err
	}

	if err := validateConf(serverConf); err != nil {
		return serverConf, err
	}

	return serverConf, nil
}
