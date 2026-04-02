package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/generator"
	"github.com/whinchman/jobhuntr/internal/notifier"
	"github.com/whinchman/jobhuntr/internal/pdf"
	"github.com/whinchman/jobhuntr/internal/scraper"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err, "path", *cfgPath)
		os.Exit(1)
	}

	dsn := cfg.Database.URL
	if dsn == "" {
		slog.Error("database.url is not set in config; set DATABASE_URL environment variable")
		os.Exit(1)
	}

	fmt.Printf("jobhuntr starting on :%d\n", cfg.Server.Port)
	slog.Info("jobhuntr starting", "port", cfg.Server.Port, "base_url", cfg.Server.BaseURL)

	db, err := store.Open(dsn)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	interval, err := time.ParseDuration(cfg.Scraper.Interval)
	if err != nil {
		slog.Error("invalid scraper interval", "error", err, "interval", cfg.Scraper.Interval)
		os.Exit(1)
	}

	src := scraper.NewSerpAPISource(cfg.Scraper.SerpAPIKey)
	ntfyNotifier := notifier.NewNtfyNotifier(cfg.Ntfy.Server, cfg.Server.BaseURL)
	summarizer := generator.NewAnthropicSummarizer(cfg.Claude.APIKey, "")
	sched := scraper.NewScheduler(src, db, db, interval, logger).
		WithNotifier(ntfyNotifier).
		WithUserReader(db).
		WithSummarizer(summarizer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Start PDF converter and background worker.
	pdfConverter, err := pdf.NewRodConverter()
	if err != nil {
		slog.Error("failed to start PDF converter", "error", err)
		os.Exit(1)
	}
	defer pdfConverter.Close()

	claudeGen := generator.NewAnthropicGenerator(cfg.Claude.APIKey, cfg.Claude.Model)
	worker := generator.NewWorker(db, claudeGen, pdfConverter, cfg.Output.Dir, cfg.Resume.Path, 30*time.Second, logger)

	// Start HTTP server.
	webSrv := web.NewServerWithConfig(db, db, db, cfg).
		WithLastScrapeFn(sched.LastScrapeAt)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: webSrv.Handler(),
	}
	go func() {
		slog.Info("http server listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
		}
	}()

	// Start background goroutines; WaitGroup lets shutdown wait for them.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		slog.Info("starting generator worker")
		worker.Start(ctx)
	}()
	go func() {
		defer wg.Done()
		slog.Info("starting scheduler", "interval", interval)
		sched.Start(ctx)
	}()

	// Block until shutdown signal.
	<-sig
	slog.Info("shutdown signal received")
	cancel()

	// Gracefully stop HTTP server (stop accepting new requests).
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := httpServer.Shutdown(shutCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	// Wait for scheduler and worker to finish their current operation.
	wg.Wait()

	slog.Info("jobhuntr stopped")
}
