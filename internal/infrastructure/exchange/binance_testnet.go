package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nexustrader/nexus-order-manager/internal/domain"
)

// BinanceClient defines the interface for Binance API interactions
type BinanceClient interface {
	ExecuteTrade(ctx context.Context, order *domain.Order) error
}

// BinanceTestnetClient is a client for Binance Testnet
type BinanceTestnetClient struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client
}

// NewBinanceTestnetClient creates a new Binance Testnet client
func NewBinanceTestnetClient(key, secret string) *BinanceTestnetClient {
	return &BinanceTestnetClient{
		apiKey:     key,
		apiSecret:  secret,
		baseURL:    "https://testnet.binance.vision",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// signRequest generates the HMAC-SHA256 signature required by Binance
func (b *BinanceTestnetClient) signRequest(params url.Values) string {
	mac := hmac.New(sha256.New, []byte(b.apiSecret))
	mac.Write([]byte(params.Encode()))
	return hex.EncodeToString(mac.Sum(nil))
}

// ExecuteTrade executes a trade on Binance Testnet
func (b *BinanceTestnetClient) ExecuteTrade(ctx context.Context, order *domain.Order) error {
	endpoint := "/api/v3/order"
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	params := url.Values{}
	params.Add("symbol", order.Symbol)
	params.Add("side", string(order.Side))
	params.Add("type", string(order.Type))
	params.Add("quantity", fmt.Sprintf("%.8f", order.Quantity))
	if order.Type == domain.TypeLimit {
		params.Add("price", fmt.Sprintf("%.2f", order.Price))
		params.Add("timeInForce", "GTC")
	}
	params.Add("timestamp", timestamp)
	params.Add("signature", b.signRequest(params))

	fullURL := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("X-MBX-APIKEY", b.apiKey)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binance API error (Status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetAccountBalance retrieves the account balance from Testnet
func (b *BinanceTestnetClient) GetAccountBalance(ctx context.Context) error {
	endpoint := "/api/v3/account"
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	params := url.Values{}
	params.Add("timestamp", timestamp)
	params.Add("signature", b.signRequest(params))

	fullURL := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return err
	}

	req.Header.Add("X-MBX-APIKEY", b.apiKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error getting account (Status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
