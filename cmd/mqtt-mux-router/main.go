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
	// Command line flags
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

	// Setup metrics
	reg := prometheus.NewRegistry()
	metricsService, err := metrics.NewMetrics(reg)
	if err != nil {
		logger.Fatal("failed to create metrics service", "error", err)
	}

	// Create metrics server if enabled
	var metricsServer *http.Server
	if cfg.Metrics.Enabled {
		updateInterval, err := time.ParseDuration(cfg.Metrics.UpdateInterval)
		if err != nil {
			logger.Fatal("invalid metrics update interval", "error", err)
		}

		collector := metrics.NewMetricsCollector(metricsService, updateInterval)
		collector.Start()
		defer collector.Stop()

		mux := http.NewServeMux()
		mux.Handle(cfg.Metrics.Path, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
			Registry:          reg,
			EnableOpenMetrics: true,
		}))

		metricsServer = &http.Server{
			Addr:    cfg.Metrics.Address,
			Handler: mux,
		}

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

	// Create broker manager
	brokerManager := broker.NewManager(logger, metricsService)

	// Initialize brokers from configuration
	for id, brokerCfg := range cfg.Brokers {
		if err := brokerManager.AddBroker(&brokerCfg); err != nil {
			logger.Fatal("failed to add broker",
				"id", id,
				"error", err)
		}
	}

	// Validate we have at least one source and one target broker
	sourceBrokers := brokerManager.GetBrokersByRole(broker.BrokerRoleSource)
	targetBrokers := brokerManager.GetBrokersByRole(broker.BrokerRoleTarget)

	if len(sourceBrokers) == 0 {
		logger.Fatal("no source brokers configured")
	}
	if len(targetBrokers) == 0 {
		logger.Fatal("no target brokers configured")
	}

	// Load and validate rules
	rulesLoader := rule.NewRulesLoader(logger)
	rules, err := rulesLoader.LoadFromDirectory(*rulesPath)
	if err != nil {
		logger.Fatal("failed to load rules", "error", err)
	}

	// Create and initialize rule processor
	processor := rule.NewProcessor(cfg.Processing.Workers, logger, metricsService)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start broker manager
	if err := brokerManager.Start(ctx); err != nil {
		logger.Fatal("failed to start broker manager", "error", err)
	}

	// Start rule processor
	processor.Start()

	logger.Info("mqtt-mux-router started",
		"workers", cfg.Processing.Workers,
		"queueSize", cfg.Processing.QueueSize,
		"batchSize", cfg.Processing.BatchSize,
		"rulesCount", len(rules),
		"sourceBrokers", len(sourceBrokers),
		"targetBrokers", len(targetBrokers),
		"totalBrokers", len(cfg.Brokers),
		"metricsEnabled", cfg.Metrics.Enabled)

	// Handle signals
	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGHUP:
			logger.Info("received SIGHUP, reloading configuration")
			
			// Reload configuration
			if _, err := config.Load(*configPath); err != nil {
				logger.Error("failed to reload config", "error", err)
				continue
			}

			// TODO: Implement hot reload of brokers and rules
			logger.Info("configuration reloaded")

		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("shutting down...")

			// Graceful shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			// Stop rule processor
			processor.Stop()

			// Stop broker manager
			if err := brokerManager.Stop(shutdownCtx); err != nil {
				logger.Error("failed to stop broker manager", "error", err)
			}

			// Shutdown metrics server if enabled
			if cfg.Metrics.Enabled && metricsServer != nil {
				if err := metricsServer.Shutdown(shutdownCtx); err != nil {
					logger.Error("failed to shutdown metrics server", "error", err)
				}
			}

			logger.Info("shutdown complete")
			return
		}
	}
}
