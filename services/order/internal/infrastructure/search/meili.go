package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"myproject/order/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type MeiliSearch struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewMeiliSearch(baseURL, apiKey string, client *http.Client) *MeiliSearch {
	if client == nil {
		client = NewHTTPClient(3 * time.Second)
	}
	return &MeiliSearch{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: client}
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: otelhttp.NewTransport(http.DefaultTransport,
			otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
				return req.Method + " " + meiliRouteTemplate(req.URL.Path)
			}),
		),
	}
}

func (s *MeiliSearch) SearchProducts(ctx context.Context, query string, limit, offset int) ([]domain.Product, error) {
	payload, _ := json.Marshal(gin.H{"q": query, "limit": limit, "offset": offset})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/indexes/products/search", bytes.NewReader(payload))
	s.setHeaders(req)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, errors.New("search unavailable")
	}
	var out struct {
		Hits []domain.Product `json:"hits"`
	}
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out.Hits, err
}

func (s *MeiliSearch) IndexProduct(ctx context.Context, product domain.Product) error {
	body, _ := json.Marshal([]domain.Product{product})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/indexes/products/documents", bytes.NewReader(body))
	s.setHeaders(req)
	resp, err := s.http.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

func (s *MeiliSearch) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, s.baseURL+"/indexes/products/documents/"+id.String(), nil)
	s.setHeaders(req)
	resp, err := s.http.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

func (s *MeiliSearch) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
}

func meiliRouteTemplate(path string) string {
	switch {
	case path == "/indexes/products/search":
		return "/indexes/products/search"
	case path == "/indexes/products/documents":
		return "/indexes/products/documents"
	case strings.HasPrefix(path, "/indexes/products/documents/"):
		return "/indexes/products/documents/:id"
	default:
		return "/meilisearch"
	}
}
