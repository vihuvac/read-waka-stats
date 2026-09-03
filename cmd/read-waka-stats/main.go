package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/app"
	"github.com/vihuvac/read-waka-stats/internal/config"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

func main() {
	cfg, err := config.Load()
	log := logging.New(false)
	if err != nil {
		log.Fatal("%v", err)
	}
	log = logging.New(cfg.DebugLogging)
	log.Success("Program execution started at %s", time.Now().UTC().Format(time.RFC3339))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	start := time.Now()
	if err := app.New(cfg, log).Run(ctx); err != nil {
		log.Fatal("%v", err)
	}
	log.Success("Program finished in %s", time.Since(start))
}
