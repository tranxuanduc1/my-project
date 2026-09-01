package orderclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"myproject/payment/internal/domain"
)

type Client struct {
	baseURL     string
	internalKey string
	http        *http.Client
}

func New(baseURL, internalKey string, client *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), internalKey: internalKey, http: client}
}

func (c *Client) CheckOrder(ctx context.Context, payment domain.Payment) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/orders/"+payment.OrderID.String(), nil)
	req.Header.Set("X-Internal-API-Key", c.internalKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("order returned %d", resp.StatusCode)
	}
	var order struct {
		Status      string `json:"status"`
		AmountCents int64  `json:"amount_cents"`
		Currency    string `json:"currency"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return err
	}
	if order.Status != "pending_payment" || order.AmountCents != payment.AmountCents || order.Currency != payment.Currency {
		return errors.New("order state or amount mismatch")
	}
	return nil
}
