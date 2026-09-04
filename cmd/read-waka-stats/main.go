package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/app"
	"github.com/vihuvac/read-waka-stats/internal/config"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

var exitFunc = os.Exit

func main() {
	exitFunc(run())
}

func run() int {
	cfg, err := config.Load()
	log := logging.New(false)
	if err != nil {
		log.Error("%v", err)
		return 1
	}
	log = logging.New(cfg.DebugLogging)
	log.Success("Program execution started at %s", time.Now().UTC().Format(time.RFC3339))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	start := time.Now()
	application := app.New(cfg, log)
	if base := strings.TrimSpace(os.Getenv("GITHUB_API_BASE")); base != "" {
		application.GitHubAPIBase = base
	}
	if err := application.Run(ctx); err != nil {
		log.Error("%v", err)
		return 1
	}
	log.Success("Program finished in %s", time.Since(start))
	return 0
}
