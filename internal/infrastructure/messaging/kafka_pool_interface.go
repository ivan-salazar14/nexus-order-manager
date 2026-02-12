package messaging

import (
	"context"

	"github.com/ivan-salazar14/nexus-order-manager/internal/domain"
)

// KafkaPoolInterface defines the interface for Kafka operations
type KafkaPoolInterface interface {
	PublishOrderEvent(ctx context.Context, event *domain.Order) error
	PublishGenericEvent(ctx context.Context, topic string, key string, value interface{}) error
	ConsumeOrderEvents(handler func(*domain.Order) error)
	Close() error
	EnsureTopicsExist() error
}
