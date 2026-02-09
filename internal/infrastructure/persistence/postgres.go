package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/nexustrader/nexus-order-manager/internal/config"
	"github.com/nexustrader/nexus-order-manager/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresRepository handles database operations
type PostgresRepository struct {
	db *gorm.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(cfg *config.DatabaseConfig) (*PostgresRepository, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &PostgresRepository{db: db}, nil
}

// AutoMigrate runs database migrations
func (r *PostgresRepository) AutoMigrate() error {
	return r.db.AutoMigrate(
		&domain.Order{},
		&domain.OutboxEvent{},
	)
}

// CreateOrder creates a new order in the database
func (r *PostgresRepository) CreateOrder(ctx context.Context, order *domain.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// GetOrder retrieves an order by ID
func (r *PostgresRepository) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	var order domain.Order
	err := r.db.WithContext(ctx).First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// UpdateOrder updates an existing order
func (r *PostgresRepository) UpdateOrder(ctx context.Context, order *domain.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

// ListOrders retrieves orders with optional filters
func (r *PostgresRepository) ListOrders(ctx context.Context, status domain.OrderStatus, limit int) ([]*domain.Order, error) {
	var orders []*domain.Order
	query := r.db.WithContext(ctx).Order("created_at DESC")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&orders).Error
	return orders, err
}

// CreateOutboxEvent creates a new outbox event
func (r *PostgresRepository) CreateOutboxEvent(ctx context.Context, event *domain.OutboxEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetUnprocessedOutboxEvents retrieves unprocessed events
func (r *PostgresRepository) GetUnprocessedOutboxEvents(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	var events []*domain.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("processed = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// MarkOutboxEventProcessed marks an outbox event as processed
func (r *PostgresRepository) MarkOutboxEventProcessed(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"processed":    true,
			"processed_at": now,
		}).Error
}

// WithTransaction executes operations within a transaction
func (r *PostgresRepository) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

// Close closes the database connection
func (r *PostgresRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// DB returns the underlying GORM database
func (r *PostgresRepository) DB() *gorm.DB {
	return r.db
}
