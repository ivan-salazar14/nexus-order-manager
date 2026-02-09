package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/nexustrader/nexus-order-manager/internal/application"
	"github.com/nexustrader/nexus-order-manager/internal/config"
	"github.com/nexustrader/nexus-order-manager/internal/domain"
	exchange "github.com/nexustrader/nexus-order-manager/internal/infrastructure/exchange"
	"github.com/nexustrader/nexus-order-manager/internal/infrastructure/messaging"
	"github.com/nexustrader/nexus-order-manager/internal/infrastructure/persistence"
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

	// Initialize Echo HTTP server
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	// CORS
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// API routes
	api := e.Group("/api/v1")

	// Order handlers
	api.POST("/orders", func(c echo.Context) error {
		var req struct {
			ID       string  `json:"id"`
			Symbol   string  `json:"symbol"`
			Side     string  `json:"side"`
			Type     string  `json:"type"`
			Quantity float64 `json:"quantity"`
			Price    float64 `json:"price"`
		}

		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid request body",
			})
		}

		order := domain.NewOrder(
			req.ID,
			req.Symbol,
			domain.OrderSide(req.Side),
			domain.OrderType(req.Type),
			req.Quantity,
			req.Price,
		)

		if !order.IsValid() {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid order data",
			})
		}

		if err := orchestrator.SubmitOrder(c.Request().Context(), order); err != nil {
			logger.Error("Failed to submit order", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to submit order",
			})
		}

		// Send to worker pool for processing
		orderChan <- order

		return c.JSON(http.StatusAccepted, order)
	})

	api.GET("/orders/:id", func(c echo.Context) error {
		orderID := c.Param("id")
		order, err := repo.GetOrder(c.Request().Context(), orderID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "Order not found",
			})
		}
		return c.JSON(http.StatusOK, order)
	})

	api.GET("/orders", func(c echo.Context) error {
		status := domain.OrderStatus(c.QueryParam("status"))
		limit := 50

		orders, err := repo.ListOrders(c.Request().Context(), status, limit)
		if err != nil {
			logger.Error("Failed to list orders", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to list orders",
			})
		}
		return c.JSON(http.StatusOK, orders)
	})

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down gracefully...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			logger.Error("Error during shutdown", zap.Error(err))
		}
	}()

	// Start server
	addr := fmt.Sprintf(":%d", 8080)
	logger.Info("Starting HTTP server", zap.String("addr", addr))
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed", zap.Error(err))
	}
}
