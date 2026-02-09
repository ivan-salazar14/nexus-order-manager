package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ivan-salazar14/nexus-order-manager/internal/application"
	"github.com/ivan-salazar14/nexus-order-manager/internal/config"
	"github.com/ivan-salazar14/nexus-order-manager/internal/domain"
	exchange "github.com/ivan-salazar14/nexus-order-manager/internal/infrastructure/exchange"
	"github.com/ivan-salazar14/nexus-order-manager/internal/infrastructure/http"
	"github.com/ivan-salazar14/nexus-order-manager/internal/infrastructure/messaging"
	"github.com/ivan-salazar14/nexus-order-manager/internal/infrastructure/persistence"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.Info("Starting Nexus Trading Engine",
		zap.String("app", cfg.App.Name),
		zap.String("environment", cfg.App.Environment),
	)

	// Initialize PostgreSQL repository
	repo, err := persistence.NewPostgresRepository(&cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer repo.Close()

	// Run migrations
	if err := repo.AutoMigrate(); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize Kafka pool
	kafkaPool := messaging.NewKafkaPool(&cfg.Kafka, logger)
	if err := kafkaPool.EnsureTopicsExist(); err != nil {
		logger.Warn("Failed to ensure Kafka topics exist", zap.Error(err))
	}
	defer kafkaPool.Close()

	// Initialize Binance client
	binanceClient := exchange.NewBinanceTestnetClient(
		cfg.Binance.Testnet.APIKey,
		cfg.Binance.Testnet.APISecret,
	)

	// Initialize trading orchestrator
	orchestrator := application.NewTradingOrchestrator(
		repo,
		binanceClient,
		kafkaPool,
		logger,
		3, // worker pool size
	)
	defer orchestrator.Stop()

	// Start background processes
	orderChan := make(chan *domain.Order, 100)
	orchestrator.StartWorkerPool(orderChan)
	orchestrator.StartOutboxRelay(cfg.Outbox.PollInterval())

	// Initialize HTTP server (infrastructure layer)
	httpServer := http.NewHTTPServer(
		cfg,
		logger,
		orchestrator,
		repo,
	)

	// Start HTTP server
	if err := httpServer.Start(); err != nil {
		logger.Fatal("Failed to start HTTP server", zap.Error(err))
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down gracefully...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Stop HTTP server
		if err := httpServer.Stop(ctx); err != nil {
			logger.Error("Error during HTTP server shutdown", zap.Error(err))
		}
	}()

	// Block main thread
	select {}
}
