package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ivan-salazar14/nexus-order-manager/internal/domain"
	"github.com/ivan-salazar14/nexus-order-manager/internal/infrastructure/exchange"
	"github.com/ivan-salazar14/nexus-order-manager/internal/infrastructure/messaging"
	"github.com/ivan-salazar14/nexus-order-manager/internal/infrastructure/persistence"
	"go.uber.org/zap"
)

// TradingOrchestrator coordinates order processing
type TradingOrchestrator struct {
	repo       *persistence.PostgresRepository
	exchange   exchange.BinanceClient
	kafkaPool  *messaging.KafkaPool
	logger     *zap.Logger
	workerPool int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewTradingOrchestrator creates a new trading orchestrator
func NewTradingOrchestrator(
	repo *persistence.PostgresRepository,
	exchangeClient exchange.BinanceClient,
	kafkaPool *messaging.KafkaPool,
	logger *zap.Logger,
	workerPool int,
) *TradingOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	return &TradingOrchestrator{
		repo:       repo,
		exchange:   exchangeClient,
		kafkaPool:  kafkaPool,
		logger:     logger,
		workerPool: workerPool,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SubmitOrder submits a new order for processing
func (to *TradingOrchestrator) SubmitOrder(ctx context.Context, order *domain.Order) error {
	to.logger.Info("Submitting order",
		zap.String("order_id", order.ID),
		zap.String("symbol", order.Symbol),
		zap.String("side", string(order.Side)),
	)

	// Create order in database
	if err := to.repo.CreateOrder(ctx, order); err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	// Create outbox event
	event := &domain.OutboxEvent{
		Aggregate:   "Order",
		AggregateID: order.ID,
		EventType:   "OrderSubmitted",
		Payload:     mustMarshal(order),
		Processed:   false,
	}

	if err := to.repo.CreateOutboxEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	return nil
}

// ProcessOrder processes an order from the queue
func (to *TradingOrchestrator) ProcessOrder(ctx context.Context, orderID string) error {
	order, err := to.repo.GetOrder(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	if !order.CanTransitionTo(domain.StatusExecuting) {
		return fmt.Errorf("order cannot transition to EXECUTING from %s", order.Status)
	}

	// Update status to EXECUTING
	order.Status = domain.StatusExecuting
	if err := to.repo.UpdateOrder(ctx, order); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Execute trade on exchange
	if err := to.exchange.ExecuteTrade(ctx, order); err != nil {
		order.Status = domain.StatusFailed
		order.UpdatedAt = time.Now()
		if updateErr := to.repo.UpdateOrder(ctx, order); updateErr != nil {
			return fmt.Errorf("trade failed and update failed: %w (original: %v)", updateErr, err)
		}
		return fmt.Errorf("trade execution failed: %w", err)
	}

	// Update status to COMPLETED
	order.Status = domain.StatusCompleted
	order.UpdatedAt = time.Now()
	if err := to.repo.UpdateOrder(ctx, order); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Publish completion event
	if err := to.kafkaPool.PublishOrderEvent(ctx, order); err != nil {
		to.logger.Error("Failed to publish order completion event", zap.Error(err))
	}

	return nil
}

// StartWorkerPool starts the worker pool for order processing
func (to *TradingOrchestrator) StartWorkerPool(orderChan chan *domain.Order) {
	for i := 0; i < to.workerPool; i++ {
		to.wg.Add(1)
		go func(workerID int) {
			defer to.wg.Done()
			to.logger.Info("Starting worker", zap.Int("worker_id", workerID))

			for {
				select {
				case <-to.ctx.Done():
					to.logger.Info("Worker shutting down", zap.Int("worker_id", workerID))
					return
				case order := <-orderChan:
					if err := to.ProcessOrder(to.ctx, order.ID); err != nil {
						to.logger.Error("Failed to process order",
							zap.String("order_id", order.ID),
							zap.Error(err),
						)
					}
				}
			}
		}(i)
	}
}

// StartOutboxRelay starts the outbox relay process
func (to *TradingOrchestrator) StartOutboxRelay(interval time.Duration) {
	to.wg.Add(1)
	go func() {
		defer to.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-to.ctx.Done():
				return
			case <-ticker.C:
				events, err := to.repo.GetUnprocessedOutboxEvents(to.ctx, 100)
				if err != nil {
					to.logger.Error("Failed to get unprocessed events", zap.Error(err))
					continue
				}

				for _, event := range events {
					if err := to.kafkaPool.PublishGenericEvent(
						to.ctx,
						"nexus.events",
						event.AggregateID,
						event,
					); err != nil {
						to.logger.Error("Failed to publish event", zap.Error(err))
						continue
					}

					if err := to.repo.MarkOutboxEventProcessed(to.ctx, event.ID); err != nil {
						to.logger.Error("Failed to mark event as processed", zap.Error(err))
					}
				}
			}
		}
	}()
}

// Stop stops the orchestrator gracefully
func (to *TradingOrchestrator) Stop() {
	to.cancel()
	to.wg.Wait()
}

// mustMarshal marshals an object to JSON, panicking on error
func mustMarshal(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return string(data)
}
