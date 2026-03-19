package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/amidexe/teleprint/internal/bot"
	"github.com/amidexe/teleprint/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Ошибка конфигурации", "err", err)
		os.Exit(1)
	}

	b, err := bot.New(cfg)
	if err != nil {
		slog.Error("Ошибка запуска бота", "err", err)
		os.Exit(1)
	}

	// Graceful shutdown по Ctrl+C / SIGTERM
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		slog.Info("Завершение работы...")
		b.Stop()
	}()

	b.Start()
}
