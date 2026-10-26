package domain

import (
	"context"

	"github.com/google/uuid"
)

// ProductRepository is the persistence port for the Product aggregate.
// Implemented in internal/repository/postgres.
type ProductRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Product, error)
	List(ctx context.Context, filter ListFilter) ([]*Product, string, error)
	Save(ctx context.Context, p *Product) error   // insert or full replace
	Patch(ctx context.Context, p *Product) error  // partial update (status + listings)
	Delete(ctx context.Context, id uuid.UUID, version int) error
}

// OutboxRepository writes domain events to the transactional outbox.
type OutboxRepository interface {
	Append(ctx context.Context, events []Event) error
}

// ListFilter carries search/filter parameters for the List query.
type ListFilter struct {
	Locale   string
	Channel  string
	MinPrice *float64
	MaxPrice *float64
	Status   *ProductStatus
	Cursor   string
	Limit    int
}
