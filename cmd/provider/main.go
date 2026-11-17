package main

import (
	"context"
	"flag"
	"github.com/flemingdp/pocket-id-docker-provider/internal/provider"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	apply := flag.Bool("apply", false, "write changes to Pocket ID (default is dry-run)")
	once := flag.Bool("once", false, "reconcile once and exit")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	p, err := provider.New(provider.Config{BaseURL: os.Getenv("POCKET_ID_URL"), APIKey: os.Getenv("POCKET_ID_API_KEY"), DockerSocket: os.Getenv("DOCKER_SOCKET"), Apply: *apply}, logger)
	if err != nil {
		logger.Error("configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run := func() {
		ctxRun, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		if err := p.Reconcile(ctxRun); err != nil {
			logger.Error("reconcile failed", "error", err)
		}
	}
	run()
	if *once {
		return
	}
	ticker := time.NewTicker(provider.IntervalFromEnv())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
