package domain

import (
	"time"
)

// OrderStatus represents the status of an order
type OrderStatus string

const (
	StatusPending   OrderStatus = "PENDING"
	StatusExecuting OrderStatus = "EXECUTING"
	StatusCompleted OrderStatus = "COMPLETED"
	StatusFailed    OrderStatus = "FAILED"
)

// OrderSide represents the side of an order
type OrderSide string

const (
	SideBuy  OrderSide = "BUY"
	SideSell OrderSide = "SELL"
)

// OrderType represents the type of an order
type OrderType string

const (
	TypeMarket OrderType = "MARKET"
	TypeLimit  OrderType = "LIMIT"
)

// Order represents a trading order
type Order struct {
	ID        string      `json:"id" gorm:"primaryKey;size:64"`
	Symbol    string      `json:"symbol" gorm:"size:20;index"`
	Side      OrderSide   `json:"side" gorm:"size:10"`
	Type      OrderType   `json:"type" gorm:"size:10"`
	Quantity  float64     `json:"quantity" gorm:"type:decimal(20,8)"`
	Price     float64     `json:"price" gorm:"type:decimal(20,8);default:0"`
	Status    OrderStatus `json:"status" gorm:"size:20;index"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// OutboxEvent represents an event to be published to Kafka
type OutboxEvent struct {
	ID          uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Aggregate   string     `json:"aggregate" gorm:"size:100;index"`
	AggregateID string     `json:"aggregate_id" gorm:"size:64;index"`
	EventType   string     `json:"event_type" gorm:"size:100"`
	Payload     string     `json:"payload" gorm:"type:text"`
	Processed   bool       `json:"processed" gorm:"default:false;index"`
	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// NewOrder creates a new order with PENDING status
func NewOrder(id, symbol string, side OrderSide, orderType OrderType, quantity, price float64) *Order {
	now := time.Now()
	return &Order{
		ID:        id,
		Symbol:    symbol,
		Side:      side,
		Type:      orderType,
		Quantity:  quantity,
		Price:     price,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsValid checks if the order is valid
func (o *Order) IsValid() bool {
	return o.ID != "" && o.Symbol != "" && o.Quantity > 0
}

// CanTransitionTo checks if the order can transition to the given status
func (o *Order) CanTransitionTo(newStatus OrderStatus) bool {
	transitions := map[OrderStatus][]OrderStatus{
		StatusPending:   {StatusExecuting, StatusFailed},
		StatusExecuting: {StatusCompleted, StatusFailed},
		StatusCompleted: {},
		StatusFailed:    {},
	}

	for _, allowed := range transitions[o.Status] {
		if allowed == newStatus {
			return true
		}
	}
	return false
}
