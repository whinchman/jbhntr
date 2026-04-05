package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/whinchman/jobhuntr/internal/config"
	"github.com/whinchman/jobhuntr/internal/generator"
	"github.com/whinchman/jobhuntr/internal/mailer"
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
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		slog.Error("database.url is not set in config; set DATABASE_URL environment variable")
		os.Exit(1)
	}

	port := cfg.Server.Port
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	fmt.Printf("jobhuntr starting on :%d\n", port)
	slog.Info("jobhuntr starting", "port", port, "base_url", cfg.Server.BaseURL)

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

	var sources []scraper.Source
	if cfg.Scraper.SerpAPIKey != "" && isEnabled(cfg.Scraper.EnabledSources, "serpapi") {
		sources = append(sources, scraper.NewSerpAPISource(cfg.Scraper.SerpAPIKey))
	}
	if cfg.Scraper.JSearchKey != "" && isEnabled(cfg.Scraper.EnabledSources, "jsearch") {
		sources = append(sources, scraper.NewJSearchSource(cfg.Scraper.JSearchKey))
	}
	if len(sources) == 0 {
		slog.Warn("no job sources configured — scraper will be idle")
	}
	ntfyNotifier := notifier.NewNtfyNotifier(cfg.Ntfy.Server, cfg.Server.BaseURL)
	summarizer := generator.NewAnthropicSummarizer(cfg.Claude.APIKey, "")
	sched := scraper.NewScheduler(sources, db, db, interval, logger).
		WithNotifier(ntfyNotifier).
		WithUserReader(db).
		WithSummarizer(summarizer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Start PDF converter and background worker.
	// PDF conversion is optional — if the converter fails to start (e.g. no
	// browser available in the environment), we log a warning and continue
	// with pdfConverter=nil so the worker skips PDF generation gracefully.
	var pdfConverter pdf.Converter
	if rc, err := pdf.NewRodConverter(); err != nil {
		slog.Warn("pdf converter unavailable, PDF generation will be skipped", "error", err)
	} else {
		pdfConverter = rc
		defer rc.Close()
	}

	claudeGen := generator.NewAnthropicGenerator(cfg.Claude.APIKey, cfg.Claude.Model)
	worker := generator.NewWorker(db, claudeGen, pdfConverter, cfg.Output.Dir, 30*time.Second, logger)

	// Construct and inject the mailer. Falls back to NoopMailer if SMTP is not configured.
	var m web.EmailSender
	if cfg.SMTP.Host != "" {
		m = mailer.NewSMTPMailer(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From)
	} else {
		slog.Warn("SMTP not configured — emails will be dropped (NoopMailer)")
		m = &mailer.NoopMailer{}
	}

	// Start HTTP server.
	webSrv := web.NewServerWithConfig(db, db, db, cfg).
		WithAdminStore(db).
		WithStatsStore(db).
		WithLastScrapeFn(sched.LastScrapeAt).
		WithScrapeInterval(interval).
		WithMailer(m)
	// Wire Drive token store when Google Drive OAuth is configured.
	if cfg.GoogleDrive.ClientID != "" {
		webSrv.WithDriveTokenStore(db)
	}
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
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

// isEnabled reports whether name is enabled in the sources list.
// An empty or nil slice means all sources are enabled (backward-compatible default).
func isEnabled(sources []string, name string) bool {
	if len(sources) == 0 {
		return true
	}
	for _, s := range sources {
		if s == name {
			return true
		}
	}
	return false
}
