package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/relay-forge/relay-forge/services/media/internal/config"
)

type S3Store struct {
	client        *minio.Client
	bucketUploads string
	bucketAvatars string
	bucketEmoji   string
	presignExpiry time.Duration
}

func NewS3Store(cfg config.S3Config) (*S3Store, error) {
	endpoint := cfg.Endpoint
	secure := false

	parsed, err := url.Parse(endpoint)
	if err == nil {
		endpoint = parsed.Host
		secure = parsed.Scheme == "https"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	store := &S3Store{
		client:        client,
		bucketUploads: cfg.BucketUploads,
		bucketAvatars: cfg.BucketAvatars,
		bucketEmoji:   cfg.BucketEmoji,
		presignExpiry: cfg.PresignExpiry,
	}

	return store, nil
}

func (s *S3Store) Upload(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *S3Store) Download(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *S3Store) Delete(ctx context.Context, bucket, key string) error {
	return s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3Store) PresignedPutURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, bucket, key, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *S3Store) PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := s.client.PresignedGetObject(ctx, bucket, key, expiry, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *S3Store) BucketUploads() string { return s.bucketUploads }
func (s *S3Store) BucketAvatars() string { return s.bucketAvatars }
func (s *S3Store) BucketEmoji() string   { return s.bucketEmoji }
func (s *S3Store) PresignExpiry() time.Duration { return s.presignExpiry }
