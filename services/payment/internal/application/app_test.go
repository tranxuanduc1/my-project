package application

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"myproject/payment/internal/infrastructure/orderclient"

	"github.com/google/uuid"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCheckOrderRejectsAmountMismatch(t *testing.T) {
	client := orderclient.New("http://order", "internal-key", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"pending_payment","amount_cents":999,"currency":"USD"}`))}, nil
	})})
	err := client.CheckOrder(t.Context(), Payment{OrderID: uuid.New(), AmountCents: 1000, Currency: "USD"})
	if err == nil {
		t.Fatal("expected amount mismatch")
	}
}
