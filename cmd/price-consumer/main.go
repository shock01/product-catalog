package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// price-consumer is a long-running ECS Service that consumes the external
// pricing.price-changed Kafka topic, batches records by time window or count,
// and writes JSON lines to S3 for the price-importer to process.
//
// S3 landing path: s3://{S3_BUCKET}/pricing/stream/{date}/{hour}/{uuid}.json
//
// TODO: implement Kafka consumer (franz-go or confluent-kafka-go).
// TODO: implement S3 batch writer with time/count flush trigger.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("price-consumer starting",
		"topic", os.Getenv("KAFKA_TOPIC"),
		"brokers", os.Getenv("KAFKA_BROKERS"),
		"group", os.Getenv("KAFKA_GROUP"),
		"bucket", os.Getenv("S3_BUCKET"),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// TODO: connect to Kafka, join consumer group
	// TODO: poll records in a loop until ctx is cancelled
	// TODO: accumulate records into a batch (flush every 5 min or 1,000 records)
	// TODO: on flush: write JSON lines to S3, emit S3 ObjectCreated (automatic)
	// TODO: commit Kafka offsets only after S3 write succeeds

	<-ctx.Done()
	logger.Info("price-consumer shutdown complete")
}
