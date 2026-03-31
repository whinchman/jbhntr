package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"net/http"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/generator"
	"github.com/whinchman/jobhuntr/internal/models"
	"github.com/whinchman/jobhuntr/internal/notifier"
	"github.com/whinchman/jobhuntr/internal/pdf"
	"github.com/whinchman/jobhuntr/internal/scraper"
	"github.com/whinchman/jobhuntr/internal/store"
	"github.com/whinchman/jobhuntr/internal/web"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	dbPath := flag.String("db", "jobhuntr.db", "path to SQLite database file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err, "path", *cfgPath)
		os.Exit(1)
	}

	fmt.Printf("jobhuntr starting on :%d\n", cfg.Server.Port)
	slog.Info("jobhuntr starting", "port", cfg.Server.Port, "base_url", cfg.Server.BaseURL)

	db, err := store.Open(*dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err, "path", *dbPath)
		os.Exit(1)
	}
	defer db.Close()

	interval, err := time.ParseDuration(cfg.Scraper.Interval)
	if err != nil {
		slog.Error("invalid scraper interval", "error", err, "interval", cfg.Scraper.Interval)
		os.Exit(1)
	}

	filters := make([]models.SearchFilter, len(cfg.SearchFilters))
	for i, f := range cfg.SearchFilters {
		filters[i] = models.SearchFilter{
			Keywords:  f.Keywords,
			Location:  f.Location,
			MinSalary: f.MinSalary,
			MaxSalary: f.MaxSalary,
			Title:     f.Title,
		}
	}

	src := scraper.NewSerpAPISource(cfg.Scraper.SerpAPIKey)
	ntfyNotifier := notifier.NewNtfyNotifier(cfg.Ntfy.Server, cfg.Ntfy.Topic, cfg.Server.BaseURL)
	sched := scraper.NewScheduler(src, db, filters, interval, logger).WithNotifier(ntfyNotifier)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		slog.Info("shutdown signal received")
		cancel()
	}()

	// Start HTTP server.
	webSrv := web.NewServerWithConfig(db, cfg, *cfgPath, cfg.Resume.Path)
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

	// Start PDF converter and background worker.
	pdfConverter, err := pdf.NewRodConverter()
	if err != nil {
		slog.Error("failed to start PDF converter", "error", err)
		os.Exit(1)
	}
	defer pdfConverter.Close()

	claudeGen := generator.NewAnthropicGenerator(cfg.Claude.APIKey, cfg.Claude.Model)
	worker := generator.NewWorker(db, claudeGen, pdfConverter, cfg.Output.Dir, 30*time.Second, logger)
	go worker.Start(ctx)

	// Start background scheduler.
	slog.Info("starting scheduler", "interval", interval, "filters", len(filters))
	go sched.Start(ctx)

	// Block until shutdown signal.
	<-ctx.Done()

	// Gracefully stop HTTP server.
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := httpServer.Shutdown(shutCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}
	slog.Info("jobhuntr stopped")
}
