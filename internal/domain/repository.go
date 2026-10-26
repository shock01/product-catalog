package domain

import (
	"context"

	"github.com/google/uuid"
)

// Tx is a handle to an in-progress database transaction.
// Passed into repository methods that must execute within the same transaction.
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// ProductRepository is the persistence port for the Product aggregate.
// Implemented in internal/repository/postgres.
type ProductRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Product, error)
	List(ctx context.Context, filter ListFilter) ([]*Product, string, error)
	BeginTx(ctx context.Context) (Tx, error)
	SaveTx(ctx context.Context, tx Tx, p *Product) error   // insert or full replace, within tx
	PatchTx(ctx context.Context, tx Tx, p *Product) error  // partial update, within tx
	DeleteTx(ctx context.Context, tx Tx, id uuid.UUID, version int) error
	Delete(ctx context.Context, id uuid.UUID, version int) error
}

// OutboxRepository writes domain events to the transactional outbox.
type OutboxRepository interface {
	AppendTx(ctx context.Context, tx Tx, events []Event) error
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
