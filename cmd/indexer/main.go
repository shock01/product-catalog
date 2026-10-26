package main

import (
	"log/slog"
	"os"
)

// Indexer is an ECS Task triggered by EventBridge on snapshot.imported events.
// It reads products from Postgres in batches and bulk-writes to the
// per-channel×locale OpenSearch index, then performs a blue/green alias swap.
//
// TODO: implement batch reader, OpenSearch bulk writer, alias swap.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("indexer starting",
		"channel", os.Getenv("CHANNEL"),
		"locale", os.Getenv("LOCALE"),
	)

	// TODO: connect to Postgres and OpenSearch
	// TODO: read products in batches of 500
	// TODO: bulk-write to products-{channel}-{locale}-{timestamp}
	// TODO: swap alias products-{channel}-{locale}-active

	logger.Info("indexer complete")
}
