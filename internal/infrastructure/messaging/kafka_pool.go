package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ivan-salazar14/nexus-order-manager/internal/config"
	"github.com/ivan-salazar14/nexus-order-manager/internal/domain"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaPool manages Kafka producers and consumers
type KafkaPool struct {
	producer *kafka.Writer
	reader   *kafka.Reader
	logger   *zap.Logger
	topics   config.KafkaTopicsConfig
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewKafkaPool creates a new Kafka pool
func NewKafkaPool(cfg *config.KafkaConfig, logger *zap.Logger) *KafkaPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &KafkaPool{
		producer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 10 * time.Millisecond,
			Async:        false,
		},
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  cfg.Brokers,
			GroupID:  cfg.ConsumerGroup,
			Topic:    cfg.Topics.Orders,
			MinBytes: 10,
			MaxBytes: 10e6,
		}),
		logger: logger,
		topics: cfg.Topics,
		ctx:    ctx,
		cancel: cancel,
	}
}

// PublishOrderEvent publishes an order event to Kafka
func (kp *KafkaPool) PublishOrderEvent(ctx context.Context, event *domain.Order) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal order event: %w", err)
	}

	msg := kafka.Message{
		Topic: kp.topics.Orders,
		Key:   []byte(event.ID),
		Value: data,
		Time:  time.Now(),
	}

	if err := kp.producer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish order event: %w", err)
	}

	kp.logger.Info("Published order event to Kafka",
		zap.String("order_id", event.ID),
		zap.String("topic", kp.topics.Orders),
	)

	return nil
}

// PublishGenericEvent publishes a generic event to Kafka
func (kp *KafkaPool) PublishGenericEvent(ctx context.Context, topic string, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
	}

	if err := kp.producer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// ConsumeOrderEvents starts consuming order events
func (kp *KafkaPool) ConsumeOrderEvents(handler func(*domain.Order) error) {
	kp.wg.Add(1)
	go func() {
		defer kp.wg.Done()
		for {
			select {
			case <-kp.ctx.Done():
				return
			default:
				msg, err := kp.reader.FetchMessage(kp.ctx)
				if err != nil {
					if kp.ctx.Err() != nil {
						return
					}
					kp.logger.Error("Error fetching message", zap.Error(err))
					continue
				}

				var order domain.Order
				if err := json.Unmarshal(msg.Value, &order); err != nil {
					kp.logger.Error("Error unmarshaling message", zap.Error(err))
					continue
				}

				if err := handler(&order); err != nil {
					kp.logger.Error("Error handling order", zap.Error(err))
					continue
				}

				if err := kp.reader.CommitMessages(kp.ctx, msg); err != nil {
					kp.logger.Error("Error committing message", zap.Error(err))
				}
			}
		}
	}()
}

// Close closes the Kafka pool
func (kp *KafkaPool) Close() error {
	kp.cancel()
	kp.wg.Wait()

	if err := kp.producer.Close(); err != nil {
		return fmt.Errorf("failed to close producer: %w", err)
	}

	if err := kp.reader.Close(); err != nil {
		return fmt.Errorf("failed to close reader: %w", err)
	}

	return nil
}

// EnsureTopicsExist creates topics if they don't exist
func (kp *KafkaPool) EnsureTopicsExist() error {
	conn, err := kafka.Dial("tcp", kp.producer.Addr.String())
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer controllerConn.Close()

	topics := []kafka.TopicConfig{
		{
			Topic:             kp.topics.Orders,
			NumPartitions:     3,
			ReplicationFactor: 1,
		},
		{
			Topic:             kp.topics.Events,
			NumPartitions:     3,
			ReplicationFactor: 1,
		},
	}

	err = controllerConn.CreateTopics(topics...)
	if err != nil {
		// Ignore "topic already exists" error
		return nil
	}

	return nil
}
