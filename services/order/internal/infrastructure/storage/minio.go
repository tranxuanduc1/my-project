package storage

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type MinIOStorage struct {
	client *minio.Client
	bucket string
}

func NewMinIOStorage(ctx context.Context, endpoint, publicEndpoint, accessKey, secretKey, bucket string) (*MinIOStorage, error) {
	opts := &minio.Options{
		Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
		Region:    "us-east-1",
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	privateClient, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, err
	}
	if ok, _ := privateClient.BucketExists(ctx, bucket); !ok {
		if err = privateClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	publicClient, err := minio.New(publicEndpoint, opts)
	if err != nil {
		return nil, err
	}
	return &MinIOStorage{client: publicClient, bucket: bucket}, nil
}

func (s *MinIOStorage) PresignProductImage(ctx context.Context, productID uuid.UUID, ttl time.Duration) (string, string, error) {
	ctx, span := storageTracer.Start(ctx, "minio.presign_product_image", trace.WithAttributes(attribute.String("storage.operation", "presign_put")))
	defer span.End()
	startedAt := time.Now()
	key := "products/" + productID.String() + "/" + uuid.NewString()
	url, err := s.client.PresignedPutObject(ctx, s.bucket, key, ttl)
	if err != nil {
		recordStorageOperation(ctx, "presign_put", "error", startedAt)
		span.RecordError(err)
		span.SetStatus(codes.Error, "presign failed")
		return "", "", err
	}
	recordStorageOperation(ctx, "presign_put", "success", startedAt)
	return url.String(), key, nil
}

func (s *MinIOStorage) PresignProductImageView(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	ctx, span := storageTracer.Start(ctx, "minio.presign_product_image_view", trace.WithAttributes(attribute.String("storage.operation", "presign_get")))
	defer span.End()
	startedAt := time.Now()
	url, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, ttl, nil)
	if err != nil {
		recordStorageOperation(ctx, "presign_get", "error", startedAt)
		span.RecordError(err)
		span.SetStatus(codes.Error, "presign failed")
		return "", err
	}
	recordStorageOperation(ctx, "presign_get", "success", startedAt)
	return url.String(), nil
}

var (
	storageTracer   = otel.Tracer("myproject/order/storage")
	storageDuration metric.Float64Histogram
)

func init() {
	var err error
	storageDuration, err = otel.Meter("myproject/order/storage").Float64Histogram(
		"storage.operation.duration",
		metric.WithDescription("Storage operation duration by bounded operation and result."),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Warn("storage metric initialization failed", "error", err)
	}
}

func recordStorageOperation(ctx context.Context, operation, result string, startedAt time.Time) {
	if storageDuration == nil {
		return
	}
	storageDuration.Record(ctx, time.Since(startedAt).Seconds(), metric.WithAttributes(
		attribute.String("storage.system", "minio"),
		attribute.String("operation", operation),
		attribute.String("result", result),
	))
}
