# Nexus Trading System

A high-performance order management system built in Go, designed for paper trading on Binance Testnet with event-driven architecture using the Transactional Outbox pattern.

![Go Version](https://img.shields.io/badge/Go-1.23-blue)
![License](https://img.shields.io/badge/License-MIT-green)

## 🚀 Features

- **High Performance**: Worker pool for parallel order processing
- **Fault Tolerance**: Transactional Outbox pattern ensures no events are lost
- **Event-Driven**: Kafka integration for order event streaming
- **Structured Logging**: Zap-based structured logging for observability
- **Graceful Shutdown**: Proper cleanup of resources on termination
- **Docker Ready**: Containerized deployment with Docker Compose

## 📁 Project Structure

```
NexusTrader/
├── cmd/
│   └── order-manager/
│       └── main.go                    # Application entry point
├── internal/
│   ├── application/
│   │   └── orchestrator.go           # Order processing orchestration
│   ├── config/
│   │   └── config.go                 # Configuration management
│   ├── domain/
│   │   ├── order.go                  # Domain entities (Order, OutboxEvent)
│   │   └── order_test.go             # Unit tests
│   └── infrastructure/
│       ├── exchange/
│       │   └── binance_testnet.go    # Binance API client
│       ├── messaging/
│       │   └── kafka_pool.go         # Kafka producer/consumer
│       └── persistence/
│           └── postgres.go           # PostgreSQL repository
├── config.yaml                        # Application configuration
├── docker-compose.yml                 # Development infrastructure
├── Dockerfile                         # Production container
├── go.mod                            # Go module definition
├── README.md                          # This file
└── plans/
    └── NEXUS_TRADING_SYSTEM_PLAN.md  # Architectural plan
```

## 🛠️ Technology Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| HTTP Framework | Echo v4 | High-performance HTTP routing |
| Logging | Zap | Structured, performant logging |
| ORM | Gorm | Database object-relational mapping |
| Kafka Client | Sarama | Apache Kafka client library |
| Database | PostgreSQL | Primary data store |
| Runtime | Go 1.23 | Application runtime |

## 📋 Prerequisites

- Go 1.23 or later
- Docker and Docker Compose
- Binance Testnet API credentials

## 🔧 Installation

### 1. Clone the Repository

```bash
git clone https://github.com/your-org/NexusTrader.git
cd NexusTrader
```

### 2. Set Environment Variables

Create a `.env` file or export the following variables:

```bash
export BINANCE_TESTNET_API_KEY="your_testnet_api_key"
export BINANCE_TESTNET_API_SECRET="your_testnet_api_secret"
```

### 3. Start Infrastructure

```bash
docker-compose up -d
```

This starts:
- PostgreSQL (port 5432)
- Kafka (port 9092)
- Zookeeper (port 2181)

### 4. Install Dependencies

```bash
go mod download
go mod tidy
```

### 5. Build and Run

```bash
# Build
go build -o order-manager ./cmd/order-manager/

# Run
./order-manager
```

## 🔐 Binance Testnet Setup

1. Visit [Binance Testnet](https://testnet.binance.vision)
2. Create an account and generate API keys
3. Copy API Key and Secret to your environment
4. Use the keys in your `.env` file

**Note**: Testnet uses fake funds - no real money is at risk.

## 📡 API Reference

### Health Check

```http
GET /health
```

**Response** (200 OK):
```json
{
  "status": "healthy",
  "time": "2024-01-15T10:30:00Z"
}
```

### Create Order

```http
POST /api/v1/orders
Content-Type: application/json

{
  "id": "ORDER-001",
  "symbol": "BTCUSDT",
  "side": "BUY",
  "type": "MARKET",
  "quantity": 0.001,
  "price": 0
}
```

**Response** (202 Accepted):
```json
{
  "id": "ORDER-001",
  "symbol": "BTCUSDT",
  "side": "BUY",
  "type": "MARKET",
  "quantity": 0.001,
  "price": 0,
  "status": "PENDING"
}
```

### Get Order

```http
GET /api/v1/orders/{order_id}
```

**Response** (200 OK):
```json
{
  "id": "ORDER-001",
  "symbol": "BTCUSDT",
  "side": "BUY",
  "type": "MARKET",
  "quantity": 0.001,
  "price": 0,
  "status": "COMPLETED",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:01Z"
}
```

### List Orders

```http
GET /api/v1/orders?status=PENDING&limit=50
```

## ⚙️ Configuration

### config.yaml

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
  ssl_mode: "disable"

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

## 🏗️ Architecture

### Order Status Flow

```
┌──────────┐     ┌──────────────┐     ┌─────────────┐
│ PENDING  │────▶│ EXECUTING    │────▶│ COMPLETED   │
└──────────┘     └──────────────┘     └─────────────┘
      │                │
      │                ▼
      │           ┌─────────┐
      └──────────▶│  FAILED │
                  └─────────┘
```

### Transactional Outbox Pattern

```
┌──────────┐     ┌──────────────────┐     ┌──────────────┐
│  Client  │────▶│  Order Service   │────▶│   Database   │
└──────────┘     └──────────────────┘     └──────────────┘
                                               │
                                               ▼
                                      ┌──────────────────┐
                                      │   Outbox Table   │
                                      └──────────────────┘
                                               │
                              ┌────────────────┼────────────────┐
                              ▼                                 ▼
                       ┌───────────┐                   ┌───────────┐
                       │   Kafka   │◀──────────────────│   Relay   │
                       └───────────┘                   └───────────┘
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## 🚀 Deployment

### Docker

```bash
# Build image
docker build -t nexus-order-manager .

# Run container
docker run -d \
  -p 8080:8080 \
  -e BINANCE_TESTNET_API_KEY="${BINANCE_TESTNET_API_KEY}" \
  -e BINANCE_TESTNET_API_SECRET="${BINANCE_TESTNET_API_SECRET}" \
  --name nexus-order-manager \
  nexus-order-manager
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nexus-order-manager
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nexus-order-manager
  template:
    metadata:
      labels:
        app: nexus-order-manager
    spec:
      containers:
      - name: order-manager
        image: nexus-order-manager:latest
        ports:
        - containerPort: 8080
        env:
        - name: BINANCE_TESTNET_API_KEY
          valueFrom:
            secretKeyRef:
              name: binance-credentials
              key: api_key
        - name: BINANCE_TESTNET_API_SECRET
          valueFrom:
            secretKeyRef:
              name: binance-credentials
              key: api_secret
```

## 📊 Monitoring

### Prometheus Metrics

The application exposes metrics at `/metrics` endpoint:

- `orders_total`: Total orders processed
- `orders_processing`: Currently processing orders
- `orders_completed`: Successfully completed orders
- `orders_failed`: Failed orders

### Health Endpoints

- `/health`: Application health
- `/ready`: Readiness probe

## 🔒 Security

- API keys stored in environment variables
- HMAC-SHA256 request signing for Binance API
- TLS recommended for production deployment
- Principle of least privilege for database access

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Binance](https://binance.com) for providing testnet access
- [Go](https://golang.org) for the excellent programming language
- All open-source contributors

## 📧 Support

For support, please open an issue on GitHub or contact the maintainers.

---

**Built with ❤️ by Nexus Trading Team**
