package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	client *minio.Client
	bucket string
}

func NewMinIOStorage(ctx context.Context, endpoint, publicEndpoint, accessKey, secretKey, bucket string) (*MinIOStorage, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Region: "us-east-1",
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
	key := "products/" + productID.String() + "/" + uuid.NewString()
	url, err := s.client.PresignedPutObject(ctx, s.bucket, key, ttl)
	if err != nil {
		return "", "", err
	}
	return url.String(), key, nil
}

func (s *MinIOStorage) PresignProductImageView(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, ttl, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}
