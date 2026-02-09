package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/nexustrader/nexus-order-manager/internal/application"
	"github.com/nexustrader/nexus-order-manager/internal/config"
	"github.com/nexustrader/nexus-order-manager/internal/domain"
	"github.com/nexustrader/nexus-order-manager/internal/infrastructure/persistence"
	"go.uber.org/zap"
)

// HTTPServer wraps the Echo HTTP server
type HTTPServer struct {
	e            *echo.Echo
	logger       *zap.Logger
	orchestrator *application.TradingOrchestrator
	repo         *persistence.PostgresRepository
	cfg          *config.Config
	addr         string
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(
	cfg *config.Config,
	logger *zap.Logger,
	orchestrator *application.TradingOrchestrator,
	repo *persistence.PostgresRepository,
) *HTTPServer {
	e := echo.New()
	e.HideBanner = true

	srv := &HTTPServer{
		e:            e,
		logger:       logger,
		orchestrator: orchestrator,
		repo:         repo,
		cfg:          cfg,
		addr:         fmt.Sprintf(":%d", 8080),
	}

	srv.setupMiddleware()
	srv.setupRoutes()

	return srv
}

// setupMiddleware configures HTTP middleware
func (s *HTTPServer) setupMiddleware() {
	s.e.Use(middleware.Logger())
	s.e.Use(middleware.Recover())
	s.e.Use(middleware.RequestID())
	s.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))
}

// setupRoutes configures API routes
func (s *HTTPServer) setupRoutes() {
	// Health check
	s.e.GET("/health", s.healthCheck)

	// API v1 routes
	api := s.e.Group("/api/v1")

	// Order handlers
	api.POST("/orders", s.createOrder)
	api.GET("/orders/:id", s.getOrder)
	api.GET("/orders", s.listOrders)
}

// healthCheck handles health check requests
func (s *HTTPServer) healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// createOrder handles order creation
func (s *HTTPServer) createOrder(c echo.Context) error {
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

	if err := s.orchestrator.SubmitOrder(c.Request().Context(), order); err != nil {
		s.logger.Error("Failed to submit order", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to submit order",
		})
	}

	return c.JSON(http.StatusAccepted, order)
}

// getOrder handles order retrieval
func (s *HTTPServer) getOrder(c echo.Context) error {
	orderID := c.Param("id")
	order, err := s.repo.GetOrder(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Order not found",
		})
	}
	return c.JSON(http.StatusOK, order)
}

// listOrders handles order listing
func (s *HTTPServer) listOrders(c echo.Context) error {
	status := domain.OrderStatus(c.QueryParam("status"))
	limit := 50

	orders, err := s.repo.ListOrders(c.Request().Context(), status, limit)
	if err != nil {
		s.logger.Error("Failed to list orders", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list orders",
		})
	}
	return c.JSON(http.StatusOK, orders)
}

// Start starts the HTTP server
func (s *HTTPServer) Start() error {
	s.logger.Info("Starting HTTP server", zap.String("addr", s.addr))

	go func() {
		if err := s.e.Start(s.addr); err != nil && err != http.ErrServerClosed {
			s.logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *HTTPServer) Stop(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server...")
	return s.e.Shutdown(ctx)
}

// Echo returns the underlying Echo instance
func (s *HTTPServer) Echo() *echo.Echo {
	return s.e
}

// AddRoute adds a custom route to the server
func (s *HTTPServer) AddRoute(method, path string, handler echo.HandlerFunc) {
	s.e.Add(method, path, handler)
}

// AddGroup adds a route group
func (s *HTTPServer) AddGroup(prefix string) *echo.Group {
	return s.e.Group(prefix)
}
