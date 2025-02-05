package main

import (
    "context"
    "flag"
    "log"
    "net/http"
    "os"
    "os/signal"
    "runtime"
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
    workers := flag.Int("workers", runtime.NumCPU(), "number of worker threads")
    queueSize := flag.Int("queue-size", 1000, "size of processing queue")
    batchSize := flag.Int("batch-size", 100, "message batch size")
    metricsAddr := flag.String("metrics-addr", ":2112", "metrics server address")
    metricsPath := flag.String("metrics-path", "/metrics", "metrics endpoint path")
    metricsInterval := flag.Duration("metrics-interval", 15*time.Second, "metrics collection interval")
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

    // Initialize metrics
    reg := prometheus.NewRegistry()
    metricsService, err := metrics.NewMetrics(reg)
    if err != nil {
        logger.Fatal("failed to create metrics service", "error", err)
    }

    // Create metrics collector for system metrics
    metricsCollector := metrics.NewMetricsCollector(metricsService, *metricsInterval)
    metricsCollector.Start()
    defer metricsCollector.Stop()

    // Setup metrics HTTP server
    mux := http.NewServeMux()
    mux.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
        Registry:          reg,
        EnableOpenMetrics: true,
    }))

    metricsServer := &http.Server{
        Addr:    *metricsAddr,
        Handler: mux,
    }

    // Start metrics server in a goroutine
    go func() {
        logger.Info("starting metrics server", 
            "address", *metricsAddr,
            "path", *metricsPath)
        if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Error("metrics server error", "error", err)
        }
    }()

    // Setup signal handlers
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

    // Create broker with processing configuration
    brokerCfg := broker.BrokerConfig{
        ProcessorWorkers: *workers,
        QueueSize:       *queueSize,
        BatchSize:       *batchSize,
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
        "workers", *workers,
        "queueSize", *queueSize,
        "batchSize", *batchSize,
        "rulesCount", len(rules),
        "metricsAddr", *metricsAddr)

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

            // Shutdown metrics server
            if err := metricsServer.Shutdown(shutdownCtx); err != nil {
                logger.Error("failed to shutdown metrics server", "error", err)
            }

            // Shutdown other components
            cancel()
            mqttBroker.Close()
            return
        }
    }
}
