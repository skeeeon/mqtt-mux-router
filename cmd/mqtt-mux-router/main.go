package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/broker"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
	"mqtt-mux-router/internal/rule"
)

func main() {
	// Command line flags for config and rules
	configPath := flag.String("config", "config/config.json", "path to config file")
	rulesPath := flag.String("rules", "rules", "path to rules directory")

	// Optional override flags
	workersOverride := flag.Int("workers", 0, "override number of worker threads (0 = use config)")
	queueSizeOverride := flag.Int("queue-size", 0, "override size of processing queue (0 = use config)")
	batchSizeOverride := flag.Int("batch-size", 0, "override message batch size (0 = use config)")
	metricsAddrOverride := flag.String("metrics-addr", "", "override metrics server address (empty = use config)")
	metricsPathOverride := flag.String("metrics-path", "", "override metrics endpoint path (empty = use config)")
	metricsIntervalOverride := flag.Duration("metrics-interval", 0, "override metrics collection interval (0 = use config)")

	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Apply any command line overrides
	cfg.ApplyOverrides(
		*workersOverride,
		*queueSizeOverride,
		*batchSizeOverride,
		*metricsAddrOverride,
		*metricsPathOverride,
		*metricsIntervalOverride,
	)

	// Initialize logger
	logger, err := logger.NewLogger(&cfg.Logging)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Setup metrics if enabled
	var metricsService *metrics.Metrics
	var metricsCollector *metrics.MetricsCollector
	var metricsServer *http.Server

	if cfg.Metrics.Enabled {
		// Initialize metrics
		reg := prometheus.NewRegistry()
		metricsService, err = metrics.NewMetrics(reg)
		if err != nil {
			logger.Fatal("failed to create metrics service", "error", err)
		}

		// Parse metrics update interval
		updateInterval, err := time.ParseDuration(cfg.Metrics.UpdateInterval)
		if err != nil {
			logger.Fatal("invalid metrics update interval", "error", err)
		}

		// Create metrics collector
		metricsCollector = metrics.NewMetricsCollector(metricsService, updateInterval)
		metricsCollector.Start()
		defer metricsCollector.Stop()

		// Setup metrics HTTP server
		mux := http.NewServeMux()
		mux.Handle(cfg.Metrics.Path, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
			Registry:          reg,
			EnableOpenMetrics: true,
		}))

		metricsServer = &http.Server{
			Addr:    cfg.Metrics.Address,
			Handler: mux,
		}

		// Start metrics server
		go func() {
			logger.Info("starting metrics server",
				"address", cfg.Metrics.Address,
				"path", cfg.Metrics.Path)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics server error", "error", err)
			}
		}()
	}

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Create broker with processing configuration
	brokerCfg := broker.BrokerConfig{
		ProcessorWorkers: cfg.Processing.Workers,
		QueueSize:       cfg.Processing.QueueSize,
		BatchSize:       cfg.Processing.BatchSize,
	}

	mqttBroker, err := broker.NewBroker(cfg, logger, brokerCfg, metricsService)
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
		"workers", cfg.Processing.Workers,
		"queueSize", cfg.Processing.QueueSize,
		"batchSize", cfg.Processing.BatchSize,
		"rulesCount", len(rules),
		"metricsEnabled", cfg.Metrics.Enabled)

	// Handle signals
	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGHUP:
			logger.Info("received SIGHUP, reopening logs")
			logger.Sync()
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("shutting down...")

			// Graceful shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			// Shutdown metrics server if enabled
			if cfg.Metrics.Enabled && metricsServer != nil {
				if err := metricsServer.Shutdown(shutdownCtx); err != nil {
					logger.Error("failed to shutdown metrics server", "error", err)
				}
			}

			// Shutdown other components
			cancel()
			mqttBroker.Close()
			return
		}
	}
}
