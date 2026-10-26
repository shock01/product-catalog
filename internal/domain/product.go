package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

type ProductStatus string

const (
	StatusDraft     ProductStatus = "draft"
	StatusActive    ProductStatus = "active"
	StatusPublished ProductStatus = "published"
	StatusArchived  ProductStatus = "archived"
)

type StockStatus string

const (
	StockInStock StockStatus = "IN_STOCK"
	StockLow     StockStatus = "LOW"
	StockOOS     StockStatus = "OOS"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrNotFound          = errors.New("product not found")
	ErrDeleted           = errors.New("product deleted")
	ErrVersionConflict   = errors.New("version conflict")
	ErrSKUConflict       = errors.New("SKU already exists")
	ErrInvariantViolation = errors.New("invariant violation")
)

// ---------------------------------------------------------------------------
// Entities
// ---------------------------------------------------------------------------

type Product struct {
	ID         uuid.UUID
	SnapshotID *uuid.UUID
	Status     ProductStatus
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time

	Translations      []ProductTranslation
	Options           []Option
	Variants          []Variant
	Assets            []Asset
	ChannelListings   []ChannelListing
	MarketplaceListings []MarketplaceListing
}

type ProductTranslation struct {
	Locale           string
	Title            string
	Slug             string
	Description      string
	ShortDescription string
	MetaTitle        string
	MetaDescription  string
}

type Option struct {
	ID           uuid.UUID
	Code         string
	SortOrder    int
	Translations []OptionTranslation
	Values       []OptionValue
}

type OptionTranslation struct {
	Locale string
	Name   string
}

type OptionValue struct {
	ID           uuid.UUID
	Code         string
	SortOrder    int
	Translations []OptionValueTranslation
}

type OptionValueTranslation struct {
	Locale string
	Value  string
}

type Variant struct {
	ID             uuid.UUID
	SnapshotID     *uuid.UUID
	SKU            string
	Price          float64
	Currency       string
	Version        int
	OptionValueIDs []uuid.UUID
	Assets         []Asset
	StockLevels    []StockLevel
	Translations   []VariantTranslation
}

type VariantTranslation struct {
	Locale      string
	Title       string
	Description string
}

type Asset struct {
	ID        uuid.UUID
	URL       string
	Type      string // "image" | "video"
	Scope     string // "product" | "variant"
	VariantID *uuid.UUID
	SortOrder int
}

type StockLevel struct {
	WarehouseCode string
	Level         StockStatus
	Version       int
}

type ChannelListing struct {
	ChannelID     uuid.UUID
	Enabled       bool
	PriceOverride *float64
	Currency      string
	Version       int
}

type MarketplaceListing struct {
	MarketplaceID  uuid.UUID
	ExternalID     string
	FeedAttributes map[string]any
	Version        int
}

// ---------------------------------------------------------------------------
// Invariants
// ---------------------------------------------------------------------------

// Validate checks all aggregate invariants. Called before any persistence.
func (p *Product) Validate() error {
	if len(p.Variants) == 0 {
		return fmt.Errorf("%w: product must have at least one variant", ErrInvariantViolation)
	}
	if len(p.Translations) == 0 {
		return fmt.Errorf("%w: product must have at least one translation", ErrInvariantViolation)
	}
	if p.Status == StatusPublished {
		hasEnabled := false
		for _, cl := range p.ChannelListings {
			if cl.Enabled {
				hasEnabled = true
				break
			}
		}
		if !hasEnabled {
			return fmt.Errorf("%w: published product must have at least one enabled channel listing", ErrInvariantViolation)
		}
	}

	// Build option value code set for cross-reference check
	validCodes := map[uuid.UUID]bool{}
	for _, opt := range p.Options {
		for _, ov := range opt.Values {
			validCodes[ov.ID] = true
		}
	}
	for _, v := range p.Variants {
		for _, ovID := range v.OptionValueIDs {
			if !validCodes[ovID] {
				return fmt.Errorf("%w: variant %s references unknown option value %s", ErrInvariantViolation, v.SKU, ovID)
			}
		}
	}
	return nil
}

// IndexEligible returns true when the product should appear in OpenSearch.
func (p *Product) IndexEligible() bool {
	if p.Status != StatusPublished || p.DeletedAt != nil {
		return false
	}
	for _, v := range p.Variants {
		for _, sl := range v.StockLevels {
			if sl.Level != StockOOS {
				return true
			}
		}
	}
	return false
}
