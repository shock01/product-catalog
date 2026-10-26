module github.com/shock01/product-catalog

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/opensearch-project/opensearch-go/v2 v2.3.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/aws/aws-sdk-go-v2 v1.27.0
	github.com/aws/aws-sdk-go-v2/config v1.27.13
	github.com/aws/aws-sdk-go-v2/service/sqs v1.32.0
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.31.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.54.3
	go.opentelemetry.io/otel v1.26.0
	go.opentelemetry.io/otel/trace v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.26.0
	go.opentelemetry.io/otel/sdk v1.26.0
)
