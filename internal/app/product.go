package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shock01/product-catalog/internal/domain"
)

// Cache is the minimal interface the app layer needs from Valkey.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

// ProductService orchestrates domain + repository + cache.
type ProductService struct {
	products domain.ProductRepository
	outbox   domain.OutboxRepository
	cache    Cache
	cacheTTL time.Duration
}

func NewProductService(
	products domain.ProductRepository,
	outbox domain.OutboxRepository,
	cache Cache,
	cacheTTL time.Duration,
) *ProductService {
	return &ProductService{products: products, outbox: outbox, cache: cache, cacheTTL: cacheTTL}
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

func (s *ProductService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, string, error) {
	key := "product:" + id.String()

	if b, err := s.cache.Get(ctx, key); err == nil && b != nil {
		var p domain.Product
		if err := json.Unmarshal(b, &p); err == nil {
			etag := contentETag(b)
			return &p, etag, nil
		}
	}

	p, err := s.products.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}

	b, _ := json.Marshal(p)
	_ = s.cache.Set(ctx, key, b, s.cacheTTL)
	return p, contentETag(b), nil
}

func (s *ProductService) List(ctx context.Context, f domain.ListFilter) ([]*domain.Product, string, error) {
	return s.products.List(ctx, f)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (s *ProductService) Create(ctx context.Context, p *domain.Product) error {
	p.ID = uuid.New()
	p.Version = 1
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt

	if err := p.Validate(); err != nil {
		return err
	}
	if err := s.products.Save(ctx, p); err != nil {
		return err
	}
	return s.outbox.Append(ctx, []domain.Event{newEvent(domain.EventProductCreated, p.ID, nil)})
}

func (s *ProductService) Update(ctx context.Context, p *domain.Product) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := s.products.Save(ctx, p); err != nil {
		return err
	}
	s.invalidate(ctx, p.ID)
	return s.outbox.Append(ctx, []domain.Event{newEvent(domain.EventProductUpdated, p.ID, nil)})
}

func (s *ProductService) Patch(ctx context.Context, p *domain.Product) error {
	if err := s.products.Patch(ctx, p); err != nil {
		return err
	}
	s.invalidate(ctx, p.ID)
	return s.outbox.Append(ctx, []domain.Event{newEvent(domain.EventProductUpdated, p.ID, nil)})
}

func (s *ProductService) Delete(ctx context.Context, id uuid.UUID, version int) error {
	if err := s.products.Delete(ctx, id, version); err != nil {
		return err
	}
	s.invalidate(ctx, id)
	return s.outbox.Append(ctx, []domain.Event{newEvent(domain.EventProductDeleted, id, nil)})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *ProductService) invalidate(ctx context.Context, id uuid.UUID) {
	_ = s.cache.Del(ctx, "product:"+id.String())
}

func contentETag(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf(`"%x"`, h[:8])
}

func newEvent(t domain.EventType, aggID uuid.UUID, payload map[string]any) domain.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return domain.Event{
		ID:          uuid.New(),
		Type:        t,
		AggregateID: aggID,
		Payload:     payload,
		OccurredAt:  time.Now(),
	}
}
