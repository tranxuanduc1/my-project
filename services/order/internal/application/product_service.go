package application

import (
	"context"
	"strings"
	"time"

	"myproject/order/internal/application/ports"
	"myproject/order/internal/domain"

	"github.com/google/uuid"
)

type Product = domain.Product
type Order = domain.Order
type OrderItem = domain.OrderItem
type Outbox = domain.Outbox
type Inbox = domain.Inbox

type ProductService struct {
	products ports.ProductRepository
	cache    ports.ProductCache
	search   ports.ProductSearch
	storage  ports.ProductStorage
}

func NewProductService(products ports.ProductRepository, cache ports.ProductCache, search ports.ProductSearch, storage ports.ProductStorage) *ProductService {
	return &ProductService{products: products, cache: cache, search: search, storage: storage}
}

func (s *ProductService) List(ctx context.Context, query string, limit, offset int) ([]domain.Product, error) {
	query = strings.TrimSpace(query)
	if query != "" && s.search != nil {
		if products, err := s.search.SearchProducts(ctx, query, limit, offset); err == nil {
			return products, nil
		}
	}
	return s.products.ListProducts(ctx, query, limit, offset)
}

func (s *ProductService) Get(ctx context.Context, id string) (domain.Product, error) {
	if s.cache != nil {
		if product, ok := s.cache.GetProduct(ctx, id); ok {
			return product, nil
		}
	}
	product, err := s.products.GetProduct(ctx, id)
	if err == nil && s.cache != nil {
		s.cache.SetProduct(ctx, product)
	}
	return product, err
}

func (s *ProductService) Create(ctx context.Context, product domain.Product) (domain.Product, error) {
	if product.SKU == "" || product.Name == "" || product.PriceCents < 0 || product.Stock < 0 {
		return domain.Product{}, ErrInvalidInput
	}
	product.ID = uuid.New()
	if product.Currency == "" {
		product.Currency = "USD"
	}
	product.Active = true
	if err := s.products.CreateProduct(ctx, product); err != nil {
		return domain.Product{}, err
	}
	go s.index(context.Background(), product, false)
	return product, nil
}

func (s *ProductService) Update(ctx context.Context, id string, product domain.Product) (domain.Product, error) {
	if product.Name == "" || product.PriceCents < 0 || product.Stock < 0 {
		return domain.Product{}, ErrInvalidInput
	}
	updated, err := s.products.UpdateProduct(ctx, id, product)
	if err != nil {
		return domain.Product{}, err
	}
	if s.cache != nil {
		s.cache.DeleteProduct(ctx, id)
	}
	go s.index(context.Background(), updated, false)
	return updated, nil
}

func (s *ProductService) Delete(ctx context.Context, id string) error {
	product, err := s.products.GetProduct(ctx, id)
	if err != nil {
		return err
	}
	if err := s.products.DeleteProduct(ctx, id); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.DeleteProduct(ctx, id)
	}
	go s.index(context.Background(), product, true)
	return nil
}

func (s *ProductService) PresignImage(ctx context.Context, id, contentType string) (string, string, error) {
	if !strings.HasPrefix(contentType, "image/") {
		return "", "", ErrInvalidInput
	}
	product, err := s.products.GetProduct(ctx, id)
	if err != nil {
		return "", "", err
	}
	url, key, err := s.storage.PresignProductImage(ctx, product.ID, 15*time.Minute)
	if err != nil {
		return "", "", err
	}
	return url, key, s.products.SetProductImage(ctx, id, key)
}

func (s *ProductService) index(ctx context.Context, product domain.Product, deleted bool) {
	if s.search == nil {
		return
	}
	if deleted {
		_ = s.search.DeleteProduct(ctx, product.ID)
		return
	}
	_ = s.search.IndexProduct(ctx, product)
}
