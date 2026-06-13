package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"temp/internal/app"
	"temp/internal/pkg/config"
	"temp/internal/pkg/logger"
)

func main() {
	log, err := logger.New()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	cfg := config.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := app.Run(ctx, cfg, log); err != nil {
		log.Error("application error", zap.Error(err))
		os.Exit(1)
	}
}
