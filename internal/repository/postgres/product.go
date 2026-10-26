package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shock01/product-catalog/internal/domain"
)

type ProductRepo struct{ db *pgxpool.Pool }

func NewProductRepo(db *pgxpool.Pool) *ProductRepo { return &ProductRepo{db: db} }

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (r *ProductRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, snapshot_id, status, version, created_at, updated_at, deleted_at
		FROM products WHERE id = $1`, id)

	p := &domain.Product{}
	err := row.Scan(&p.ID, &p.SnapshotID, &p.Status, &p.Version,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("products scan: %w", err)
	}
	if p.DeletedAt != nil {
		return nil, domain.ErrDeleted
	}

	if err := r.loadTranslations(ctx, p); err != nil {
		return nil, err
	}
	if err := r.loadOptions(ctx, p); err != nil {
		return nil, err
	}
	if err := r.loadVariants(ctx, p); err != nil {
		return nil, err
	}
	if err := r.loadChannelListings(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *ProductRepo) loadTranslations(ctx context.Context, p *domain.Product) error {
	rows, err := r.db.Query(ctx, `
		SELECT locale_code, title, slug,
		       COALESCE(description,''), COALESCE(short_description,''),
		       COALESCE(meta_title,''), COALESCE(meta_description,'')
		FROM product_translations WHERE product_id = $1`, p.ID)
	if err != nil {
		return fmt.Errorf("product_translations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t domain.ProductTranslation
		if err := rows.Scan(&t.Locale, &t.Title, &t.Slug, &t.Description,
			&t.ShortDescription, &t.MetaTitle, &t.MetaDescription); err != nil {
			return err
		}
		p.Translations = append(p.Translations, t)
	}
	return rows.Err()
}

func (r *ProductRepo) loadOptions(ctx context.Context, p *domain.Product) error {
	rows, err := r.db.Query(ctx, `
		SELECT o.id, o.code, o.sort_order,
		       ot.locale_code, ot.name,
		       ov.id, ov.code, ov.sort_order,
		       ovt.locale_code, ovt.value
		FROM options o
		JOIN option_translations ot   ON ot.option_id = o.id
		JOIN option_values ov         ON ov.option_id = o.id
		JOIN option_value_translations ovt ON ovt.option_value_id = ov.id
		WHERE o.product_id = $1
		ORDER BY o.sort_order, ov.sort_order`, p.ID)
	if err != nil {
		return fmt.Errorf("options: %w", err)
	}
	defer rows.Close()

	optMap := map[uuid.UUID]*domain.Option{}
	var optOrder []uuid.UUID

	for rows.Next() {
		var (
			oID, ovID                          uuid.UUID
			oCode, oTLocale, oTName            string
			oSort                              int
			ovCode, ovTLocale, ovTValue        string
			ovSort                             int
		)
		if err := rows.Scan(&oID, &oCode, &oSort, &oTLocale, &oTName,
			&ovID, &ovCode, &ovSort, &ovTLocale, &ovTValue); err != nil {
			return err
		}
		if _, ok := optMap[oID]; !ok {
			optMap[oID] = &domain.Option{ID: oID, Code: oCode, SortOrder: oSort}
			optOrder = append(optOrder, oID)
		}
		opt := optMap[oID]

		// add translation if not already present
		hasTrans := false
		for _, t := range opt.Translations {
			if t.Locale == oTLocale {
				hasTrans = true
				break
			}
		}
		if !hasTrans {
			opt.Translations = append(opt.Translations, domain.OptionTranslation{Locale: oTLocale, Name: oTName})
		}

		// add option value if not already present
		ovIdx := -1
		for i, ov := range opt.Values {
			if ov.ID == ovID {
				ovIdx = i
				break
			}
		}
		if ovIdx == -1 {
			opt.Values = append(opt.Values, domain.OptionValue{ID: ovID, Code: ovCode, SortOrder: ovSort})
			ovIdx = len(opt.Values) - 1
		}
		opt.Values[ovIdx].Translations = append(opt.Values[ovIdx].Translations,
			domain.OptionValueTranslation{Locale: ovTLocale, Value: ovTValue})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range optOrder {
		p.Options = append(p.Options, *optMap[id])
	}
	return nil
}

func (r *ProductRepo) loadVariants(ctx context.Context, p *domain.Product) error {
	rows, err := r.db.Query(ctx, `
		SELECT v.id, v.sku, v.price, v.currency, v.version,
		       sl.warehouse_code, sl.level, sl.version,
		       vov.option_value_id
		FROM variants v
		LEFT JOIN stock_levels sl             ON sl.variant_id = v.id
		LEFT JOIN variant_option_values vov   ON vov.variant_id = v.id
		WHERE v.product_id = $1`, p.ID)
	if err != nil {
		return fmt.Errorf("variants: %w", err)
	}
	defer rows.Close()

	varMap := map[uuid.UUID]*domain.Variant{}
	var varOrder []uuid.UUID

	for rows.Next() {
		var (
			vID              uuid.UUID
			sku, currency    string
			price            float64
			version          int
			whCode           *string
			slLevel          *string
			slVersion        *int
			ovID             *uuid.UUID
		)
		if err := rows.Scan(&vID, &sku, &price, &currency, &version,
			&whCode, &slLevel, &slVersion, &ovID); err != nil {
			return err
		}
		if _, ok := varMap[vID]; !ok {
			varMap[vID] = &domain.Variant{ID: vID, SKU: sku, Price: price, Currency: currency, Version: version}
			varOrder = append(varOrder, vID)
		}
		v := varMap[vID]
		if whCode != nil {
			hasWH := false
			for _, sl := range v.StockLevels {
				if sl.WarehouseCode == *whCode {
					hasWH = true
					break
				}
			}
			if !hasWH {
				v.StockLevels = append(v.StockLevels, domain.StockLevel{
					WarehouseCode: *whCode,
					Level:         domain.StockStatus(*slLevel),
					Version:       *slVersion,
				})
			}
		}
		if ovID != nil {
			hasOV := false
			for _, id := range v.OptionValueIDs {
				if id == *ovID {
					hasOV = true
					break
				}
			}
			if !hasOV {
				v.OptionValueIDs = append(v.OptionValueIDs, *ovID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range varOrder {
		p.Variants = append(p.Variants, *varMap[id])
	}
	return nil
}

func (r *ProductRepo) loadChannelListings(ctx context.Context, p *domain.Product) error {
	rows, err := r.db.Query(ctx, `
		SELECT channel_id, enabled, price_override, COALESCE(currency,''), version
		FROM channel_products WHERE product_id = $1`, p.ID)
	if err != nil {
		return fmt.Errorf("channel_products: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cl domain.ChannelListing
		if err := rows.Scan(&cl.ChannelID, &cl.Enabled, &cl.PriceOverride, &cl.Currency, &cl.Version); err != nil {
			return err
		}
		p.ChannelListings = append(p.ChannelListings, cl)
	}
	return rows.Err()
}

// ---------------------------------------------------------------------------
// List — cursor-based pagination
// ---------------------------------------------------------------------------

func (r *ProductRepo) List(ctx context.Context, f domain.ListFilter) ([]*domain.Product, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	args := []any{}
	where := "deleted_at IS NULL"
	if f.Status != nil {
		args = append(args, *f.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if f.Cursor != "" {
		args = append(args, f.Cursor)
		where += fmt.Sprintf(" AND id > $%d", len(args))
	}
	args = append(args, limit+1)
	q := fmt.Sprintf("SELECT id FROM products WHERE %s ORDER BY id LIMIT $%d", where, len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, "", err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(ids) > limit {
		nextCursor = ids[limit].String()
		ids = ids[:limit]
	}

	products := make([]*domain.Product, 0, len(ids))
	for _, id := range ids {
		p, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}
		products = append(products, p)
	}
	return products, nextCursor, nil
}

// ---------------------------------------------------------------------------
// Save — full upsert
// ---------------------------------------------------------------------------

func (r *ProductRepo) Save(ctx context.Context, p *domain.Product) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	tag, err := tx.Exec(ctx, `
		INSERT INTO products (id, snapshot_id, status, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE
		  SET snapshot_id=$2, status=$3, version=$4, updated_at=$6
		  WHERE products.version = $4-1`,
		p.ID, p.SnapshotID, p.Status, p.Version, now, now)
	if err != nil {
		return fmt.Errorf("upsert product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}

	if _, err := tx.Exec(ctx, `DELETE FROM product_translations WHERE product_id=$1`, p.ID); err != nil {
		return err
	}
	for _, t := range p.Translations {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_translations
			  (product_id,locale_code,title,slug,description,short_description,meta_title,meta_description)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			p.ID, t.Locale, t.Title, t.Slug, t.Description,
			t.ShortDescription, t.MetaTitle, t.MetaDescription); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM channel_products WHERE product_id=$1`, p.ID); err != nil {
		return err
	}
	for _, cl := range p.ChannelListings {
		if _, err := tx.Exec(ctx, `
			INSERT INTO channel_products (channel_id,product_id,enabled,price_override,currency,version)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			cl.ChannelID, p.ID, cl.Enabled, cl.PriceOverride, cl.Currency, cl.Version); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Patch — partial update (status + channel listings)
// ---------------------------------------------------------------------------

func (r *ProductRepo) Patch(ctx context.Context, p *domain.Product) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE products SET status=$1, version=version+1, updated_at=now()
		WHERE id=$2 AND version=$3 AND deleted_at IS NULL`,
		p.Status, p.ID, p.Version)
	if err != nil {
		return fmt.Errorf("patch product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}

	if p.ChannelListings != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM channel_products WHERE product_id=$1`, p.ID); err != nil {
			return err
		}
		for _, cl := range p.ChannelListings {
			if _, err := tx.Exec(ctx, `
				INSERT INTO channel_products (channel_id,product_id,enabled,price_override,currency,version)
				VALUES ($1,$2,$3,$4,$5,1)`,
				cl.ChannelID, p.ID, cl.Enabled, cl.PriceOverride, cl.Currency); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Delete — soft delete
// ---------------------------------------------------------------------------

func (r *ProductRepo) Delete(ctx context.Context, id uuid.UUID, version int) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE products SET deleted_at=now(), updated_at=now(), version=version+1
		WHERE id=$1 AND version=$2 AND deleted_at IS NULL`, id, version)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return nil
}

// ---------------------------------------------------------------------------
// OutboxRepo
// ---------------------------------------------------------------------------

type OutboxRepo struct{ db *pgxpool.Pool }

func NewOutboxRepo(db *pgxpool.Pool) *OutboxRepo { return &OutboxRepo{db: db} }

func (r *OutboxRepo) Append(ctx context.Context, events []domain.Event) error {
	for _, e := range events {
		payload, err := json.Marshal(e.Payload)
		if err != nil {
			return err
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO outbox (id,aggregate,aggregate_id,event_type,payload,trace_id,span_id)
			VALUES ($1,'product',$2,$3,$4,$5,$6)`,
			e.ID, e.AggregateID, string(e.Type), payload, e.TraceID, e.SpanID); err != nil {
			return fmt.Errorf("outbox append: %w", err)
		}
	}
	return nil
}
