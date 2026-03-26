package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
	AdminID       int64
	ProxyURL      string

	PrinterHost   string
	PrinterPort   int
	PrinterFormat string // urfgray | urfrgb | pdf

	JobTTLMinutes int
	MaxCopies     int
	DataDir       string // директория для users.json
}

func Load() (*Config, error) {
	// Загружаем .env если есть (не ошибка если нет — в Docker используем env vars)
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN не задан")
	}

	adminStr := os.Getenv("ADMIN_ID")
	if adminStr == "" {
		return nil, fmt.Errorf("ADMIN_ID не задан")
	}
	adminID, err := strconv.ParseInt(adminStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ADMIN_ID должен быть числом: %w", err)
	}
	cfg.AdminID = adminID

	cfg.PrinterHost = os.Getenv("PRINTER_HOST")
	if cfg.PrinterHost == "" {
		return nil, fmt.Errorf("PRINTER_HOST не задан")
	}

	cfg.PrinterPort = 631
	if portStr := os.Getenv("PRINTER_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("PRINTER_PORT должен быть числом: %w", err)
		}
		cfg.PrinterPort = port
	}

	cfg.ProxyURL = os.Getenv("PROXY_URL")

	cfg.JobTTLMinutes = 15
	if ttlStr := os.Getenv("JOB_TTL_MINUTES"); ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err == nil && ttl > 0 {
			cfg.JobTTLMinutes = ttl
		}
	}

	cfg.DataDir = os.Getenv("DATA_DIR") // пусто = текущая директория

	// PRINTER_FORMAT: urfgray (по умолчанию), urfrgb, pdf
	cfg.PrinterFormat = os.Getenv("PRINTER_FORMAT")
	switch cfg.PrinterFormat {
	case "urfrgb", "pdf":
		// ok
	default:
		cfg.PrinterFormat = "urfgray"
	}

	cfg.MaxCopies = 10
	if mcStr := os.Getenv("MAX_COPIES"); mcStr != "" {
		mc, err := strconv.Atoi(mcStr)
		if err == nil && mc > 0 {
			cfg.MaxCopies = mc
		}
	}

	return cfg, nil
}
