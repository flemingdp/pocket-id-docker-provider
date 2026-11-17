package main

import (
	"context"
	"flag"
	"github.com/flemingdp/pocket-id-docker-provider/internal/provider"
	"log/slog"
 "strings"
 "fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	apply := flag.Bool("apply", false, "write changes to Pocket ID (default is dry-run)")
	once := flag.Bool("once", false, "reconcile once and exit")
	flag.Parse()
	level, err := logLevel(os.Getenv("LOG_LEVEL"))
 if err != nil {fmt.Fprintln(os.Stderr,err);os.Exit(2)}
 opts := &slog.HandlerOptions{Level:level}
 var handler slog.Handler
 switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT"))) {
 case "", "json": handler=slog.NewJSONHandler(os.Stdout,opts)
 case "text": handler=slog.NewTextHandler(os.Stdout,opts)
 default: fmt.Fprintln(os.Stderr,"LOG_FORMAT must be json or text");os.Exit(2)
 }
 logger := slog.New(handler)
	p, err := provider.New(provider.Config{BaseURL: os.Getenv("POCKET_ID_URL"), APIKey: os.Getenv("POCKET_ID_API_KEY"), DockerSocket: os.Getenv("DOCKER_SOCKET"), Apply: *apply}, logger)
	if err != nil {
		logger.Error("configuration", "error", err)
		os.Exit(1)
	}
	mode:="dry-run"
 if *apply {mode="apply"}
 logger.Info("provider started","mode",mode,"once",*once,"interval",provider.IntervalFromEnv().String(),"log_format",strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT"))))
 ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run := func() {
		ctxRun, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		start:=time.Now()
 summary,err:=p.ReconcileWithSummary(ctxRun)
 args:=[]any{
 "duration_ms",time.Since(start).Milliseconds(),
 "containers_scanned",summary.ContainersScanned,
 "clients_discovered",summary.ClientsDiscovered,
 "clients_created",summary.ClientsCreated,
 "clients_updated",summary.ClientsUpdated,
 "clients_unchanged",summary.ClientsUnchanged,
 "invalid_labels",summary.InvalidLabels,
 "errors",summary.Errors,
 }
 if err!=nil {logger.Error("reconciliation failed",append(args,"error",err)...)
 } else if summary.ClientsCreated>0 || summary.ClientsUpdated>0 || summary.InvalidLabels>0 || *once {
 logger.Info("reconciliation complete",args...)
 } else {logger.Debug("reconciliation complete",args...)}

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

func logLevel(v string) (slog.Level,error) {
 switch strings.ToLower(strings.TrimSpace(v)) {
 case "","info":return slog.LevelInfo,nil
 case "debug":return slog.LevelDebug,nil
 case "warn","warning":return slog.LevelWarn,nil
 case "error":return slog.LevelError,nil
 default:return 0,fmt.Errorf("invalid LOG_LEVEL %q: expected debug, info, warn, error",v)
 }
}
