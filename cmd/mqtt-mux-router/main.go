package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/broker"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/rule"
)

func main() {
	configPath := flag.String("config", "config/config.json", "path to config file")
	rulesPath := flag.String("rules", "rules", "path to rules directory")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := logger.NewLogger(&cfg.Logging)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	// Load rules
	ruleProcessor, err := rule.NewProcessor(*rulesPath, logger)
	if err != nil {
		logger.Fatal("failed to load rules", "error", err)
	}

	// Create broker
	mqttBroker, err := broker.NewBroker(cfg, logger)
	if err != nil {
		logger.Fatal("failed to create broker", "error", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start processing
	if err := mqttBroker.Start(ctx, ruleProcessor); err != nil {
		logger.Fatal("failed to start broker", "error", err)
	}

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	logger.Info("shutting down...")
	cancel()
	mqttBroker.Close()
}
