# Nexus Trading System - Architectural Plan

## Overview
High Performance Order Manager built in Go, designed for paper trading on Binance Testnet with event-driven architecture using the Transactional Outbox pattern.

## Technology Stack
- **HTTP Framework**: Echo (高性能、轻量级)
- **Logging**: Zap (结构化日志)
- **ORM**: Gorm (PostgreSQL)
- **Kafka Client**: Sarama
- **Database**: PostgreSQL (事务性发件箱模式)

## Project Structure
```
NexusTrader/
├── cmd/
│   └── order-manager/
│       └── main.go
├── internal/
│   ├── domain/
│   │   └── order.go
│   ├── application/
│   │   └── orchestrator.go
│   ├── infrastructure/
│   │   ├── persistence/
│   │   │   └── postgres.go
│   │   ├── exchange/
│   │   │   └── binance_testnet.go
│   │   └── messaging/
│   │       └── kafka_pool.go
│   └── config/
│       └── config.go
├── config.yaml
├── go.mod
├── go.sum
└── Dockerfile
```

## Implementation Details

### 1. Configuration Management (config.yaml)
```yaml
app:
  name: "nexus-order-manager"
  environment: "development"

database:
  host: "localhost"
  port: 5432
  username: "nexus"
  password: "secure_password"
  name: "nexus_trading"
  max_open_conns: 25
  max_idle_conns: 5

binance:
  testnet:
    enabled: true
    api_key: "${BINANCE_TESTNET_API_KEY}"
    api_secret: "${BINANCE_TESTNET_API_SECRET}"
    base_url: "https://testnet.binance.vision"

kafka:
  brokers:
    - "localhost:9092"
  consumer_group: "nexus-order-workers"
  topics:
    orders: "nexus.orders"
    events: "nexus.events"

outbox:
  poll_interval_ms: 500
  batch_size: 100

logging:
  level: "info"
  format: "json"
```

### 2. Domain Layer (internal/domain/order.go)

**Entities:**
- `Order` - 主订单实体
- `OrderStatus` - 订单状态枚举
- `OrderEvent` - 订单事件（用于发件箱）

**Order Status Flow:**
```mermaid
stateDiagram-v2
    [*] --> PENDING
    PENDING --> EXECUTING: Order Submitted
    EXECUTING --> COMPLETED: Trade Filled
    EXECUTING --> FAILED: Error/Timeout
    COMPLETED --> [*]
    FAILED --> [*]
```

### 3. Infrastructure Layer

#### 3.1 PostgreSQL Persistence (internal/infrastructure/persistence/postgres.go)
- GORM models for Orders and OutboxEvents
- Transaction support for atomic operations
- Repository pattern for data access

#### 3.2 Binance Testnet Client (internal/infrastructure/exchange/binance_testnet.go)
- HMAC-SHA256 request signing
- HTTP client with configurable timeouts
- Error handling and retry logic
- Mock client for testing

#### 3.3 Kafka Pool (internal/infrastructure/messaging/kafka_pool.go)
- Sarama async producer
- Consumer group management
- Topic subscription for order events

### 4. Application Layer (internal/application/orchestrator.go)

**Responsibilities:**
- Order validation and enrichment
- Order submission to exchange
- Event publishing via Outbox pattern
- Status tracking and updates

### 5. Transactional Outbox Pattern

```mermaid
sequenceDiagram
    participant Client
    participant Orchestrator
    participant Database
    participant OutboxRelay
    participant Kafka
    participant Exchange

    Client->>Orchestrator: Submit Order
    Orchestrator->>Database: Begin Transaction
    Database->>Database: Save Order (PENDING)
    Database->>Database: Save Outbox Event
    Database->>Orchestrator: Commit Transaction
    Orchestrator->>Client: Order Accepted
    
    loop Polling
        OutboxRelay->>Database: Fetch Unprocessed Events
        Database->>OutboxRelay: Events
        OutboxRelay->>Kafka: Publish Event
        Kafka->>OutboxRelay: Ack
        OutboxRelay->>Database: Mark Event Processed
    end
    
    Kafka->>Orchestrator: Order Event
    Orchestrator->>Exchange: Execute Trade
    Exchange->>Orchestrator: Trade Result
    Orchestrator->>Database: Update Order Status
```

## Key Features

1. **Fault Tolerance**: Transactional Outbox ensures no events are lost
2. **High Performance**: Worker pool for parallel order processing
3. **Observability**: Structured logging with Zap
4. **Graceful Shutdown**: Proper cleanup of resources
5. **Configuration Management**: YAML-based with environment variable support

## API Endpoints (Echo Framework)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/v1/orders | Create new order |
| GET | /api/v1/orders/:id | Get order status |
| GET | /api/v1/orders | List orders |
| DELETE | /api/v1/orders/:id | Cancel order |

## Next Steps

1. Initialize Go module and create directory structure
2. Implement configuration loader
3. Implement domain models
4. Implement infrastructure layer
5. Implement application orchestrator
6. Create main entry point
7. Add unit tests
8. Create Docker configuration

## Environment Variables Required

```bash
BINANCE_TESTNET_API_KEY=your_api_key
BINANCE_TESTNET_API_SECRET=your_api_secret
DATABASE_URL=postgres://user:pass@localhost:5432/nexus_trading
KAFKA_BROKERS=localhost:9092
```
