package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LencoCollectionClient initiates and verifies subscription collections using the
// Lenco API secret held only by the backend process.
type LencoCollectionClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

func NewLencoCollectionClient(baseURL, secret string) *LencoCollectionClient {
	return &LencoCollectionClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		secret:  strings.TrimSpace(secret),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// InitiateMobileMoneyCollection requests a collection using provider-safe data
// built from a database-locked checkout. It is never called directly by a browser.
func (c *LencoCollectionClient) InitiateMobileMoneyCollection(ctx context.Context, collection MobileMoneyCollectionRequest) (*ProviderCollection, error) {
	if err := c.validateConfiguration(); err != nil {
		return nil, err
	}
	if collection.Amount <= 0 || strings.TrimSpace(collection.Reference) == "" || strings.TrimSpace(collection.Phone) == "" || strings.TrimSpace(collection.Operator) == "" {
		return nil, fmt.Errorf("amount, reference, phone, and operator are required")
	}
	if !strings.EqualFold(collection.Currency, "ZMW") {
		return nil, fmt.Errorf("mobile-money collection currency must be ZMW")
	}

	payload := struct {
		Amount    float64 `json:"amount"`
		Reference string  `json:"reference"`
		Phone     string  `json:"phone"`
		Operator  string  `json:"operator"`
		Country   string  `json:"country"`
		Bearer    string  `json:"bearer"`
	}{
		Amount:    collection.Amount,
		Reference: collection.Reference,
		Phone:     collection.Phone,
		Operator:  collection.Operator,
		Country:   collection.Country,
		Bearer:    collection.Bearer,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode mobile-money collection request: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/collections/mobile-money", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doCollectionRequest(req, "initiate mobile-money collection")
}

// VerifyCollection retrieves the provider's current collection status by the
// server-created reference. Subscription activation is based on this response.
func (c *LencoCollectionClient) VerifyCollection(ctx context.Context, reference string) (*ProviderCollection, error) {
	if err := c.validateConfiguration(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(reference) == "" {
		return nil, fmt.Errorf("collection reference is required")
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/collections/status/"+url.PathEscape(reference), nil)
	if err != nil {
		return nil, err
	}
	return c.doCollectionRequest(req, "verify collection")
}

func (c *LencoCollectionClient) validateConfiguration() error {
	if c.baseURL == "" || c.secret == "" {
		return fmt.Errorf("subscription payment collection is not configured")
	}
	return nil
}

func (c *LencoCollectionClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build collection provider request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Accept", "application/json")
	// The provider edge rejects generic command-line user agents. A stable service
	// identifier allows secure backend calls without exposing credentials to the browser.
	req.Header.Set("User-Agent", "Printa-Subscription-Service/1.0")
	return req, nil
}

type lencoCollectionResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    *struct {
		ID        string `json:"id"`
		Reference string `json:"reference"`
		Amount    string `json:"amount"`
		Currency  string `json:"currency"`
		Status    string `json:"status"`
		Reason    string `json:"reasonForFailure"`
	} `json:"data"`
}

func (c *LencoCollectionClient) doCollectionRequest(req *http.Request, operation string) (*ProviderCollection, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	defer resp.Body.Close()

	var body lencoCollectionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", operation, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !body.Status || body.Data == nil {
		message := strings.TrimSpace(body.Message)
		if message == "" {
			message = "provider rejected the collection request"
		}
		return nil, fmt.Errorf("%s: %s", operation, message)
	}
	amount, err := strconv.ParseFloat(body.Data.Amount, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return nil, fmt.Errorf("%s returned an invalid amount", operation)
	}
	return &ProviderCollection{
		ID:        strings.TrimSpace(body.Data.ID),
		Reference: strings.TrimSpace(body.Data.Reference),
		Amount:    amount,
		Currency:  strings.TrimSpace(body.Data.Currency),
		Status:    strings.TrimSpace(body.Data.Status),
		Reason:    strings.TrimSpace(body.Data.Reason),
	}, nil
}
