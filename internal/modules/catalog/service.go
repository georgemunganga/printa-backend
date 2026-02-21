package catalog

import (
"context"

"github.com/google/uuid"
)

// Service defines catalog business logic.
type Service interface {
CreateProduct(ctx context.Context, req CreateProductRequest) (*PlatformProduct, error)
GetProduct(ctx context.Context, id string) (*PlatformProduct, error)
ListProducts(ctx context.Context, category string, activeOnly bool) ([]*PlatformProduct, error)
UpdateProduct(ctx context.Context, id string, req CreateProductRequest) (*PlatformProduct, error)
}

// CreateProductRequest holds the data for creating a platform product.
type CreateProductRequest struct {
Name        string  `json:"name"`
Description string  `json:"description"`
Category    string  `json:"category"`
BasePrice   float64 `json:"base_price"`
Currency    string  `json:"currency"`
SKU         string  `json:"sku"`
ImageURL    string  `json:"image_url"`
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) CreateProduct(ctx context.Context, req CreateProductRequest) (*PlatformProduct, error) {
currency := req.Currency
if currency == "" {
currency = "ZMW"
}
p := &PlatformProduct{
ID:          uuid.New(),
Name:        req.Name,
Description: req.Description,
Category:    req.Category,
BasePrice:   req.BasePrice,
Currency:    currency,
SKU:         req.SKU,
ImageURL:    req.ImageURL,
IsActive:    true,
}
if err := s.repo.Create(ctx, p); err != nil {
return nil, err
}
return p, nil
}

func (s *service) GetProduct(ctx context.Context, id string) (*PlatformProduct, error) {
return s.repo.GetByID(ctx, id)
}

func (s *service) ListProducts(ctx context.Context, category string, activeOnly bool) ([]*PlatformProduct, error) {
return s.repo.List(ctx, category, activeOnly)
}

func (s *service) UpdateProduct(ctx context.Context, id string, req CreateProductRequest) (*PlatformProduct, error) {
p, err := s.repo.GetByID(ctx, id)
if err != nil {
return nil, err
}
p.Name = req.Name
p.Description = req.Description
p.Category = req.Category
p.BasePrice = req.BasePrice
if req.Currency != "" {
p.Currency = req.Currency
}
p.SKU = req.SKU
p.ImageURL = req.ImageURL
if err := s.repo.Update(ctx, p); err != nil {
return nil, err
}
return p, nil
}
