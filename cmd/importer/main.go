package main

import (
	"log/slog"
	"os"
)

// Importer is an ECS Task triggered by EventBridge on s3:ObjectCreated events.
// It reads Parquet files produced by Athena from S3 and upserts products into
// Postgres, recording a snapshot row and using snapshot_id as a write-guard.
//
// TODO: implement S3 Parquet reader, snapshot upsert, write-guard logic.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("importer starting",
		"bucket", os.Getenv("S3_BUCKET"),
		"key", os.Getenv("S3_KEY"),
	)

	// TODO: download Parquet file from S3
	// TODO: insert snapshot row, obtain snapshot_id
	// TODO: upsert products/variants where incoming dumped_at > existing dumped_at
	// TODO: publish snapshot.imported event to EventBridge

	logger.Info("importer complete")
}
