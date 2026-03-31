package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/whinchman/jobhuntr/internal/config"
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

	fmt.Printf("jobhuntr starting on :%d\n", cfg.Server.Port)
	slog.Info("jobhuntr starting", "port", cfg.Server.Port, "base_url", cfg.Server.BaseURL)

	// Subsystems will be wired here in subsequent features.
	select {}
}
