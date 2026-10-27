package main

import (
	"log/slog"
	"os"
)

// price-importer is an ECS Task triggered by EventBridge on s3:ObjectCreated
// events for both Kafka stream landings and Athena full-price dumps.
// It reads JSON lines from S3, inserts a price_snapshot_imports row as a
// write-guard, and appends rows to price_snapshots only when the incoming
// effective_from is strictly newer than the latest existing row for that
// product × channel × marketplace combination.
// On completion it emits a price.imported outbox event to trigger re-indexing.
//
// TODO: implement S3 JSON reader.
// TODO: implement price_snapshot_imports write-guard.
// TODO: implement price_snapshots append with effective_from guard.
// TODO: publish price.imported event via outbox.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("price-importer starting",
		"bucket", os.Getenv("S3_BUCKET"),
		"key", os.Getenv("S3_KEY"),
	)

	// TODO: download JSON lines file from S3
	// TODO: INSERT INTO price_snapshot_imports (s3_uri, dumped_at) status='processing'
	// TODO: for each record: INSERT INTO price_snapshots
	//         where no newer effective_from exists for product×channel×marketplace
	// TODO: UPDATE price_snapshot_imports SET status='completed', row_count=$n
	// TODO: INSERT INTO outbox (event_type='price.imported', payload={s3_uri, row_count})

	logger.Info("price-importer complete")
}
