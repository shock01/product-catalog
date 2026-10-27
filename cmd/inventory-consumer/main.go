package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// inventory-consumer is a long-running ECS Service that consumes the external
// inventory.stock-changed Kafka topic and writes directly to Postgres.
//
// For each record it opens a transaction and:
//  1. UPSERTs stock_levels (last-write-wins, IS DISTINCT FROM guard)
//  2. INSERTs an outbox row (event_type='stock.updated') only when the level changed
//  3. COMMITs
//
// The outbox worker publishes stock.updated to EventBridge, which triggers the
// indexer to perform a partial OpenSearch _update on the affected product
// document(s) — not a full re-index.
//
// No S3 landing, no separate importer Task. The consumer is the write path.
//
// TODO: implement Kafka consumer (franz-go or confluent-kafka-go).
// TODO: implement Postgres upsert + outbox in a single transaction.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("inventory-consumer starting",
		"topic", os.Getenv("KAFKA_TOPIC"),
		"brokers", os.Getenv("KAFKA_BROKERS"),
		"group", os.Getenv("KAFKA_GROUP"),
		"dsn", os.Getenv("DATABASE_URL"),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// TODO: connect to Postgres (pgx pool)
	// TODO: connect to Kafka, join consumer group
	// TODO: for each record:
	//   BEGIN
	//   INSERT INTO stock_levels (variant_id, warehouse_code, level, updated_at)
	//   VALUES ($1, $2, $3, now())
	//   ON CONFLICT (variant_id, warehouse_code)
	//   DO UPDATE SET level = EXCLUDED.level,
	//                 version = stock_levels.version + 1,
	//                 updated_at = now()
	//   WHERE stock_levels.level IS DISTINCT FROM EXCLUDED.level
	//   -- only insert outbox row if the level actually changed (rows_affected > 0)
	//   INSERT INTO outbox (aggregate, aggregate_id, event_type, payload)
	//   VALUES ('variant', $variant_id, 'stock.updated', $payload)
	//   COMMIT
	// TODO: commit Kafka offset after successful transaction

	<-ctx.Done()
	logger.Info("inventory-consumer shutdown complete")
}
