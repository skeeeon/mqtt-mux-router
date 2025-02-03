package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/broker"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/rule"
)

func main() {
	// Command line flags
	configPath := flag.String("config", "config/config.json", "path to config file")
	rulesPath := flag.String("rules", "rules", "path to rules directory")
	workers := flag.Int("workers", runtime.NumCPU(), "number of worker threads")
	queueSize := flag.Int("queue-size", 1000, "size of processing queue")
	batchSize := flag.Int("batch-size", 100, "message batch size")
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
	defer logger.Sync()

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Create broker with processing configuration
	brokerCfg := broker.BrokerConfig{
		ProcessorWorkers: *workers,
		QueueSize:       *queueSize,
		BatchSize:       *batchSize,
	}

	mqttBroker, err := broker.NewBroker(cfg, logger, brokerCfg)
	if err != nil {
		logger.Fatal("failed to create broker", "error", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load rules from directory
	rulesLoader := rule.NewRulesLoader(logger)
	rules, err := rulesLoader.LoadFromDirectory(*rulesPath)
	if err != nil {
		logger.Fatal("failed to load rules", "error", err)
	}

	// Start processing
	if err := mqttBroker.Start(ctx, rules); err != nil {
		logger.Fatal("failed to start broker", "error", err)
	}

	logger.Info("mqtt-mux-router started",
		"workers", *workers,
		"queueSize", *queueSize,
		"batchSize", *batchSize,
		"rulesCount", len(rules))

	// Handle signals
	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGHUP:
			logger.Info("received SIGHUP, reopening logs")
			logger.Sync()
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("shutting down...")
			cancel()
			mqttBroker.Close()
			return
		}
	}
}
