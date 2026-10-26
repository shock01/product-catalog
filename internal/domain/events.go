package domain

import (
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventProductCreated       EventType = "product.created"
	EventProductUpdated       EventType = "product.updated"
	EventProductDeleted       EventType = "product.deleted"
	EventStockUpdated         EventType = "product.stock_updated"
	EventSnapshotImported     EventType = "snapshot.imported"
)

type Event struct {
	ID          uuid.UUID
	Type        EventType
	AggregateID uuid.UUID
	Payload     map[string]any
	TraceID     string
	SpanID      string
	OccurredAt  time.Time
}
