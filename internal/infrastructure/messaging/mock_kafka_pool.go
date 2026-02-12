package messaging

import (
	"context"
	"sync"
	"time"

	"github.com/ivan-salazar14/nexus-order-manager/internal/config"
	"github.com/ivan-salazar14/nexus-order-manager/internal/domain"
	"go.uber.org/zap"
)

// MockKafkaPool is a no-op implementation for development when Kafka is unavailable
type MockKafkaPool struct {
	logger   *zap.Logger
	topics   config.KafkaTopicsConfig
	mu       sync.Mutex
	messages []map[string]interface{}
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMockKafkaPool creates a mock Kafka pool for development
func NewMockKafkaPool(cfg *config.KafkaConfig, logger *zap.Logger) *MockKafkaPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockKafkaPool{
		logger: logger,
		topics: cfg.Topics,
		ctx:    ctx,
		cancel: cancel,
	}
}

// PublishOrderEvent publishes an order event (logs only in mock)
func (mkp *MockKafkaPool) PublishOrderEvent(ctx context.Context, event *domain.Order) error {
	mkp.mu.Lock()
	defer mkp.mu.Unlock()

	mkp.messages = append(mkp.messages, map[string]interface{}{
		"topic":   mkp.topics.Orders,
		"key":     event.ID,
		"value":   event,
		"created": time.Now(),
	})

	mkp.logger.Info("Mock: Published order event",
		zap.String("order_id", event.ID),
		zap.String("topic", mkp.topics.Orders),
	)

	return nil
}

// PublishGenericEvent publishes a generic event (logs only in mock)
func (mkp *MockKafkaPool) PublishGenericEvent(ctx context.Context, topic string, key string, value interface{}) error {
	mkp.mu.Lock()
	defer mkp.mu.Unlock()

	mkp.messages = append(mkp.messages, map[string]interface{}{
		"topic":   topic,
		"key":     key,
		"value":   value,
		"created": time.Now(),
	})

	mkp.logger.Info("Mock: Published generic event",
		zap.String("topic", topic),
		zap.String("key", key),
	)

	return nil
}

// ConsumeOrderEvents is a no-op in mock
func (mkp *MockKafkaPool) ConsumeOrderEvents(handler func(*domain.Order) error) {
	mkp.wg.Add(1)
	go func() {
		defer mkp.wg.Done()
		<-mkp.ctx.Done()
	}()
}

// Close closes the mock pool
func (mkp *MockKafkaPool) Close() error {
	mkp.cancel()
	mkp.wg.Wait()
	mkp.logger.Info("Mock Kafka pool closed")
	return nil
}

// EnsureTopicsExist is a no-op in mock
func (mkp *MockKafkaPool) EnsureTopicsExist() error {
	mkp.logger.Info("Mock: Topics would be created", zap.String("orders_topic", mkp.topics.Orders))
	return nil
}

// GetMessages returns all messages (for testing)
func (mkp *MockKafkaPool) GetMessages() []map[string]interface{} {
	mkp.mu.Lock()
	defer mkp.mu.Unlock()
	return mkp.messages
}
