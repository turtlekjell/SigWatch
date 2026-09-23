package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sigwatch/internal/app"
	"sigwatch/internal/config"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML configuration")
	cacheDir := flag.String("cache-dir", "", "persistent cache directory (default: platform user cache directory)")
	check := flag.Bool("check", false, "validate configuration and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("SigWatch", version)
		return
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(2)
	}
	if *check {
		fmt.Println("configuration valid:", *configPath)
		return
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv, err := app.New(cfg, logger, app.WithCacheDir(*cacheDir))
	if err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	srv.StartRefreshers(ctx)
	httpServer := &http.Server{Addr: cfg.Listen, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = httpServer.Shutdown(shutCtx)
	}()
	logger.Info("SigWatch starting", "listen", cfg.Listen, "theme", cfg.Theme, "regions", len(cfg.Regions), "version", version)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
