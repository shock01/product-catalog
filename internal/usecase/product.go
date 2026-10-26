package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shock01/product-catalog/internal/domain"
)

// ProductQueries is the read side consumed by the HTTP handler.
type ProductQueries interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, string, error)
}

// ProductCommands is the write side consumed by the HTTP handler.
type ProductCommands interface {
	Create(ctx context.Context, p *domain.Product) error
	Update(ctx context.Context, p *domain.Product) error
	Patch(ctx context.Context, p *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID, version int) error
}

// Cache is the minimal interface the use-case layer needs from Valkey.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

// ProductService implements both ProductQueries and ProductCommands.
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
			return &p, contentETag(b), nil
		}
	}

	p, err := s.products.GetByID(ctx, id)
	if err != nil {
		return nil, "", fmt.Errorf("get product: %w", err)
	}

	b, _ := json.Marshal(p)
	_ = s.cache.Set(ctx, key, b, s.cacheTTL)
	return p, contentETag(b), nil
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
		return fmt.Errorf("validate: %w", err)
	}
	tx, err := s.products.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := s.products.SaveTx(ctx, tx, p); err != nil {
		return fmt.Errorf("save product: %w", err)
	}
	if err := s.outbox.AppendTx(ctx, tx, []domain.Event{newEvent(domain.EventProductCreated, p.ID, nil)}); err != nil {
		return fmt.Errorf("append outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *ProductService) Update(ctx context.Context, p *domain.Product) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	tx, err := s.products.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := s.products.SaveTx(ctx, tx, p); err != nil {
		return fmt.Errorf("save product: %w", err)
	}
	if err := s.outbox.AppendTx(ctx, tx, []domain.Event{newEvent(domain.EventProductUpdated, p.ID, nil)}); err != nil {
		return fmt.Errorf("append outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	s.invalidate(ctx, p.ID)
	return nil
}

func (s *ProductService) Patch(ctx context.Context, p *domain.Product) error {
	tx, err := s.products.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := s.products.PatchTx(ctx, tx, p); err != nil {
		return fmt.Errorf("patch product: %w", err)
	}
	if err := s.outbox.AppendTx(ctx, tx, []domain.Event{newEvent(domain.EventProductUpdated, p.ID, nil)}); err != nil {
		return fmt.Errorf("append outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	s.invalidate(ctx, p.ID)
	return nil
}

func (s *ProductService) Delete(ctx context.Context, id uuid.UUID, version int) error {
	tx, err := s.products.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := s.products.DeleteTx(ctx, tx, id, version); err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if err := s.outbox.AppendTx(ctx, tx, []domain.Event{newEvent(domain.EventProductDeleted, id, nil)}); err != nil {
		return fmt.Errorf("append outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	s.invalidate(ctx, id)
	return nil
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
